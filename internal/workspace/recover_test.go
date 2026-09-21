package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
)

func TestPlanStaleRecoveryAuditsThenReleases(t *testing.T) {
	root, at := activeFixture(t)
	req := core.RecoveryRequest{ControllerID: "human-1", Reason: "confirmed crash", At: at.Add(time.Hour)}
	p, err := PlanStaleRecoveryAt(root, req)
	if err != nil {
		t.Fatal(err)
	}
	c := p.Changes()
	if len(c) != 2 || c[0].Path != ".uawp/DECISIONS.md" || c[1].Path != ".uawp/ACTIVE_WORKER.md" {
		t.Fatalf("changes=%#v", c)
	}
	if p.Metadata().ControllerID != "human-1" || p.Metadata().Reason != "confirmed crash" {
		t.Fatalf("meta=%#v", p.Metadata())
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	owner, _ := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if owner.Status != core.Released {
		t.Fatalf("owner=%#v", owner)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err == nil {
		t.Fatal("replayed recovery")
	}
	decisions, _ := os.ReadFile(filepath.Join(root.Path(), ".uawp", "DECISIONS.md"))
	if len(decisions) == 0 {
		t.Fatal("missing audit")
	}
}

func TestPlanStaleRecoveryRejectsReleasedAndDrift(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	if _, err := PlanStaleRecoveryAt(root, core.RecoveryRequest{ControllerID: "h", Reason: "r", At: time.Now()}); err == nil {
		t.Fatal("planned RELEASED recovery")
	}
	root, at := activeFixture(t)
	p, _ := PlanStaleRecoveryAt(root, core.RecoveryRequest{ControllerID: "h", Reason: "r", At: at.Add(time.Hour)})
	mustWrite(t, filepath.Join(root.Path(), ".uawp", "DECISIONS.md"), "changed")
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err == nil {
		t.Fatal("applied drifted recovery")
	}
}
