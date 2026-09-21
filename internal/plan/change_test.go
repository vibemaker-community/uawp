package plan

import (
	"strings"
	"testing"
)

func TestPlanIDIsDeterministic(t *testing.T) {
	changes := []Change{{Kind: CreateFile, Path: ".uawp/manifest.json", AfterSHA256: "abc"}}
	a := New("init", changes)
	b := New("init", changes)
	if a.ID != b.ID {
		t.Fatalf("%q != %q", a.ID, b.ID)
	}
}

func TestPlanSortsAndRendersChanges(t *testing.T) {
	value := New("init", []Change{
		{Kind: CreateFile, Path: ".uawp/z.md", BeforeSHA256: MissingSHA256, AfterSHA256: "z", Size: 7},
		{Kind: CreateDir, Path: ".uawp", BeforeSHA256: MissingSHA256, AfterSHA256: DirectorySHA256},
	})
	changes := value.Changes()
	if changes[0].Path != ".uawp" || changes[1].Path != ".uawp/z.md" {
		t.Fatalf("changes not sorted: %#v", changes)
	}
	preview := value.Preview()
	for _, want := range []string{"CREATE_DIR", ".uawp", MissingSHA256, DirectorySHA256, "CREATE_FILE", ".uawp/z.md", "z", "7 bytes"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q missing %q", preview, want)
		}
	}
}

func TestPlanCopiesContent(t *testing.T) {
	content := []byte("original")
	change := NewFile(".uawp/CONTEXT.md", 0o600, MissingSHA256, content)
	content[0] = 'X'
	value := New("init", []Change{change})
	got := value.Changes()[0].Content()
	got[0] = 'Y'
	if string(value.Changes()[0].Content()) != "original" {
		t.Fatal("plan content was mutable through caller-owned slices")
	}
}

func TestUpdateFileCarriesExactFingerprints(t *testing.T) {
	before := HashBytes([]byte("before"))
	change := NewUpdateFile(".uawp/CONTEXT.md", 0o600, before, []byte("after"))
	if change.Kind != UpdateFile || change.BeforeSHA256 != before || change.AfterSHA256 != HashBytes([]byte("after")) {
		t.Fatalf("change=%#v", change)
	}
	if got := string(change.Content()); got != "after" {
		t.Fatalf("content=%q", got)
	}
}

func TestDeleteFileCarriesExactFingerprint(t *testing.T) {
	before := HashBytes([]byte("owned"))
	change := NewDeleteFile("AGENTS.md", before)
	if change.Kind != DeleteFile || change.BeforeSHA256 != before || change.AfterSHA256 != MissingSHA256 {
		t.Fatalf("change=%#v", change)
	}
}
