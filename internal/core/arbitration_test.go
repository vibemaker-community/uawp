package core

import (
	"testing"
	"time"
)

func TestRecoverStaleOwnershipReleasesOldClaim(t *testing.T) {
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	current := Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 3, Agent: "Agent A", AcquiredAt: at.Add(-time.Hour), Purpose: "work"}
	next, audit, err := RecoverStaleOwnership(current, RecoveryRequest{ControllerID: "human-1", Reason: "worker crashed", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != Released || next.WorkerID != "worker-a" || next.SessionID != "session-a" || next.Generation != 3 || next.Agent != "Agent A" || next.ReleasedAt == nil {
		t.Fatalf("next=%#v", next)
	}
	if audit.ControllerID != "human-1" || audit.OldWorkerID != "worker-a" || audit.OldSessionID != "session-a" || audit.OldGeneration != 3 {
		t.Fatalf("audit=%#v", audit)
	}
}

func TestRecoverStaleOwnershipRejectsInvalidRequest(t *testing.T) {
	at := time.Now()
	active := Ownership{Status: Active, WorkerID: "w", SessionID: "s", Generation: 1, Agent: "a", AcquiredAt: at.Add(-time.Hour), Purpose: "p"}
	for _, req := range []RecoveryRequest{{Reason: "r", At: at}, {ControllerID: "h", At: at}, {ControllerID: "h", Reason: "r"}} {
		if _, _, err := RecoverStaleOwnership(active, req); err == nil {
			t.Fatal("accepted invalid recovery")
		}
	}
	releasedAt := at
	released := active
	released.Status = Released
	released.ReleasedAt = &releasedAt
	if _, _, err := RecoverStaleOwnership(released, RecoveryRequest{ControllerID: "h", Reason: "r", At: at}); err == nil {
		t.Fatal("recovered RELEASED")
	}
}
