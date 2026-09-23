package releasecontract

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var markdownLink = regexp.MustCompile(`\[[^]]+\]\(([^)]+)\)`)
var testableShell = regexp.MustCompile("(?s)<!-- testable-shell -->\\s*```(?:sh|bash)\\n(.*?)```")

func TestPublicDocumentationContract(t *testing.T) {
	root := repositoryRoot(t)
	files := []string{
		"README.md", "CHANGELOG.md", "ROADMAP.md", "docs/architecture.md",
		"docs/supported-platforms.md", "docs/verify-release.md",
		"docs/agent-integration.md", "docs/release-gates.md",
	}
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("required public document %s: %v", name, err)
		}
	}

	readme := readContractFile(t, filepath.Join(root, "README.md"))
	for _, required := range []string{
		"https://github.com/vibemaker-community/uawp", "GPL-3.0-only", ".uawp/",
		"Agent-neutral Core", "DIRECT", "IMPORT", "MANAGED_BLOCK", "Single Active Worker",
		"Human Controller", "init", "resume", "sync", "checkpoint", "handoff", "uninstall",
		"ordinary user", "expert", "Agent automation", "CLA.md", "TRADEMARKS.md", "SECURITY.md", "SUPPORT.md",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README missing %q", required)
		}
	}
	if strings.Contains(readme, "Uawp") {
		t.Error("brand must be spelled UAWP")
	}

	platforms := readContractFile(t, filepath.Join(root, "docs", "supported-platforms.md"))
	for _, target := range []string{"macOS arm64", "macOS amd64", "Linux arm64", "Linux amd64", "Windows amd64"} {
		if !strings.Contains(platforms, target) {
			t.Errorf("supported-platforms missing %q", target)
		}
	}
}

func TestPublicDocumentationInternalLinksResolve(t *testing.T) {
	root := repositoryRoot(t)
	files := append([]string{filepath.Join(root, "README.md")}, publicMarkdownFiles(t, filepath.Join(root, "docs"))...)
	for _, filename := range files {
		text := readContractFile(t, filename)
		for _, match := range markdownLink.FindAllStringSubmatch(text, -1) {
			target := strings.Split(match[1], "#")[0]
			if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(filename), filepath.FromSlash(target))); err != nil {
				t.Errorf("%s: link %q does not resolve", filepath.Clean(strings.TrimPrefix(filename, root+string(filepath.Separator))), match[1])
			}
		}
	}
}

func TestMarkedShellExamplesExecute(t *testing.T) {
	root := repositoryRoot(t)
	binary := filepath.Join(t.TempDir(), "uawp")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-o", binary, "./cmd/uawp")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	files := append([]string{filepath.Join(root, "README.md")}, publicMarkdownFiles(t, filepath.Join(root, "docs"))...)
	count := 0
	for _, filename := range files {
		for _, match := range testableShell.FindAllStringSubmatch(readContractFile(t, filename), -1) {
			count++
			workspace := t.TempDir()
			run := exec.Command("sh", "-eu", "-c", match[1])
			run.Dir = root
			run.Env = append(os.Environ(), "UAWP_BIN="+binary, "WORKSPACE="+workspace)
			if output, err := run.CombinedOutput(); err != nil {
				t.Errorf("%s testable shell block failed: %v\n%s", filepath.Base(filename), err, output)
			}
		}
	}
	if count == 0 {
		t.Fatal("no testable shell examples found")
	}
}

func publicMarkdownFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".md") && !strings.Contains(path, string(filepath.Separator)+"superpowers"+string(filepath.Separator)) && !strings.Contains(path, string(filepath.Separator)+"handoffs"+string(filepath.Separator)) {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func readContractFile(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
