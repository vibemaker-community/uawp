package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/identity"
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

func TestHumanLifecycleUsesCurrentDirectoryAndBinding(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	config := t.TempDir()
	randomData := make([]byte, 512)
	for index := range randomData {
		randomData[index] = byte(index)
	}
	newRuntime := func(input string) (runtime, *bytes.Buffer, *bytes.Buffer) {
		var stdout, stderr bytes.Buffer
		return runtime{
			stdin: strings.NewReader(input), stdout: &stdout, stderr: &stderr,
			getwd: func() (string, error) { return dir, nil }, userConfigDir: func() (string, error) { return config, nil },
			now: func() time.Time { return time.Date(2026, 9, 23, 13, 0, 0, 0, time.UTC) }, random: bytes.NewReader(randomData),
			stdinTTY: true, stdoutTTY: true,
		}, &stdout, &stderr
	}
	rt, stdout, stderr := newRuntime("Rock's Codex\nyes\n")
	if code := runWithRuntime([]string{"resume"}, rt); code != exitOK {
		t.Fatalf("resume code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}

	contextPath := filepath.Join(t.TempDir(), "final-context.md")
	if err := os.WriteFile(contextPath, []byte("# Updated context\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt, stdout, stderr = newRuntime("yes\n")
	if code := runWithRuntime([]string{"sync", "--context-file", contextPath}, rt); code != exitOK {
		t.Fatalf("sync code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	contextRaw, _ := os.ReadFile(filepath.Join(dir, ".uawp", "CONTEXT.md"))
	if string(contextRaw) != "# Updated context\n" {
		t.Fatalf("context=%q", contextRaw)
	}

	rt, stdout, stderr = newRuntime("Phase 2 complete\nyes\n")
	if code := runWithRuntime([]string{"checkpoint"}, rt); code != exitOK {
		t.Fatalf("checkpoint code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "Milestone ID") || !strings.Contains(stdout.String(), "Checkpoint name: ") {
		t.Fatalf("checkpoint prompts=%s", stdout.String())
	}
	entries, _ := os.ReadDir(filepath.Join(dir, ".uawp", "checkpoints"))
	if len(entries) != 1 {
		t.Fatalf("checkpoint count=%d", len(entries))
	}
	if name := entries[0].Name(); !strings.Contains(name, "phase-2-complete-") {
		t.Fatalf("generated checkpoint name=%q", name)
	}

	rt, stdout, stderr = newRuntime("pause for another agent\nyes\n")
	if code := runWithRuntime([]string{"handoff", "--context-file", contextPath}, rt); code != exitOK {
		t.Fatalf("handoff code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	registry, _, err := loadIdentityRegistry(rt)
	if err != nil {
		t.Fatal(err)
	}
	profile, _ := registry.Selected("", true)
	if _, found := registry.Binding(dir, profile.ProfileID); found {
		t.Fatal("handoff retained local binding")
	}
}

func TestAutomationCheckpointGeneratesStableMilestoneIDAcrossApproval(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	acquire := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, acquire, exitApprovalRequired))
	runCLI(t, append(acquire, "--approve", token), exitOK)

	var stdout, stderr bytes.Buffer
	rt := runtime{
		stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr,
		getwd: func() (string, error) { return dir, nil }, userConfigDir: os.UserConfigDir,
		now:    func() time.Time { return time.Date(2026, 9, 23, 13, 5, 7, 0, time.UTC) },
		random: bytes.NewReader([]byte{0xde, 0xad, 0xbe, 0xef, 1, 2, 3, 4}),
	}
	args := []string{"checkpoint", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1", "--label", "阶段 二 / Review", "--format", "json", "--non-interactive"}
	if code := runWithRuntime(args, rt); code != exitApprovalRequired {
		t.Fatalf("preview code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	var preview struct {
		PlanID       string `json:"planID"`
		CheckpointID string `json:"checkpointID"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &preview); err != nil || preview.PlanID == "" || preview.CheckpointID != "20260923t130507-review-deadbeef01020304" {
		t.Fatalf("preview=%s err=%v", stdout.String(), err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithRuntime(append(args, "--approve", preview.PlanID), rt); code != exitOK {
		t.Fatalf("apply code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".uawp", "checkpoints"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	if name := entries[0].Name(); !strings.Contains(name, preview.CheckpointID) {
		t.Fatalf("generated checkpoint name=%q", name)
	}
}

func TestHumanSyncWithoutContextFileDoesNotConsumePromptAsContext(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	rt, stdout, stderr, _ := identityRuntime(t, "yes\n")
	rt.getwd = func() (string, error) { return dir, nil }
	before, _ := os.ReadFile(filepath.Join(dir, ".uawp", "CONTEXT.md"))
	if code := runWithRuntime([]string{"sync"}, rt); code != exitUsage {
		t.Fatalf("code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	after, _ := os.ReadFile(filepath.Join(dir, ".uawp", "CONTEXT.md"))
	if !bytes.Equal(before, after) || !strings.Contains(stderr.String(), "--context-file") {
		t.Fatalf("out=%s err=%s", stdout.String(), stderr.String())
	}
}

func TestAutomationLifecycleEmitsOneStructuredJSONDocument(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json", "--non-interactive"}
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr); code != exitApprovalRequired {
		t.Fatalf("preview code=%d stderr=%s", code, stderr.String())
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var result struct {
		PlanID, WorkerID, SessionID, Code string
		OwnershipGeneration               uint64 `json:"ownershipGeneration"`
		Mutated                           bool   `json:"mutated"`
	}
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("extra JSON or prompt output: %v: %s", err, stdout.String())
	}
	if result.PlanID == "" || result.WorkerID != "worker-a" || result.SessionID != "session-a" || result.OwnershipGeneration != 1 || result.Code != codeApprovalRequired || result.Mutated {
		t.Fatalf("result=%#v", result)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(append(args, "--approve", result.PlanID), &stdout, &stderr); code != exitOK {
		t.Fatalf("apply code=%d stderr=%s", code, stderr.String())
	}
}

func TestAutomationResumeClassifiesOtherSessionAndOtherWorker(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, exitApprovalRequired))
	runCLI(t, append(args, "--approve", token), exitOK)
	for _, test := range []struct{ worker, session, code string }{
		{"worker-a", "session-b", codeActiveOtherSession},
		{"worker-b", "session-b", codeActiveOtherWorker},
	} {
		raw := runCLI(t, []string{"resume", "--workspace", dir, "--worker-id", test.worker, "--session-id", test.session, "--generation", "1", "--format", "json"}, exitOK)
		var result struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || result.Code != test.code {
			t.Fatalf("actor=%s/%s output=%s err=%v", test.worker, test.session, raw, err)
		}
	}
}

func TestAutomationMissingSessionReturnsOneStructuredError(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"resume", "--workspace", dir, "--worker-id", "worker-a", "--format", "json"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var result struct {
		Code    string `json:"code"`
		Mutated bool   `json:"mutated"`
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&result); err != nil || result.Code != codeSessionRequired || result.Mutated {
		t.Fatalf("result=%#v err=%v output=%s", result, err, stdout.String())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("extra output: %v", err)
	}
}

func TestAutomationInvalidWorkspaceAndApprovalReturnStructuredErrors(t *testing.T) {
	for _, args := range [][]string{
		{"resume", "--workspace", t.TempDir(), "--worker-id", "worker-a", "--session-id", "session-a", "--format", "json"},
		{"acquire", "--workspace", t.TempDir(), "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "A", "--purpose", "work", "--format", "json"},
		{"acquire", "--workspace", filepath.Join(t.TempDir(), "missing"), "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "A", "--purpose", "work", "--format", "json"},
		{"acquire", "--workspace", initializedCLIWorkspace(t), "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "A", "--purpose", "work", "--approve", "bad", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run(args, &stdout, &stderr)
		if code != exitInvalidState && code != exitApprovalRequired && code != exitInternal {
			t.Fatalf("args=%v code=%d", args, code)
		}
		var result struct {
			Code string `json:"code"`
		}
		decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
		if err := decoder.Decode(&result); err != nil || result.Code == "" {
			t.Fatalf("args=%v result=%#v err=%v output=%s", args, result, err, stdout.String())
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("extra output: %v", err)
		}
	}
}

func TestAutomationArgumentConflictReturnsStructuredJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"release", "--workspace", t.TempDir(), "--worker-id", "worker-a", "--profile", "profile-a", "--session-id", "session-a", "--generation", "1", "--format", "json"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var result struct {
		Code string `json:"code"`
	}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&result); err != nil || result.Code != codeProfileAmbiguous {
		t.Fatalf("result=%#v err=%v output=%s", result, err, stdout.String())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("extra output: %v", err)
	}
}

func TestAutomationParseFailureFindsTrailingJSONFormat(t *testing.T) {
	for _, args := range [][]string{
		{"release", "--workspace", t.TempDir(), "--generation", "not-a-number", "--format", "json"},
		{"resume", "--workspace", t.TempDir(), "--unknown-option", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != exitUsage {
			t.Fatalf("args=%v code=%d", args, code)
		}
		var result struct {
			Code string `json:"code"`
		}
		decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
		if err := decoder.Decode(&result); err != nil || result.Code != codeInvalidArguments {
			t.Fatalf("args=%v result=%#v err=%v output=%s", args, result, err, stdout.String())
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("extra output: %v", err)
		}
	}
}

func TestHumanSyncPreviewShowsReplacementContent(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	rt, _, _, _ := identityRuntime(t, "Rock\nyes\n")
	rt.getwd = func() (string, error) { return dir, nil }
	if code := runWithRuntime([]string{"resume"}, rt); code != exitOK {
		t.Fatalf("resume code=%d", code)
	}
	contextPath := filepath.Join(t.TempDir(), "context.md")
	if err := os.WriteFile(contextPath, []byte("# Exact replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rt.stdout, rt.stderr, rt.stdin = &stdout, &stderr, strings.NewReader("no\n")
	if code := runWithRuntime([]string{"sync", "--context-file", contextPath}, rt); code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Exact replacement") || !strings.Contains(stdout.String(), dir) {
		t.Fatalf("preview=%s", stdout.String())
	}
}

func TestHumanExplicitReleaseDoesNotRequireLocalRegistry(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, exitApprovalRequired))
	runCLI(t, append(args, "--approve", token), exitOK)
	var stdout, stderr bytes.Buffer
	rt := runtime{stdin: strings.NewReader("yes\n"), stdout: &stdout, stderr: &stderr, getwd: func() (string, error) { return dir, nil }, userConfigDir: func() (string, error) { return t.TempDir(), nil }, now: time.Now, random: bytes.NewReader(make([]byte, 64)), stdinTTY: true, stdoutTTY: true}
	if code := runWithRuntime([]string{"release", "--worker-id", "worker-a", "--session-id", "session-a", "--generation", "1"}, rt); code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestHumanExplicitProfileSessionIsNotReplacedByBinding(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	rt, stdout, stderr, config := identityRuntime(t, "")
	if code := runWithRuntime([]string{"identity", "create", "--name", "A", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("identity code=%d", code)
	}
	var created struct {
		Profile struct{ ProfileID, WorkerID string } `json:"profile"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	args := []string{"acquire", "--workspace", dir, "--worker-id", created.Profile.WorkerID, "--session-id", "session-a", "--agent", "A", "--purpose", "work", "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, exitApprovalRequired))
	runCLI(t, append(args, "--approve", token), exitOK)
	registry, path, err := loadIdentityRegistry(rt)
	if err != nil {
		t.Fatal(err)
	}
	registry.UpsertBinding(identity.SessionBinding{Workspace: dir, ProfileID: created.Profile.ProfileID, SessionID: "session-a", Generation: 1})
	if err := identity.Save(path, registry); err != nil {
		t.Fatal(err)
	}
	contextPath := filepath.Join(config, "context.md")
	if err := os.WriteFile(contextPath, []byte("# next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	rt.stdin = strings.NewReader("no\n")
	if code := runWithRuntime([]string{"sync", "--workspace", dir, "--profile", created.Profile.ProfileID, "--session-id", "session-b", "--generation", "1", "--context-file", contextPath}, rt); code == exitOK {
		t.Fatalf("explicit conflicting Session was replaced by binding; output=%s", stdout.String())
	}
	contextRaw, err := os.ReadFile(filepath.Join(dir, ".uawp", "CONTEXT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contextRaw, []byte("# next")) {
		t.Fatalf("conflicting explicit Session mutated context: out=%s err=%s", stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	rt.stdin = strings.NewReader("no\n")
	if code := runWithRuntime([]string{"sync", "--workspace", dir, "--session-id", "session-b", "--generation", "1", "--context-file", contextPath}, rt); code != exitUsage {
		t.Fatalf("default profile silently replaced explicit Session; code=%d out=%s err=%s", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	rt.stdin = strings.NewReader("no\n")
	if code := runWithRuntime([]string{"release", "--workspace", dir, "--generation", "0"}, rt); code != exitUsage {
		t.Fatalf("explicit zero generation silently replaced by binding; code=%d out=%s err=%s", code, stdout.String(), stderr.String())
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
