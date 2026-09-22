package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/uawp/uawp/internal/core"
)

func TestDiscoverClassifiesNamespace(t *testing.T) {
	tests := []struct {
		name string
		set  func(t *testing.T, root string)
		want NamespaceKind
	}{
		{"absent", func(t *testing.T, root string) {}, NamespaceAbsent},
		{"owned", func(t *testing.T, root string) {
			mustMkdir(t, filepath.Join(root, ".uawp"))
			mustWrite(t, filepath.Join(root, ".uawp", "manifest.json"), `{"protocol":"UAWP","stateVersion":"1.0.0"}`)
		}, NamespaceOwned},
		{"unknown directory", func(t *testing.T, root string) {
			mustMkdir(t, filepath.Join(root, ".uawp"))
		}, NamespaceUnknown},
		{"regular file", func(t *testing.T, root string) {
			mustWrite(t, filepath.Join(root, ".uawp"), "not a directory")
		}, NamespaceUnknown},
		{"invalid manifest", func(t *testing.T, root string) {
			mustMkdir(t, filepath.Join(root, ".uawp"))
			mustWrite(t, filepath.Join(root, ".uawp", "manifest.json"), `{"protocol":"other","stateVersion":"1.0.0"}`)
		}, NamespaceInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.set(t, dir)
			root, err := OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Discover(root)
			if err != nil {
				t.Fatal(err)
			}
			if got.Namespace != tt.want {
				t.Fatalf("Namespace = %q, want %q", got.Namespace, tt.want)
			}
		})
	}
}

func TestDiscoverVersionCompatibility(t *testing.T) {
	for version, want := range map[string]core.StateCompatibility{
		"1.0.0":  core.StateUpgradeRequired,
		"1.1.0":  core.StateUpgradeRequired,
		"1.2.0":  core.StateCurrent,
		"1.99.0": core.StateUnsupported,
		"2.0.0":  core.StateFutureMajor,
	} {
		t.Run(version, func(t *testing.T) {
			dir := t.TempDir()
			mustMkdir(t, filepath.Join(dir, ".uawp"))
			mustWrite(t, filepath.Join(dir, ".uawp", "manifest.json"), `{"protocol":"UAWP","stateVersion":"`+version+`"}`)
			root, err := OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			inventory, err := Discover(root)
			if err != nil {
				t.Fatal(err)
			}
			if inventory.Namespace != NamespaceOwned || inventory.StateCompatibility != want {
				t.Fatalf("Discover(%s) = namespace %s compatibility %s, want OWNED %s", version, inventory.Namespace, inventory.StateCompatibility, want)
			}
		})
	}
}

func TestDiscoverReportsRootContextWithoutMutation(t *testing.T) {
	dir := t.TempDir()
	contextPath := filepath.Join(dir, "context.md")
	mustWrite(t, contextPath, "project-owned\n")
	before, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if !inventory.RootContextPresent {
		t.Fatal("root context.md was not reported")
	}
	after, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("discovery mutated project context.md")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
