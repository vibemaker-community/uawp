package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSmokeRunsPackagedLifecycleAndPreservesProjectFiles(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	binary := filepath.Join(t.TempDir(), "uawp")
	command := exec.Command("go", "build", "-o", binary, "./cmd/uawp")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	workspace := t.TempDir()
	project := filepath.Join(workspace, "proposal.docx")
	want := []byte{0x50, 0x4b, 0x03, 0x04, 0x01}
	if err := os.WriteFile(project, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := smoke(binary, workspace); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(project)
	if err != nil || string(got) != string(want) {
		t.Fatalf("project file changed: %x err=%v", got, err)
	}
}
