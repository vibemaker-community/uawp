package adapter

import "fmt"

func Lookup(id string) (Adapter, error) {
	switch id {
	case "codex":
		return NewCodex(), nil
	case "claude-code":
		return NewClaudeCode(), nil
	case "workbuddy":
		return NewWorkBuddy(), nil
	default:
		return nil, fmt.Errorf("unsupported adapter %q", id)
	}
}
func CandidatePaths(id string, facts RuntimeFacts) []string {
	switch id {
	case "codex":
		return []string{"AGENTS.override.md", "AGENTS.md"}
	case "claude-code":
		return []string{"CLAUDE.md", ".claude/CLAUDE.md", "CLAUDE.local.md", "AGENTS.md"}
	case "workbuddy":
		return []string{"CODEBUDDY.md", "AGENTS.md", ".codebuddy/rules/uawp/RULE.mdc"}
	default:
		return nil
	}
}
func ArtifactID(path string) string {
	switch path {
	case "AGENTS.md", "AGENTS.override.md":
		return "uawp-entry-agents-v1"
	case "CLAUDE.md", ".claude/CLAUDE.md":
		return "uawp-entry-claude-v1"
	case "CODEBUDDY.md":
		return "uawp-entry-codebuddy-v1"
	default:
		return "uawp-entry-native-v1"
	}
}
func BridgeBody() string { return "Read `.uawp/INSTRUCTIONS.md` before UAWP work." }
