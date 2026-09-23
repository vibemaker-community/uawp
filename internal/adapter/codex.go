package adapter

import (
	"path"
	"strings"

	"github.com/vibemaker-community/uawp/internal/core"
)

type codexAdapter struct{}

func NewCodex() Adapter         { return codexAdapter{} }
func (codexAdapter) ID() string { return "codex" }
func (codexAdapter) Evidence() Evidence {
	return Evidence{OfficialURLs: []string{"https://learn.chatgpt.com/docs/agent-configuration/agents-md"}, VerifiedAt: "2026-09-22", Versions: "current documented behavior"}
}
func (codexAdapter) Resolve(s Snapshot) Resolution {
	r := Resolution{Provider: "codex", Confidence: Verified, Health: "HEALTHY"}
	names := []string{"AGENTS.override.md", "AGENTS.md"}
	fallback := s.Options["fallbackFilenames"]
	if fallback == "?" {
		r.Confidence = Conditional
		r.Findings = append(r.Findings, Finding{Code: "CODEX_CONFIG_UNKNOWN", Severity: "warning", Message: "Fallback filename configuration is not observable."})
	} else if fallback != "" {
		for _, n := range strings.Split(fallback, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				names = append(names, n)
			}
		}
	}
	effective := ""
	total := 0
	for _, name := range names {
		fact, ok := s.Files[name]
		if !ok {
			fact = FileFact{Path: name, State: FileMissing}
		}
		state := Missing
		reason := "not present"
		if fact.State == FileRegular && len(fact.Content) > 0 {
			total += len(fact.Content)
			if effective == "" {
				state = Effective
				effective = name
				reason = "first non-empty entry by documented precedence"
			} else {
				state = Shadowed
				reason = "higher-priority entry is effective"
			}
		} else if fact.State != FileMissing && fact.State != FileRegular {
			state = Unavailable
			reason = "entry is unsafe or unreadable"
		}
		r.Candidates = append(r.Candidates, Candidate{Path: name, State: state, Reason: reason})
	}
	if effective == "" {
		effective = "AGENTS.md"
		r.Route = Route{Path: effective, Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md", Create: true}
	} else {
		r.Route = Route{Path: effective, Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md"}
	}
	working := strings.Trim(s.Options["workingDirectory"], "/")
	if working != "" && working != "." {
		parts := strings.Split(working, "/")
		dir := ""
		for _, part := range parts {
			dir = path.Join(dir, part)
			chosen := ""
			for _, base := range []string{"AGENTS.override.md", "AGENTS.md"} {
				name := path.Join(dir, base)
				fact, ok := s.Files[name]
				if !ok {
					continue
				}
				state := Missing
				if fact.State == FileRegular && len(fact.Content) > 0 && chosen == "" {
					state = CoLoaded
					chosen = name
					total += len(fact.Content)
				} else if fact.State == FileRegular && len(fact.Content) > 0 {
					state = Shadowed
				} else if fact.State != FileMissing && fact.State != FileRegular {
					state = Unavailable
				}
				r.Candidates = append(r.Candidates, Candidate{Path: name, State: state})
			}
		}
	}
	if total > 32*1024 {
		r.Findings = append(r.Findings, Finding{Code: "CODEX_INSTRUCTION_LIMIT", Severity: "warning", Message: "Discovered instructions exceed the documented default 32 KiB aggregate limit."})
		r.Health = "DEGRADED"
	}
	return r.Canonical()
}
