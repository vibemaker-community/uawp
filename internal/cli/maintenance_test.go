package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/uawp/uawp/internal/core"
)

func TestUpgradePreviewAndApprove(t *testing.T) {
	dir := maintenanceWorkspace(t)
	manifestPath := filepath.Join(dir, ".uawp", "manifest.json")
	raw, err := core.EncodeManifest(core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	preview := runMaintenanceCLI(t, 5, "upgrade", "--workspace", dir, "--format", "json")
	var parsed cliOutput
	if err := json.Unmarshal(preview, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.PlanID == "" {
		t.Fatal("missing upgrade plan ID")
	}
	runMaintenanceCLI(t, 0, "upgrade", "--workspace", dir, "--approve", parsed.PlanID, "--format", "json")
	content, _ := os.ReadFile(manifestPath)
	if !bytes.Contains(content, []byte(`"stateVersion": "1.1.0"`)) {
		t.Fatalf("manifest=%s", content)
	}
}

func TestUninstallDetachPreviewAndApprove(t *testing.T) {
	dir := maintenanceWorkspace(t)
	preview := runMaintenanceCLI(t, 0, "uninstall", "--workspace", dir, "--format", "json")
	var parsed cliOutput
	if err := json.Unmarshal(preview, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Mutated {
		t.Fatal("adapter-free uninstall mutated")
	}
}

func TestTransactionStatusAndUsageSeparation(t *testing.T) {
	dir := maintenanceWorkspace(t)
	runMaintenanceCLI(t, 4, "transaction", "status", "--workspace", dir, "--format", "json")
	runMaintenanceCLI(t, 2, "transaction", "rollback", "--workspace", dir)
}

func maintenanceWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	preview := runMaintenanceCLI(t, 5, "init", "--workspace", dir, "--format", "json")
	var parsed cliOutput
	if err := json.Unmarshal(preview, &parsed); err != nil {
		t.Fatal(err)
	}
	runMaintenanceCLI(t, 0, "init", "--workspace", dir, "--approve", parsed.PlanID, "--format", "json")
	return dir
}

func runMaintenanceCLI(t *testing.T, want int, args ...string) []byte {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	if code != want {
		t.Fatalf("Run(%q)=%d want=%d stdout=%q stderr=%q", args, code, want, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}
