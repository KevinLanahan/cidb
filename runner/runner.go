package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type stepResult struct {
	name     string
	passed   bool
	skipped  bool
	warned   bool   // failed but continue-on-error: true
	aborted  bool
	output   string // captured stdout+stderr, capped at 10k chars
	analysis string // AI failure analysis, if any
}

func loadEnv() {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, strings.TrimSuffix(exe, "/lokal")+"/.env")
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

func collectStepNames(wf *Workflow) []string {
	var names []string
	for _, jobID := range wf.JobOrder {
		job := wf.Jobs[jobID]
		for i, step := range job.Steps {
			names = append(names, stepName(step, i))
		}
	}
	return names
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

	fmt.Printf("\n  lokal  ·  %s\n", wf.Name)
	fmt.Printf("  Workflow: %s\n", path)
	fmt.Printf("  Jobs: %d\n", len(wf.Jobs))

	// Offer live share before the run starts.
	var live *liveSession
	scanner := bufio.NewScanner(os.Stdin)
	if os.Getenv("SUPABASE_URL") != "" {
		fmt.Print("\n  Share live session? [y/N] > ")
		if scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "y" || answer == "yes" {
				stepNames := collectStepNames(wf)
				live, err = CreateLiveSession(wf.Name, wf.Platform, stepNames)
				if err != nil {
					fmt.Printf("  Warning: could not create live session: %v\n", err)
					live = nil
				} else {
					fmt.Printf("  Live: https://lokal-kappa.vercel.app/s/%s\n", live.slug)
				}
			}
		}
	}

	ctx := context.Background()

	jobPassed := make(map[string]bool)
	anyJobFailed := false
	var allResults []stepResult

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

		// Evaluate job-level if: condition.
		if job.If != "" {
			jobState := newJobState()
			expanded := evalCtx.expand(job.If)
			if !evalIf(expanded, jobState) {
				fmt.Printf("\n  ─── Job: %s (skipped — if: condition false)\n", jobID)
				continue
			}
		}

		results, err := runJob(ctx, jobID, job, evalCtx, live)
		allResults = append(allResults, results...)
		if err != nil {
			anyJobFailed = true
			jobPassed[jobID] = false
		} else {
			jobPassed[jobID] = true
		}
	}

	// Finalize live session.
	if live != nil {
		if err := live.Finish(anyJobFailed); err != nil {
			fmt.Printf("  Warning: could not finalize session: %v\n", err)
		}
	}

	// Offer one-shot share if not already shared live.
	if live == nil && len(allResults) > 0 && os.Getenv("SUPABASE_URL") != "" {
		fmt.Print("\n  Share this session? [y/N] > ")
		if scanner.Scan() {
			answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if answer == "y" || answer == "yes" {
				fmt.Print("  Uploading... ")
				slug, err := ShareSession(wf.Name, wf.Platform, allResults)
				if err != nil {
					fmt.Printf("failed: %v\n", err)
				} else {
					fmt.Printf("done!\n")
					fmt.Printf("  https://lokal-kappa.vercel.app/s/%s\n", slug)
				}
			}
		}
	}

	if anyJobFailed {
		return fmt.Errorf("workflow finished with failures")
	}
	return nil
}

