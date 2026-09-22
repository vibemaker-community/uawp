package adapter

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiscoveryClassifiesNativeInputs(t *testing.T) {
	root := t.TempDir()
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("regular.md", []byte("hello"))
	write("binary.md", []byte{'a', 0, 'b'})
	write("invalid.md", []byte{0xff})
	write("large.md", []byte(strings.Repeat("x", maxNativeBytes+1)))
	if err := os.Mkdir(filepath.Join(root, "directory.md"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("regular.md", filepath.Join(root, "link.md")); err != nil {
		t.Skip(err)
	}
	s, err := DiscoverSnapshot(root, []string{"missing.md", "regular.md", "binary.md", "invalid.md", "large.md", "directory.md", "link.md"}, "", map[string]string{"setting": ""})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]FileState{"missing.md": FileMissing, "regular.md": FileRegular, "binary.md": FileBinary, "invalid.md": FileInvalidUTF8, "large.md": FileTooLarge, "directory.md": FileUnsafe, "link.md": FileUnsafe}
	for path, state := range want {
		if s.Files[path].State != state {
			t.Errorf("%s=%s want %s", path, s.Files[path].State, state)
		}
	}
	if s.Options["setting"] != "" {
		t.Fatal("option changed")
	}
}

func TestDiscoveryRejectsSymlinkedCandidateAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires elevated privileges")
	}
	root, outside := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "api"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "api", "AGENTS.md"), []byte("external secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "services")); err != nil {
		t.Skip(err)
	}
	snapshot, err := DiscoverSnapshot(root, []string{"services/api/AGENTS.md"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	fact := snapshot.Files["services/api/AGENTS.md"]
	if fact.State != FileUnsafe || len(fact.Content) != 0 {
		t.Fatalf("fact=%#v", fact)
	}
}

func TestDiscoveryRejectsEscapingCandidate(t *testing.T) {
	if _, err := DiscoverSnapshot(t.TempDir(), []string{"../escape"}, "", nil); err == nil {
		t.Fatal("accepted traversal")
	}
}
