package runner

import (
	"bufio"
	"os"
	"regexp"
	"runtime"
	"strings"
)

// exprPattern matches any ${{ ... }} expression.
var exprPattern = regexp.MustCompile(`\$\{\{\s*(.+?)\s*\}\}`)

// evalContext holds all the values needed to evaluate ${{ }} expressions.
type evalContext struct {
	secrets map[string]string // from .env file
	env     map[string]string // from step/job env: fields
	github  map[string]string // github.* context
	runner  map[string]string // runner.* context
	steps   map[string]map[string]string // steps.<id>.outputs.<key>
}

// newEvalContext builds the context, auto-populating github.* and runner.* values.
func newEvalContext(secrets map[string]string) *evalContext {
	// Detect git info from the working directory.
	gitRef := "refs/heads/" + detectGitRef()
	gitSHA := detectGitSHA()
	gitRepo := detectGitRepo()

	osName := "Linux"
	if runtime.GOOS == "darwin" {
		osName = "macOS"
	} else if runtime.GOOS == "windows" {
		osName = "Windows"
	}

	return &evalContext{
		secrets: secrets,
		env:     map[string]string{},
		github: map[string]string{
			"ref":        gitRef,
			"sha":        gitSHA,
			"repository": gitRepo,
			"event_name": "push",
			"actor":      detectGitActor(),
			"workspace":  "/workspace",
			"run_id":     "0",
			"run_number": "1",
			"server_url": "https://github.com",
			"api_url":    "https://api.github.com",
		},
		runner: map[string]string{
			"os":        osName,
			"arch":      runtime.GOARCH,
			"temp":      "/tmp",
			"tool_cache": "/opt/hostedtoolcache",
		},
		steps: map[string]map[string]string{},
	}
}

// recordStepOutput stores an output value from a step so later steps can reference it.
// Call this after parsing "::set-output name=key::value" or "echo key=value >> $GITHUB_OUTPUT"
// from step output.
func (c *evalContext) recordStepOutput(stepID, key, value string) {
	if _, ok := c.steps[stepID]; !ok {
		c.steps[stepID] = map[string]string{}
	}
	c.steps[stepID][key] = value
}

// expand replaces all ${{ }} expressions in s using the context.
func (c *evalContext) expand(s string) string {
	return exprPattern.ReplaceAllStringFunc(s, func(match string) string {
		inner := strings.TrimSpace(exprPattern.FindStringSubmatch(match)[1])
		return c.evalExpr(inner)
	})
}

// evalExpr evaluates a single expression (the part inside ${{ }}).
func (c *evalContext) evalExpr(expr string) string {
	lower := strings.ToLower(expr)

	// secrets.X
	if strings.HasPrefix(lower, "secrets.") {
		key := expr[len("secrets."):]
		if val, ok := c.secrets[key]; ok {
			return val
		}
		return ""
	}

	// env.X
	if strings.HasPrefix(lower, "env.") {
		key := expr[len("env."):]
		if val, ok := c.env[key]; ok {
			return val
		}
		// Fall back to actual environment variable
		if val := os.Getenv(key); val != "" {
			return val
		}
		return ""
	}

	// vars.X (same as env for our purposes)
	if strings.HasPrefix(lower, "vars.") {
		key := expr[len("vars."):]
		if val, ok := c.secrets[key]; ok {
			return val
		}
		return ""
	}

	// github.X
	if strings.HasPrefix(lower, "github.") {
		key := expr[len("github."):]
		if val, ok := c.github[strings.ToLower(key)]; ok {
			return val
		}
		return ""
	}

	// runner.X
	if strings.HasPrefix(lower, "runner.") {
		key := expr[len("runner."):]
		if val, ok := c.runner[strings.ToLower(key)]; ok {
			return val
		}
		return ""
	}

	// steps.ID.outputs.KEY or steps.ID.outcome or steps.ID.conclusion
	if strings.HasPrefix(lower, "steps.") {
		parts := strings.SplitN(expr[len("steps."):], ".", 3)
		if len(parts) >= 3 {
			stepID := parts[0]
			field := strings.ToLower(parts[1])
			key := parts[2]
			if field == "outputs" {
				if outputs, ok := c.steps[stepID]; ok {
					if val, ok := outputs[key]; ok {
						return val
					}
				}
				return ""
			}
		}
		if len(parts) == 2 {
			stepID := parts[0]
			field := strings.ToLower(parts[1])
			// outcome/conclusion: we don't track these yet, return "success"
			if field == "outcome" || field == "conclusion" {
				if _, ok := c.steps[stepID]; ok {
					return "success"
				}
				return "skipped"
			}
		}
		return ""
	}

	// Boolean literals
	switch lower {
	case "true":
		return "true"
	case "false":
		return "false"
	}

	// Unknown — return empty string rather than leaving the raw expression
	return ""
}

