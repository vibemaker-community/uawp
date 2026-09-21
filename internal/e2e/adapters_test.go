package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/cli"
	"github.com/uawp/uawp/internal/workspace"
)

type adapterCLIOutput struct {
	PlanID string `json:"planID"`
}

func initializedAdapterWorkspace(t *testing.T, parent string) string {
	t.Helper()
	dir := filepath.Join(parent, "workspace-项目")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	preview, _, code := runAdapterCLI(t, "init", "--workspace", dir, "--format", "json")
	if code != 5 {
		t.Fatalf("init preview code=%d output=%s", code, preview)
	}
	var result adapterCLIOutput
	if err := json.Unmarshal(preview, &result); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runAdapterCLI(t, "init", "--workspace", dir, "--format", "json", "--approve", result.PlanID)
	if code != 0 {
		t.Fatalf("init apply code=%d stderr=%s", code, stderr)
	}
	return dir
}

func applyAdapterCLI(t *testing.T, dir, id string, extra ...string) {
	t.Helper()
	args := []string{"adapter", "add", id, "--workspace", dir, "--format", "json"}
	args = append(args, extra...)
	preview, stderr, code := runAdapterCLI(t, args...)
	if code != 5 {
		t.Fatalf("%s preview code=%d stderr=%s", id, code, stderr)
	}
	var result adapterCLIOutput
	if err := json.Unmarshal(preview, &result); err != nil {
		t.Fatal(err)
	}
	args = append(args, "--approve", result.PlanID)
	_, stderr, code = runAdapterCLI(t, args...)
	if code != 0 {
		t.Fatalf("%s apply code=%d stderr=%s", id, code, stderr)
	}
}

func removeAdapterCLI(t *testing.T, dir, id string) {
	t.Helper()
	args := []string{"adapter", "remove", id, "--workspace", dir, "--format", "json"}
	preview, stderr, code := runAdapterCLI(t, args...)
	if code != 5 {
		t.Fatalf("remove preview code=%d stderr=%s", code, stderr)
	}
	var result adapterCLIOutput
	if err := json.Unmarshal(preview, &result); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runAdapterCLI(t, append(args, "--approve", result.PlanID)...)
	if code != 0 {
		t.Fatalf("remove apply code=%d stderr=%s", code, stderr)
	}
}

func runAdapterCLI(t *testing.T, args ...string) ([]byte, []byte, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, &stdout, &stderr)
	return stdout.Bytes(), stderr.Bytes(), code
}

func TestAdapterGreenfieldAllProvidersShareOneBridge(t *testing.T) {
	dir := initializedAdapterWorkspace(t, t.TempDir())
	applyAdapterCLI(t, dir, "codex")
	applyAdapterCLI(t, dir, "claude-code", "--provider-version", "2.1.277", "--direct-agents-support", "true")
	applyAdapterCLI(t, dir, "workbuddy")
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(content, []byte("Read `.uawp/INSTRUCTIONS.md`")) != 1 || !bytes.Contains(content, []byte("consumers=claude-code,codex,workbuddy")) {
		t.Fatalf("shared bridge=%q", content)
	}
	removeAdapterCLI(t, dir, "workbuddy")
	content, _ = os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if !bytes.Contains(content, []byte("consumers=claude-code,codex")) || bytes.Contains(content, []byte("workbuddy")) {
		t.Fatalf("bridge after single removal=%q", content)
	}
}

func TestAdapterClaudeImportAndWorkBuddyEntryDrift(t *testing.T) {
	claudeDir := initializedAdapterWorkspace(t, t.TempDir())
	applyAdapterCLI(t, claudeDir, "claude-code")
	claude, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil || string(claude) != "@.uawp/INSTRUCTIONS.md" {
		t.Fatalf("CLAUDE.md=%q err=%v", claude, err)
	}

	dir := initializedAdapterWorkspace(t, t.TempDir())
	applyAdapterCLI(t, dir, "codex")
	applyAdapterCLI(t, dir, "workbuddy")
	if err := os.WriteFile(filepath.Join(dir, "CODEBUDDY.md"), []byte("project rules\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runAdapterCLI(t, "status", "--workspace", dir, "--format", "json")
	if code != 4 || !bytes.Contains(stdout, []byte("ENTRY_DRIFT")) {
		t.Fatalf("status code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	applyAdapterCLI(t, dir, "workbuddy")
	stdout, stderr, code = runAdapterCLI(t, "status", "--workspace", dir, "--format", "json")
	if code != 0 || bytes.Contains(stdout, []byte("ENTRY_DRIFT")) {
		t.Fatalf("post-migration status code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestAdapterBrownfieldPreservationAndApprovalDrift(t *testing.T) {
	dir := initializedAdapterWorkspace(t, t.TempDir())
	original := []byte("# Existing\r\nkeep exactly")
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), original, 0o640); err != nil {
		t.Fatal(err)
	}
	args := []string{"adapter", "add", "codex", "--workspace", dir, "--format", "json"}
	preview, _, code := runAdapterCLI(t, args...)
	if code != 5 {
		t.Fatalf("preview code=%d", code)
	}
	var result adapterCLIOutput
	if err := json.Unmarshal(preview, &result); err != nil {
		t.Fatal(err)
	}
	if _, _, code = runAdapterCLI(t, append(args, "--approve", result.PlanID+"wrong")...); code != 5 {
		t.Fatalf("wrong approval code=%d", code)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), append(original, []byte("\r\nconcurrent")...), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, code = runAdapterCLI(t, append(args, "--approve", result.PlanID)...); code != 5 {
		t.Fatalf("drift approval code=%d", code)
	}
	applyAdapterCLI(t, dir, "codex")
	path := filepath.Join(dir, "AGENTS.md")
	file, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = file.WriteString("\r\nuser addition")
	_ = file.Close()
	removeAdapterCLI(t, dir, "codex")
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(content, []byte("# Existing\r\nkeep exactly\r\nconcurrent")) || !bytes.Contains(content, []byte("user addition")) || bytes.Contains(content, []byte("UAWP:BEGIN")) {
		t.Fatalf("preserved content=%q err=%v", content, err)
	}
}

func TestAdapterMalformedBinaryAndInterruptedApplyFailClosed(t *testing.T) {
	dir := initializedAdapterWorkspace(t, t.TempDir())
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte{0, 1, 2}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runAdapterCLI(t, "adapter", "add", "codex", "--workspace", dir); code != 3 {
		t.Fatalf("binary entry code=%d", code)
	}
	malformed := initializedAdapterWorkspace(t, t.TempDir())
	applyAdapterCLI(t, malformed, "codex")
	markerPath := filepath.Join(malformed, "AGENTS.md")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	marker = bytes.Replace(marker, []byte("UAWP:END"), []byte("UAWP:BROKEN"), 1)
	if err := os.WriteFile(markerPath, marker, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runAdapterCLI(t, "adapter", "add", "codex", "--workspace", malformed); code != 4 {
		t.Fatalf("malformed marker code=%d", code)
	}

	interrupted := initializedAdapterWorkspace(t, t.TempDir())
	root, err := workspace.OpenRoot(interrupted)
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := workspace.PlanAdapterAddAt(root, "codex", adapter.RuntimeFacts{}, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, err = workspace.Apply(root, p, workspace.ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, index int) error {
		if stage == "after-publish" && index == 0 {
			return errors.New("power loss")
		}
		return nil
	}})
	if err == nil || workspace.Status(root).Code != workspace.CodeRecoveryRequired {
		t.Fatalf("interruption err=%v status=%s", err, workspace.Status(root).Code)
	}
}
