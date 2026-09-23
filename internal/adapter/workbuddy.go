package adapter

import "github.com/vibemaker-community/uawp/internal/core"

type workBuddyAdapter struct{}

func NewWorkBuddy() Adapter         { return workBuddyAdapter{} }
func (workBuddyAdapter) ID() string { return "workbuddy" }
func (workBuddyAdapter) Evidence() Evidence {
	return Evidence{OfficialURLs: []string{"https://www.workbuddy.ai/docs/zh/ide/User-guide/Rules"}, VerifiedAt: "2026-09-22", Versions: "current documented WorkBuddy/CodeBuddy behavior"}
}
func (workBuddyAdapter) Resolve(s Snapshot) Resolution {
	r := Resolution{Provider: "workbuddy", Confidence: Verified, Health: "HEALTHY"}
	present := func(p string) bool { f, ok := s.Files[p]; return ok && f.State == FileRegular && len(f.Content) > 0 }
	code := present("CODEBUDDY.md")
	agents := present("AGENTS.md")
	codeState, agentsState := Missing, Missing
	switch {
	case code:
		codeState = Effective
		if agents {
			agentsState = Shadowed
		}
		r.Route = Route{Path: "CODEBUDDY.md", Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md"}
	case agents:
		agentsState = FallbackEffective
		r.Route = Route{Path: "AGENTS.md", Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md"}
	default:
		r.Route = Route{Path: "CODEBUDDY.md", Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md", Create: true}
	}
	r.Candidates = []Candidate{{Path: "CODEBUDDY.md", State: codeState}, {Path: "AGENTS.md", State: agentsState}}
	registered := s.Options["registeredPath"]
	if registered != "" && registered != r.Route.Path {
		r.Health = "ENTRY_DRIFT"
		r.Findings = append(r.Findings, Finding{Code: "ENTRY_DRIFT", Severity: "error", Message: "The documented effective WorkBuddy entry changed after integration.", NextAction: "Review migration to the current effective entry."})
	}
	return r.Canonical()
}
