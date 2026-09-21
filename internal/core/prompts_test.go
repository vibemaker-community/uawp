package core

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptLibraryIsClosedNeutralAndImmutable(t *testing.T) {
	names := PromptNames()
	if len(names) != 4 {
		t.Fatalf("names=%v", names)
	}
	for _, name := range names {
		content, err := Prompt(name)
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if !strings.Contains(text, "UAWP Prompt Version: 1") || !strings.Contains(text, ".uawp/") {
			t.Fatalf("prompt %s incomplete", name)
		}
		for _, forbidden := range []string{"Claude", "Codex", "AGENTS.md", "CLAUDE.md"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("prompt %s contains %s", name, forbidden)
			}
		}
		copy(content, bytes.Repeat([]byte("x"), len(content)))
		again, _ := Prompt(name)
		if bytes.Equal(content, again) {
			t.Fatal("prompt storage mutated")
		}
	}
	if _, err := Prompt(PromptName("UNKNOWN")); err == nil {
		t.Fatal("unknown prompt accepted")
	}
}

func TestPromptLifecycleRequirements(t *testing.T) {
	checks := map[PromptName][]string{
		ResumeWork:       {"read-only", "never acquire", "ACTIVE_WORKER.md", "CONTEXT.md"},
		ContextSync:      {"ACTIVE owner", "retain ownership", "CONTEXT.md"},
		CreateCheckpoint: {"explicit milestone", "checkpoints/", "never overwrite"},
		PauseAndHandoff:  {"verify", "release", "no automatic checkpoint", "no further shared writes"},
	}
	for name, terms := range checks {
		raw, _ := Prompt(name)
		text := string(raw)
		for _, term := range terms {
			if !strings.Contains(text, term) {
				t.Errorf("%s missing %q", name, term)
			}
		}
	}
}
