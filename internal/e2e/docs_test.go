package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDocumentationExamples(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	for _, path := range []string{
		"README.md",
		"CHANGELOG.md",
		"docs/protocol/state-v1.md",
		"docs/user/safe-init.md",
		"docs/user/diagnostics.md",
		"docs/engineering/package-boundaries.md",
	} {
		if _, err := os.Stat(filepath.Join(repo, path)); err != nil {
			t.Fatalf("document %s is missing: %v", path, err)
		}
	}

	workspace := t.TempDir()
	binary := filepath.Join(t.TempDir(), "uawp")
	build := exec.Command("go", "build", "-o", binary, "./cmd/uawp")
	build.Dir = repo
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build documentation binary: %v (%s)", err, output)
	}
	commands := [][]string{
		{binary, "init", "--workspace", workspace, "--format", "json"},
		{binary, "status", "--workspace", workspace, "--format", "json"},
		{binary, "doctor", "--workspace", workspace, "--format", "json"},
	}
	want := []int{5, 0, 0}
	for index, args := range commands {
		command := exec.Command(args[0], args[1:]...)
		command.Dir = repo
		output, err := command.CombinedOutput()
		exitCode := 0
		if exit, ok := err.(*exec.ExitError); ok {
			exitCode = exit.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v (%s)", args, err, output)
		}
		if exitCode != want[index] {
			t.Fatalf("run %v exit=%d want=%d output=%s", args, exitCode, want[index], output)
		}
	}
}
