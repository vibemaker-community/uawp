package workspace

import (
	"errors"
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

func TestStatusDiscoversCommittedPurgeTombstone(t *testing.T) {
	root := adapterRoot(t)
	tombstone := filepath.Join(root.Path(), ".uawp-purge-0123456789abcdef")
	if err := os.Rename(filepath.Join(root.Path(), ".uawp"), tombstone); err != nil {
		t.Fatal(err)
	}
	if report := Status(root); report.Code != CodeRecoveryRequired {
		t.Fatalf("status=%#v", report)
	}
}

func TestCommittedPurgeTombstoneCanBeVerifiedAndContinued(t *testing.T) {
	root := adapterRoot(t)
	destination := filepath.Join(t.TempDir(), "state.tar.gz")
	p, _, err := PlanPurgeAt(root, adapter.RuntimeFacts{}, destination, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ApplyPurge(root, p, PurgeOptions{ApplyOptions: ApplyOptions{ApprovedPlanID: p.ID}, Failpoint: func(stage string) error { return errors.New("crash") }})
	if err == nil || !HasPurgeTombstone(root) {
		t.Fatalf("err=%v tombstone=%v", err, HasPurgeTombstone(root))
	}
	cleanup, err := PlanPurgeCleanupAt(root, "human-1", "verified archive", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyPurgeCleanup(root, cleanup, cleanup.ID); err != nil {
		t.Fatal(err)
	}
	if HasPurgeTombstone(root) {
		t.Fatal("tombstone remains")
	}
}
