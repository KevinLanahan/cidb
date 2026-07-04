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
	var passed, failed, skipped, warned int

	fmt.Println()
	fmt.Println("  ─── Summary ────────────────────────────────────")
	for _, r := range results {
		switch {
		case r.skipped:
			fmt.Printf("  ⏭  SKIP  %s\n", r.name)
			skipped++
		case r.warned:
			fmt.Printf("  ⚠  WARN  %s\n", r.name)
			warned++
		case r.passed:
			fmt.Printf("  ✓  PASS  %s\n", r.name)
			passed++
		default:
			fmt.Printf("  ✗  FAIL  %s\n", r.name)
			failed++
		}
	}
	fmt.Println("  ─────────────────────────────────────────────────")
	line := fmt.Sprintf("  %d passed  %d failed  %d skipped", passed, failed, skipped)
	if warned > 0 {
		line += fmt.Sprintf("  %d warned", warned)
	}
	fmt.Println(line)
	fmt.Println()
}
