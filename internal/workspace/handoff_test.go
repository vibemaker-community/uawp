package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
)

func TestPlanHandoffOrdersContextBeforeRelease(t *testing.T) {
	root, at := activeFixture(t)
	p, err := PlanHandoffAt(root, HandoffRequest{Actor: activeTestActor(), FinalContext: []byte("# Final\n"), Purpose: "handoff", At: at.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	changes := p.Changes()
	if len(changes) != 2 {
		t.Fatalf("changes=%#v", changes)
	}
	if changes[0].Path != ".uawp/CONTEXT.md" || changes[1].Path != ".uawp/ACTIVE_WORKER.md" {
		t.Fatalf("order=%#v", changes)
	}
	if p.Metadata().ActorWorkerID != "worker-a" || p.Metadata().Reason != "handoff" {
		t.Fatalf("metadata=%#v", p.Metadata())
	}
	if _, err := PlanHandoffAt(root, HandoffRequest{Actor: core.Actor{WorkerID: "worker-b", SessionID: "session-b", Generation: 1}, FinalContext: []byte("x"), Purpose: "x", At: at}); err == nil {
		t.Fatal("non-owner handoff")
	}
}

func TestPlanHandoffInterruptionRequiresRecoveryBeforeRelease(t *testing.T) {
	root, at := activeFixture(t)
	keep := filepath.Join(root.Path(), "keep.txt")
	mustWrite(t, keep, "keep")
	p, _ := PlanHandoffAt(root, HandoffRequest{Actor: activeTestActor(), FinalContext: []byte("# Final\n"), Purpose: "handoff", At: at.Add(time.Hour)})
	_, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, index int) error {
		if stage == "before-action" && index == 1 {
			return os.ErrClosed
		}
		return nil
	}})
	if err == nil {
		t.Fatal("handoff succeeded")
	}
	if Status(root).Code != CodeRecoveryRequired {
		t.Fatalf("status=%s", Status(root).Code)
	}
	owner, _ := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if owner.Status != "ACTIVE" {
		t.Fatalf("owner=%#v", owner)
	}
	content, _ := os.ReadFile(filepath.Join(root.Path(), ".uawp", "CONTEXT.md"))
	if string(content) != "# Final\n" {
		t.Fatalf("context=%q", content)
	}
	project, _ := os.ReadFile(keep)
	if string(project) != "keep" {
		t.Fatalf("project=%q", project)
	}
}
