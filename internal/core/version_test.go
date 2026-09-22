package core

import "testing"

func TestClassifyStateVersion(t *testing.T) {
	tests := map[string]StateCompatibility{
		"1.2.0":  StateCurrent,
		"1.1.0":  StateUpgradeRequired,
		"1.0.0":  StateUpgradeRequired,
		"1.0.1":  StateUnsupported,
		"1.99.0": StateUnsupported,
		"2.0.0":  StateFutureMajor,
		"0.9.0":  StateUnsupported,
		"broken": StateUnsupported,
	}
	for version, want := range tests {
		t.Run(version, func(t *testing.T) {
			if got := ClassifyStateVersion(version); got != want {
				t.Fatalf("ClassifyStateVersion(%q) = %q, want %q", version, got, want)
			}
		})
	}
}
