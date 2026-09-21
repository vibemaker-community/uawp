package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/uawp/uawp/internal/plan"
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
	assertNamespaceAbsent(t, root)
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
	assertNamespaceAbsent(t, root)
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
	assertNamespaceAbsent(t, root)
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
