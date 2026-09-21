package adapter

import (
	"bytes"
	"testing"

	"github.com/uawp/uawp/internal/plan"
)

func TestManagedBlockPreservesOutsideBytesAndIsIdempotent(t *testing.T) {
	before := []byte("# Project\r\nkeep  \r\n")
	spec := BlockSpec{ArtifactID: "uawp-entry-agents-v1", Target: ".uawp/INSTRUCTIONS.md", Consumers: []string{"workbuddy", "codex"}, Body: "Read `.uawp/INSTRUCTIONS.md` before UAWP work."}
	after, meta, err := UpsertManagedBlock(before, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, before) {
		t.Fatal("outside bytes changed")
	}
	if meta.OutsideSHA256 != plan.HashBytes(before) {
		t.Fatal("outside hash mismatch")
	}
	if !bytes.Contains(after, []byte("consumers=codex,workbuddy")) {
		t.Fatalf("unsorted consumers: %q", after)
	}
	again, _, err := UpsertManagedBlock(after, spec)
	if err != nil || !bytes.Equal(after, again) {
		t.Fatalf("not idempotent: %v", err)
	}
	removed, err := RemoveManagedBlock(after, spec)
	if err != nil || !bytes.Equal(removed, before) {
		t.Fatalf("remove=%q err=%v", removed, err)
	}
}

func TestManagedBlockRejectsMalformedOrEditedContent(t *testing.T) {
	spec := BlockSpec{ArtifactID: "entry", Target: ".uawp/INSTRUCTIONS.md", Consumers: []string{"codex"}, Body: "expected"}
	valid, _, err := UpsertManagedBlock([]byte("prefix\n"), spec)
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		bytes.Replace(valid, []byte("<!-- UAWP:END"), []byte("<!-- missing:END"), 1),
		append(append([]byte(nil), valid...), valid...),
		bytes.Replace(valid, []byte("expected"), []byte("edited"), 1),
	}
	for _, content := range cases {
		if _, _, err := UpsertManagedBlock(content, spec); err == nil {
			t.Fatalf("accepted malformed %q", content)
		}
	}
}