func runJob(ctx context.Context, jobID string, job Job, evalCtx *evalContext, live *liveSession) ([]stepResult, error) {
	if job.Image != "" {
		fmt.Printf("\n  ┌─ Job: %s (image: %s)\n\n", jobID, job.Image)
	} else {
		fmt.Printf("\n  ┌─ Job: %s (runs-on: %s)\n\n", jobID, job.RunsOn)
	}

	ctr, err := startContainer(ctx, job)
	if err != nil {
		return nil, err
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
			r := stepResult{name: name, skipped: true}
			results = append(results, r)
			if live != nil {
				_ = live.UpdateStep(name, "skipped", "", "")
			}
			continue
		}

		if step.Uses != "" && step.Run == "" {
			fmt.Printf("\n  ─── Step %d: %s\n", i+1, name)
			if live != nil {
				_ = live.UpdateStep(name, "running", "", "")
			}
			handled, err := runAction(ctr, step)
			if err != nil {
				fmt.Printf("  ✗  FAIL  %s (%v)\n", name, err)
				r := stepResult{name: name, passed: false}
				results = append(results, r)
				if live != nil {
					_ = live.UpdateStep(name, "failed", err.Error(), "")
				}
				state.anyFailed = true
				printSummary(results)
				return results, fmt.Errorf("job %q stopped: action %q failed", jobID, name)
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
						r := stepResult{name: name, aborted: true}
						results = append(results, r)
						if live != nil {
							_ = live.UpdateStep(name, "aborted", "", "")
						}
						printSummary(results)
						return results, nil
					}
					switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
					case "s", "skip", "":
						r := stepResult{name: name, skipped: true}
						results = append(results, r)
						if live != nil {
							_ = live.UpdateStep(name, "skipped", "", "")
						}
						printStepResult(name, false, true)
						skipped = true
					case "sh", "shell":
						if err := ctr.dropShell(); err != nil {
							fmt.Printf("\n  Shell error: %v\n", err)
						}
					case "a", "abort":
						fmt.Println("\n  Aborted.")
						printSummary(results)
						return results, nil
					default:
						fmt.Println("  Options: s, sh, a")
					}
				}
			} else {
				r := stepResult{name: name, passed: true}
				results = append(results, r)
				if live != nil {
					_ = live.UpdateStep(name, "passed", "", "")
				}
				printStepResult(name, true, false)
			}
			continue
		}

		if live != nil {
			_ = live.UpdateStep(name, "running", "", "")
		}
		result := runStep(ctr, i+1, name, step, evalCtx)
		results = append(results, result)

		if live != nil {
			_ = live.UpdateStep(name, stepStatusStr(result), result.output, result.analysis)
		}

		if !result.passed && !result.skipped && !result.warned {
			state.anyFailed = true
		}

		if result.aborted {
			fmt.Println("\n  Aborted.")
			printSummary(results)
			return results, nil
		}

		if !result.passed && !result.skipped && !result.warned {
			// Don't stop the job — let subsequent steps with if: failure() or if: always() still run.
			continue
		}
	}

	printSummary(results)
	if state.anyFailed {
		return results, fmt.Errorf("job %q finished with failures", jobID)
	}
	return results, nil
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
			var timeout time.Duration
			if step.TimeoutMinutes > 0 {
				timeout = time.Duration(step.TimeoutMinutes * float64(time.Minute))
			}
			fmt.Println()
			exitCode, output, err := ctr.exec(step.Run, step.Env, step.WorkingDirectory, timeout)
			fmt.Println()

			if err != nil {
				fmt.Printf("  Exec error: %v\n", err)
				printStepResult(name, false, false)
				return stepResult{name: name, passed: false, output: capOutput(output)}
			}

			if exitCode == 0 {
				parseStepOutputs(output, step.ID, evalCtx)
				printStepResult(name, true, false)
				return stepResult{name: name, passed: true, output: capOutput(output)}
			}

			fmt.Printf("  Step exited with code %d\n", exitCode)

			// continue-on-error: true — log the failure but let the job continue.
			if step.ContinueOnError {
				fmt.Printf("  ⚠  WARN  %s (failed but continue-on-error is set)\n", name)
				return stepResult{name: name, warned: true, output: capOutput(output), analysis: analyzeFailure(step.Run, output, exitCode)}
			}

			printStepResult(name, false, false)

			analysis := analyzeFailure(step.Run, output, exitCode)
			if analysis != "" {
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
					exitCode, output, err = ctr.exec(step.Run, step.Env, step.WorkingDirectory, timeout)
					fmt.Println()
					if err != nil {
						fmt.Printf("  Exec error: %v\n", err)
						printStepResult(name, false, false)
					} else if exitCode == 0 {
						parseStepOutputs(output, step.ID, evalCtx)
						printStepResult(name, true, false)
						return stepResult{name: name, passed: true, output: capOutput(output)}
					} else {
						fmt.Printf("  Step exited with code %d\n", exitCode)
						printStepResult(name, false, false)
						analysis = analyzeFailure(step.Run, output, exitCode)
						if analysis != "" {
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
					return stepResult{name: name, aborted: true, output: capOutput(output), analysis: analysis}
				case ActionSkip:
					printStepResult(name, false, true)
					return stepResult{name: name, skipped: true, output: capOutput(output), analysis: analysis}
				}
			}
		}
	}
}

func capOutput(s string) string {
	const max = 10000
	if len(s) > max {
		return s[:max] + "\n... (truncated)"
	}
	return s
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
