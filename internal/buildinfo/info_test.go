package buildinfo

import "testing"

func TestCurrentUsesDeterministicDevelopmentDefaults(t *testing.T) {
	withValues(t, "dev", "unknown", "", func() {
		got := Current()
		want := Info{Version: "dev", Commit: "unknown", BuiltAt: ""}
		if got != want {
			t.Fatalf("Current() = %#v, want %#v", got, want)
		}
	})
}

func TestCurrentNormalizesReleaseVersion(t *testing.T) {
	withValues(t, "v1.2.3", "0123456789abcdef", "2026-09-23T08:00:00Z", func() {
		got := Current()
		want := Info{Version: "1.2.3", Commit: "0123456789abcdef", BuiltAt: "2026-09-23T08:00:00Z"}
		if got != want {
			t.Fatalf("Current() = %#v, want %#v", got, want)
		}
	})
}

func withValues(t *testing.T, version, commit, builtAt string, test func()) {
	t.Helper()
	oldVersion, oldCommit, oldBuiltAt := Version, Commit, BuiltAt
	Version, Commit, BuiltAt = version, commit, builtAt
	t.Cleanup(func() { Version, Commit, BuiltAt = oldVersion, oldCommit, oldBuiltAt })
	test()
}
