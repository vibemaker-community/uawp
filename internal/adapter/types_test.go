package adapter

import "testing"

func TestResolutionVocabulary(t *testing.T) {
	states := []EntryState{Effective, CoLoaded, FallbackEffective, Shadowed, Unavailable, Unknown, Missing}
	seen := map[EntryState]bool{}
	for _, state := range states {
		if state == "" || seen[state] {
			t.Fatalf("bad state %q", state)
		}
		seen[state] = true
	}
	for _, confidence := range []Confidence{Verified, Conditional, Unsupported} {
		if confidence == "" {
			t.Fatal("empty confidence")
		}
	}
}

func TestResolutionSortsCandidatesAndFindings(t *testing.T) {
	r := Resolution{Candidates: []Candidate{{Path: "z"}, {Path: "a"}}, Findings: []Finding{{Code: "Z"}, {Code: "A"}}}.Canonical()
	if r.Candidates[0].Path != "a" || r.Findings[0].Code != "A" {
		t.Fatalf("not canonical: %#v", r)
	}
}
