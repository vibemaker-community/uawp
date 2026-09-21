package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAdapterAddPreviewApprovalAndList(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"adapter", "add", "codex", "--workspace", dir, "--format", "json"}
	preview := runCLI(t, args, exitApprovalRequired)
	token := decodePlanToken(t, preview)
	if _, err := os.Lstat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("preview mutated native entry: %v", err)
	}
	runCLI(t, append(args, "--approve", token), exitOK)

	raw := runCLI(t, []string{"adapter", "list", "--workspace", dir, "--format", "json"}, exitOK)
	var result struct {
		Adapters []struct {
			Provider   string `json:"provider"`
			Confidence string `json:"confidence"`
			Health     string `json:"health"`
			Route      struct {
				Path string `json:"path"`
				Mode string `json:"mode"`
			} `json:"route"`
		} `json:"adapters"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Adapters) != 3 {
		t.Fatalf("adapters=%#v", result.Adapters)
	}
	found := false
	for _, item := range result.Adapters {
		if item.Provider == "codex" && item.Route.Path == "AGENTS.md" && item.Route.Mode == "MANAGED_BLOCK" && item.Confidence == "VERIFIED" && item.Health != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing Codex diagnostics: %#v", result.Adapters)
	}
}

func TestAdapterRemoveRequiresExactApproval(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	addArgs := []string{"adapter", "add", "codex", "--workspace", dir, "--format", "json"}
	token := decodePlanToken(t, runCLI(t, addArgs, exitApprovalRequired))
	runCLI(t, append(addArgs, "--approve", token), exitOK)

	removeArgs := []string{"adapter", "remove", "codex", "--workspace", dir, "--format", "json"}
	removeToken := decodePlanToken(t, runCLI(t, removeArgs, exitApprovalRequired))
	if code := Run(append(removeArgs, "--approve", removeToken+"wrong"), &bytes.Buffer{}, &bytes.Buffer{}); code != exitApprovalRequired {
		t.Fatalf("wrong approval code=%d", code)
	}
	runCLI(t, append(removeArgs, "--approve", removeToken), exitOK)
	if _, err := os.Lstat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("created bridge remains: %v", err)
	}
}

func TestAdapterCLIExitCategories(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	if got := Run([]string{"adapter", "add", "unknown", "--workspace", dir}, &bytes.Buffer{}, &bytes.Buffer{}); got != exitUsage {
		t.Fatalf("unknown provider code=%d", got)
	}
	if err := os.Mkdir(filepath.Join(dir, "AGENTS.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := Run([]string{"adapter", "add", "codex", "--workspace", dir}, &bytes.Buffer{}, &bytes.Buffer{}); got != exitUnsafe {
		t.Fatalf("unsafe native code=%d", got)
	}
	conditionalDir := initializedCLIWorkspace(t)
	if err := os.WriteFile(filepath.Join(conditionalDir, "AGENTS.md"), []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := Run([]string{"adapter", "add", "claude-code", "--workspace", conditionalDir}, &bytes.Buffer{}, &bytes.Buffer{}); got != exitInvalidState {
		t.Fatalf("conditional route code=%d", got)
	}
}

func TestAdapterListTextAndStatusAreReadOnly(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	addArgs := []string{"adapter", "add", "codex", "--workspace", dir, "--format", "json"}
	token := decodePlanToken(t, runCLI(t, addArgs, exitApprovalRequired))
	runCLI(t, append(addArgs, "--approve", token), exitOK)
	before, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := runCLI(t, []string{"adapter", "list", "--workspace", dir}, exitOK)
	status := runCLI(t, []string{"status", "--workspace", dir, "--format", "json"}, exitOK)
	doctor := runCLI(t, []string{"doctor", "--workspace", dir, "--format", "json"}, exitOK)
	for label, raw := range map[string][]byte{"list": text, "status": status, "doctor": doctor} {
		if !bytes.Contains(raw, []byte("codex")) || !bytes.Contains(raw, []byte("AGENTS.md")) {
			t.Fatalf("%s output=%s", label, raw)
		}
	}
	after, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("diagnostics mutated entry: %v", err)
	}
}

func TestStatusAndDoctorReportCorruptAdapterBridge(t *testing.T) {
	dir := initializedCLIWorkspace(t)
	args := []string{"adapter", "add", "codex", "--workspace", dir, "--format", "json"}
	token := decodePlanToken(t, runCLI(t, args, exitApprovalRequired))
	runCLI(t, append(args, "--approve", token), exitOK)
	path := filepath.Join(dir, "AGENTS.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content = bytes.Replace(content, []byte("UAWP:END"), []byte("UAWP:BROKEN"), 1)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"status", "doctor"} {
		raw := runCLI(t, []string{command, "--workspace", dir, "--format", "json"}, exitInvalidState)
		if !bytes.Contains(raw, []byte("INTEGRATION_DRIFT")) {
			t.Fatalf("%s output=%s", command, raw)
		}
	}
}
