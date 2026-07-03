package runner

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Action int

const (
	ActionContinue Action = iota
	ActionSkip
	ActionShell
	ActionRetry
	ActionAbort
)

func pause(stepNum int, stepName string, command string) Action {
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Printf("  Step %d: %s\n", stepNum, stepName)
	fmt.Println("  ─────────────────────────────────────────────────")

	if command != "" {
		fmt.Println("  Command:")
		for _, line := range strings.Split(strings.TrimSpace(command), "\n") {
			fmt.Printf("    $ %s\n", line)
		}
		fmt.Println()
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("  [c]ontinue  [s]kip  [sh]ell  [a]bort  > ")
		if !scanner.Scan() {
			return ActionAbort
		}
		switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
		case "c", "continue", "":
			return ActionContinue
		case "s", "skip":
			return ActionSkip
		case "sh", "shell":
			return ActionShell
		case "a", "abort", "q", "quit", "exit":
			return ActionAbort
		default:
			fmt.Println("  Unknown command. Options: c, s, sh, a")
		}
	}
}

func pauseFailed(stepNum int, stepName string) Action {
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Printf("  Step %d: %s (failed — what next?)\n", stepNum, stepName)
	fmt.Println("  ─────────────────────────────────────────────────")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("  [r]etry  [sh]ell  [s]kip  [a]bort  > ")
		if !scanner.Scan() {
			return ActionAbort
		}
		switch strings.TrimSpace(strings.ToLower(scanner.Text())) {
		case "r", "retry":
			return ActionRetry
		case "sh", "shell":
			return ActionShell
		case "s", "skip", "c", "continue":
			return ActionSkip
		case "a", "abort", "q", "quit":
			return ActionAbort
		default:
			fmt.Println("  Unknown command. Options: r, sh, s, a")
		}
	}
}

func printStepResult(name string, passed, skipped bool) {
	switch {
	case skipped:
		fmt.Printf("  ⏭  SKIP  %s\n", name)
	case passed:
		fmt.Printf("  ✓  PASS  %s\n", name)
	default:
		fmt.Printf("  ✗  FAIL  %s\n", name)
	}
}

func printSummary(results []stepResult) {
	var passed, failed, skipped int

	fmt.Println()
	fmt.Println("  ─── Summary ────────────────────────────────────")
	for _, r := range results {
		printStepResult(r.name, r.passed, r.skipped)
		switch {
		case r.skipped:
			skipped++
		case r.passed:
			passed++
		default:
			failed++
		}
	}
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Printf("  %d passed  %d failed  %d skipped\n\n", passed, failed, skipped)
}
