package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/transaction"
)

func interruptedTwoFileTransaction(t *testing.T) Root {
	t.Helper()
	root := openTempRoot(t)
	initializeFixture(t, root)
	contextPath := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	decisionsPath := filepath.Join(root.Path(), ".uawp", "DECISIONS.md")
	context, _ := os.ReadFile(contextPath)
	decisions, _ := os.ReadFile(decisionsPath)
	p := plan.NewForWorkspace("multi", root.Path(), []plan.Change{
		plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(context), []byte("new context\n")).WithSequence(10),
		plan.NewUpdateFile(".uawp/DECISIONS.md", 0o600, plan.HashBytes(decisions), []byte("new decisions\n")).WithSequence(20),
	})
	_, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, index int) error {
		if stage == "after-action-verify" && index == 0 {
			return errors.New("power loss")
		}
		return nil
	}})
	if err == nil {
		t.Fatal("Apply succeeded despite interruption")
	}
	return root
}

func TestTransactionStatusAndRecoveryPlansAreEvidenceBound(t *testing.T) {
	root := interruptedTwoFileTransaction(t)
	report, err := TransactionStatus(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Classification != transaction.BothAvailable {
		t.Fatalf("classification = %s", report.Classification)
	}
	at := time.Date(2026, 9, 22, 13, 0, 0, 0, time.UTC)
	rollback, err := PlanTransactionRollbackAt(root, "human-1", "confirmed interrupted apply", at)
	if err != nil || rollback.Metadata().RecoveryAction != "ROLLBACK" || rollback.Metadata().TransactionID == "" {
		t.Fatalf("rollback plan=%#v err=%v", rollback, err)
	}
	continuation, err := PlanTransactionContinueAt(root, "human-1", "confirmed interrupted apply", at)
	if err != nil || continuation.Metadata().RecoveryAction != "CONTINUE" || len(continuation.Changes()) != 1 {
		t.Fatalf("continuation plan=%#v err=%v", continuation, err)
	}
	if _, err := PlanTransactionRollbackAt(root, "", "reason", at); err == nil {
		t.Fatal("rollback accepted empty controller")
	}
}

func TestApplyTransactionRollbackRestoresBeforeStateAndCannotReplay(t *testing.T) {
	root := interruptedTwoFileTransaction(t)
	at := time.Date(2026, 9, 22, 13, 0, 0, 0, time.UTC)
	p, err := PlanTransactionRollbackAt(root, "human-1", "confirmed interrupted apply", at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransactionRecovery(root, p, ApplyOptions{ApprovedPlanID: p.ID, Clock: func() time.Time { return at }}); err != nil {
		t.Fatal(err)
	}
	if Status(root).Code == CodeRecoveryRequired {
		t.Fatal("recovery pointer remains")
	}
	if _, err := ApplyTransactionRecovery(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err == nil {
		t.Fatal("replayed recovery succeeded")
	}
}

func TestApplyTransactionContinuePublishesOnlyPendingAction(t *testing.T) {
	root := interruptedTwoFileTransaction(t)
	at := time.Date(2026, 9, 22, 13, 0, 0, 0, time.UTC)
	p, err := PlanTransactionContinueAt(root, "human-1", "confirmed interrupted apply", at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransactionRecovery(root, p, ApplyOptions{ApprovedPlanID: p.ID, Clock: func() time.Time { return at }}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root.Path(), ".uawp", "DECISIONS.md"))
	if err != nil || string(content) != "new decisions\n" {
		t.Fatalf("decisions=%q err=%v", content, err)
	}
}
