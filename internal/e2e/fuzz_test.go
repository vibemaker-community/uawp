package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vibemaker-community/uawp/internal/adapter"
	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/workspace"
)

func FuzzDecodeManifest(f *testing.F) {
	for _, seed := range []string{
		`{"protocol":"UAWP","stateVersion":"1.0.0"}`,
		`{"protocol":"UAWP","stateVersion":"1.0.0"} {}`,
		`{"protocol":"UAWP","protocol":"other","stateVersion":"1.0.0"}`,
		"",
		"{",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = core.DecodeManifest(strings.NewReader(input))
	})
}

func FuzzManagedBlockEditing(f *testing.F) {
	for _, seed := range []string{"", "# existing\n", "```\n<!-- UAWP:BEGIN -->\n```", "说明\r\n", string([]byte{0, 1})} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		spec := adapter.BlockSpec{ArtifactID: "uawp-entry-agents-v1", Target: ".uawp/INSTRUCTIONS.md", Consumers: []string{"codex"}, Body: "Read `.uawp/INSTRUCTIONS.md` before UAWP work."}
		after, _, err := adapter.UpsertManagedBlock([]byte(input), spec)
		if err != nil {
			return
		}
		_, _ = adapter.RemoveManagedBlock(after, spec)
	})
}

func FuzzResolveUAWP(f *testing.F) {
	for _, seed := range []string{
		"manifest.json", "../escape", "checkpoints/CP-001.md", "a\\b", "NUL", "说明.md", "x\x00y", strings.Repeat("a", 4096),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, relative string) {
		root, err := workspace.OpenRoot(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := root.ResolveUAWP(relative)
		if err != nil {
			return
		}
		prefix := filepath.Join(root.Path(), ".uawp") + string(filepath.Separator)
		if resolved != filepath.Join(root.Path(), ".uawp") && !strings.HasPrefix(resolved, prefix) {
			t.Fatalf("accepted path escapes workspace: relative=%q resolved=%q", relative, resolved)
		}
	})
}
