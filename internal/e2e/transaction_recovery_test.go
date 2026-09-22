package e2e

import (
	"errors"
	"testing"

	"github.com/uawp/uawp/internal/workspace"
)

func TestInterruptedInitIsDiagnosedForTransactionRecovery(t *testing.T) {
	dir := t.TempDir()
	root, err := workspace.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := workspace.PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = workspace.Apply(root, p, workspace.ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, index int) error {
		if stage == "after-pointer-sync" {
			return errors.New("interrupt")
		}
		return nil
	}})
	if err == nil {
		t.Fatal("expected interruption")
	}
	if report := workspace.Status(root); report.Code != workspace.CodeRecoveryRequired {
		t.Fatalf("status=%#v", report)
	}
	if _, err := workspace.TransactionStatus(root); err != nil {
		t.Fatal(err)
	}
}
