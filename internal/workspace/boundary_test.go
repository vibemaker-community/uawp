package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenRootAcceptsDirectory(t *testing.T) {
	dir := t.TempDir()
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(root.Path()) {
		t.Fatalf("root path %q is not absolute", root.Path())
	}
}

func TestOpenRootRejectsMissingAndFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "missing"), file} {
		if _, err := OpenRoot(path); err == nil {
			t.Fatalf("OpenRoot(%q) succeeded, want error", path)
		}
	}
}

func TestResolveUAWPRejectsEscape(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../escape", "/absolute", "checkpoints\\escape.md", "bad\x00name"} {
		if _, err := root.ResolveUAWP(path); err == nil {
			t.Fatalf("ResolveUAWP(%q) succeeded, want error", path)
		}
	}
}

func TestResolveUAWPAllowsCanonicalInstructions(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := root.ResolveUAWP("INSTRUCTIONS.md")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "INSTRUCTIONS.md" {
		t.Fatalf("resolved %q", got)
	}
}

func TestRecoveryBoundaryAllowsOnlyExactMaintenanceLayout(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{
		"migrations", "migrations/20260922T120000Z-1.0.0-to-1.1.0-aabbccdd.json",
		"recovery", "recovery/txn-001", "recovery/txn-001/journal.json",
		"recovery/txn-001/plan.json", "recovery/txn-001/receipt.json",
		"recovery/txn-001/backups", "recovery/txn-001/backups/0000.bak",
	}
	for _, relative := range allowed {
		if _, err := root.ResolveUAWP(relative); err != nil {
			t.Errorf("ResolveUAWP(%q): %v", relative, err)
		}
	}
	for _, relative := range []string{
		"migrations/not-json.txt", "migrations/nested/receipt.json",
		"recovery/txn-001/unknown.json", "recovery/txn-001/backups/nested/file",
		"recovery/../manifest.json", "recovery/txn-001\\journal.json",
	} {
		if _, err := root.ResolveUAWP(relative); err == nil {
			t.Errorf("ResolveUAWP(%q) succeeded", relative)
		}
	}
}

func TestResolveNativeAllowsObservedCodexCandidatesButRejectsArbitraryPaths(t *testing.T) {
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{"TEAM.md", "services/api/AGENTS.md", "services/api/AGENTS.override.md"} {
		if _, err := root.resolveNative(candidate); err != nil {
			t.Fatalf("resolveNative(%q): %v", candidate, err)
		}
	}
	for _, candidate := range []string{"README.txt", "services/api/CLAUDE.md", "../TEAM.md"} {
		if _, err := root.resolveNative(candidate); err == nil {
			t.Fatalf("resolveNative(%q) succeeded", candidate)
		}
	}
}

func TestResolveUAWPRejectsSymlinkAncestors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated Windows privileges")
	}
	for _, intermediate := range []bool{false, true} {
		t.Run(map[bool]string{false: "namespace", true: "intermediate"}[intermediate], func(t *testing.T) {
			dir := t.TempDir()
			outside := t.TempDir()
			if intermediate {
				if err := os.Mkdir(filepath.Join(dir, ".uawp"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(dir, ".uawp", "checkpoints")); err != nil {
					t.Skipf("cannot create symlink: %v", err)
				}
			} else if err := os.Symlink(outside, filepath.Join(dir, ".uawp")); err != nil {
				t.Skipf("cannot create symlink: %v", err)
			}
			root, err := OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			path := "manifest.json"
			if intermediate {
				path = "checkpoints/cp.md"
			}
			if _, err := root.ResolveUAWP(path); err == nil {
				t.Fatal("ResolveUAWP() followed symlink ancestor")
			}
		})
	}
}

func TestPortableSegment(t *testing.T) {
	valid := []string{"manifest.json", "CP-001.md", "上下文.md"}
	invalid := []string{"", ".", "..", "CON", "com1.txt", "COM¹.txt", "LPT³", "name.", "name ", "a:b", "a/b", "a\\b", "x\x01"}
	for _, segment := range valid {
		if !validPortableSegment(segment) {
			t.Errorf("validPortableSegment(%q) = false", segment)
		}
	}
	for _, segment := range invalid {
		if validPortableSegment(segment) {
			t.Errorf("validPortableSegment(%q) = true", segment)
		}
	}
	if !strings.EqualFold("con", "CON") {
		t.Fatal("test assumption failed")
	}
}
