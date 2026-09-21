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
	invalid := []string{"", ".", "..", "CON", "com1.txt", "name.", "name ", "a:b", "a/b", "a\\b", "x\x01"}
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
