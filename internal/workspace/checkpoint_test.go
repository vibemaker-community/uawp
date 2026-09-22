package workspace

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
)

func TestPlanCheckpointCreatesImmutableSnapshot(t *testing.T) {
	root, at := activeFixture(t)
	req := CheckpointRequest{Actor: activeTestActor(), MilestoneID: "phase-2", Label: "Phase 2 ready", DecisionReferences: []string{"DEC-001"}, At: at.Add(time.Hour)}
	p, err := PlanCheckpointAt(root, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes()) != 1 || p.Changes()[0].BeforeSHA256 != "MISSING" {
		t.Fatalf("changes=%#v", p.Changes())
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanCheckpointAt(root, req); err == nil {
		t.Fatal("planned overwrite")
	}
	if _, err := filepath.Glob(filepath.Join(root.Path(), ".uawp", "checkpoints", "*.md")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckpointNameAndAuthorization(t *testing.T) {
	root, at := activeFixture(t)
	invalid := []string{"", "../x", "a/b", `a\b`, "COM¹", "UPPER", "double--dash"}
	for _, id := range invalid {
		if _, err := PlanCheckpointAt(root, CheckpointRequest{Actor: activeTestActor(), MilestoneID: id, Label: "label", At: at}); err == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	if _, err := PlanCheckpointAt(root, CheckpointRequest{Actor: core.Actor{WorkerID: "worker-b", SessionID: "session-b", Generation: 1}, MilestoneID: "valid", Label: "label", At: at}); err == nil {
		t.Fatal("non-owner checkpoint")
	}
}
