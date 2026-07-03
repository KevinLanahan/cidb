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
	evalCtx := newEvalContext(loadSecrets())
	path, err := findWorkflow(workflowPath)
	if err != nil {
		return err
	}

	wf, err := parseAnyWorkflow(path)
	if err != nil {
		return err
	}

	fmt.Printf("\n  cidb  ·  %s\n", wf.Name)
	fmt.Printf("  Workflow: %s\n", path)
	fmt.Printf("  Jobs: %d\n", len(wf.Jobs))

	ctx := context.Background()

	jobPassed := make(map[string]bool) // tracks which jobs succeeded
	anyJobFailed := false

	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]

		// Check needs: skip this job if any dependency failed.
		if len(job.Needs) > 0 {
			allPassed := true
			for _, dep := range job.Needs {
				if !jobPassed[dep] {
					allPassed = false
					break
				}
			}
			if !allPassed {
				fmt.Printf("\n  ─── Job: %s (skipped — dependency failed)\n", jobID)
				continue
			}
		}

		err := runJob(ctx, jobID, job, evalCtx)
		if err != nil {
			anyJobFailed = true
			jobPassed[jobID] = false
		} else {
			jobPassed[jobID] = true
		}
	}

	if anyJobFailed {
		return fmt.Errorf("workflow finished with failures")
	}
	return nil
}

func runJob(ctx context.Context, jobID string, job Job, evalCtx *evalContext) error {
	if job.Image != "" {
		fmt.Printf("\n  ┌─ Job: %s (image: %s)\n\n", jobID, job.Image)
	} else {
		fmt.Printf("\n  ┌─ Job: %s (runs-on: %s)\n\n", jobID, job.RunsOn)
	}

	ctr, err := startContainer(ctx, job)
	if err != nil {
		return err
	}
	defer ctr.stop()

	var results []stepResult
	state := newJobState()

	for i, step := range job.Steps {
		step = expandStep(step, evalCtx)
		name := stepName(step, i)

		// Evaluate if: condition — auto-skip if false.
		if !evalIf(step.If, state) {
			reason := "if: condition false"
			if step.If == "" {
				reason = "previous step failed"
			}
			fmt.Printf("\n  ─── Step %d: %s\n", i+1, name)
			fmt.Printf("  ⏭  SKIP  %s (%s)\n", name, reason)
			results = append(results, stepResult{name: name, skipped: true})
			continue
		}

		if step.Uses != "" && step.Run == "" {
			fmt.Printf("\n  ─── Step %d: %s\n", i+1, name)
			handled, err := runAction(ctr, step)
			if err != nil {
				fmt.Printf("  ✗  FAIL  %s (%v)\n", name, err)
				results = append(results, stepResult{name: name, passed: false})
				state.anyFailed = true
				printSummary(results)
				return fmt.Errorf("job %q stopped: action %q failed", jobID, name)
			}
			if !handled {
				fmt.Printf("  ⚠  Action not yet supported: %s\n", step.Uses)
				fmt.Println("  This step will be skipped. If your pipeline needs it,")
				fmt.Println("  drop into a shell and set it up manually before continuing.")
				fmt.Println()
				scanner := bufio.NewScanner(os.Stdin)
				skipped := false
				for !skipped {
					fmt.Print("  [s]kip  [sh]ell  [a]bort  > ")
					if !scanner.Scan() {
						results = append(results, stepResult{name: name, aborted: true})
						printSummary(results)
						return nil
					}
					switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
					case "s", "skip", "":
						results = append(results, stepResult{name: name, skipped: true})
						printStepResult(name, false, true)
						skipped = true
					case "sh", "shell":
						if err := ctr.dropShell(); err != nil {
							fmt.Printf("\n  Shell error: %v\n", err)
						}
					case "a", "abort":
						fmt.Println("\n  Aborted.")
						printSummary(results)
						return nil
					default:
						fmt.Println("  Options: s, sh, a")
					}
				}
			} else {
				results = append(results, stepResult{name: name, passed: true})
				printStepResult(name, true, false)
			}
			continue
		}

		result := runStep(ctr, i+1, name, step, evalCtx)
		results = append(results, result)

		if !result.passed && !result.skipped {
			state.anyFailed = true
		}

		if result.aborted {
			fmt.Println("\n  Aborted.")
			printSummary(results)
			return nil
		}

		if !result.passed && !result.skipped {
			// Don't stop the job — let subsequent steps with if: failure() or if: always() still run.
			continue
		}
	}

	printSummary(results)
	if state.anyFailed {
		return fmt.Errorf("job %q finished with failures", jobID)
	}
	return nil
}

func runStep(ctr *Container, num int, name string, step Step, evalCtx *evalContext) stepResult {
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
			exitCode, output, err := ctr.exec(step.Run, step.Env, step.WorkingDirectory)
			fmt.Println()

			if err != nil {
				fmt.Printf("  Exec error: %v\n", err)
				printStepResult(name, false, false)
				return stepResult{name: name, passed: false}
			}

			if exitCode == 0 {
				parseStepOutputs(output, step.ID, evalCtx)
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
				action = pauseFailed(num, name)
				switch action {
				case ActionRetry:
					fmt.Println()
					exitCode, output, err = ctr.exec(step.Run, step.Env, step.WorkingDirectory)
					fmt.Println()
					if err != nil {
						fmt.Printf("  Exec error: %v\n", err)
						printStepResult(name, false, false)
					} else if exitCode == 0 {
						parseStepOutputs(output, step.ID, evalCtx)
						printStepResult(name, true, false)
						return stepResult{name: name, passed: true}
					} else {
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
					}
				case ActionShell:
					if err := ctr.dropShell(); err != nil {
						fmt.Printf("\n  Shell error: %v\n", err)
					}
				case ActionAbort:
					return stepResult{name: name, aborted: true}
				case ActionSkip:
					printStepResult(name, false, true)
					return stepResult{name: name, skipped: true}
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
