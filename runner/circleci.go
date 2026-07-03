package runner

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// circleJob is the raw YAML shape of a CircleCI job.
type circleJob struct {
	Docker []struct {
		Image string `yaml:"image"`
	} `yaml:"docker"`
	Machine *struct {
		Image string `yaml:"image"`
	} `yaml:"machine"`
	Steps []yaml.Node `yaml:"steps"`
}

// circleConfig is the top-level .circleci/config.yml structure.
type circleConfig struct {
	Jobs      map[string]circleJob `yaml:"jobs"`
	Workflows map[string]struct {
		Jobs []yaml.Node `yaml:"jobs"`
	} `yaml:"workflows"`
}

func parseCircleCI(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg circleConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if len(cfg.Jobs) == 0 {
		return nil, fmt.Errorf("no jobs found in %s", path)
	}

	// Determine job order from the first workflow definition.
	// If no workflows section, just use map keys.
	var jobOrder []string
	for _, wf := range cfg.Workflows {
		for _, node := range wf.Jobs {
			name := circleJobName(node)
			if name != "" {
				jobOrder = append(jobOrder, name)
			}
		}
		break // use first workflow only
	}
	// Fill in any jobs not mentioned in workflows (shouldn't happen but be safe).
	mentioned := map[string]bool{}
	for _, id := range jobOrder {
		mentioned[id] = true
	}
	for id := range cfg.Jobs {
		if !mentioned[id] {
			jobOrder = append(jobOrder, id)
		}
	}

	wf := &Workflow{
		Name:     fmt.Sprintf("CircleCI · %s", path),
		Jobs:     make(map[string]Job),
		JobOrder: jobOrder,
	}

	for id, cj := range cfg.Jobs {
		wf.Jobs[id] = convertCircleJob(id, cj)
	}

	return wf, nil
}

// circleJobName extracts the job name from a workflow jobs list entry.
// Entries can be either a plain string ("- build") or a map ("- test:\n    requires: [build]").
func circleJobName(node yaml.Node) string {
	switch node.Kind {
	case yaml.ScalarNode:
		return node.Value
	case yaml.MappingNode:
		if len(node.Content) >= 1 {
			return node.Content[0].Value
		}
	}
	return ""
}

func convertCircleJob(id string, cj circleJob) Job {
	// Pick Docker image.
	image := ""
	if len(cj.Docker) > 0 {
		image = cj.Docker[0].Image
	} else if cj.Machine != nil {
		image = cj.Machine.Image
	}

	var steps []Step
	for _, node := range cj.Steps {
		step := parseCircleStep(node)
		if step != nil {
			steps = append(steps, *step)
		}
	}

	return Job{
		RunsOn: "ubuntu-latest",
		Image:  image,
		Steps:  steps,
	}
}

// parseCircleStep converts a CircleCI step node into our Step struct.
// CircleCI steps can be:
//   - A scalar: "checkout" or "run: echo hello"
//   - A mapping: {run: {name: "...", command: "..."}}, {run: "echo hello"}, etc.
func parseCircleStep(node yaml.Node) *Step {
	switch node.Kind {

	case yaml.ScalarNode:
		// Bare "checkout" or similar built-in.
		name := node.Value
		if name == "checkout" {
			return &Step{
				Name: "checkout",
				Run:  "",   // handled like actions/checkout — workspace already mounted
				Uses: "circleci/checkout",
			}
		}
		// Bare string run command.
		return &Step{Name: name, Run: name}

	case yaml.MappingNode:
		// Should have one key: the step type.
		if len(node.Content) < 2 {
			return nil
		}
		stepType := node.Content[0].Value
		valueNode := node.Content[1]

		switch stepType {
		case "checkout":
			return &Step{
				Name: "checkout",
				Uses: "circleci/checkout",
			}

		case "run":
			return parseCircleRun(*valueNode)

		case "restore_cache", "save_cache", "store_artifacts", "store_test_results",
			"persist_to_workspace", "attach_workspace", "add_ssh_keys":
			// Known built-ins we skip for now.
			return &Step{
				Name: stepType,
				Uses: "circleci/" + stepType,
			}

		default:
			// Orb step or unknown — treat as unsupported.
			return &Step{
				Name: stepType,
				Uses: "circleci/" + stepType,
			}
		}
	}
	return nil
}

// parseCircleRun handles the `run:` step which can be a string or an object.
func parseCircleRun(node yaml.Node) *Step {
	switch node.Kind {
	case yaml.ScalarNode:
		// run: echo hello
		cmd := node.Value
		return &Step{Name: cmd, Run: cmd}

	case yaml.MappingNode:
		// run:
		//   name: Install deps
		//   command: pip install ...
		//   environment:
		//     KEY: value
		//   working_directory: /some/dir
		var name, command, workDir string
		env := map[string]string{}

		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val := node.Content[i+1]
			switch key {
			case "name":
				name = val.Value
			case "command":
				command = val.Value
			case "working_directory":
				workDir = val.Value
			case "environment":
				// Decode env map
				for j := 0; j+1 < len(val.Content); j += 2 {
					env[val.Content[j].Value] = val.Content[j+1].Value
				}
			}
		}

		if command == "" {
			return nil
		}
		if name == "" {
			// Use first line of command as name
			name = strings.SplitN(strings.TrimSpace(command), "\n", 2)[0]
			if len(name) > 60 {
				name = name[:60] + "…"
			}
		}

		step := &Step{
			Name:             name,
			Run:              command,
			WorkingDirectory: workDir,
		}
		if len(env) > 0 {
			step.Env = env
		}
		return step
	}
	return nil
}
