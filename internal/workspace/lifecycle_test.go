package workspace

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/vibemaker-community/uawp/internal/core"
)

func TestResumeOutcomesAreReadOnly(t *testing.T) {
	tests := []struct {
		name  string
		actor core.Actor
		setup func(*testing.T, Root)
		want  ResumeOutcome
	}{
		{"released", core.Actor{WorkerID: "worker-b", SessionID: "session-b"}, initializeFixture, ResumeAcquireAvailable},
		{"same owner", activeTestActor(), func(t *testing.T, root Root) { initializeFixture(t, root); writeOwnership(t, root, "ACTIVE", "none") }, ResumeOwnedByCaller},
		{"same worker other session", core.Actor{WorkerID: "worker-a", SessionID: "session-b", Generation: 1}, func(t *testing.T, root Root) { initializeFixture(t, root); writeOwnership(t, root, "ACTIVE", "none") }, ResumeBlockedBySession},
		{"expired generation", core.Actor{WorkerID: "worker-a", SessionID: "session-a", Generation: 2}, func(t *testing.T, root Root) { initializeFixture(t, root); writeOwnership(t, root, "ACTIVE", "none") }, ResumeExpiredGeneration},
		{"other owner", core.Actor{WorkerID: "worker-b", SessionID: "session-b", Generation: 1}, func(t *testing.T, root Root) { initializeFixture(t, root); writeOwnership(t, root, "ACTIVE", "none") }, ResumeBlockedByOther},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := openTempRoot(t)
			tt.setup(t, root)
			before := snapshotTree(t, root.Path())
			report, err := Resume(root, tt.actor)
			if err != nil {
				t.Fatal(err)
			}
			if report.Outcome != tt.want || report.NextAction == "" {
				t.Fatalf("report=%#v", report)
			}
			if !reflect.DeepEqual(before, snapshotTree(t, root.Path())) {
				t.Fatal("Resume mutated workspace")
			}
		})
	}
}

func TestPlanAcquireAndReleaseBindActorAndOwnership(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	acquire, err := PlanAcquireAt(root, core.AcquireRequest{WorkerID: "worker-a", SessionID: "session-a", Agent: "Test Agent", Purpose: "phase 2", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if acquire.Metadata().ActorWorkerID != "worker-a" || acquire.Metadata().ActorSessionID != "session-a" || acquire.Metadata().OwnershipGeneration != 1 || len(acquire.Changes()) != 1 {
		t.Fatalf("plan=%#v", acquire)
	}
	if _, err := Apply(root, acquire, ApplyOptions{ApprovedPlanID: acquire.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanAcquireAt(root, core.AcquireRequest{WorkerID: "worker-b", SessionID: "session-b", Agent: "Other", Purpose: "conflict", At: at.Add(time.Minute)}); err == nil {
		t.Fatal("planned competing acquire")
	}
	release, err := PlanReleaseAt(root, core.ReleaseRequest{Actor: activeTestActor(), At: at.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if release.Metadata().ActorWorkerID != "worker-a" || len(release.Changes()) != 1 {
		t.Fatalf("release=%#v", release)
	}
	if _, err := PlanReleaseAt(root, core.ReleaseRequest{Actor: core.Actor{WorkerID: "worker-b", SessionID: "session-b", Generation: 1}, At: at.Add(time.Hour)}); err == nil {
		t.Fatal("planned non-owner release")
	}
}

func TestCompetingAcquirePreviewIsRejectedAfterFirstApply(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	first, _ := PlanAcquireAt(root, core.AcquireRequest{WorkerID: "worker-a", SessionID: "session-a", Agent: "A", Purpose: "first", At: at})
	second, _ := PlanAcquireAt(root, core.AcquireRequest{WorkerID: "worker-b", SessionID: "session-b", Agent: "B", Purpose: "second", At: at})
	if _, err := Apply(root, first, ApplyOptions{ApprovedPlanID: first.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, second, ApplyOptions{ApprovedPlanID: second.ID}); err == nil {
		t.Fatal("stale competing plan applied")
	}
	owner, _ := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if owner.WorkerID != "worker-a" {
		t.Fatalf("owner=%s", owner.WorkerID)
	}
}

func TestResumeRejectsUnsafeState(t *testing.T) {
	tests := []struct {
		name  string
		actor core.Actor
		setup func(*testing.T, Root)
	}{
		{"empty worker", core.Actor{}, initializeFixture},
		{"malformed ownership", core.Actor{WorkerID: "w", SessionID: "s"}, func(t *testing.T, root Root) {
			initializeFixture(t, root)
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"), "bad")
		}},
		{"recovery", core.Actor{WorkerID: "w", SessionID: "s"}, func(t *testing.T, root Root) {
			initializeFixture(t, root)
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "RECOVERY.json"), "{}")
		}},
		{"missing context", core.Actor{WorkerID: "w", SessionID: "s"}, func(t *testing.T, root Root) {
			initializeFixture(t, root)
			if err := os.Remove(filepath.Join(root.Path(), ".uawp", "CONTEXT.md")); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := openTempRoot(t)
			tt.setup(t, root)
			before := snapshotTree(t, root.Path())
			if _, err := Resume(root, tt.actor); err == nil {
				t.Fatal("Resume accepted unsafe state")
			}
			if !reflect.DeepEqual(before, snapshotTree(t, root.Path())) {
				t.Fatal("Resume mutated workspace")
			}
		})
	}
}
