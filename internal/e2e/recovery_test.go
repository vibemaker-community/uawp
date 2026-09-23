package e2e

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/vibemaker-community/uawp/internal/cli"
)

func TestStaleRecoveryRequiresExactOneTimeApproval(t *testing.T) {
	dir := t.TempDir()
	init := runInit(t, dir, "")
	runInit(t, dir, init.PlanID)
	previewApply(t, []string{"acquire", "--workspace", dir, "--worker-id", "worker-a", "--session-id", "session-a", "--agent", "Agent A", "--purpose", "work", "--format", "json"})
	args := []string{"recover", "--workspace", dir, "--controller-id", "human-1", "--reason", "confirmed crash", "--format", "json"}
	preview := lifecycleCommand(t, args, 5)
	token := preview["planID"].(string)
	lifecycleCommand(t, append(args, "--approve", token), 0)
	var out, err bytes.Buffer
	if code := cli.Run(append(args, "--approve", token), &out, &err); code == 0 {
		t.Fatal("replayed recovery")
	}
	previewApply(t, []string{"acquire", "--workspace", dir, "--worker-id", "worker-b", "--session-id", "session-b", "--agent", "Agent B", "--purpose", "continue", "--format", "json"})
	if _, err := filepath.Glob(filepath.Join(dir, ".uawp", "checkpoints", "*.md")); err != nil {
		t.Fatal(err)
	}
}
