package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vibemaker-community/uawp/internal/adapter"
)

func TestRepairRegeneratesOnlyGeneratedInstructions(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	target := filepath.Join(root.Path(), ".uawp", "INSTRUCTIONS.md")
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	p, findings, err := PlanRepairAt(root, adapter.RuntimeFacts{}, []string{string(CodeRepairAvailable)}, time.Date(2026, 9, 22, 15, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 || len(p.Changes()) != 1 || p.Changes()[0].Path != ".uawp/INSTRUCTIONS.md" {
		t.Fatalf("plan=%#v findings=%#v", p, findings)
	}
	if _, err := Apply(root, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(target); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("instructions not repaired: %v", err)
	}
}

func TestRepairRefusesUserStateLossAndActiveOwnership(t *testing.T) {
	root := openTempRoot(t)
	initializeFixture(t, root)
	if err := os.Remove(filepath.Join(root.Path(), ".uawp", "CONTEXT.md")); err != nil {
		t.Fatal(err)
	}
	p, findings, err := PlanRepairAt(root, adapter.RuntimeFacts{}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes()) != 0 || !hasStatusCode(findings, CodeManualRepairRequired) {
		t.Fatalf("plan=%#v findings=%#v", p, findings)
	}

	active := openTempRoot(t)
	initializeFixture(t, active)
	writeOwnership(t, active, "ACTIVE", "none")
	if _, _, err := PlanRepairAt(active, adapter.RuntimeFacts{}, nil, time.Now()); err == nil {
		t.Fatal("repair accepted ACTIVE ownership")
	}
}

func hasStatusCode(findings []StatusReport, code FindingCode) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func TestRepairRestoresProvablyOwnedBlockInExistingFile(t *testing.T) {
	root := adapterRoot(t)
	mustWrite(t, filepath.Join(root.Path(), "AGENTS.md"), "# project rules\n")
	addAdapter(t, root, "codex")
	manifest, _, _ := readAdapterManifest(root)
	artifact := manifest.Integrations[0]
	path := filepath.Join(root.Path(), "AGENTS.md")
	content, _ := os.ReadFile(path)
	outside, err := adapter.RemoveManagedBlock(content, adapter.BlockSpec{ArtifactID: artifact.ID, Target: artifact.Target, Consumers: artifact.Consumers, Body: adapter.BridgeBody()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, outside, 0o600); err != nil {
		t.Fatal(err)
	}
	p, _, err := PlanRepairAt(root, adapter.RuntimeFacts{}, []string{"native"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes()) != 1 || p.Changes()[0].Path != "AGENTS.md" {
		t.Fatalf("changes=%#v", p.Changes())
	}
}
