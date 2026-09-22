package e2e

import (
	"bytes"
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
		"docs/user/adapters.md",
		"docs/user/identity-and-sessions.md",
		"docs/user/agent-automation.md",
		"docs/adapters/authoring.md",
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

func TestPlan45DocumentationContract(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	paths := []string{"README.md", "docs/protocol/state-v1.md", "docs/user/lifecycle.md", "docs/user/identity-and-sessions.md", "docs/user/agent-automation.md"}
	var combined []byte
	for _, name := range paths {
		raw, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		combined = append(combined, raw...)
	}
	for _, required := range [][]byte{
		[]byte("1.2.0"), []byte("uawp init"), []byte("uawp resume"), []byte("uawp sync"), []byte("uawp checkpoint"), []byte("uawp handoff"),
		[]byte("--workspace"), []byte("--worker-id"), []byte("--session-id"), []byte("--generation"), []byte("--format json"), []byte("--approve"),
		[]byte("exit code `5`"), []byte("Git is optional"), []byte("same-directory parallel writing is not supported"),
	} {
		if !bytes.Contains(combined, required) {
			t.Errorf("documentation missing %q", required)
		}
	}
}
