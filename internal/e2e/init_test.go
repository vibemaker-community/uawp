package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/uawp/uawp/internal/cli"
)

type fixtureManifest struct {
	Files []fixtureFile `json:"files"`
}
type fixtureFile struct {
	Path   string      `json:"path"`
	Mode   fs.FileMode `json:"mode"`
	SHA256 string      `json:"sha256"`
}
type initOutput struct {
	PlanID  string            `json:"planID"`
	Changes []json.RawMessage `json:"changes"`
	Mutated bool              `json:"mutated"`
}

func TestBrownfieldInitPreservesProtectedFiles(t *testing.T) {
	dir := copyBrownfield(t)
	manifest := loadFixtureManifest(t)
	preview := runInit(t, dir, "")
	if preview.Mutated || preview.PlanID == "" || len(preview.Changes) == 0 {
		t.Fatalf("bad preview: %#v", preview)
	}
	assertFixture(t, dir, manifest)
	apply := runInit(t, dir, preview.PlanID)
	if !apply.Mutated {
		t.Fatalf("apply did not mutate UAWP state: %#v", apply)
	}
	if _, err := os.Stat(filepath.Join(dir, ".uawp", "manifest.json")); err != nil {
		t.Fatal(err)
	}
	assertFixture(t, dir, manifest)
	repeat := runInit(t, dir, "")
	if len(repeat.Changes) != 0 || repeat.Mutated {
		t.Fatalf("repeat init planned changes: %#v", repeat)
	}
}

func TestGreenfieldAndUnknownNamespace(t *testing.T) {
	greenfield := t.TempDir()
	preview := runInit(t, greenfield, "")
	if len(preview.Changes) == 0 {
		t.Fatal("greenfield preview has no changes")
	}
	unknown := t.TempDir()
	if err := os.Mkdir(filepath.Join(unknown, ".uawp"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"init", "--workspace", unknown, "--format", "json"}, &stdout, &stderr); code != 3 {
		t.Fatalf("unknown namespace exit=%d stderr=%q", code, stderr.String())
	}
}

func TestPreviewRejectsChangedProjectInput(t *testing.T) {
	dir := copyBrownfield(t)
	preview := runInit(t, dir, "")
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"init", "--workspace", dir, "--format", "json", "--approve", preview.PlanID}, &stdout, &stderr); code != 5 {
		t.Fatalf("changed preview exit=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Lstat(filepath.Join(dir, ".uawp")); !os.IsNotExist(err) {
		t.Fatalf("state created after input drift: %v", err)
	}
}

func runInit(t *testing.T, dir, approval string) initOutput {
	t.Helper()
	args := []string{"init", "--workspace", dir, "--format", "json"}
	if approval != "" {
		args = append(args, "--approve", approval)
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, &stdout, &stderr)
	if approval == "" && code != 5 {
		t.Fatalf("preview code=%d stderr=%q", code, stderr.String())
	}
	if approval != "" && code != 0 {
		t.Fatalf("apply code=%d stderr=%q", code, stderr.String())
	}
	var result initOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode output: %v; %q", err, stdout.String())
	}
	return result
}

func copyBrownfield(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "..", "testdata", "brownfield")
	target := t.TempDir()
	manifest := loadFixtureManifest(t)
	expectedMode := map[string]fs.FileMode{}
	for _, entry := range manifest.Files {
		expectedMode[entry.Path] = entry.Mode
	}
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := os.WriteFile(destination, content, info.Mode()); err != nil {
			return err
		}
		if mode, ok := expectedMode[filepath.ToSlash(rel)]; ok {
			return os.Chmod(destination, mode)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return target
}

func loadFixtureManifest(t *testing.T) fixtureManifest {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "brownfield", ".fixture-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func assertFixture(t *testing.T, root string, manifest fixtureManifest) {
	t.Helper()
	for _, entry := range manifest.Files {
		path := filepath.Join(root, filepath.FromSlash(entry.Path))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing %s: %v", entry.Path, err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != entry.Mode {
			t.Fatalf("mode %s = %o, want %o", entry.Path, info.Mode().Perm(), entry.Mode)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(content)
		if got := hex.EncodeToString(sum[:]); got != entry.SHA256 {
			t.Fatalf("hash %s = %s, want %s", entry.Path, got, entry.SHA256)
		}
	}
}
