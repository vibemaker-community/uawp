package adapter

import (
	"github.com/vibemaker-community/uawp/internal/core"
	"testing"
)

func workBuddySnapshot(names ...string) Snapshot {
	s := Snapshot{Files: map[string]FileFact{}, Options: map[string]string{}}
	for _, p := range []string{"CODEBUDDY.md", "AGENTS.md", ".codebuddy/rules/uawp/RULE.mdc"} {
		s.Files[p] = FileFact{Path: p, State: FileMissing}
	}
	for _, p := range names {
		s.Files[p] = FileFact{Path: p, State: FileRegular, Content: []byte("rules")}
	}
	return s
}
func TestWorkBuddyCodeBuddyShadowsFallback(t *testing.T) {
	r := NewWorkBuddy().Resolve(workBuddySnapshot("AGENTS.md", "CODEBUDDY.md"))
	if stateOf(t, r, "CODEBUDDY.md") != Effective || stateOf(t, r, "AGENTS.md") != Shadowed || r.Route.Path != "CODEBUDDY.md" || r.Route.Mode != core.ManagedBlock {
		t.Fatal(r)
	}
}
func TestWorkBuddyUsesAgentsFallback(t *testing.T) {
	r := NewWorkBuddy().Resolve(workBuddySnapshot("AGENTS.md"))
	if stateOf(t, r, "AGENTS.md") != FallbackEffective || r.Route.Path != "AGENTS.md" {
		t.Fatal(r)
	}
}
func TestWorkBuddyCreatesNativeEntryWhenEmpty(t *testing.T) {
	r := NewWorkBuddy().Resolve(workBuddySnapshot())
	if r.Route.Path != "CODEBUDDY.md" || !r.Route.Create {
		t.Fatal(r)
	}
}
func TestWorkBuddyDetectsFallbackDrift(t *testing.T) {
	s := workBuddySnapshot("AGENTS.md", "CODEBUDDY.md")
	s.Options["registeredPath"] = "AGENTS.md"
	r := NewWorkBuddy().Resolve(s)
	if !hasFinding(r, "ENTRY_DRIFT") || r.Health != "ENTRY_DRIFT" || r.Route.Path != "CODEBUDDY.md" {
		t.Fatal(r)
	}
}
func TestWorkBuddyDoesNotClaimRulesImport(t *testing.T) {
	r := NewWorkBuddy().Resolve(workBuddySnapshot(".codebuddy/rules/uawp/RULE.mdc"))
	if r.Route.Path != "CODEBUDDY.md" || r.Route.Mode != core.ManagedBlock {
		t.Fatal(r)
	}
}
