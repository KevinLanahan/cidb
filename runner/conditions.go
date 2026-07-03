package runner

import (
	"os/exec"
	"strings"
)

// jobState tracks what's happened so far in a job, used for evaluating if: conditions.
type jobState struct {
	anyFailed bool   // true if any step has failed (not skipped)
	gitRef    string // e.g. "refs/heads/main"
	eventName string // e.g. "push"
}

// newJobState initialises state, attempting to detect the current git ref.
func newJobState() jobState {
	s := jobState{eventName: "push"} // sensible default
	if ref := detectGitRef(); ref != "" {
		s.gitRef = "refs/heads/" + ref
	}
	return s
}

// detectGitRef runs `git rev-parse --abbrev-ref HEAD` in the working directory.
func detectGitRef() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// evalIf evaluates a GitHub Actions if: expression.
// Returns true if the step should run, false if it should be skipped.
// Unknown expressions default to true (be permissive — don't silently skip steps).
func evalIf(expr string, state jobState) bool {
	if expr == "" {
		// No condition — run by default only if no previous step failed.
		// (GitHub Actions default behaviour: steps don't run after a failure
		// unless they opt in with if: failure() or if: always())
		return !state.anyFailed
	}

	// Strip ${{ }} wrapper if present.
	inner := strings.TrimSpace(expr)
	if strings.HasPrefix(inner, "${{") && strings.HasSuffix(inner, "}}") {
		inner = strings.TrimSpace(inner[3 : len(inner)-2])
	}
	inner = strings.ToLower(inner)

	switch inner {
	case "failure()":
		return state.anyFailed
	case "success()":
		return !state.anyFailed
	case "always()":
		return true
	case "cancelled()":
		return false // cidb never cancels
	case "!cancelled()":
		return true
	case "success() || failure()", "failure() || success()":
		return true
	}

	// github.ref checks
	if strings.Contains(inner, "github.ref") {
		return evalComparison(inner, "github.ref", state.gitRef)
	}

	// github.event_name checks
	if strings.Contains(inner, "github.event_name") {
		return evalComparison(inner, "github.event_name", state.eventName)
	}

	// contains() function: contains(github.ref, 'value')
	if strings.HasPrefix(inner, "contains(") {
		return evalContains(inner, state)
	}

	// startsWith() / endsWith()
	if strings.HasPrefix(inner, "startswith(") {
		return evalStartsWith(inner, state)
	}
	if strings.HasPrefix(inner, "endswith(") {
		return evalEndsWith(inner, state)
	}

	// Unknown expression — default to true so we don't silently skip things.
	return true
}

// evalComparison handles expressions like: github.ref == 'refs/heads/main'
func evalComparison(expr, varName, actualValue string) bool {
	// Normalise: remove spaces around == and !=
	expr = strings.ReplaceAll(expr, " ", "")
	varName = strings.ReplaceAll(varName, " ", "")
	actualLower := strings.ToLower(actualValue)

	if idx := strings.Index(expr, varName+"=="); idx >= 0 {
		rhs := strings.Trim(expr[idx+len(varName)+2:], "'\"")
		return actualLower == strings.ToLower(rhs)
	}
	if idx := strings.Index(expr, varName+"!="); idx >= 0 {
		rhs := strings.Trim(expr[idx+len(varName)+2:], "'\"")
		return actualLower != strings.ToLower(rhs)
	}

	// Can't parse — default to true
	return true
}

// evalContains handles: contains(github.ref, 'value')
func evalContains(expr string, state jobState) bool {
	// Extract args from contains(arg1, arg2)
	inner := strings.TrimPrefix(expr, "contains(")
	inner = strings.TrimSuffix(inner, ")")
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return true
	}
	subject := resolveVar(strings.TrimSpace(parts[0]), state)
	value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
	return strings.Contains(strings.ToLower(subject), strings.ToLower(value))
}

func evalStartsWith(expr string, state jobState) bool {
	inner := strings.TrimPrefix(expr, "startswith(")
	inner = strings.TrimSuffix(inner, ")")
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return true
	}
	subject := resolveVar(strings.TrimSpace(parts[0]), state)
	value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
	return strings.HasPrefix(strings.ToLower(subject), strings.ToLower(value))
}

func evalEndsWith(expr string, state jobState) bool {
	inner := strings.TrimPrefix(expr, "endswith(")
	inner = strings.TrimSuffix(inner, ")")
	parts := strings.SplitN(inner, ",", 2)
	if len(parts) != 2 {
		return true
	}
	subject := resolveVar(strings.TrimSpace(parts[0]), state)
	value := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
	return strings.HasSuffix(strings.ToLower(subject), strings.ToLower(value))
}

// resolveVar substitutes known github.* context variables.
func resolveVar(s string, state jobState) string {
	switch s {
	case "github.ref":
		return state.gitRef
	case "github.event_name":
		return state.eventName
	}
	return s
}
