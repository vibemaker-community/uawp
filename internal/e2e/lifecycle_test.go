package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/uawp/uawp/internal/cli"
)

func lifecycleCommand(t *testing.T, args []string, want int) map[string]any {
	t.Helper()
	var out, err bytes.Buffer
	if code := cli.Run(args, &out, &err); code != want {
		t.Fatalf("%s code=%d stderr=%q", args[0], code, err.String())
	}
	var result map[string]any
	if len(out.Bytes()) > 0 {
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatal(e)
		}
	}
	return result
}
func previewApply(t *testing.T, args []string) {
	t.Helper()
	result := lifecycleCommand(t, args, 5)
	token := result["planID"].(string)
	lifecycleCommand(t, append(args, "--approve", token), 0)
}

func TestLifecycleEndToEnd(t *testing.T) {
	dir := copyBrownfield(t)
	fixture := loadFixtureManifest(t)
	init := runInit(t, dir, "")
	runInit(t, dir, init.PlanID)
	previewApply(t, []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--agent", "Agent A", "--purpose", "phase two", "--format", "json"})
	lifecycleCommand(t, []string{"resume", "--workspace", dir, "--worker-id", "worker-a", "--format", "json"}, 0)
	lifecycleCommand(t, []string{"resume", "--workspace", dir, "--worker-id", "worker-b", "--format", "json"}, 0)
	context := filepath.Join(t.TempDir(), "context.md")
	os.WriteFile(context, []byte("# Current\n\nReady.\n"), 0o600)
	previewApply(t, []string{"sync", "--workspace", dir, "--worker-id", "worker-a", "--context-file", context, "--format", "json"})
	previewApply(t, []string{"checkpoint", "--workspace", dir, "--worker-id", "worker-a", "--milestone-id", "phase-2", "--label", "Phase 2", "--format", "json"})
	previewApply(t, []string{"handoff", "--workspace", dir, "--worker-id", "worker-a", "--purpose", "handoff", "--context-file", context, "--format", "json"})
	previewApply(t, []string{"acquire", "--workspace", dir, "--worker-id", "worker-b", "--agent", "Agent B", "--purpose", "continue", "--format", "json"})
	matches, _ := filepath.Glob(filepath.Join(dir, ".uawp", "checkpoints", "*.md"))
	if len(matches) != 1 {
		t.Fatalf("checkpoints=%v", matches)
	}
	assertFixture(t, dir, fixture)
}
