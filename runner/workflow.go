package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name     string         `yaml:"name"`
	Jobs     map[string]Job `yaml:"jobs"`
	JobOrder []string       // ordered job IDs (populated by parsers)
}

type Job struct {
	RunsOn string        `yaml:"runs-on"`
	Image  string        // explicit Docker image override (e.g. GitLab CI image: field)
	If     string        `yaml:"if"`
	Needs  stringOrSlice `yaml:"needs"`
	Steps  []Step        `yaml:"steps"`
}

// stringOrSlice handles YAML fields that can be either a string or a list of strings.
// GitHub Actions `needs:` can be either:
//
//	needs: build
//	needs: [build, test]
type stringOrSlice []string

func (s *stringOrSlice) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*s = []string{value.Value}
		return nil
	}
	return value.Decode((*[]string)(s))
}

type Step struct {
	ID               string            `yaml:"id"`
	Name             string            `yaml:"name"`
	Run              string            `yaml:"run"`
	Uses             string            `yaml:"uses"`
	If               string            `yaml:"if"`
	ContinueOnError  bool              `yaml:"continue-on-error"`
	Env              map[string]string `yaml:"env"`
	With             map[string]string `yaml:"with"`
	WorkingDirectory string            `yaml:"working-directory"`
}

func findWorkflow(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	// CircleCI
	for _, name := range []string{".circleci/config.yml", ".circleci/config.yaml"} {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}

	// GitLab CI
	for _, name := range []string{".gitlab-ci.yml", ".gitlab-ci.yaml"} {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}

	// GitHub Actions
	matches, _ := filepath.Glob(".github/workflows/*.yml")
	if len(matches) == 0 {
		matches, _ = filepath.Glob(".github/workflows/*.yaml")
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no workflow files found — pass a file path explicitly")
	}

	return matches[0], nil
}

// parseAnyWorkflow detects the CI platform from the file path and routes to the correct parser.
func parseAnyWorkflow(path string) (*Workflow, error) {
	base := filepath.Base(path)
	dir := filepath.Base(filepath.Dir(path))

	if dir == ".circleci" && (base == "config.yml" || base == "config.yaml") {
		return parseCircleCI(path)
	}
	if base == ".gitlab-ci.yml" || base == ".gitlab-ci.yaml" {
		return parseGitLabCI(path)
	}
	return parseGitHubWorkflow(path)
}

func parseGitHubWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(wf.Jobs) == 0 {
		return nil, fmt.Errorf("no jobs found in %s", path)
	}

	wf.JobOrder = topoSort(wf.Jobs)
	return &wf, nil
}

// topoSort returns job IDs in dependency order using Kahn's algorithm.
// Jobs with no needs come first; a job only appears after all its dependencies.
func topoSort(jobs map[string]Job) []string {
	// Build in-degree count and adjacency list.
	inDegree := make(map[string]int, len(jobs))
	dependents := make(map[string][]string) // dependency -> jobs that need it

	for id := range jobs {
		inDegree[id] = 0
	}
	for id, job := range jobs {
		for _, dep := range job.Needs {
			if _, exists := jobs[dep]; exists {
				inDegree[id]++
				dependents[dep] = append(dependents[dep], id)
			}
		}
	}

	// Seed queue with jobs that have no dependencies.
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	// Sort for determinism.
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		deps := dependents[cur]
		sortStrings(deps)
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	// If there's a cycle or unresolved deps, append remaining jobs.
	seen := make(map[string]bool, len(order))
	for _, id := range order {
		seen[id] = true
	}
	for id := range jobs {
		if !seen[id] {
			order = append(order, id)
		}
	}

	return order
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// parseWorkflow is kept for backward compatibility (tests use it).
func parseWorkflow(path string) (*Workflow, error) {
	return parseGitHubWorkflow(path)
}
