package e2e

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestHumanStyleCurrentDirectoryLifecyclePreservesOfficeFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"proposal.docx": {0x50, 0x4b, 0x03, 0x04, 0x01},
		"budget.xlsx":   {0x50, 0x4b, 0x03, 0x04, 0x02},
		"notes.txt":     []byte("keep this project material\n"),
	}
	wants := map[string][32]byte{}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
		wants[name] = sha256.Sum256(content)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	init := lifecycleCommand(t, []string{"init", "--format", "json"}, 5)
	lifecycleCommand(t, []string{"init", "--format", "json", "--approve", init["planID"].(string)}, 0)
	previewApply(t, []string{"acquire", "--worker-id", "worker-office", "--session-id", "session-office", "--agent", "Office Agent", "--purpose", "organize", "--format", "json"})
	contextPath := filepath.Join(t.TempDir(), "context.md")
	if err := os.WriteFile(contextPath, []byte("# Office work\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previewApply(t, []string{"sync", "--worker-id", "worker-office", "--session-id", "session-office", "--generation", "1", "--context-file", contextPath, "--format", "json"})
	previewApply(t, []string{"checkpoint", "--worker-id", "worker-office", "--session-id", "session-office", "--generation", "1", "--milestone-id", "phase-2", "--label", "Phase 2 complete", "--format", "json"})
	previewApply(t, []string{"handoff", "--worker-id", "worker-office", "--session-id", "session-office", "--generation", "1", "--purpose", "pause", "--context-file", contextPath, "--format", "json"})

	for name, want := range wants {
		content, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil || sha256.Sum256(content) != want {
			t.Fatalf("%s changed: %v", name, readErr)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".uawp", "checkpoints", "*.md"))
	if len(matches) != 1 {
		t.Fatalf("checkpoints=%v", matches)
	}
}
