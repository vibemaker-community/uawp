package transaction

import (
	"strings"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/plan"
)

func validJournal(t *testing.T) Journal {
	t.Helper()
	p := plan.NewForWorkspace("sync", "/workspace", []plan.Change{
		plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes([]byte("before")), []byte("after")),
	})
	return Journal{
		SchemaVersion: JournalSchemaVersion,
		TransactionID: "txn-001",
		Operation:     p.Operation,
		PlanID:        p.ID,
		Workspace:     p.Workspace,
		Phase:         PhaseApplying,
		StartedAt:     time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
		Actions: []Action{{Index: 0, Change: p.Persisted().Changes[0], State: Pending,
			BackupPath: "backups/0000.bak", BackupSHA256: plan.HashBytes([]byte("before"))}},
	}
}

func TestJournalValidationAcceptsExactActions(t *testing.T) {
	journal := validJournal(t)
	if err := journal.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestJournalValidationRejectsHostileRecords(t *testing.T) {
	tests := map[string]func(*Journal){
		"schema":           func(j *Journal) { j.SchemaVersion = "2" },
		"transaction id":   func(j *Journal) { j.TransactionID = "../escape" },
		"workspace":        func(j *Journal) { j.Workspace = "relative" },
		"time":             func(j *Journal) { j.StartedAt = "yesterday" },
		"state":            func(j *Journal) { j.Actions[0].State = "UNKNOWN" },
		"index":            func(j *Journal) { j.Actions[0].Index = 2 },
		"backup traversal": func(j *Journal) { j.Actions[0].BackupPath = "../secret" },
		"backup hash":      func(j *Journal) { j.Actions[0].BackupSHA256 = "bad" },
		"duplicate path": func(j *Journal) {
			j.Actions = append(j.Actions, j.Actions[0])
			j.Actions[1].Index = 1
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			journal := validJournal(t)
			mutate(&journal)
			if err := journal.Validate(); err == nil {
				t.Fatal("Validate accepted hostile journal")
			}
		})
	}
}

func TestActionStateTransitionsAreExplicit(t *testing.T) {
	valid := [][2]ActionState{{Pending, InProgress}, {InProgress, Applied}, {Applied, Verified}, {Verified, RolledBack}, {RolledBack, RollbackVerified}}
	for _, pair := range valid {
		if !CanTransition(pair[0], pair[1]) {
			t.Fatalf("transition %s -> %s rejected", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]ActionState{{Pending, Verified}, {Verified, Applied}, {RollbackVerified, Pending}} {
		if CanTransition(pair[0], pair[1]) {
			t.Fatalf("transition %s -> %s accepted", pair[0], pair[1])
		}
	}
}

func TestReceiptValidationRejectsInvalidHashesAndResult(t *testing.T) {
	receipt := Receipt{SchemaVersion: ReceiptSchemaVersion, TransactionID: "txn-001", Operation: "sync", PlanID: strings.Repeat("a", 64), CompletedAt: "2026-09-22T12:00:00Z", Result: ResultCompleted, Hashes: map[string]string{".uawp/CONTEXT.md": strings.Repeat("b", 64)}}
	if err := receipt.Validate(); err != nil {
		t.Fatal(err)
	}
	receipt.Hashes[".uawp/CONTEXT.md"] = "bad"
	if err := receipt.Validate(); err == nil {
		t.Fatal("accepted invalid receipt hash")
	}
}

func TestBundleValidationBindsPointerJournalAndPlan(t *testing.T) {
	journal := validJournal(t)
	persisted := plan.PersistedPlan{
		ID: journal.PlanID, Operation: journal.Operation, Workspace: journal.Workspace,
		Changes: []plan.PersistedChange{journal.Actions[0].Change}, Inputs: []plan.Input{}, Metadata: plan.Metadata{},
	}
	// Rebuild the canonical ID because the journal helper intentionally creates
	// its plan through the public constructor.
	rebuilt := plan.NewForWorkspace(journal.Operation, journal.Workspace, []plan.Change{
		plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes([]byte("before")), []byte("after")),
	})
	persisted = rebuilt.Persisted()
	journal.PlanID = rebuilt.ID
	journal.Actions[0].Change = persisted.Changes[0]
	pointer := LivePointer{SchemaVersion: PointerSchemaVersion, TransactionID: journal.TransactionID, Operation: journal.Operation, PlanID: journal.PlanID, Workspace: journal.Workspace, Phase: journal.Phase}
	bundle := Bundle{Pointer: pointer, Journal: journal, Plan: persisted}
	if err := bundle.Validate(); err != nil {
		t.Fatal(err)
	}

	bundle.Pointer.PlanID = strings.Repeat("f", 64)
	if err := bundle.Validate(); err == nil {
		t.Fatal("Bundle.Validate accepted pointer/plan disagreement")
	}
}
