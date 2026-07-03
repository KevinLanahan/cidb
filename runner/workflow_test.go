package runner

import (
	"testing"
)

func TestParseWorkflow(t *testing.T) {
	wf, err := parseWorkflow("../testdata/ci.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wf.Name == "" {
		t.Error("expected workflow name to be set")
	}

	if len(wf.Jobs) == 0 {
		t.Error("expected at least one job")
	}

	for jobID, job := range wf.Jobs {
		if len(job.Steps) == 0 {
			t.Errorf("job %q has no steps", jobID)
		}
	}
}

func TestParseWorkflowMissingFile(t *testing.T) {
	_, err := parseWorkflow("nonexistent.yml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestStepName(t *testing.T) {
	tests := []struct {
		step     Step
		index    int
		expected string
	}{
		{Step{Name: "My Step"}, 0, "My Step"},
		{Step{Uses: "actions/checkout@v4"}, 0, "actions/checkout@v4"},
		{Step{}, 2, "step 3"},
	}

	for _, tt := range tests {
		got := stepName(tt.step, tt.index)
		if got != tt.expected {
			t.Errorf("stepName(%+v, %d) = %q, want %q", tt.step, tt.index, got, tt.expected)
		}
	}
}
