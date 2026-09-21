package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/adapter"
)

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
	if _, _, err := PlanAdapterAddAt(r, "claude-code", facts, []string{"CLAUDE_CREATION_CHANGES_SELECTION"}, time.Unix(1, 0)); err != nil {
		t.Fatal(err)
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
