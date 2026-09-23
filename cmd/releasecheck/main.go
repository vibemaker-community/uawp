package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	tagPattern    = regexp.MustCompile(`^v([0-9]+\.[0-9]+\.[0-9]+(?:-rc\.[1-9][0-9]*)?)$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

type request struct {
	Mode              string
	Tag               string
	Version           string
	Commit            string
	Changelog         string
	ExistingTagCommit string
	EvidenceDir       string
}

type evidence struct {
	Tag    string `json:"tag"`
	Commit string `json:"commit"`
	Target string `json:"target,omitempty"`
	Status string `json:"status"`
}

func main() {
	mode := flag.String("mode", "prepare", "validation mode: prepare, tagged, or publish")
	tag := flag.String("tag", "", "immutable release tag")
	version := flag.String("version", "", "build version without leading v")
	commit := flag.String("commit", "", "full release commit SHA")
	changelogPath := flag.String("changelog", "CHANGELOG.md", "changelog path")
	evidenceDir := flag.String("evidence-dir", "", "native release evidence directory")
	flag.Parse()
	if flag.NArg() != 0 {
		exitError(errors.New("unexpected positional arguments"))
	}
	changelog, err := os.ReadFile(*changelogPath)
	if err != nil {
		exitError(fmt.Errorf("read changelog: %w", err))
	}
	existing, err := resolveTag(*tag)
	if err != nil {
		exitError(err)
	}
	err = validate(request{
		Mode:              *mode,
		Tag:               *tag,
		Version:           *version,
		Commit:            *commit,
		Changelog:         string(changelog),
		ExistingTagCommit: existing,
		EvidenceDir:       *evidenceDir,
	})
	if err != nil {
		exitError(err)
	}
	fmt.Printf("release state valid for %s at %s (%s mode)\n", *tag, *commit, *mode)
}

func validate(req request) error {
	match := tagPattern.FindStringSubmatch(req.Tag)
	if match == nil {
		return fmt.Errorf("tag %q is not vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-rc.N", req.Tag)
	}
	if req.Version != match[1] {
		return fmt.Errorf("build version %q does not match tag %q", req.Version, req.Tag)
	}
	if !commitPattern.MatchString(req.Commit) {
		return fmt.Errorf("commit must be a full lowercase 40-character SHA")
	}
	if !strings.Contains(req.Version, "-rc.") && !finalChangelogHeading(req.Changelog, req.Version) {
		return fmt.Errorf("changelog still identifies %s as a candidate or unreleased", req.Tag)
	}
	switch req.Mode {
	case "prepare":
		if req.ExistingTagCommit != "" {
			return fmt.Errorf("tag %s already exists; never move or reuse a release tag", req.Tag)
		}
	case "tagged", "publish":
		if req.ExistingTagCommit == "" {
			return fmt.Errorf("tag %s does not exist", req.Tag)
		}
		if req.ExistingTagCommit != req.Commit {
			return fmt.Errorf("tag %s does not point to release commit %s", req.Tag, req.Commit)
		}
		if req.Mode == "publish" {
			if err := verifyEvidence(req); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown mode %q", req.Mode)
	}
	return nil
}

func finalChangelogHeading(changelog, version string) bool {
	plain := "## " + version
	linked := "## [" + version + "]"
	for _, line := range strings.Split(changelog, "\n") {
		line = strings.TrimSpace(line)
		if line == plain || strings.HasPrefix(line, plain+" ") || line == linked || strings.HasPrefix(line, linked+" ") {
			return true
		}
	}
	return false
}

func verifyEvidence(req request) error {
	if req.EvidenceDir == "" {
		return errors.New("native smoke evidence directory is required")
	}
	for _, target := range []string{"darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64"} {
		filename := filepath.Join(req.EvidenceDir, "smoke-"+target+".json")
		if err := verifyEvidenceFile(filename, req.Tag, req.Commit, target); err != nil {
			return fmt.Errorf("native smoke evidence for %s: %w", target, err)
		}
	}
	if err := verifyEvidenceFile(filepath.Join(req.EvidenceDir, "checksums-verified.json"), req.Tag, req.Commit, ""); err != nil {
		return fmt.Errorf("checksum verification evidence: %w", err)
	}
	return nil
}

func verifyEvidenceFile(filename, tag, commit, target string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	var got evidence
	if err := json.Unmarshal(data, &got); err != nil {
		return err
	}
	if got.Tag != tag || got.Commit != commit || got.Status != "passed" || got.Target != target {
		return fmt.Errorf("evidence does not match tag, commit, target, and passed status")
	}
	return nil
}

func resolveTag(tag string) (string, error) {
	if tag == "" {
		return "", nil
	}
	command := exec.Command("git", "rev-list", "-n", "1", "refs/tags/"+tag)
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", nil
		}
		return "", fmt.Errorf("inspect tag %s: %w", tag, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func exitError(err error) {
	fmt.Fprintln(os.Stderr, "release check failed:", err)
	os.Exit(1)
}
