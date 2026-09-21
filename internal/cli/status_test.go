package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

type cliOutput struct {
	SchemaVersion string            `json:"schemaVersion"`
	Command       string            `json:"command"`
	Workspace     string            `json:"workspace"`
	PlanID        string            `json:"planID"`
	Findings      []json.RawMessage `json:"findings"`
	Changes       []json.RawMessage `json:"changes"`
	Mutated       bool              `json:"mutated"`
	NextAction    string            `json:"nextAction"`
}

func TestStatusAndDoctorJSON(t *testing.T) {
	for _, command := range []string{"status", "doctor"} {
		t.Run(command, func(t *testing.T) {
			dir := t.TempDir()
			var stdout, stderr bytes.Buffer
			code := Run([]string{command, "--workspace", dir, "--format", "json"}, &stdout, &stderr)
			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			var got cliOutput
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.SchemaVersion != "1" || got.Command != command || got.Workspace == "" || len(got.Findings) != 1 || got.Mutated || got.NextAction == "" {
				t.Fatalf("output=%#v", got)
			}
		})
	}
}

func TestExitCategories(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"unknown command", []string{"unknown"}, 2},
		{"missing workspace", []string{"status"}, 2},
		{"unexpected root failure", []string{"status", "--workspace", "/definitely/not/a/uawp/workspace"}, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := Run(tt.args, &stdout, &stderr); got != tt.want {
				t.Fatalf("Run(%q)=%d stderr=%q, want %d", tt.args, got, stderr.String(), tt.want)
			}
		})
	}
}
