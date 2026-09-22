package workspace

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
)

func activeFixture(t *testing.T) (Root, time.Time) {
	t.Helper()
	root := openTempRoot(t)
	initializeFixture(t, root)
	at := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	p, err := PlanAcquireAt(root, core.AcquireRequest{WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", Purpose: "work", At: at})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	return root, at
}

func activeTestActor() core.Actor {
	return core.Actor{WorkerID: "worker-a", SessionID: "session-a", Generation: 1}
}

func TestPlanContextSyncOwnerAndNoop(t *testing.T) {
	root, _ := activeFixture(t)
	content := []byte("# Context\n\nCurrent.\n")
	p, err := PlanContextSync(root, activeTestActor(), content)
	if err != nil {
		t.Fatal(err)
	}
	if p.Metadata().ActorWorkerID != "worker-a" || len(p.Changes()) != 1 {
		t.Fatalf("plan=%#v", p)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	noop, err := PlanContextSync(root, activeTestActor(), content)
	if err != nil || len(noop.Changes()) != 0 {
		t.Fatalf("noop=%#v err=%v", noop, err)
	}
	if _, err := PlanContextSync(root, core.Actor{WorkerID: "worker-b", SessionID: "session-b", Generation: 1}, content); err == nil {
		t.Fatal("non-owner sync planned")
	}
}

func TestPlanContextSyncRejectsInvalidContent(t *testing.T) {
	root, _ := activeFixture(t)
	for _, content := range [][]byte{nil, bytes.Repeat([]byte("x"), (1<<20)+1)} {
		if _, err := PlanContextSync(root, activeTestActor(), content); err == nil {
			t.Fatal("accepted invalid context")
		}
	}
	if _, err := filepath.Abs(root.Path()); err != nil {
		t.Fatal(err)
	}
}

func TestSameWorkerDifferentSessionCannotSync(t *testing.T) {
	root, _ := activeFixture(t)
	otherSession := core.Actor{WorkerID: "worker-a", SessionID: "session-b", Generation: 1}
	if _, err := PlanContextSync(root, otherSession, []byte("# Other session\n")); err == nil {
		t.Fatal("same Worker with another Session planned context sync")
	}
}
