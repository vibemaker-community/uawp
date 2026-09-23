package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
)

func TestApplyCreatesNamespace(t *testing.T) {
	root := openTempRoot(t)
	value, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Applied) != len(value.Changes()) || !report.Verified {
		t.Fatalf("report = %#v", report)
	}
	for _, path := range []string{"manifest.json", "CONTEXT.md", "ACTIVE_WORKER.md", "DECISIONS.md"} {
		if _, err := os.Stat(filepath.Join(root.Path(), ".uawp", path)); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
}

func TestApplyRejectsWithoutApproval(t *testing.T) {
	root := openTempRoot(t)
	value, _ := PlanInit(root)
	if _, err := Apply(root, value, ApplyOptions{}); err == nil {
		t.Fatal("Apply() succeeded without approval")
	}
	assertNamespaceAbsent(t, root)
}

func TestApplyRejectsPlanRootMismatch(t *testing.T) {
	first := openTempRoot(t)
	second := openTempRoot(t)
	value, _ := PlanInit(first)
	if _, err := Apply(second, value, ApplyOptions{ApprovedPlanID: value.ID}); err == nil {
		t.Fatal("Apply() accepted plan for another root")
	}
	assertNamespaceAbsent(t, second)
}

func TestApplyRejectsDrift(t *testing.T) {
	root := openTempRoot(t)
	value, _ := PlanInit(root)
	if err := os.Mkdir(filepath.Join(root.Path(), ".uawp"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID}); err == nil {
		t.Fatal("Apply() accepted a stale plan")
	}
}

