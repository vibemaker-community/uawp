package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestValidateAcceptsCandidateAndFinalTags(t *testing.T) {
	for _, test := range []struct {
		name      string
		tag       string
		version   string
		changelog string
	}{
		{"candidate", "v1.0.0-rc.1", "1.0.0-rc.1", "# Changelog\n\n## Unreleased\n"},
		{"final", "v1.0.0", "1.0.0", "# Changelog\n\n## 1.0.0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validate(request{Mode: "prepare", Tag: test.tag, Version: test.version, Commit: testCommit, Changelog: test.changelog})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateRejectsInvalidPrepareState(t *testing.T) {
	for _, test := range []struct {
		name string
		req  request
		want string
	}{
		{"malformed tag", request{Mode: "prepare", Tag: "release-1", Version: "1.0.0", Commit: testCommit, Changelog: "## 1.0.0"}, "tag"},
		{"version mismatch", request{Mode: "prepare", Tag: "v1.0.0-rc.1", Version: "1.0.0", Commit: testCommit, Changelog: "## Unreleased"}, "version"},
		{"candidate changelog", request{Mode: "prepare", Tag: "v1.0.0", Version: "1.0.0", Commit: testCommit, Changelog: "## 1.0.0-rc.1"}, "changelog"},
		{"pre-existing tag", request{Mode: "prepare", Tag: "v1.0.0-rc.1", Version: "1.0.0-rc.1", Commit: testCommit, Changelog: "## Unreleased", ExistingTagCommit: testCommit}, "already exists"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validate(test.req)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestValidatePublishRejectsMovedTagAndMissingEvidence(t *testing.T) {
	base := request{Mode: "publish", Tag: "v1.0.0-rc.1", Version: "1.0.0-rc.1", Commit: testCommit, Changelog: "## Unreleased"}

	moved := base
	moved.ExistingTagCommit = "1123456789abcdef0123456789abcdef01234567"
	moved.EvidenceDir = completeEvidence(t, base.Tag, base.Commit)
	if err := validate(moved); err == nil || !strings.Contains(err.Error(), "does not point") {
		t.Fatalf("moved tag error = %v", err)
	}

	missingSmoke := base
	missingSmoke.ExistingTagCommit = base.Commit
	missingSmoke.EvidenceDir = t.TempDir()
	os.WriteFile(filepath.Join(missingSmoke.EvidenceDir, "checksums-verified.json"), []byte("{}\n"), 0o600)
	if err := validate(missingSmoke); err == nil || !strings.Contains(err.Error(), "native smoke") {
		t.Fatalf("missing smoke error = %v", err)
	}

	missingChecksum := base
	missingChecksum.ExistingTagCommit = base.Commit
	missingChecksum.EvidenceDir = completeEvidence(t, base.Tag, base.Commit)
	os.Remove(filepath.Join(missingChecksum.EvidenceDir, "checksums-verified.json"))
	if err := validate(missingChecksum); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("missing checksum error = %v", err)
	}
}

func TestValidatePublishAcceptsCompleteEvidence(t *testing.T) {
	req := request{Mode: "publish", Tag: "v1.0.0-rc.1", Version: "1.0.0-rc.1", Commit: testCommit, ExistingTagCommit: testCommit, Changelog: "## Unreleased"}
	req.EvidenceDir = completeEvidence(t, req.Tag, req.Commit)
	if err := validate(req); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTaggedModeRequiresTagAtReleaseCommitWithoutEvidence(t *testing.T) {
	req := request{Mode: "tagged", Tag: "v1.0.0-rc.1", Version: "1.0.0-rc.1", Commit: testCommit, ExistingTagCommit: testCommit, Changelog: "## Unreleased"}
	if err := validate(req); err != nil {
		t.Fatal(err)
	}
	req.ExistingTagCommit = "1123456789abcdef0123456789abcdef01234567"
	if err := validate(req); err == nil || !strings.Contains(err.Error(), "does not point") {
		t.Fatalf("moved tag error = %v", err)
	}
}

func completeEvidence(t *testing.T, tag, commit string) string {
	t.Helper()
	dir := t.TempDir()
	for _, target := range []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"} {
		data := []byte(`{"tag":"` + tag + `","commit":"` + commit + `","target":"` + target + `","status":"passed"}` + "\n")
		if err := os.WriteFile(filepath.Join(dir, "smoke-"+target+".json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "checksums-verified.json"), []byte(`{"tag":"`+tag+`","commit":"`+commit+`","status":"passed"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}
