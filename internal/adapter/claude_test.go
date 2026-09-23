package adapter

import (
	"github.com/vibemaker-community/uawp/internal/core"
	"testing"
)

func claudeSnapshot(names ...string) Snapshot {
	s := Snapshot{Files: map[string]FileFact{}, Options: map[string]string{"instructionFiles": "claude-md-or-agents-md"}, ProviderVersion: "2.1.277"}
	for _, p := range []string{"CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md", "AGENTS.md"} {
		s.Files[p] = FileFact{Path: p, State: FileMissing}
	}
	for _, p := range names {
		s.Files[p] = FileFact{Path: p, State: FileRegular, Content: []byte("rules")}
	}
	return s
}
func hasFinding(r Resolution, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestClaudeDefaultShadowsAgents(t *testing.T) {
	r := NewClaudeCode().Resolve(claudeSnapshot("CLAUDE.md", "AGENTS.md"))
	if stateOf(t, r, "CLAUDE.md") != Effective || stateOf(t, r, "AGENTS.md") != Shadowed {
		t.Fatal(r)
	}
	if r.Route.Path != "CLAUDE.md" || r.Route.Mode != core.Import {
		t.Fatal(r.Route)
	}
}
func TestClaudeDirectAgentsFallbackWhenVerified(t *testing.T) {
	r := NewClaudeCode().Resolve(claudeSnapshot("AGENTS.md"))
	if stateOf(t, r, "AGENTS.md") != FallbackEffective || r.Route.Path != "AGENTS.md" || r.Route.Mode != core.ManagedBlock {
		t.Fatal(r)
	}
}
func TestClaudeUnverifiedAgentsProposesPreservingImport(t *testing.T) {
	s := claudeSnapshot("AGENTS.md")
	s.ProviderVersion = "2.1.100"
	r := NewClaudeCode().Resolve(s)
	if r.Route.Path != "CLAUDE.md" || !r.Route.Create || r.Route.Mode != core.Import || !hasFinding(r, "CLAUDE_CREATION_CHANGES_SELECTION") {
		t.Fatal(r)
	}
}
func TestClaudeInstructionSelectionSettings(t *testing.T) {
	both := claudeSnapshot("CLAUDE.md", "AGENTS.md")
	both.Options["instructionFiles"] = "claude-md-and-agents-md"
	r := NewClaudeCode().Resolve(both)
	if stateOf(t, r, "AGENTS.md") != CoLoaded {
		t.Fatal(r)
	}
	managed := claudeSnapshot("CLAUDE.md")
	managed.Options["instructionFiles"] = "managed-only"
	r = NewClaudeCode().Resolve(managed)
	if r.Confidence != Unsupported {
		t.Fatal(r)
	}
}
func TestClaudeLocalAndDotClaudeSelection(t *testing.T) {
	r := NewClaudeCode().Resolve(claudeSnapshot(".claude/CLAUDE.md", "CLAUDE.local.md", "AGENTS.md"))
	if r.Route.Path != ".claude/CLAUDE.md" || stateOf(t, r, "CLAUDE.local.md") != CoLoaded || stateOf(t, r, "AGENTS.md") != Shadowed {
		t.Fatal(r)
	}
}
func TestClaudeUnavailableEnvironmentFailsClosed(t *testing.T) {
	s := claudeSnapshot("AGENTS.md")
	s.Options["directAgentsSupport"] = "false"
	s.Options["providerEnvironment"] = "third-party"
	r := NewClaudeCode().Resolve(s)
	if r.Route.Path != "CLAUDE.md" || r.Confidence != Verified || !hasFinding(r, "CLAUDE_AGENTS_UNAVAILABLE") {
		t.Fatal(r)
	}
}
func TestClaudeUnknownVersionIsConditional(t *testing.T) {
	s := claudeSnapshot("AGENTS.md")
	s.ProviderVersion = ""
	s.Options["directAgentsSupport"] = "unknown"
	r := NewClaudeCode().Resolve(s)
	if r.Confidence != Conditional {
		t.Fatal(r)
	}
}
