package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/uawp/uawp/internal/plan"
)

func TestPlanInitAbsentNamespace(t *testing.T) {
	dir := t.TempDir()
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := planInitAt(root, time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	changes := value.Changes()
	if len(changes) != 6 {
		t.Fatalf("len(changes) = %d, want 6", len(changes))
	}
	want := map[string]plan.ChangeKind{
		".uawp":                  plan.CreateDir,
		".uawp/checkpoints":      plan.CreateDir,
		".uawp/manifest.json":    plan.CreateFile,
		".uawp/CONTEXT.md":       plan.CreateFile,
		".uawp/ACTIVE_WORKER.md": plan.CreateFile,
		".uawp/DECISIONS.md":     plan.CreateFile,
	}
	for _, change := range changes {
		kind, ok := want[change.Path]
		if !ok || kind != change.Kind {
			t.Fatalf("unexpected change: %#v", change)
		}
		delete(want, change.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing changes: %#v", want)
	}
}

func TestPlanInitRejectsManifestOnlyNamespace(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".uawp"))
	mustWrite(t, filepath.Join(dir, ".uawp", "manifest.json"), `{"protocol":"UAWP","stateVersion":"1.0.0"}`)
	root, _ := OpenRoot(dir)
	if _, err := PlanInit(root); err == nil {
		t.Fatal("PlanInit accepted manifest-only namespace")
	}
}

func TestPlanInitPreservesBrownfieldFiles(t *testing.T) {
	dir := t.TempDir()
	for path, content := range map[string]string{
		"context.md": "project context",
		"AGENTS.md":  "project rules",
		"CLAUDE.md":  "project rules",
		"src/app.go": "package app",
	} {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		mustWrite(t, full, content)
	}
	root, _ := OpenRoot(dir)
	value, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range value.Changes() {
		if filepath.Clean(change.Path) == "context.md" || change.Path == "AGENTS.md" || change.Path == "CLAUDE.md" || change.Path == "src/app.go" {
			t.Fatalf("planned project-owned change: %#v", change)
		}
	}
}

func TestPlanInitRejectsUnknownNamespace(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".uawp"))
	root, _ := OpenRoot(dir)
	if _, err := PlanInit(root); err == nil {
		t.Fatal("PlanInit() accepted unknown .uawp namespace")
	}
}
