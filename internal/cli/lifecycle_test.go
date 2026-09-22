package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
)

func initializedCLIWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	preview := runCLI(t, []string{"init", "--workspace", dir, "--format", "json"}, 5)
	token := decodePlanToken(t, preview)
	runCLI(t, []string{"init", "--workspace", dir, "--format", "json", "--approve", token}, 0)
	return dir
}

func TestGuidedResumeCreatesProfileAndAcquiresInSameInvocation(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	rt, stdout, stderr, config := identityRuntime(t, "Rock's Codex\nyes\n")
	rt.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	if code := runWithRuntime([]string{"resume", "--workspace", dir}, rt); code != exitOK {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Zhang San's Codex") || !strings.Contains(stdout.String(), "Ownership acquired") {
		t.Fatalf("guided output=%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(config, "uawp", "identity.json")); err != nil {
		t.Fatalf("profile not saved: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		t.Fatal(err)
	}
	owner, err := core.DecodeOwnership(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if owner.Status != core.Active || owner.Generation != 1 {
		t.Fatalf("ownership=%#v", owner)
	}
}

func TestGuidedResumeContinuesExactLocalBinding(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	rt, stdout, stderr, _ := identityRuntime(t, "Rock's Codex\nyes\n")
	rt.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	if code := runWithRuntime([]string{"resume", "--workspace", dir}, rt); code != exitOK {
		t.Fatalf("first code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	rt.stdin = strings.NewReader("")
	if code := runWithRuntime([]string{"resume", "--workspace", dir}, rt); code != exitOK {
		t.Fatalf("second code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "OWNED_BY_CALLER") {
		t.Fatalf("output=%s", stdout.String())
	}
}

func TestJSONResumeRequiresExplicitUnambiguousActor(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	for _, args := range [][]string{
		{"resume", "--workspace", dir, "--format", "json"},
		{"resume", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--profile", "profile-a", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != exitUsage {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
		}
	}
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
	raw := runCLI(t, []string{"resume", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--format", "json"}, 0)
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
		{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"},
		{"release", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1", "--format", "json"},
		{"sync", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1", "--context-file", contextFile, "--format", "json"},
		{"checkpoint", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1", "--milestone-id", "phase-2", "--label", "Phase 2", "--format", "json"},
		{"handoff", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1", "--purpose", "pause", "--context-file", contextFile, "--format", "json"},
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
	args := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, 5))
	runCLI(t, append(args, "--approve", token), 0)
	raw := runCLI(t, []string{"resume", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1", "--format", "json"}, 0)
	if !bytes.Contains(raw, []byte("OWNED_BY_CALLER")) {
		t.Fatalf("output=%s", raw)
	}
}

func TestRecoverPreviewIncludesReviewableDocumentsAndMetadata(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, 5))
	runCLI(t, append(args, "--approve", token), 0)
	raw := runCLI(t, []string{"recover", "--workspace", dir, "--controller-id", "human-1", "--reason", "confirmed crash", "--format", "json"}, 5)
	for _, term := range [][]byte{[]byte("worker-a"), []byte("human-1"), []byte("confirmed crash"), []byte("RELEASED"), []byte("ACTIVE")} {
		if !bytes.Contains(raw, term) {
			t.Fatalf("preview missing %q: %s", term, raw)
		}
	}
}
