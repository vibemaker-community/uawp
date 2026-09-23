package transaction

import (
	"testing"

	"github.com/vibemaker-community/uawp/internal/plan"
)

func classificationBundle(t *testing.T) Bundle {
	t.Helper()
	p := plan.NewForWorkspace("multi", "/workspace", []plan.Change{
		plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes([]byte("before-context")), []byte("after-context")).WithSequence(10),
		plan.NewUpdateFile(".uawp/DECISIONS.md", 0o640, plan.HashBytes([]byte("before-decisions")), []byte("after-decisions")).WithSequence(20),
	})
	j := Journal{SchemaVersion: JournalSchemaVersion, TransactionID: "txn-001", Operation: p.Operation, PlanID: p.ID, Workspace: p.Workspace, Phase: PhaseApplying, StartedAt: "2026-09-22T12:00:00Z"}
	for index, change := range p.Persisted().Changes {
		j.Actions = append(j.Actions, Action{Index: index, Change: change, State: Pending, BackupPath: "backups/000" + string(rune('0'+index)) + ".bak", BackupSHA256: change.BeforeSHA256, BackupMode: change.Mode})
	}
	pointer := LivePointer{SchemaVersion: PointerSchemaVersion, TransactionID: j.TransactionID, Operation: j.Operation, PlanID: j.PlanID, Workspace: j.Workspace, Phase: j.Phase}
	return Bundle{Pointer: pointer, Journal: j, Plan: p.Persisted()}
}

func TestClassifyUsesLiveEvidenceRatherThanJournalLabels(t *testing.T) {
	bundle := classificationBundle(t)
	observation := Observation{Paths: map[string]PathState{
		".uawp/CONTEXT.md":   {Kind: PathFile, SHA256: bundle.Journal.Actions[0].Change.AfterSHA256},
		".uawp/DECISIONS.md": {Kind: PathFile, SHA256: bundle.Journal.Actions[1].Change.BeforeSHA256},
	}, Backups: map[int]BackupState{
		0: {Valid: true, SHA256: bundle.Journal.Actions[0].BackupSHA256, Mode: 0o600},
		1: {Valid: true, SHA256: bundle.Journal.Actions[1].BackupSHA256, Mode: 0o640},
	}}
	report := Classify(bundle, observation)
	if report.Classification != BothAvailable || len(report.Evidence) != 2 {
		t.Fatalf("Classify() = %#v", report)
	}
	bundle.Journal.Actions[0].State = Verified
	if got := Classify(bundle, observation).Classification; got != BothAvailable {
		t.Fatalf("journal label overrode live evidence: %s", got)
	}
}

func TestClassifyCompleteManualAndBackupFailure(t *testing.T) {
	bundle := classificationBundle(t)
	allAfter := Observation{Paths: map[string]PathState{}, Backups: map[int]BackupState{}}
	for index, action := range bundle.Journal.Actions {
		allAfter.Paths[action.Change.Path] = PathState{Kind: PathFile, SHA256: action.Change.AfterSHA256}
		allAfter.Backups[index] = BackupState{Valid: true, SHA256: action.BackupSHA256, Mode: action.Change.Mode}
	}
	if got := Classify(bundle, allAfter).Classification; got != TransactionComplete {
		t.Fatalf("all-after classification = %s", got)
	}
	drift := allAfter
	drift.Paths[bundle.Journal.Actions[0].Change.Path] = PathState{Kind: PathFile, SHA256: plan.HashBytes([]byte("user drift"))}
	if got := Classify(bundle, drift).Classification; got != ManualRequired {
		t.Fatalf("drift classification = %s", got)
	}
	missingBackup := classificationBundle(t)
	before := Observation{Paths: map[string]PathState{}, Backups: map[int]BackupState{}}
	for _, action := range missingBackup.Journal.Actions {
		before.Paths[action.Change.Path] = PathState{Kind: PathFile, SHA256: action.Change.BeforeSHA256}
	}
	if got := Classify(missingBackup, before).Classification; got != ManualRequired {
		t.Fatalf("missing-backup classification = %s", got)
	}
}
