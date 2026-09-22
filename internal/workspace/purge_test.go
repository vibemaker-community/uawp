package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/adapter"
)

func TestPurgeRequiresExternalExportAndRemovesNamespace(t *testing.T) {
	root := adapterRoot(t)
	addAdapter(t, root, "codex")
	destination := filepath.Join(t.TempDir(), "uawp-export.tar.gz")
	p, preview, err := PlanPurgeAt(root, adapter.RuntimeFacts{}, destination, time.Date(2026, 9, 22, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if preview.ExportPath != destination || len(p.Changes()) == 0 {
		t.Fatalf("plan=%#v report=%#v", p, preview)
	}
	report, err := ApplyPurge(root, p, PurgeOptions{ApplyOptions: ApplyOptions{ApprovedPlanID: p.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Verified || report.ArchiveSHA256 == "" {
		t.Fatalf("report=%#v", report)
	}
	if _, err := os.Lstat(filepath.Join(root.Path(), ".uawp")); !os.IsNotExist(err) {
		t.Fatalf("namespace remains: %v", err)
	}
	if info, err := os.Stat(destination); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("export missing: %v", err)
	}
}

func TestPurgeRejectsUnsafeDestinationAndActiveOwnership(t *testing.T) {
	root := adapterRoot(t)
	inside := filepath.Join(root.Path(), ".uawp", "bad.tar.gz")
	if _, _, err := PlanPurgeAt(root, adapter.RuntimeFacts{}, inside, time.Now()); err == nil {
		t.Fatal("purge accepted internal export")
	}
	writeOwnership(t, root, "ACTIVE", "none")
	if _, _, err := PlanPurgeAt(root, adapter.RuntimeFacts{}, filepath.Join(t.TempDir(), "safe.tar.gz"), time.Now()); err == nil {
		t.Fatal("purge accepted ACTIVE ownership")
	}
}
