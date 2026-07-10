package runner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

const analyzeEndpoint = "https://lokal-kappa.vercel.app/api/analyze"

func analyzeFailure(command string, output string, exitCode int) string {
	// Allow override via env for self-hosters.
	endpoint := os.Getenv("LOKAL_ANALYZE_URL")
	if endpoint == "" {
		endpoint = analyzeEndpoint
	}

	body, _ := json.Marshal(map[string]any{
		"command":  command,
		"output":   output,
		"exitCode": exitCode,
	})

	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var result struct {
		Analysis string `json:"analysis"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	return strings.TrimSpace(result.Analysis)
}
