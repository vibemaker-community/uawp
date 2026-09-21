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
