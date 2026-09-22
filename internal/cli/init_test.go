package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInteractiveInitUsesCurrentDirectoryAndSameInvocationConfirmation(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rt := runtime{
		stdin: strings.NewReader("yes\n"), stdout: &stdout, stderr: &stderr,
		getwd:         func() (string, error) { return dir, nil },
		userConfigDir: func() (string, error) { return t.TempDir(), nil },
		now:           func() time.Time { return time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC) },
		random:        rand.Reader, stdinTTY: true, stdoutTTY: true,
	}
	if code := runWithRuntime([]string{"init"}, rt); code != 0 {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".uawp", "manifest.json")); err != nil {
		t.Fatalf("interactive init did not apply: %v", err)
	}
}

func TestInteractiveInitCancellationDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rt := runtime{stdin: strings.NewReader("no\n"), stdout: &stdout, stderr: &stderr, getwd: func() (string, error) { return dir, nil }, now: time.Now, random: rand.Reader, stdinTTY: true, stdoutTTY: true}
	if code := runWithRuntime([]string{"init"}, rt); code != 0 {
		t.Fatalf("code=%d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, ".uawp")); !os.IsNotExist(err) {
		t.Fatalf("cancelled init mutated: %v", err)
	}
}

func TestInitPreviewThenApprove(t *testing.T) {
	dir := t.TempDir()
	var previewOut, previewErr bytes.Buffer
	code := Run([]string{"init", "--workspace", dir, "--format", "json"}, &previewOut, &previewErr)
	if code != 5 {
		t.Fatalf("preview code=%d stdout=%q stderr=%q, want approval-required 5", code, previewOut.String(), previewErr.String())
	}
	if _, err := os.Lstat(filepath.Join(dir, ".uawp")); !os.IsNotExist(err) {
		t.Fatalf("preview mutated workspace: %v", err)
	}
	var preview cliOutput
	if err := json.Unmarshal(previewOut.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v; output=%q", err, previewOut.String())
	}
	if preview.PlanID == "" || len(preview.Changes) == 0 || preview.Mutated {
		t.Fatalf("incomplete preview: %#v", preview)
	}

	var applyOut, applyErr bytes.Buffer
	code = Run([]string{"init", "--workspace", dir, "--approve", preview.PlanID, "--format", "json"}, &applyOut, &applyErr)
	if code != 0 {
		t.Fatalf("apply code=%d stdout=%q stderr=%q", code, applyOut.String(), applyErr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".uawp", "manifest.json")); err != nil {
		t.Fatalf("approved init did not apply: %v", err)
	}
}

func TestInitRejectsWrongApprovalAndUnsafeWorkspace(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", "--workspace", dir, "--approve", "wrong"}, &stdout, &stderr); code != 5 {
		t.Fatalf("wrong approval exit=%d, want 5", code)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".uawp")); !os.IsNotExist(err) {
		t.Fatalf("wrong approval mutated workspace: %v", err)
	}

	unsafe := t.TempDir()
	if err := os.Mkdir(filepath.Join(unsafe, ".uawp"), 0o700); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"init", "--workspace", unsafe}, &stdout, &stderr); code != 3 {
		t.Fatalf("unsafe workspace exit=%d stderr=%q, want 3", code, stderr.String())
	}
}

func TestInitRejectsInvalidFormatBeforeWorkspaceAccess(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", "--workspace", missing, "--format", "yaml"}, &stdout, &stderr); code != 2 {
		t.Fatalf("invalid format exit=%d stderr=%q, want 2", code, stderr.String())
	}
}
