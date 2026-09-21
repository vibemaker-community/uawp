package adapter

import "testing"

func TestLookupLaunchAdapter(t *testing.T) {
	for _, id := range []string{"codex", "claude-code", "workbuddy"} {
		a, err := Lookup(id)
		if err != nil || a.ID() != id {
			t.Fatalf("%s: %v", id, err)
		}
	}
	if _, err := Lookup("unknown"); err == nil {
		t.Fatal("accepted unknown adapter")
	}
}

func TestCandidatePathsIncludeObservableCodexFacts(t *testing.T) {
	facts := RuntimeFacts{Options: map[string]map[string]string{"codex": {"fallbackFilenames": "TEAM.md, AGENTS.md", "workingDirectory": "services/api"}}}
	got := CandidatePaths("codex", facts)
	for _, want := range []string{"TEAM.md", "services/AGENTS.md", "services/AGENTS.override.md", "services/api/AGENTS.md", "services/api/AGENTS.override.md"} {
		found := false
		for _, path := range got {
			found = found || path == want
		}
		if !found {
			t.Fatalf("missing %s in %#v", want, got)
		}
	}
}
