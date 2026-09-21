package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func initializedCLIWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	preview := runCLI(t, []string{"init", "--workspace", dir, "--format", "json"}, 5)
	token := decodePlanToken(t, preview)
	runCLI(t, []string{"init", "--workspace", dir, "--format", "json", "--approve", token}, 0)
	return dir
}
func runCLI(t *testing.T, args []string, want int) []byte {
	t.Helper()
	var out, err bytes.Buffer
	if got := Run(args, &out, &err); got != want {
		t.Fatalf("code=%d want=%d stderr=%q", got, want, err.String())
	}
	return out.Bytes()
}
func decodePlanToken(t *testing.T, raw []byte) string {
	t.Helper()
	var v struct {
		PlanID string `json:"planID"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.PlanID == "" {
		t.Fatal("missing token")
	}
	return v.PlanID
}

func TestResumeCommandReportsAvailability(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	raw := runCLI(t, []string{"resume", "--workspace", dir, "--worker-id", "worker-a", "--format", "json"}, 0)
	if !bytes.Contains(raw, []byte("ACQUIRE_AVAILABLE")) {
		t.Fatalf("output=%s", raw)
	}
}

func TestAcquireReleaseSyncCheckpointHandoffRecoverCommandsPreview(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	contextFile := filepath.Join(t.TempDir(), "context.md")
	if err := os.WriteFile(contextFile, []byte("# New\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	commands := [][]string{
		{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"},
		{"release", "--workspace", dir, "--worker-id", "worker-a", "--format", "json"},
		{"sync", "--workspace", dir, "--worker-id", "worker-a", "--context-file", contextFile, "--format", "json"},
		{"checkpoint", "--workspace", dir, "--worker-id", "worker-a", "--milestone-id", "phase-2", "--label", "Phase 2", "--format", "json"},
		{"handoff", "--workspace", dir, "--worker-id", "worker-a", "--purpose", "pause", "--context-file", contextFile, "--format", "json"},
		{"recover", "--workspace", dir, "--controller-id", "human-1", "--reason", "crash", "--format", "json"},
	}
	// Acquire can preview while RELEASED; the remaining commands must parse but
	// reject the current state without mutation.
	decodePlanToken(t, runCLI(t, commands[0], 5))
	for _, args := range commands[1:] {
		var out, err bytes.Buffer
		code := Run(args, &out, &err)
		if code == 2 {
			t.Fatalf("command %s failed parsing: %s", args[0], err.String())
		}
	}
}

func TestAcquireCommandPreviewAndApply(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, 5))
	runCLI(t, append(args, "--approve", token), 0)
	raw := runCLI(t, []string{"resume", "--workspace", dir, "--worker-id", "worker-a", "--format", "json"}, 0)
	if !bytes.Contains(raw, []byte("OWNED_BY_CALLER")) {
		t.Fatalf("output=%s", raw)
	}
}
