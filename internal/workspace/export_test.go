package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExportIsVerifiedAndDeterministicForSnapshot(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	if err := os.WriteFile(filepath.Join(root.Path(), ".uawp", "checkpoints", "你好.md"), []byte("checkpoint"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := SnapshotNamespace(root)
	if err != nil {
		t.Fatal(err)
	}
	one := filepath.Join(t.TempDir(), "one.tar.gz")
	two := filepath.Join(t.TempDir(), "two.tar.gz")
	first, err := CreateVerifiedExport(root, one, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateVerifiedExport(root, two, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(one)
	b, _ := os.ReadFile(two)
	if !bytes.Equal(a, b) || first.ArchiveSHA256 != second.ArchiveSHA256 || len(first.Entries) == 0 {
		t.Fatalf("exports differ: %#v %#v", first, second)
	}
}

func TestExportRejectsExistingDestinationAndDrift(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	snapshot, err := SnapshotNamespace(root)
	if err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "state.tar.gz")
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateVerifiedExport(root, destination, snapshot); err == nil {
		t.Fatal("export overwrote destination")
	}
	if err := os.WriteFile(filepath.Join(root.Path(), ".uawp", "CONTEXT.md"), []byte("drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateVerifiedExport(root, filepath.Join(t.TempDir(), "drift.tar.gz"), snapshot); err == nil {
		t.Fatal("export accepted source drift")
	}
}
