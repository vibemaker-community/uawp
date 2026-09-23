package cli

import (
	"bytes"
	"testing"

	"github.com/vibemaker-community/uawp/internal/buildinfo"
)

func TestVersionHumanOutput(t *testing.T) {
	setBuildInfo(t, "dev", "unknown", "")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)

	const want = "uawp dev\ncommit: unknown\nbuilt at: unknown\n"
	if code != exitOK || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestVersionJSONOutputIsStable(t *testing.T) {
	setBuildInfo(t, "v1.2.3", "0123456789abcdef", "2026-09-23T08:00:00Z")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"version", "--format", "json"}, &stdout, &stderr)

	const want = "{\"version\":\"1.2.3\",\"commit\":\"0123456789abcdef\",\"builtAt\":\"2026-09-23T08:00:00Z\"}\n"
	if code != exitOK || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestVersionRejectsUnsupportedFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"version", "--format", "xml"}, &stdout, &stderr)
	if code != exitUsage || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func setBuildInfo(t *testing.T, version, commit, builtAt string) {
	t.Helper()
	oldVersion, oldCommit, oldBuiltAt := buildinfo.Version, buildinfo.Commit, buildinfo.BuiltAt
	buildinfo.Version, buildinfo.Commit, buildinfo.BuiltAt = version, commit, builtAt
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.BuiltAt = oldVersion, oldCommit, oldBuiltAt
	})
}
