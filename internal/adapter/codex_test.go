package adapter

import (
	"strings"
	"testing"

	"github.com/uawp/uawp/internal/core"
)

func codexSnapshot(files map[string]string) Snapshot {
	s := Snapshot{Files: map[string]FileFact{}, Options: map[string]string{}}
	for _, p := range []string{"AGENTS.override.md", "AGENTS.md"} {
		s.Files[p] = FileFact{Path: p, State: FileMissing}
	}
	for p, c := range files {
		s.Files[p] = FileFact{Path: p, State: FileRegular, Content: []byte(c)}
	}
	return s
}
func stateOf(t *testing.T, r Resolution, path string) EntryState {
	t.Helper()
	for _, c := range r.Candidates {
		if c.Path == path {
			return c.State
		}
	}
	t.Fatalf("missing %s", path)
	return ""
}

func TestCodexPrefersOverrideAtSameDirectory(t *testing.T) {
	r := NewCodex().Resolve(codexSnapshot(map[string]string{"AGENTS.md": "base", "AGENTS.override.md": "override"}))
	if stateOf(t, r, "AGENTS.override.md") != Effective || stateOf(t, r, "AGENTS.md") != Shadowed {
		t.Fatal(r)
	}
	if r.Route.Path != "AGENTS.override.md" || r.Route.Mode != core.ManagedBlock {
		t.Fatal(r.Route)
	}
}
func TestCodexUsesAgentsAndCreatesWhenMissing(t *testing.T) {
	r := NewCodex().Resolve(codexSnapshot(map[string]string{"AGENTS.md": "base"}))
	if stateOf(t, r, "AGENTS.md") != Effective || r.Route.Path != "AGENTS.md" {
		t.Fatal(r)
	}
	empty := NewCodex().Resolve(codexSnapshot(nil))
	if !empty.Route.Create || empty.Route.Path != "AGENTS.md" {
		t.Fatal(empty)
	}
}
func TestCodexSkipsEmptyOverrideAndReportsSize(t *testing.T) {
	r := NewCodex().Resolve(codexSnapshot(map[string]string{"AGENTS.override.md": "", "AGENTS.md": strings.Repeat("x", 32769)}))
	if stateOf(t, r, "AGENTS.md") != Effective {
		t.Fatal(r)
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == "CODEX_INSTRUCTION_LIMIT" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing limit finding")
	}
}
func TestCodexObservableFallbackAndUnknownConfiguration(t *testing.T) {
	s := codexSnapshot(nil)
	s.Files["TEAM.md"] = FileFact{Path: "TEAM.md", State: FileRegular, Content: []byte("team")}
	s.Options["fallbackFilenames"] = "TEAM.md"
	r := NewCodex().Resolve(s)
	if r.Route.Path != "TEAM.md" {
		t.Fatal(r)
	}
	s.Options["fallbackFilenames"] = "?"
	r = NewCodex().Resolve(s)
	if r.Confidence != Conditional {
		t.Fatal(r.Confidence)
	}
}

func TestCodexReportsNestedCoLoadedEntries(t *testing.T) {
	s := codexSnapshot(map[string]string{"AGENTS.md": "root"})
	s.Files["services/AGENTS.md"] = FileFact{Path: "services/AGENTS.md", State: FileRegular, Content: []byte("service")}
	s.Options["workingDirectory"] = "services/api"
	r := NewCodex().Resolve(s)
	if stateOf(t, r, "services/AGENTS.md") != CoLoaded {
		t.Fatal(r)
	}
	if r.Route.Path != "AGENTS.md" {
		t.Fatal(r.Route)
	}
}
