package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
)

type directTestAdapter struct{}

func (directTestAdapter) ID() string                 { return "direct-test" }
func (directTestAdapter) Evidence() adapter.Evidence { return adapter.Evidence{} }
func (directTestAdapter) Resolve(adapter.Snapshot) adapter.Resolution {
	return adapter.Resolution{Provider: "direct-test", Confidence: adapter.Verified, Health: "HEALTHY", Route: adapter.Route{Path: ".uawp/INSTRUCTIONS.md", Mode: core.Direct, Target: ".uawp/INSTRUCTIONS.md"}}
}

func adapterRoot(t *testing.T) Root {
	t.Helper()
	r := openTempRoot(t)
	p, err := PlanInit(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	return r
}
func applyAdapterPlan(t *testing.T, r Root, p interface{ GetID() string }) { t.Helper() }
func addAdapter(t *testing.T, r Root, id string) {
	t.Helper()
	p, _, err := PlanAdapterAddAt(r, id, adapter.RuntimeFacts{}, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
}
func removeAdapterPlan(t *testing.T, r Root, id string) {
	t.Helper()
	p, err := PlanAdapterRemoveAt(r, id, adapter.RuntimeFacts{}, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterAddIsIdempotent(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	p, _, err := PlanAdapterAddAt(r, "codex", adapter.RuntimeFacts{}, nil, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes()) != 0 {
		t.Fatalf("changes=%#v", p.Changes())
	}
}
func TestAdapterSharedBridgeSurvivesOneConsumerRemoval(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	addAdapter(t, r, "workbuddy")
	content, err := os.ReadFile(filepath.Join(r.Path(), "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("consumers=codex,workbuddy")) {
		t.Fatalf("content=%q", content)
	}
	removeAdapterPlan(t, r, "workbuddy")
	content, _ = os.ReadFile(filepath.Join(r.Path(), "AGENTS.md"))
	if !bytes.Contains(content, []byte("consumers=codex")) || bytes.Contains(content, []byte("workbuddy")) {
		t.Fatalf("content=%q", content)
	}
	removeAdapterPlan(t, r, "codex")
	if _, err := os.Lstat(filepath.Join(r.Path(), "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("created file remains: %v", err)
	}
}
func TestAdapterRemovalPreservesUserAdditions(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	path := filepath.Join(r.Path(), "AGENTS.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n# User rule\nkeep\n")
	_ = f.Close()
	removeAdapterPlan(t, r, "codex")
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(content, []byte("# User rule\nkeep")) || bytes.Contains(content, []byte("UAWP:BEGIN")) {
		t.Fatalf("content=%q err=%v", content, err)
	}
}
func TestAdapterPlanBindsOtherEntryDrift(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	p, _, err := PlanAdapterAddAt(r, "workbuddy", adapter.RuntimeFacts{}, nil, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(r.Path(), "CODEBUDDY.md"), "new entry")
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err == nil {
		t.Fatal("accepted entry drift")
	}
}
func TestAdapterConditionalRouteRequiresAcknowledgement(t *testing.T) {
	r := adapterRoot(t)
	mustWrite(t, filepath.Join(r.Path(), "AGENTS.md"), "existing")
	facts := adapter.RuntimeFacts{Versions: map[string]string{"claude-code": ""}, Options: map[string]map[string]string{"claude-code": {"directAgentsSupport": "unknown"}}}
	if _, _, err := PlanAdapterAddAt(r, "claude-code", facts, nil, time.Unix(1, 0)); err == nil {
		t.Fatal("accepted conditional route")
	}
	p, _, err := PlanAdapterAddAt(r, "claude-code", facts, []string{"CLAUDE_CREATION_CHANGES_SELECTION"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	content := p.Changes()[0].Content()
	if !bytes.Contains(content, []byte("@AGENTS.md")) || !bytes.Contains(content, []byte("@.uawp/INSTRUCTIONS.md")) {
		t.Fatalf("conditional preservation content=%q", content)
	}
}

func TestAdapterAddUpdatesExistingEmptyEntry(t *testing.T) {
	r := adapterRoot(t)
	mustWrite(t, filepath.Join(r.Path(), "AGENTS.md"), "")
	p, _, err := PlanAdapterAddAt(r, "codex", adapter.RuntimeFacts{}, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Changes()[0].Kind; got != "UPDATE_FILE" {
		t.Fatalf("kind=%s", got)
	}
}

func TestAdapterAddRejectsUnsafeCandidate(t *testing.T) {
	r := adapterRoot(t)
	if err := os.Mkdir(filepath.Join(r.Path(), "AGENTS.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanAdapterAddAt(r, "codex", adapter.RuntimeFacts{}, nil, time.Unix(1, 0)); err == nil {
		t.Fatal("accepted unsafe native candidate")
	}
}

func TestAdapterProviderFactsChangePlanIdentity(t *testing.T) {
	r := adapterRoot(t)
	facts1 := adapter.RuntimeFacts{Versions: map[string]string{"codex": "1"}}
	facts2 := adapter.RuntimeFacts{Versions: map[string]string{"codex": "2"}}
	p1, _, err := PlanAdapterAddAt(r, "codex", facts1, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	p2, _, err := PlanAdapterAddAt(r, "codex", facts2, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID == p2.ID {
		t.Fatal("provider fact drift did not invalidate approval")
	}
}

func TestAdapterInterruptedAfterNativePublicationRequiresRecovery(t *testing.T) {
	r := adapterRoot(t)
	p, _, err := PlanAdapterAddAt(r, "codex", adapter.RuntimeFacts{}, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID, Failpoint: func(stage string, index int) error {
		if stage == "after-publish" && index == 0 {
			return errors.New("interrupt")
		}
		return nil
	}})
	if err == nil {
		t.Fatal("expected interruption")
	}
	if got := Status(r).Code; got != CodeRecoveryRequired {
		t.Fatalf("status=%s", got)
	}
}

func TestResolveAdaptersReportsRegisteredEntryDrift(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	addAdapter(t, r, "workbuddy")
	mustWrite(t, filepath.Join(r.Path(), "CODEBUDDY.md"), "new preferred entry")
	resolutions, err := ResolveAdapters(r, []string{"workbuddy"}, adapter.RuntimeFacts{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range resolutions[0].Findings {
		if finding.Code == "ENTRY_DRIFT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("findings=%#v", resolutions[0].Findings)
	}
}

func TestAdapterReusesButDoesNotOwnExistingImport(t *testing.T) {
	r := adapterRoot(t)
	path := filepath.Join(r.Path(), "CLAUDE.md")
	mustWrite(t, path, "# Project\n@.uawp/INSTRUCTIONS.md\n")
	before, _ := os.ReadFile(path)
	addAdapter(t, r, "claude-code")
	manifest, _, err := readAdapterManifest(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Integrations) != 1 || manifest.Integrations[0].Inserted {
		t.Fatalf("existing import claimed as inserted: %#v", manifest.Integrations)
	}
	removeAdapterPlan(t, r, "claude-code")
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("project import changed: before=%q after=%q err=%v", before, after, err)
	}
}

func TestAdapterRejectsDuplicateImports(t *testing.T) {
	r := adapterRoot(t)
	mustWrite(t, filepath.Join(r.Path(), "CLAUDE.md"), "@.uawp/INSTRUCTIONS.md\n@.uawp/INSTRUCTIONS.md\n")
	if _, _, err := PlanAdapterAddAt(r, "claude-code", adapter.RuntimeFacts{}, nil, time.Unix(1, 0)); err == nil {
		t.Fatal("accepted duplicate imports")
	}
}

func TestAdapterAddMigratesConsumerToEffectiveEntry(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	addAdapter(t, r, "workbuddy")
	mustWrite(t, filepath.Join(r.Path(), "CODEBUDDY.md"), "# Preferred\n")
	p, resolution, err := PlanAdapterAddAt(r, "workbuddy", adapter.RuntimeFacts{}, nil, time.Unix(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Route.Path != "CODEBUDDY.md" || len(p.Changes()) != 3 {
		t.Fatalf("resolution=%#v changes=%#v", resolution, p.Changes())
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	agents, _ := os.ReadFile(filepath.Join(r.Path(), "AGENTS.md"))
	codebuddy, _ := os.ReadFile(filepath.Join(r.Path(), "CODEBUDDY.md"))
	if !bytes.Contains(agents, []byte("consumers=codex")) || bytes.Contains(agents, []byte("workbuddy")) || !bytes.Contains(codebuddy, []byte("consumers=workbuddy")) {
		t.Fatalf("AGENTS=%q CODEBUDDY=%q", agents, codebuddy)
	}
	manifest, _, _ := readAdapterManifest(r)
	if got := len(consumerArtifactIndexes(manifest.Integrations, "workbuddy")); got != 1 {
		t.Fatalf("workbuddy registrations=%d", got)
	}
}

func TestAdapterVerificationAllowsOutsideManagedBlockEdits(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	path := filepath.Join(r.Path(), "AGENTS.md")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("\n# project addition\n")
	_ = f.Close()
	p, _, err := PlanAdapterAddAt(r, "codex", adapter.RuntimeFacts{}, nil, time.Unix(2, 0))
	if err != nil || len(p.Changes()) != 0 {
		t.Fatalf("idempotent plan=%#v err=%v", p.Changes(), err)
	}
	if err := VerifyAdapterRoute(r, "codex", adapter.RuntimeFacts{}); err != nil {
		t.Fatal(err)
	}
}

func TestAdapterDiagnosticsDetectCorruptRegisteredBlock(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	path := filepath.Join(r.Path(), "AGENTS.md")
	content, _ := os.ReadFile(path)
	content = bytes.Replace(content, []byte("UAWP:END"), []byte("UAWP:BROKEN"), 1)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	resolutions, err := ResolveAdapters(r, []string{"codex"}, adapter.RuntimeFacts{})
	if err != nil {
		t.Fatal(err)
	}
	if resolutions[0].Health != "INTEGRATION_DRIFT" || !hasFinding(resolutions[0].Findings, "INTEGRATION_DRIFT") {
		t.Fatalf("resolution=%#v", resolutions[0])
	}
}

func TestAdapterDiagnosticsReusePersistedRuntimeFacts(t *testing.T) {
	r := adapterRoot(t)
	addAdapter(t, r, "codex")
	facts := adapter.RuntimeFacts{Versions: map[string]string{"claude-code": "2.1.277"}, Options: map[string]map[string]string{"claude-code": {"directAgentsSupport": "true"}}}
	p, _, err := PlanAdapterAddAt(r, "claude-code", facts, nil, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	resolutions, err := ResolveAdapters(r, []string{"claude-code"}, adapter.RuntimeFacts{})
	if err != nil {
		t.Fatal(err)
	}
	if resolutions[0].Health != "HEALTHY" || resolutions[0].Route.Path != "AGENTS.md" {
		t.Fatalf("resolution=%#v", resolutions[0])
	}
}

func TestDirectAdapterLifecycleDoesNotWriteNativeEntry(t *testing.T) {
	r := adapterRoot(t)
	instructionsPath := filepath.Join(r.Path(), ".uawp", "INSTRUCTIONS.md")
	before, err := os.ReadFile(instructionsPath)
	if err != nil {
		t.Fatal(err)
	}
	p, resolution, err := planAdapterAddAt(r, "direct-test", directTestAdapter{}, adapter.RuntimeFacts{}, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Route.Mode != core.Direct || len(p.Changes()) != 1 || p.Changes()[0].Path != ".uawp/manifest.json" {
		t.Fatalf("resolution=%#v changes=%#v", resolution, p.Changes())
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	remove, err := PlanAdapterRemoveAt(r, "direct-test", adapter.RuntimeFacts{}, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(remove.Changes()) != 1 || remove.Changes()[0].Path != ".uawp/manifest.json" {
		t.Fatalf("remove changes=%#v", remove.Changes())
	}
	if _, err = Apply(r, remove, ApplyOptions{ApprovedPlanID: remove.ID}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(instructionsPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("DIRECT target changed: %v", err)
	}
}

func TestConditionalClaudeRemovalRemovesAllOwnedImportsAfterUserEdit(t *testing.T) {
	r := adapterRoot(t)
	mustWrite(t, filepath.Join(r.Path(), "AGENTS.md"), "# Existing agent rules\n")
	facts := adapter.RuntimeFacts{Options: map[string]map[string]string{"claude-code": {"directAgentsSupport": "unknown"}}}
	p, _, err := PlanAdapterAddAt(r, "claude-code", facts, []string{"CLAUDE_CREATION_CHANGES_SELECTION"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.Path(), "CLAUDE.md")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n# User Claude rule\nkeep\n")
	_ = f.Close()
	remove, err := PlanAdapterRemoveAt(r, "claude-code", adapter.RuntimeFacts{}, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, remove, ApplyOptions{ApprovedPlanID: remove.ID}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(content, []byte("# User Claude rule\nkeep")) || bytes.Contains(content, []byte("@AGENTS.md")) || bytes.Contains(content, []byte("@.uawp/INSTRUCTIONS.md")) {
		t.Fatalf("CLAUDE.md=%q err=%v", content, err)
	}
}

func TestLegacyConditionalClaudeRemovalInfersOwnedPreservationImport(t *testing.T) {
	r := adapterRoot(t)
	mustWrite(t, filepath.Join(r.Path(), "AGENTS.md"), "# Existing agent rules\n")
	facts := adapter.RuntimeFacts{Options: map[string]map[string]string{"claude-code": {"directAgentsSupport": "unknown"}}}
	p, _, err := PlanAdapterAddAt(r, "claude-code", facts, []string{"CLAUDE_CREATION_CHANGES_SELECTION"}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, p, ApplyOptions{ApprovedPlanID: p.ID}); err != nil {
		t.Fatal(err)
	}
	manifest, _, err := readAdapterManifest(r)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Integrations[0].InsertedImports = nil
	legacy, err := core.EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Path(), ".uawp", "manifest.json"), legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.Path(), "CLAUDE.md")
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("\n# User Claude rule\nkeep\n")
	_ = f.Close()
	remove, err := PlanAdapterRemoveAt(r, "claude-code", adapter.RuntimeFacts{}, time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Apply(r, remove, ApplyOptions{ApprovedPlanID: remove.ID}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(content, []byte("# User Claude rule\nkeep")) || bytes.Contains(content, []byte("@AGENTS.md")) || bytes.Contains(content, []byte("@.uawp/INSTRUCTIONS.md")) {
		t.Fatalf("legacy CLAUDE.md=%q err=%v", content, err)
	}
}