func TestApplyRollsBackAfterActionFailure(t *testing.T) {
	root := openTempRoot(t)
	projectFile := filepath.Join(root.Path(), "keep.txt")
	mustWrite(t, projectFile, "keep")
	value, _ := PlanInit(root)
	_, err := Apply(root, value, ApplyOptions{
		ApprovedPlanID: value.ID,
		Failpoint: func(stage string, index int) error {
			if stage == "before-action" && index == 2 {
				return errors.New("injected interruption")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("Apply() succeeded despite interruption")
	}
	if got := Status(root).Code; got != CodeRecoveryRequired {
		t.Fatalf("Status = %s, want recovery required", got)
	}
	content, readErr := os.ReadFile(projectFile)
	if readErr != nil || string(content) != "keep" {
		t.Fatalf("project file changed: content=%q err=%v", content, readErr)
	}
}

func TestApplyRollsBackRenameFailure(t *testing.T) {
	root := openTempRoot(t)
	value, _ := PlanInit(root)
	_, err := Apply(root, value, ApplyOptions{
		ApprovedPlanID: value.ID,
		Failpoint: func(stage string, index int) error {
			if stage == "before-rename" {
				return errors.New("injected rename failure")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("Apply() succeeded despite rename failure")
	}
	if got := Status(root).Code; got != CodeRecoveryRequired {
		t.Fatalf("Status = %s, want recovery required", got)
	}
}

func TestApplyRollsBackTemporaryWriteFailure(t *testing.T) {
	root := openTempRoot(t)
	value, _ := PlanInit(root)
	_, err := Apply(root, value, ApplyOptions{
		ApprovedPlanID: value.ID,
		Failpoint: func(stage string, index int) error {
			if stage == "temp-write" {
				return errors.New("injected temporary write failure")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("Apply() succeeded despite temporary write failure")
	}
	if got := Status(root).Code; got != CodeRecoveryRequired {
		t.Fatalf("Status = %s, want recovery required", got)
	}
}

func TestApplyRejectsChangedInputFingerprint(t *testing.T) {
	root := openTempRoot(t)
	target := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	if err := os.Mkdir(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, target, "before")
	change := plan.NewFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes([]byte("before")), []byte("after"))
	value := plan.NewForWorkspace("test-update", root.Path(), []plan.Change{change})
	mustWrite(t, target, "changed after preview")
	if _, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID}); err == nil {
		t.Fatal("Apply() accepted changed input fingerprint")
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "changed after preview" {
		t.Fatalf("Apply() changed drifted input: content=%q err=%v", content, err)
	}
}

func TestApplyDeletesVerifiedNativeFile(t *testing.T) {
	root := openTempRoot(t)
	initPlan, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, initPlan, ApplyOptions{ApprovedPlanID: initPlan.ID}); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root.Path(), "AGENTS.md")
	mustWrite(t, target, "owned")
	change := plan.NewDeleteFile("AGENTS.md", plan.HashBytes([]byte("owned")))
	value := plan.NewForWorkspace("adapter-remove", root.Path(), []plan.Change{change})
	if _, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("target remains: %v", err)
	}
}

func TestApplyPreservesCompetingFile(t *testing.T) {
	root := openTempRoot(t)
	value, _ := PlanInit(root)
	_, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID, Failpoint: func(stage string, index int) error {
		if stage == "before-rename" && index == 1 {
			return os.WriteFile(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"), []byte("competitor"), 0o600)
		}
		return nil
	}})
	if err == nil {
		t.Fatal("Apply unexpectedly replaced competing file")
	}
	content, readErr := os.ReadFile(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if readErr != nil || string(content) != "competitor" {
		t.Fatalf("competing content=%q err=%v", content, readErr)
	}
}

func TestApplyLeavesRecoveryJournalAfterPublishedInterruption(t *testing.T) {
	root := openTempRoot(t)
	value, _ := PlanInit(root)
	_, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID, Failpoint: func(stage string, index int) error {
		if stage == "after-publish" && index == 1 {
			return errors.New("simulated process interruption")
		}
		return nil
	}})
	if err == nil {
		t.Fatal("Apply succeeded despite interruption")
	}
	if got := Status(root).Code; got != CodeRecoveryRequired {
		t.Fatalf("Status = %s, want recovery required", got)
	}
	if _, err := os.Stat(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md")); err != nil {
		t.Fatalf("published file missing: %v", err)
	}
}

func TestApplyUpdateReplacesVerifiedManagedFile(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	target := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	before, _ := os.ReadFile(target)
	change := plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(before), []byte("updated\n"))
	value := plan.NewForWorkspace("sync", root.Path(), []plan.Change{change})
	report, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(target)
	if !report.Verified || string(content) != "updated\n" {
		t.Fatalf("report=%#v content=%q", report, content)
	}
}

func TestApplyUpdateRejectsDriftWithoutOverwrite(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	target := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	before, _ := os.ReadFile(target)
	change := plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(before), []byte("updated\n"))
	value := plan.NewForWorkspace("sync", root.Path(), []plan.Change{change})
	mustWrite(t, target, "competitor")
	if _, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID}); err == nil {
		t.Fatal("updated drifted file")
	}
	content, _ := os.ReadFile(target)
	if string(content) != "competitor" {
		t.Fatalf("content=%q", content)
	}
}

func TestApplyBlocksExistingRecoveryAndRevalidatesInputsPerAction(t *testing.T) {
	root, at := activeFixture(t)
	p, _ := PlanContextSync(root, activeTestActor(), []byte("changed"))
	mustWrite(t, filepath.Join(root.Path(), ".uawp", "RECOVERY.json"), "{}")
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err == nil {
		t.Fatal("applied over recovery")
	}
	os.Remove(filepath.Join(root.Path(), ".uawp", "RECOVERY.json"))
	ownerPath := filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md")
	_, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, index int) error {
		if stage == "before-action" && index == 0 {
			next := core.Ownership{Status: core.Active, WorkerID: "worker-b", SessionID: "session-b", Generation: 2, Agent: "B", AcquiredAt: at, Purpose: "other"}
			raw, _ := core.EncodeOwnership(next)
			return os.WriteFile(ownerPath, raw, 0o600)
		}
		return nil
	}})
	if err == nil {
		t.Fatal("applied after authority drift")
	}
}

func openTempRoot(t *testing.T) Root {
	t.Helper()
	root, err := OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func assertNamespaceAbsent(t *testing.T, root Root) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(root.Path(), ".uawp")); !os.IsNotExist(err) {
		t.Fatalf(".uawp remains after failed apply: %v", err)
	}
}
