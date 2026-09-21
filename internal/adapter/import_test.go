package adapter

import (
	"bytes"
	"testing"
)

func TestImportUpsertAndRemoveAreExact(t *testing.T) {
	before := []byte("# Claude\n```\n@.uawp/INSTRUCTIONS.md\n```\n")
	after, changed, err := UpsertImport(before, ".uawp/INSTRUCTIONS.md")
	if err != nil || !changed {
		t.Fatalf("changed=%t err=%v", changed, err)
	}
	if bytes.Count(after, []byte("@.uawp/INSTRUCTIONS.md")) != 2 {
		t.Fatalf("content=%q", after)
	}
	again, changed, err := UpsertImport(after, ".uawp/INSTRUCTIONS.md")
	if err != nil || changed || !bytes.Equal(after, again) {
		t.Fatalf("duplicate import")
	}
	removed, changed, err := RemoveImport(after, ".uawp/INSTRUCTIONS.md")
	if err != nil || !changed || !bytes.Equal(removed, before) {
		t.Fatalf("remove=%q changed=%t err=%v", removed, changed, err)
	}
}

func TestImportPreservesCRLFAndNoFinalNewline(t *testing.T) {
	before := []byte("# Claude\r\nkeep")
	after, _, err := UpsertImport(before, ".uawp/INSTRUCTIONS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, append(before, []byte("\r\n")...)) || !bytes.HasSuffix(after, []byte("@.uawp/INSTRUCTIONS.md")) {
		t.Fatalf("content=%q", after)
	}
}

func TestImportRejectsDuplicateOwnedLines(t *testing.T) {
	content := []byte("@.uawp/INSTRUCTIONS.md\n@.uawp/INSTRUCTIONS.md\n")
	if _, _, err := UpsertImport(content, ".uawp/INSTRUCTIONS.md"); err == nil {
		t.Fatal("upsert accepted duplicate import")
	}
	if _, _, err := RemoveImport(content, ".uawp/INSTRUCTIONS.md"); err == nil {
		t.Fatal("remove accepted duplicate import")
	}
}
