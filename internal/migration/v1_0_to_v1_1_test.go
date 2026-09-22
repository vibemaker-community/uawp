package migration

import (
	"testing"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

func TestV1_0ToV1_1PlansDirectoriesOnly(t *testing.T) {
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
	if step.From() != "1.0.0" || step.To() != "1.1.0" || len(changes) != 2 {
		t.Fatalf("step=%s->%s changes=%#v", step.From(), step.To(), changes)
	}
	for _, change := range changes {
		if change.Kind != plan.CreateDir {
			t.Fatalf("migration step published non-directory change: %#v", change)
		}
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
	if len(changes) != 1 || changes[0].Path != ".uawp/migrations" {
		t.Fatalf("changes=%#v", changes)
	}
}
