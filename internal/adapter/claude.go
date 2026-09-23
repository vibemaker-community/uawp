package adapter

import (
	"strconv"
	"strings"

	"github.com/vibemaker-community/uawp/internal/core"
)

type claudeCodeAdapter struct{}

func NewClaudeCode() Adapter         { return claudeCodeAdapter{} }
func (claudeCodeAdapter) ID() string { return "claude-code" }
func (claudeCodeAdapter) Evidence() Evidence {
	return Evidence{OfficialURLs: []string{"https://code.claude.com/docs/en/memory#agents-md", "https://code.claude.com/docs/en/memory#import-additional-files"}, VerifiedAt: "2026-09-22", Versions: ">=2.1.277 for direct AGENTS.md support"}
}

func (claudeCodeAdapter) Resolve(s Snapshot) Resolution {
	r := Resolution{Provider: "claude-code", Confidence: Verified, Health: "HEALTHY"}
	setting := s.Options["instructionFiles"]
	if setting == "" {
		setting = "claude-md-or-agents-md"
	}
	paths := []string{"CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md", "AGENTS.md"}
	present := map[string]bool{}
	for _, p := range paths {
		f, ok := s.Files[p]
		present[p] = ok && f.State == FileRegular && len(f.Content) > 0
		r.Candidates = append(r.Candidates, Candidate{Path: p, State: Missing})
	}
	setState := func(p string, state EntryState) {
		for i := range r.Candidates {
			if r.Candidates[i].Path == p {
				r.Candidates[i].State = state
			}
		}
	}
	if setting == "managed-only" {
		r.Confidence = Unsupported
		r.Health = "BLOCKED"
		r.Findings = append(r.Findings, Finding{Code: "CLAUDE_PROJECT_INSTRUCTIONS_DISABLED", Severity: "error", Message: "Claude Code is configured for managed-only instructions."})
		return r.Canonical()
	}
	claudeEntry := ""
	for _, p := range []string{"CLAUDE.md", ".claude/CLAUDE.md"} {
		if present[p] && claudeEntry == "" {
			claudeEntry = p
			setState(p, Effective)
		} else if present[p] {
			setState(p, CoLoaded)
		}
	}
	if present["CLAUDE.local.md"] {
		setState("CLAUDE.local.md", CoLoaded)
	}
	if setting == "claude-md-and-agents-md" && present["AGENTS.md"] {
		setState("AGENTS.md", CoLoaded)
	} else if setting == "claude-md" && present["AGENTS.md"] {
		setState("AGENTS.md", Unavailable)
	} else if claudeEntry != "" || present["CLAUDE.local.md"] {
		if present["AGENTS.md"] {
			setState("AGENTS.md", Shadowed)
		}
	}
	if claudeEntry != "" {
		r.Route = Route{Path: claudeEntry, Mode: core.Import, Target: ".uawp/INSTRUCTIONS.md"}
		return r.Canonical()
	}
	if present["CLAUDE.local.md"] {
		r.Route = Route{Path: "CLAUDE.md", Mode: core.Import, Target: ".uawp/INSTRUCTIONS.md", Create: true}
		return r.Canonical()
	}
	if present["AGENTS.md"] && setting != "claude-md" {
		direct, known := claudeDirectSupport(s)
		if direct {
			setState("AGENTS.md", FallbackEffective)
			r.Route = Route{Path: "AGENTS.md", Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md"}
			return r.Canonical()
		}
		setState("AGENTS.md", Unavailable)
		r.Route = Route{Path: "CLAUDE.md", Mode: core.Import, Target: ".uawp/INSTRUCTIONS.md", Create: true}
		r.Findings = append(r.Findings, Finding{Code: "CLAUDE_CREATION_CHANGES_SELECTION", Severity: "warning", Message: "Creating CLAUDE.md changes default selection; preserve existing AGENTS.md with an explicit @AGENTS.md import."})
		if !known {
			r.Confidence = Conditional
			r.Health = "CONDITIONAL"
		} else {
			r.Findings = append(r.Findings, Finding{Code: "CLAUDE_AGENTS_UNAVAILABLE", Severity: "warning", Message: "Direct AGENTS.md loading is unavailable in the observed Claude Code environment."})
		}
		return r.Canonical()
	}
	r.Route = Route{Path: "CLAUDE.md", Mode: core.Import, Target: ".uawp/INSTRUCTIONS.md", Create: true}
	return r.Canonical()
}

func claudeDirectSupport(s Snapshot) (bool, bool) {
	switch s.Options["directAgentsSupport"] {
	case "true":
		return true, true
	case "false":
		return false, true
	case "unknown":
		return false, false
	}
	if s.Options["providerEnvironment"] != "" && s.Options["providerEnvironment"] != "anthropic" {
		return false, true
	}
	if s.ProviderVersion == "" {
		return false, false
	}
	parts := strings.Split(s.ProviderVersion, ".")
	if len(parts) != 3 {
		return false, false
	}
	numbers := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return false, false
		}
		numbers[i] = n
	}
	if numbers[0] > 2 || (numbers[0] == 2 && (numbers[1] > 1 || (numbers[1] == 1 && numbers[2] >= 277))) {
		return true, true
	}
	return false, true
}
