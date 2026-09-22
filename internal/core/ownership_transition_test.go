package core

import (
	"math"
	"testing"
	"time"
)

func TestAcquireReleasedOwnership(t *testing.T) {
	acquired := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	released := acquired.Add(-time.Hour)
	current := Ownership{Status: Released, WorkerID: "old", Agent: "old-agent", AcquiredAt: released.Add(-time.Hour), ReleasedAt: &released, Purpose: "old"}
	next, err := Acquire(current, AcquireRequest{WorkerID: " worker-b ", SessionID: " session-b ", Agent: " Agent B ", Purpose: " next ", At: acquired})
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != Active || next.WorkerID != "worker-b" || next.SessionID != "session-b" || next.Generation != 1 || next.Agent != "Agent B" || next.Purpose != "next" || next.ReleasedAt != nil {
		t.Fatalf("next=%#v", next)
	}
	if current.Status != Released || current.ReleasedAt == nil {
		t.Fatal("input mutated")
	}
}

func TestAcquireRejectsActiveOwnership(t *testing.T) {
	current := Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "a", AcquiredAt: time.Now(), Purpose: "p"}
	for _, worker := range []string{"worker-a", "worker-b"} {
		if _, err := Acquire(current, AcquireRequest{WorkerID: worker, SessionID: "session-b", Agent: "a", Purpose: "p", At: time.Now()}); err == nil {
			t.Fatalf("acquired over ACTIVE as %s", worker)
		}
	}
}

func TestReleaseRequiresCurrentOwnerAndOrderedTime(t *testing.T) {
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	current := Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "a", AcquiredAt: at, Purpose: "p"}
	if _, err := Release(current, ReleaseRequest{Actor: Actor{WorkerID: "worker-b", SessionID: "session-a", Generation: 1}, At: at.Add(time.Minute)}); err == nil {
		t.Fatal("non-owner released")
	}
	if _, err := Release(current, ReleaseRequest{Actor: Actor{WorkerID: "worker-a", SessionID: "session-a", Generation: 1}, At: at.Add(-time.Second)}); err == nil {
		t.Fatal("released before acquire")
	}
	next, err := Release(current, ReleaseRequest{Actor: Actor{WorkerID: " worker-a ", SessionID: " session-a ", Generation: 1}, At: at.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != Released || next.ReleasedAt == nil || next.WorkerID != current.WorkerID {
		t.Fatalf("next=%#v", next)
	}
}

func TestAcquireAdvancesGenerationAndReleaseFencesOldActor(t *testing.T) {
	at := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	released := at.Add(-time.Hour)
	current := Ownership{Status: Released, WorkerID: "old", SessionID: "old-session", Generation: 7, Agent: "old", AcquiredAt: at.Add(-2 * time.Hour), ReleasedAt: &released, Purpose: "old"}
	next, err := Acquire(current, AcquireRequest{WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", Purpose: "work", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if next.Generation != 8 {
		t.Fatalf("generation=%d", next.Generation)
	}
	if _, err := Release(next, ReleaseRequest{Actor: Actor{WorkerID: "worker-a", SessionID: "session-b", Generation: 8}, At: at.Add(time.Minute)}); err == nil {
		t.Fatal("different session released ownership")
	}
	if _, err := Release(next, ReleaseRequest{Actor: Actor{WorkerID: "worker-a", SessionID: "session-a", Generation: 7}, At: at.Add(time.Minute)}); err == nil {
		t.Fatal("stale generation released ownership")
	}
}

func TestAcquireRejectsGenerationOverflow(t *testing.T) {
	at := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	released := at
	current := Ownership{Status: Released, WorkerID: "old", SessionID: "old-session", Generation: math.MaxUint64, Agent: "old", AcquiredAt: at.Add(-time.Hour), ReleasedAt: &released, Purpose: "old"}
	if _, err := Acquire(current, AcquireRequest{WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", Purpose: "work", At: at.Add(time.Hour)}); err == nil {
		t.Fatal("acquired after generation overflow")
	}
}
