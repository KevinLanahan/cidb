package runner

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// exprPattern matches ${{ secrets.X }}, ${{ env.X }}, ${{ vars.X }}
var exprPattern = regexp.MustCompile(`\$\{\{\s*(secrets|env|vars)\.(\w+)\s*\}\}`)

// loadSecrets reads a .env file from the current directory and returns a map of key->value.
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
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			secrets[key] = val
		}
	}
	return secrets
}

// expand replaces ${{ secrets.X }}, ${{ env.X }}, ${{ vars.X }} with values from the secrets map.
func expand(s string, secrets map[string]string) string {
	return exprPattern.ReplaceAllStringFunc(s, func(match string) string {
		sub := exprPattern.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		key := sub[2]
		if val, ok := secrets[key]; ok {
			return val
		}
		return match // leave unexpanded if not found
	})
}

// expandStep applies expression substitution to all relevant fields of a step.
func expandStep(step Step, secrets map[string]string) Step {
	step.Run = expand(step.Run, secrets)

	expanded := make(map[string]string)
	for k, v := range step.Env {
		expanded[k] = expand(v, secrets)
	}
	step.Env = expanded

	expandedWith := make(map[string]string)
	for k, v := range step.With {
		expandedWith[k] = expand(v, secrets)
	}
	step.With = expandedWith

	return step
}
