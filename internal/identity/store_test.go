package identity

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func sampleRegistry() Registry {
	return Registry{
		SchemaVersion:    "1",
		DefaultProfileID: "profile-a",
		Profiles: []Profile{
			{ProfileID: "profile-a", WorkerID: "worker-a", DisplayName: "same"},
			{ProfileID: "profile-b", WorkerID: "worker-b", DisplayName: "same"},
		},
	}
}

func TestSaveLoadRegistryIsAtomicAndPrivate(t *testing.T) {
	path := DefaultPath(t.TempDir())
	want := sampleRegistry()
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%#o", info.Mode().Perm())
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultProfileID != "profile-a" || len(got.Profiles) != 2 {
		t.Fatalf("registry=%#v", got)
	}
	want.DefaultProfileID = "profile-b"
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.DefaultProfileID != "profile-b" {
		t.Fatalf("replacement=%#v err=%v", got, err)
	}
}

func TestLoadRejectsMalformedAmbiguousOrUnsafeRegistry(t *testing.T) {
	tests := map[string]string{
		"truncated":     `{"schemaVersion":"1"`,
		"unknown field": `{"schemaVersion":"1","profiles":[],"extra":true}`,
		"trailing data": `{"schemaVersion":"1","profiles":[]} {}`,
		"duplicate key": `{"schemaVersion":"1","schemaVersion":"1","profiles":[]}`,
		"duplicate IDs": `{"schemaVersion":"1","profiles":[{"profileID":"p","workerID":"w1","displayName":"a"},{"profileID":"p","workerID":"w2","displayName":"b"}]}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "identity.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("loaded invalid registry")
			}
		})
	}
}

func TestLoadRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"schemaVersion":"1","profiles":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "identity.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(link); err == nil {
		t.Fatal("loaded symlinked registry")
	}
}

func TestSelectedAndBindingsUseIDsNotDisplayNames(t *testing.T) {
	registry := sampleRegistry()
	selected, err := registry.Selected("", true)
	if err != nil || selected.ProfileID != "profile-a" {
		t.Fatalf("selected=%#v err=%v", selected, err)
	}
	if _, err := registry.Selected("", false); err == nil {
		t.Fatal("automation selected implicit default")
	}
	if _, err := registry.Selected("same", true); err == nil {
		t.Fatal("selected profile by duplicate display name")
	}
	binding := SessionBinding{Workspace: "/workspace", ProfileID: "profile-a", SessionID: "session-a", Generation: 3}
	registry.UpsertBinding(binding)
	got, ok := registry.Binding("/workspace", "profile-a")
	if !ok || got != binding {
		t.Fatalf("binding=%#v ok=%t", got, ok)
	}
	registry.UpsertBinding(SessionBinding{Workspace: "/workspace", ProfileID: "profile-a", SessionID: "session-b", Generation: 4})
	if len(registry.Bindings) != 1 || registry.Bindings[0].SessionID != "session-b" {
		t.Fatalf("bindings=%#v", registry.Bindings)
	}
	registry.RemoveBinding("/workspace", "profile-a")
	if _, ok := registry.Binding("/workspace", "profile-a"); ok {
		t.Fatal("binding not removed")
	}
}

func TestSaveFailurePreservesExistingRegistry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions vary on Windows")
	}
	path := DefaultPath(t.TempDir())
	if err := Save(path, sampleRegistry()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Dir(path)
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	changed := sampleRegistry()
	changed.Profiles[0].DisplayName = strings.Repeat("x", 20)
	if err := Save(path, changed); err == nil {
		t.Skip("current user can write despite directory mode")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("failed save changed existing registry")
	}
}
