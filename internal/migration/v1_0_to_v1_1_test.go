package migration

import (
	"bytes"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

func TestV1_0ToV1_1PlansDirectoriesAndManifestLast(t *testing.T) {
	before, err := core.EncodeManifest(core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	step := V1_0ToV1_1{}
	changes, err := step.Plan(Context{Manifest: core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.0.0"}, Inputs: []plan.Input{
		{Path: ".uawp/manifest.json", SHA256: plan.HashBytes(before)},
		{Path: ".uawp/migrations", SHA256: plan.MissingSHA256},
		{Path: ".uawp/recovery", SHA256: plan.MissingSHA256},
	}, GeneratedAt: time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if step.From() != "1.0.0" || step.To() != "1.1.0" || len(changes) != 3 {
		t.Fatalf("step=%s->%s changes=%#v", step.From(), step.To(), changes)
	}
	last := changes[len(changes)-1]
	if last.Path != ".uawp/manifest.json" || last.Kind != plan.UpdateFile || !bytes.Contains(last.Content(), []byte(`"stateVersion": "1.1.0"`)) {
		t.Fatalf("last change = %#v content=%s", last, last.Content())
	}
}

func TestV1_0ToV1_1PreservesIntegrationsAndExistingRecoveryDirectory(t *testing.T) {
	manifest := core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.0.0", Integrations: []core.IntegrationArtifact{{ID: "entry", Path: "AGENTS.md", Mode: core.ManagedBlock, Target: ".uawp/INSTRUCTIONS.md", Consumers: []string{"codex"}, ArtifactSHA256: plan.HashBytes([]byte("bridge"))}}}
	before, _ := core.EncodeManifest(manifest)
	changes, err := (V1_0ToV1_1{}).Plan(Context{Manifest: manifest, Inputs: []plan.Input{
		{Path: ".uawp/manifest.json", SHA256: plan.HashBytes(before)},
		{Path: ".uawp/migrations", SHA256: plan.MissingSHA256},
		{Path: ".uawp/recovery", SHA256: plan.DirectorySHA256},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 || bytes.Count(changes[1].Content(), []byte(`"codex"`)) != 1 {
		t.Fatalf("changes=%#v manifest=%s", changes, changes[len(changes)-1].Content())
	}
}
