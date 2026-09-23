package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibemaker-community/uawp/internal/plan"
	"github.com/vibemaker-community/uawp/internal/transaction"
)

func TestDurableApplyStagesPlanJournalAndBackupBeforePointer(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	target := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	p := plan.NewForWorkspace("test-update", root.Path(), []plan.Change{
		plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(before), []byte("after\n")),
	})
	_, err = Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: failAt("after-pointer-sync")})
	if err == nil {
		t.Fatal("Apply succeeded despite injected interruption")
	}
	t.Logf("Apply interruption: %v", err)
	pointer := readTestPointer(t, root)
	bundleDir := filepath.Join(root.Path(), ".uawp", "recovery", pointer.TransactionID)
	journalFile, err := os.Open(filepath.Join(bundleDir, "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	journal, err := transaction.DecodeJournal(journalFile)
	journalFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	planFile, err := os.Open(filepath.Join(bundleDir, "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := transaction.DecodePlan(planFile)
	planFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if journal.PlanID != p.ID || persisted.ID != p.ID || len(journal.Actions) != 1 {
		t.Fatalf("journal=%#v plan=%#v", journal, persisted)
	}
	backup, err := os.ReadFile(filepath.Join(bundleDir, filepath.FromSlash(journal.Actions[0].BackupPath)))
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatalf("backup=%q err=%v", backup, err)
	}
	after, _ := os.ReadFile(target)
	if !bytes.Equal(after, before) {
		t.Fatalf("target changed before publication: %q", after)
	}
}

func TestTransactionFailpointsLeaveTruthfulState(t *testing.T) {
	for _, stage := range []string{"before-bundle", "after-plan-sync", "after-backup-sync", "after-pointer-sync", "before-publish", "after-publish", "after-action-verify", "after-receipt-sync", "after-pointer-clear"} {
		t.Run(stage, func(t *testing.T) {
			root := openTempRoot(t)
			initializeFixture(t, root)
			projectPath := filepath.Join(root.Path(), "keep.txt")
			mustWrite(t, projectPath, "keep")
			contextPath := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
			before, _ := os.ReadFile(contextPath)
			p := plan.NewForWorkspace("test-update", root.Path(), []plan.Change{
				plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(before), []byte("after\n")),
			})
			_, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: failAt(stage)})
			if err == nil {
				t.Fatal("Apply succeeded despite injected interruption")
			}
			if got, _ := os.ReadFile(projectPath); string(got) != "keep" {
				t.Fatalf("project file changed: %q", got)
			}
			_, pointerErr := os.Lstat(filepath.Join(root.Path(), ".uawp", "RECOVERY.json"))
			switch stage {
			case "before-bundle", "after-plan-sync", "after-backup-sync":
				if !os.IsNotExist(pointerErr) {
					t.Fatalf("pointer exists before publication: %v", pointerErr)
				}
			case "after-pointer-clear":
				if !os.IsNotExist(pointerErr) {
					t.Fatalf("pointer remains after clear: %v", pointerErr)
				}
			default:
				if pointerErr != nil || Status(root).Code != CodeRecoveryRequired {
					t.Fatalf("stage %s pointer=%v status=%s", stage, pointerErr, Status(root).Code)
				}
			}
		})
	}
}

func TestInitRecoveryPointerIsVisibleBeforeManifest(t *testing.T) {
	root := openTempRoot(t)
	p, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: failAt("after-pointer-sync")})
	if err == nil {
		t.Fatal("Apply succeeded despite injected interruption")
	}
	if _, err := os.Stat(filepath.Join(root.Path(), ".uawp", "RECOVERY.json")); err != nil {
		t.Fatal("recovery pointer is not visible", err)
	}
	if _, err := os.Stat(filepath.Join(root.Path(), ".uawp", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest published before interruption: %v", err)
	}
}

func TestInitBootstrapDoesNotReplaceCompetingNamespace(t *testing.T) {
	root := openTempRoot(t)
	p, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, _ int) error {
		if stage != "before-bootstrap-rename" {
			return nil
		}
		if err := os.Mkdir(filepath.Join(root.Path(), ".uawp"), 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(root.Path(), ".uawp", "competitor.txt"), []byte("keep"), 0o600)
	}})
	if err == nil {
		t.Fatal("Apply replaced a competing namespace")
	}
	content, readErr := os.ReadFile(filepath.Join(root.Path(), ".uawp", "competitor.txt"))
	if readErr != nil || string(content) != "keep" {
		t.Fatalf("competitor content=%q err=%v", content, readErr)
	}
}

func failAt(want string) func(string, int) error {
	return func(stage string, index int) error {
		if stage == want {
			return errors.New("injected " + want)
		}
		return nil
	}
}

func readTestPointer(t *testing.T, root Root) transaction.LivePointer {
	t.Helper()
	file, err := os.Open(filepath.Join(root.Path(), ".uawp", "RECOVERY.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	pointer, err := transaction.DecodeLivePointer(file)
	if err != nil {
		t.Fatal(err)
	}
	return pointer
}
