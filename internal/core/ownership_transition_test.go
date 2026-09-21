package core

import (
	"testing"
	"time"
)

func TestAcquireReleasedOwnership(t *testing.T) {
	acquired := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	released := acquired.Add(-time.Hour)
	current := Ownership{Status: Released, WorkerID: "old", Agent: "old-agent", AcquiredAt: released.Add(-time.Hour), ReleasedAt: &released, Purpose: "old"}
	next, err := Acquire(current, AcquireRequest{WorkerID: " worker-b ", Agent: " Agent B ", Purpose: " next ", At: acquired})
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != Active || next.WorkerID != "worker-b" || next.Agent != "Agent B" || next.Purpose != "next" || next.ReleasedAt != nil {
		t.Fatalf("next=%#v", next)
	}
	if current.Status != Released || current.ReleasedAt == nil {
		t.Fatal("input mutated")
	}
}

func TestAcquireRejectsActiveOwnership(t *testing.T) {
	current := Ownership{Status: Active, WorkerID: "worker-a", Agent: "a", AcquiredAt: time.Now(), Purpose: "p"}
	for _, worker := range []string{"worker-a", "worker-b"} {
		if _, err := Acquire(current, AcquireRequest{WorkerID: worker, Agent: "a", Purpose: "p", At: time.Now()}); err == nil {
			t.Fatalf("acquired over ACTIVE as %s", worker)
		}
	}
}

func TestReleaseRequiresCurrentOwnerAndOrderedTime(t *testing.T) {
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	current := Ownership{Status: Active, WorkerID: "worker-a", Agent: "a", AcquiredAt: at, Purpose: "p"}
	if _, err := Release(current, ReleaseRequest{WorkerID: "worker-b", At: at.Add(time.Minute)}); err == nil {
		t.Fatal("non-owner released")
	}
	if _, err := Release(current, ReleaseRequest{WorkerID: "worker-a", At: at.Add(-time.Second)}); err == nil {
		t.Fatal("released before acquire")
	}
	next, err := Release(current, ReleaseRequest{WorkerID: " worker-a ", At: at.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != Released || next.ReleasedAt == nil || next.WorkerID != current.WorkerID {
		t.Fatalf("next=%#v", next)
	}
}
