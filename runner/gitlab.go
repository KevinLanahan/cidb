package runner

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// gitlabReservedKeys are top-level .gitlab-ci.yml keys that are NOT job definitions.
var gitlabReservedKeys = map[string]bool{
	"stages":        true,
	"variables":     true,
	"include":       true,
	"default":       true,
	"workflow":      true,
	"image":         true,
	"before_script": true,
	"after_script":  true,
	"services":      true,
	"cache":         true,
	"artifacts":     true,
}

// gitlabJobYAML mirrors the fields we care about in a GitLab CI job.
type gitlabJobYAML struct {
	Stage        string            `yaml:"stage"`
	Image        string            `yaml:"image"`
	BeforeScript []string          `yaml:"before_script"`
	Script       []string          `yaml:"script"`
	AfterScript  []string          `yaml:"after_script"`
	Variables    map[string]string `yaml:"variables"`
}

func parseGitLabCI(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Parse as raw node map so we can iterate dynamic job keys.
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	// Extract stages (defines job ordering).
	var stages []string
	if node, ok := raw["stages"]; ok {
		node.Decode(&stages) //nolint:errcheck
	}
	if len(stages) == 0 {
		stages = []string{"test"} // GitLab default
	}

	// Extract global variables.
	globalVars := map[string]string{}
	if node, ok := raw["variables"]; ok {
		node.Decode(&globalVars) //nolint:errcheck
	}

	// Extract global before_script (applies to all jobs unless overridden).
	var globalBeforeScript []string
	if node, ok := raw["before_script"]; ok {
		node.Decode(&globalBeforeScript) //nolint:errcheck
	}

	// Extract global image.
	globalImage := ""
	if node, ok := raw["image"]; ok {
		node.Decode(&globalImage) //nolint:errcheck
	}

	// Parse job definitions (any non-reserved top-level key with a script:).
	type namedJob struct {
		id  string
		job gitlabJobYAML
	}
	jobsByStage := map[string][]namedJob{}
	unstaged := []namedJob{}

	for key, node := range raw {
		if gitlabReservedKeys[key] {
			continue
		}
		// Hidden jobs start with a dot — skip them.
		if strings.HasPrefix(key, ".") {
			continue
		}
		var j gitlabJobYAML
		if err := node.Decode(&j); err != nil {
			continue
		}
		if len(j.Script) == 0 {
			continue
		}
		entry := namedJob{id: key, job: j}
		if j.Stage == "" {
			unstaged = append(unstaged, entry)
		} else {
			jobsByStage[j.Stage] = append(jobsByStage[j.Stage], entry)
		}
	}

	wf := &Workflow{
		Name: fmt.Sprintf("GitLab CI · %s", path),
		Jobs: make(map[string]Job),
	}

	addJob := func(entry namedJob) {
		wf.JobOrder = append(wf.JobOrder, entry.id)
		wf.Jobs[entry.id] = convertGitLabJob(entry.job, globalVars, globalBeforeScript, globalImage)
	}

	for _, stage := range stages {
		for _, entry := range jobsByStage[stage] {
			addJob(entry)
		}
	}
	for _, entry := range unstaged {
		addJob(entry)
	}

	if len(wf.Jobs) == 0 {
		return nil, fmt.Errorf("no jobs found in %s", path)
	}

	return wf, nil
}

func convertGitLabJob(j gitlabJobYAML, globalVars map[string]string, globalBeforeScript []string, globalImage string) Job {
	// Merge global vars then job-level vars (job wins on conflict).
	env := map[string]string{}
	for k, v := range globalVars {
		env[k] = v
	}
	for k, v := range j.Variables {
		env[k] = v
	}

	// Job before_script overrides global; if absent, use global.
	beforeScript := j.BeforeScript
	if len(beforeScript) == 0 {
		beforeScript = globalBeforeScript
	}

	var steps []Step

	if len(beforeScript) > 0 {
		steps = append(steps, Step{
			Name: "before_script",
			Run:  strings.Join(beforeScript, "\n"),
			Env:  env,
		})
	}

	if len(j.Script) > 0 {
		steps = append(steps, Step{
			Name: "script",
			Run:  strings.Join(j.Script, "\n"),
			Env:  env,
		})
	}

	if len(j.AfterScript) > 0 {
		steps = append(steps, Step{
			Name: "after_script",
			Run:  strings.Join(j.AfterScript, "\n"),
			Env:  env,
		})
	}

	// Image: job-level > global > empty (startContainer will pick a default)
	image := j.Image
	if image == "" {
		image = globalImage
	}

	return Job{
		RunsOn: "ubuntu-latest",
		Image:  image,
		Steps:  steps,
	}
}
