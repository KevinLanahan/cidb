package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

type stepResult struct {
	name    string
	passed  bool
	skipped bool
	aborted bool 
}

func loadEnv() {
	// Try current directory first, then the directory of the running binary
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, strings.TrimSuffix(exe, "/cidb")+"/.env")
	}

	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
		return
	}
}

func Run(workflowPath string) error {
	loadEnv()
	path, err := findWorkflow(workflowPath)
	if err != nil {
		return err
	}

	wf, err := parseWorkflow(path)
	if err != nil {
		return err
	}

	fmt.Printf("\n  cidb  ·  %s\n", wf.Name)
	fmt.Printf("  Workflow: %s\n", path)
	fmt.Printf("  Jobs: %d\n", len(wf.Jobs))

	ctx := context.Background()

	for jobID, job := range wf.Jobs {
		if err := runJob(ctx, jobID, job); err != nil {
			return err
		}
	}

	return nil
}

func runJob(ctx context.Context, jobID string, job Job) error {
	fmt.Printf("\n  ┌─ Job: %s (runs-on: %s)\n\n", jobID, job.RunsOn)

	ctr, err := startContainer(ctx, job.RunsOn)
	if err != nil {
		return err
	}
	defer ctr.stop()

	var results []stepResult

	for i, step := range job.Steps {
		name := stepName(step, i)

		if step.Uses != "" && step.Run == "" {
			fmt.Printf("\n  ─── Step %d: %s\n", i+1, name)
			handled, err := runAction(ctr, step)
			if err != nil {
				fmt.Printf("  ✗  FAIL  %s (%v)\n", name, err)
				results = append(results, stepResult{name: name, passed: false})
				printSummary(results)
				return fmt.Errorf("job %q stopped: action %q failed", jobID, name)
			}
			if !handled {
				fmt.Printf("  (uses: %s — not yet supported, skipping)\n", step.Uses)
				results = append(results, stepResult{name: name, skipped: true})
				printStepResult(name, false, true)
			} else {
				results = append(results, stepResult{name: name, passed: true})
				printStepResult(name, true, false)
			}
			continue
		}

		result := runStep(ctr, i+1, name, step)
		results = append(results, result)

		if result.aborted {
			fmt.Println("\n  Aborted.")
			printSummary(results)
			return nil
		}

		if !result.passed && !result.skipped {
			printSummary(results)
			return fmt.Errorf("job %q stopped: step %q failed", jobID, name)
		}
	}

	printSummary(results)
	return nil
}

func runStep(ctr *Container, num int, name string, step Step) stepResult {
	for {
		action := pause(num, name, step.Run)

		switch action {
		case ActionAbort:
			return stepResult{name: name, aborted: true}

		case ActionSkip:
			printStepResult(name, false, true)
			return stepResult{name: name, skipped: true}

		case ActionShell:
			if err := ctr.dropShell(); err != nil {
				fmt.Printf("\n  Shell error: %v\n", err)
			}
			continue

		case ActionContinue:
			fmt.Println()
			exitCode, output, err := ctr.exec(step.Run, step.Env)
			fmt.Println()

			if err != nil {
				fmt.Printf("  Exec error: %v\n", err)
				printStepResult(name, false, false)
				return stepResult{name: name, passed: false}
			}

			if exitCode == 0 {
				printStepResult(name, true, false)
				return stepResult{name: name, passed: true}
			}

			fmt.Printf("  Step exited with code %d\n", exitCode)
			printStepResult(name, false, false)

			if analysis := analyzeFailure(step.Run, output, exitCode); analysis != "" {
				fmt.Println()
				fmt.Println("  ┌─ AI Analysis ──────────────────────────────────")
				for _, line := range strings.Split(analysis, "\n") {
					fmt.Printf("  │ %s\n", line)
				}
				fmt.Println("  └────────────────────────────────────────────────")
			}

			for {
				action = pause(num, name+" (failed — what next?)", "")
				switch action {
				case ActionShell:
					if err := ctr.dropShell(); err != nil {
						fmt.Printf("\n  Shell error: %v\n", err)
					}
				case ActionAbort:
					return stepResult{name: name, aborted: true}
				case ActionContinue, ActionSkip:
					return stepResult{name: name, passed: false}
				}
			}
		}
	}
}

func stepName(step Step, index int) string {
	if step.Name != "" {
		return step.Name
	}
	if step.Uses != "" {
		return step.Uses
	}
	return fmt.Sprintf("step %d", index+1)
}
