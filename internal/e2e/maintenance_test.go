package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/uawp/uawp/internal/cli"
	"github.com/uawp/uawp/internal/core"
)

func TestMaintenanceUpgradeRepairDetachAndPurge(t *testing.T) {
	dir := initializedAdapterWorkspace(t, t.TempDir())
	manifest, _ := core.EncodeManifest(core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.0.0"})
	if err := os.WriteFile(filepath.Join(dir, ".uawp", "manifest.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	previewApplyMaintenance(t, "upgrade", "--workspace", dir, "--format", "json")
	if err := os.Remove(filepath.Join(dir, ".uawp", "INSTRUCTIONS.md")); err != nil {
		t.Fatal(err)
	}
	previewApplyMaintenance(t, "repair", "--workspace", dir, "--format", "json")
	applyAdapterCLI(t, dir, "codex")
	previewApplyMaintenance(t, "uninstall", "--workspace", dir, "--format", "json")
	if _, err := os.Lstat(filepath.Join(dir, ".uawp")); err != nil {
		t.Fatalf("detach removed state: %v", err)
	}
	export := filepath.Join(t.TempDir(), "workspace.tar.gz")
	previewApplyMaintenance(t, "uninstall", "--purge", "--export", export, "--workspace", dir, "--format", "json")
	if _, err := os.Lstat(filepath.Join(dir, ".uawp")); !os.IsNotExist(err) {
		t.Fatalf("purge retained namespace: %v", err)
	}
	if archive, err := os.ReadFile(export); err != nil || len(archive) == 0 {
		t.Fatalf("export missing: %v", err)
	}
}

func previewApplyMaintenance(t *testing.T, args ...string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := cli.Run(args, &stdout, &stderr); code != 5 {
		t.Fatalf("preview %q code=%d out=%s err=%s", args, code, stdout.String(), stderr.String())
	}
	var output struct {
		PlanID string `json:"planID"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	apply := append(append([]string(nil), args...), "--approve", output.PlanID)
	stdout.Reset()
	stderr.Reset()
	if code := cli.Run(apply, &stdout, &stderr); code != 0 {
		t.Fatalf("apply %q code=%d out=%s err=%s", args, code, stdout.String(), stderr.String())
	}
}
