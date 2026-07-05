package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type sharedStep struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Output   string `json:"output,omitempty"`
	Analysis string `json:"analysis,omitempty"`
}

type sharedSession struct {
	Slug          string       `json:"slug"`
	WorkflowName  string       `json:"workflow_name"`
	Platform      string       `json:"platform"`
	Steps         []sharedStep `json:"steps"`
	SessionStatus string       `json:"session_status"`
}

// liveSession tracks an in-progress shared session.
type liveSession struct {
	slug      string
	steps     []sharedStep
	supaURL   string
	anonKey   string
}

func stepStatusStr(r stepResult) string {
	switch {
	case r.aborted:
		return "aborted"
	case r.warned:
		return "warned"
	case r.skipped:
		return "skipped"
	case r.passed:
		return "passed"
	default:
		return "failed"
	}
}

// CreateLiveSession creates a session upfront with all steps as "pending".
func CreateLiveSession(wfName, platform string, stepNames []string) (*liveSession, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	anonKey := os.Getenv("SUPABASE_ANON_KEY")
	if supabaseURL == "" || anonKey == "" {
		return nil, fmt.Errorf("SUPABASE_URL and SUPABASE_ANON_KEY must be set in .env")
	}

	slug := randomSlug(8)
	steps := make([]sharedStep, len(stepNames))
	for i, name := range stepNames {
		steps[i] = sharedStep{Name: name, Status: "pending"}
	}

	session := sharedSession{
		Slug:          slug,
		WorkflowName:  wfName,
		Platform:      platform,
		Steps:         steps,
		SessionStatus: "running",
	}

	if err := postSession(supabaseURL, anonKey, session); err != nil {
		return nil, err
	}

	return &liveSession{slug: slug, steps: steps, supaURL: supabaseURL, anonKey: anonKey}, nil
}

// UpdateStep marks a specific step by name with a new status and optional output,
// then PATCHes the session in Supabase.
func (ls *liveSession) UpdateStep(name, status, output, analysis string) error {
	for i := range ls.steps {
		if ls.steps[i].Name == name {
			ls.steps[i].Status = status
			ls.steps[i].Output = output
			ls.steps[i].Analysis = analysis
			break
		}
	}
	return ls.patch("running")
}

// Finish sets the overall session status to completed or failed.
func (ls *liveSession) Finish(failed bool) error {
	status := "completed"
	if failed {
		status = "failed"
	}
	return ls.patch(status)
}

func (ls *liveSession) patch(sessionStatus string) error {
	payload := map[string]interface{}{
		"steps":          ls.steps,
		"session_status": sessionStatus,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := ls.supaURL + "/rest/v1/sessions?slug=eq." + ls.slug
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", ls.anonKey)
	req.Header.Set("Authorization", "Bearer "+ls.anonKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("patch failed (status %d)", resp.StatusCode)
	}
	return nil
}

// ShareSession is the original one-shot upload (used when not sharing live).
func ShareSession(wfName, platform string, results []stepResult) (string, error) {
	supabaseURL := os.Getenv("SUPABASE_URL")
	anonKey := os.Getenv("SUPABASE_ANON_KEY")
	if supabaseURL == "" || anonKey == "" {
		return "", fmt.Errorf("SUPABASE_URL and SUPABASE_ANON_KEY must be set in .env")
	}

	slug := randomSlug(8)
	steps := make([]sharedStep, len(results))
	for i, r := range results {
		steps[i] = sharedStep{Name: r.name, Status: stepStatusStr(r), Output: r.output, Analysis: r.analysis}
	}

	session := sharedSession{
		Slug:          slug,
		WorkflowName:  wfName,
		Platform:      platform,
		Steps:         steps,
		SessionStatus: "completed",
	}

	if err := postSession(supabaseURL, anonKey, session); err != nil {
		return "", err
	}
	return slug, nil
}

func postSession(supabaseURL, anonKey string, session sharedSession) error {
	body, err := json.Marshal(session)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", supabaseURL+"/rest/v1/sessions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("apikey", anonKey)
	req.Header.Set("Authorization", "Bearer "+anonKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload failed (status %d) — check your Supabase keys", resp.StatusCode)
	}
	return nil
}

func randomSlug(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}
