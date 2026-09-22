package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/adapter"
)

func TestFullDetachRemovesAllConsumersAndPreservesState(t *testing.T) {
	root := adapterRoot(t)
	addAdapter(t, root, "codex")
	facts := adapter.RuntimeFacts{Options: map[string]map[string]string{"claude-code": {"directAgentsSupport": "true"}}}
	claudePlan, _, err := PlanAdapterAddAt(root, "claude-code", facts, nil, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, claudePlan, ApplyOptions{ApprovedPlanID: claudePlan.ID}); err != nil {
		t.Fatal(err)
	}
	workbuddyPlan, _, err := PlanAdapterAddAt(root, "workbuddy", facts, nil, time.Unix(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, workbuddyPlan, ApplyOptions{ApprovedPlanID: workbuddyPlan.ID}); err != nil {
		t.Fatal(err)
	}
	p, report, err := PlanUninstallDetachAt(root, facts, time.Date(2026, 9, 22, 16, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.ConsumersRemoved) != 3 {
		t.Fatalf("report=%#v", report)
	}
	for _, change := range p.Changes() {
		if bytes.HasPrefix([]byte(change.Path), []byte(".uawp/")) && change.Path != ".uawp/manifest.json" {
			t.Fatalf("state mutation: %#v", change)
		}
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	if ids, err := ConfiguredAdapterIDs(root); err != nil || len(ids) != 0 {
		t.Fatalf("configured=%#v err=%v", ids, err)
	}
	if _, _, err := PlanUninstallDetachAt(root, adapter.RuntimeFacts{}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func TestUninstallPreservesUserAdditionsAndBlocksActive(t *testing.T) {
	root := adapterRoot(t)
	addAdapter(t, root, "codex")
	path := filepath.Join(root.Path(), "AGENTS.md")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("\n# keep me\n")
	_ = f.Close()
	p, _, err := PlanUninstallDetachAt(root, adapter.RuntimeFacts{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(content, []byte("keep me")) || bytes.Contains(content, []byte("UAWP:BEGIN")) {
		t.Fatalf("content=%q err=%v", content, err)
	}

	active := adapterRoot(t)
	addAdapter(t, active, "codex")
	writeOwnership(t, active, "ACTIVE", "none")
	if _, _, err := PlanUninstallDetachAt(active, adapter.RuntimeFacts{}, time.Now()); err == nil {
		t.Fatal("uninstall accepted ACTIVE ownership")
	}
}

func TestUninstallRejectsDriftedManagedBlock(t *testing.T) {
	root := adapterRoot(t)
	addAdapter(t, root, "codex")
	path := filepath.Join(root.Path(), "AGENTS.md")
	content, _ := os.ReadFile(path)
	content = bytes.Replace(content, []byte("Read `.uawp/INSTRUCTIONS.md`"), []byte("changed"), 1)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanUninstallDetachAt(root, adapter.RuntimeFacts{}, time.Now()); err == nil {
		t.Fatal("uninstall accepted marker drift")
	}
}
