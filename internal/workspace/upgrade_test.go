package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
)

func oldVersionWorkspace(t *testing.T) Root {
	t.Helper()
	root := openTempRoot(t)
	initializeFixture(t, root)
	manifest, _, err := readAdapterManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest.StateVersion = "1.0.0"
	raw, err := core.EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root.Path(), ".uawp", "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUpgradePlansAndAppliesV1_0ToV1_1(t *testing.T) {
	root := oldVersionWorkspace(t)
	at := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)
	p, report, err := PlanUpgradeAt(root, adapter.RuntimeFacts{}, at)
	if err != nil {
		t.Fatal(err)
	}
	if report.From != "1.0.0" || report.To != "1.1.0" || len(p.Changes()) == 0 {
		t.Fatalf("plan=%#v report=%#v", p, report)
	}
	changes := p.Changes()
	if changes[len(changes)-1].Path != ".uawp/manifest.json" {
		t.Fatalf("manifest is not last: %#v", changes)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Clock: func() time.Time { return at }}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyUpgrade(root, adapter.RuntimeFacts{}); err != nil {
		t.Fatal(err)
	}
	if matches, _ := filepath.Glob(filepath.Join(root.Path(), ".uawp", "migrations", "*.json")); len(matches) != 1 {
		t.Fatalf("migration receipts = %#v", matches)
	}
	noOp, noOpReport, err := PlanUpgradeAt(root, adapter.RuntimeFacts{}, at.Add(time.Minute))
	if err != nil || len(noOp.Changes()) != 0 || noOpReport.To != core.CurrentStateVersion {
		t.Fatalf("repeat upgrade plan=%#v report=%#v err=%v", noOp, noOpReport, err)
	}
}

func TestUpgradeRejectsUnsupportedActiveAndDriftedState(t *testing.T) {
	at := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)
	unsupported := oldVersionWorkspace(t)
	manifest, _, _ := readAdapterManifest(unsupported)
	manifest.StateVersion = "1.99.0"
	raw, _ := core.EncodeManifest(manifest)
	if err := os.WriteFile(filepath.Join(unsupported.Path(), ".uawp", "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanUpgradeAt(unsupported, adapter.RuntimeFacts{}, at); err == nil {
		t.Fatal("upgrade accepted unsupported minor")
	}

	active := oldVersionWorkspace(t)
	writeOwnership(t, active, "ACTIVE", "none")
	if _, _, err := PlanUpgradeAt(active, adapter.RuntimeFacts{}, at); err == nil {
		t.Fatal("upgrade accepted ACTIVE ownership")
	}

	drifted := oldVersionWorkspace(t)
	manifestPath := filepath.Join(drifted.Path(), ".uawp", "manifest.json")
	before, _ := os.ReadFile(manifestPath)
	p, _, err := PlanUpgradeAt(drifted, adapter.RuntimeFacts{}, at)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(before, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(drifted, p, ApplyOptions{ApprovedPlanID: p.ID}); err == nil {
		t.Fatal("upgrade applied after preview drift")
	}
}

func TestCurrentInitUsesV1_1Layout(t *testing.T) {
	root := openTempRoot(t)
	p, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	inventory, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Manifest.StateVersion != core.CurrentStateVersion {
		t.Fatalf("state version = %s", inventory.Manifest.StateVersion)
	}
	for _, directory := range []string{"migrations", "recovery"} {
		info, err := os.Stat(filepath.Join(root.Path(), ".uawp", directory))
		if err != nil || !info.IsDir() {
			t.Fatalf("missing %s: %v", directory, err)
		}
	}
}

func TestUpgradeReceiptCollisionStopsPlanning(t *testing.T) {
	root := oldVersionWorkspace(t)
	at := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)
	p, _, err := PlanUpgradeAt(root, adapter.RuntimeFacts{}, at)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := p.Metadata().MigrationReceiptPath
	if receiptPath == "" {
		t.Fatal("migration receipt path is empty")
	}
	target, err := root.ResolveUAWP(receiptPath[len(".uawp/"):])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("collision"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanUpgradeAt(root, adapter.RuntimeFacts{}, at); err == nil {
		t.Fatal("upgrade accepted receipt collision")
	}
}
