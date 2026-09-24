package releasecontract

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	canonicalModule = "github.com/vibemaker-community/uawp"
	canonicalURL    = "https://github.com/vibemaker-community/uawp"
	gpl3SHA256      = "3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986"
)

func TestCanonicalIdentity(t *testing.T) {
	root := repositoryRoot(t)

	t.Run("module and imports use canonical repository", func(t *testing.T) {
		goMod := readFile(t, filepath.Join(root, "go.mod"))
		if !strings.HasPrefix(goMod, "module "+canonicalModule+"\n") {
			t.Fatalf("go.mod must declare module %s", canonicalModule)
		}

		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".superpowers" || entry.Name() == ".worktrees") {
				return filepath.SkipDir
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			legacyModule := "github.com/" + "uawp/uawp"
			if strings.Contains(string(contents), legacyModule) {
				return fmt.Errorf("legacy module import in %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("license is unmodified GPL version 3", func(t *testing.T) {
		license := []byte(readFile(t, filepath.Join(root, "LICENSE")))
		got := fmt.Sprintf("%x", sha256.Sum256(license))
		if got != gpl3SHA256 {
			t.Fatalf("LICENSE SHA-256 = %s, want %s", got, gpl3SHA256)
		}
	})

	t.Run("notice identifies copyright owner and brand", func(t *testing.T) {
		notice := readFile(t, filepath.Join(root, "NOTICE"))
		for _, required := range []string{"Copyright 2026 Li Rui（李锐）", "Vibemaker™"} {
			if !strings.Contains(notice, required) {
				t.Errorf("NOTICE missing %q", required)
			}
		}
	})

	t.Run("public release surface uses canonical URL and pending mark", func(t *testing.T) {
		var foundCanonical bool
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if entry.IsDir() && excludedReleaseDirectory(rel) {
				return filepath.SkipDir
			}
			if entry.IsDir() || !isPublicText(path) {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(contents)
			if strings.Contains(text, canonicalURL) {
				foundCanonical = true
			}
			legacyURL := "https://github.com/" + "uawp/uawp"
			if strings.Contains(text, legacyURL) {
				return fmt.Errorf("legacy public repository URL in %s", rel)
			}
			registeredMark := "Vibemaker" + "®"
			if strings.Contains(text, registeredMark) {
				return fmt.Errorf("registered mark used before registration in %s", rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !foundCanonical {
			t.Fatalf("public release surface must link to %s", canonicalURL)
		}
	})
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test filename")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func TestExcludedReleaseDirectory(t *testing.T) {
	for _, rel := range []string{".git", ".superpowers", ".worktrees", "bin", filepath.Join("docs", "superpowers"), filepath.Join("docs", "handoffs")} {
		if !excludedReleaseDirectory(rel) {
			t.Errorf("excludedReleaseDirectory(%q) = false, want true", rel)
		}
	}
}

func excludedReleaseDirectory(rel string) bool {
	return rel == ".git" || rel == ".superpowers" || rel == ".worktrees" || rel == "bin" ||
		rel == filepath.Join("docs", "superpowers") || rel == filepath.Join("docs", "handoffs")
}

func isPublicText(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".md", ".json", ".yml", ".yaml":
		return true
	default:
		return filepath.Base(path) == "LICENSE" || filepath.Base(path) == "NOTICE"
	}
}
