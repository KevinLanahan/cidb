package runner

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name string         `yaml:"name"`
	Jobs map[string]Job `yaml:"jobs"`
}

type Job struct {
	RunsOn string `yaml:"runs-on"`
	Steps  []Step `yaml:"steps"`
}

type Step struct {
	Name             string            `yaml:"name"`
	Run              string            `yaml:"run"`
	Uses             string            `yaml:"uses"`
	Env              map[string]string `yaml:"env"`
	With             map[string]string `yaml:"with"`
	WorkingDirectory string            `yaml:"working-directory"`
}

func findWorkflow(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	matches, _ := filepath.Glob(".github/workflows/*.yml")
	if len(matches) == 0 {
		matches, _ = filepath.Glob(".github/workflows/*.yaml")
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no workflow files found in .github/workflows/ — pass a file path explicitly")
	}

	return matches[0], nil
}

func parseWorkflow(path string) (*Workflow, error) {
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

	return &wf, nil
}
