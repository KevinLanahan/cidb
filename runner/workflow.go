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
	RunsOn string `yaml:"runs-on"`
	Image  string // explicit Docker image override (e.g. GitLab CI image: field)
	Steps  []Step `yaml:"steps"`
}

type Step struct {
	Name             string            `yaml:"name"`
	Run              string            `yaml:"run"`
	Uses             string            `yaml:"uses"`
	If               string            `yaml:"if"`
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

	// Populate JobOrder from map keys (order is arbitrary but consistent)
	for id := range wf.Jobs {
		wf.JobOrder = append(wf.JobOrder, id)
	}

	return &wf, nil
}

// parseWorkflow is kept for backward compatibility (tests use it).
func parseWorkflow(path string) (*Workflow, error) {
	return parseGitHubWorkflow(path)
}
