package migration

import (
	"bytes"
	"testing"
	"time"

	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
)

const legacyReleasedOwnership = "# UAWP Active Worker\n\n- Status: RELEASED\n- Worker ID: worker-a\n- Agent: Agent A\n- Acquired At: 2026-09-21T10:00:00+08:00\n- Released At: 2026-09-21T11:00:00+08:00\n- Purpose: completed\n"

func TestV1_1ToV1_2AddsReleasedSessionFields(t *testing.T) {
	step := V1_1ToV1_2{}
	changes, err := step.Plan(Context{
		Manifest:    core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.1.0"},
		Inputs:      []plan.Input{{Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes([]byte(legacyReleasedOwnership))}},
		Files:       map[string][]byte{".uawp/ACTIVE_WORKER.md": []byte(legacyReleasedOwnership)},
		GeneratedAt: time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if step.From() != "1.1.0" || step.To() != "1.2.0" || len(changes) != 1 {
		t.Fatalf("step=%s->%s changes=%#v", step.From(), step.To(), changes)
	}
	content := changes[0].Content()
	for _, want := range [][]byte{[]byte("- Session ID: none\n"), []byte("- Generation: 0\n")} {
		if !bytes.Contains(content, want) {
			t.Fatalf("ownership missing %q: %s", want, content)
		}
	}
}

func TestV1_1ToV1_2RejectsActiveLegacyOwnership(t *testing.T) {
	active := bytes.ReplaceAll([]byte(legacyReleasedOwnership), []byte("RELEASED"), []byte("ACTIVE"))
	active = bytes.ReplaceAll(active, []byte("2026-09-21T11:00:00+08:00"), []byte("none"))
	_, err := (V1_1ToV1_2{}).Plan(Context{
		Manifest: core.Manifest{Protocol: core.ProtocolName, StateVersion: "1.1.0"},
		Inputs:   []plan.Input{{Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(active)}},
		Files:    map[string][]byte{".uawp/ACTIVE_WORKER.md": active},
	})
	if err == nil {
		t.Fatal("migration accepted ACTIVE legacy ownership")
	}
}