// expandStep applies expression substitution to all relevant fields of a step.
func expandStep(step Step, ctx *evalContext) Step {
	// Merge step-level env into context for this expansion pass.
	for k, v := range step.Env {
		ctx.env[k] = v
	}

	step.Run = ctx.expand(step.Run)
	step.If = ctx.expand(step.If)

	expanded := make(map[string]string)
	for k, v := range step.Env {
		expanded[k] = ctx.expand(v)
	}
	step.Env = expanded

	expandedWith := make(map[string]string)
	for k, v := range step.With {
		expandedWith[k] = ctx.expand(v)
	}
	step.With = expandedWith

	return step
}

// parseStepOutputs scans command output for GitHub Actions output syntax and records them.
// Handles both old syntax (::set-output name=key::value) and new syntax (key=value in GITHUB_OUTPUT).
func parseStepOutputs(output string, stepID string, ctx *evalContext) {
	if stepID == "" {
		return
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Old syntax: ::set-output name=KEY::VALUE
		if strings.HasPrefix(line, "::set-output name=") {
			rest := line[len("::set-output name="):]
			if idx := strings.Index(rest, "::"); idx >= 0 {
				key := rest[:idx]
				value := rest[idx+2:]
				ctx.recordStepOutput(stepID, key, value)
			}
		}
		// New syntax: KEY=VALUE (written to $GITHUB_OUTPUT)
		// We detect lines that look like simple KEY=VALUE assignments in output.
		// This is a best-effort heuristic.
		if strings.Contains(line, "=") && !strings.Contains(line, " ") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && isValidOutputKey(parts[0]) {
				ctx.recordStepOutput(stepID, parts[0], parts[1])
			}
		}
	}
}

func isValidOutputKey(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// loadSecrets reads .env from the current directory.
func loadSecrets() map[string]string {
	secrets := make(map[string]string)
	f, err := os.Open(".env")
	if err != nil {
		return secrets
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
			secrets[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return secrets
}

// detectGitSHA runs git rev-parse HEAD.
func detectGitSHA() string {
	return runGitCmd("rev-parse", "HEAD")
}

// detectGitActor runs git config user.name.
func detectGitActor() string {
	return runGitCmd("config", "user.name")
}

// detectGitRepo tries to get "owner/repo" from the remote URL.
func detectGitRepo() string {
	remote := runGitCmd("remote", "get-url", "origin")
	if remote == "" {
		return "local/repo"
	}
	// Strip .git suffix and parse owner/repo
	remote = strings.TrimSuffix(remote, ".git")
	if idx := strings.LastIndex(remote, "/"); idx >= 0 {
		repo := remote[idx+1:]
		if idx2 := strings.LastIndex(remote[:idx], "/"); idx2 >= 0 {
			return remote[idx2+1:idx] + "/" + repo
		}
		if idx2 := strings.LastIndex(remote[:idx], ":"); idx2 >= 0 {
			return remote[idx2+1:idx] + "/" + repo
		}
	}
	return "local/repo"
}
