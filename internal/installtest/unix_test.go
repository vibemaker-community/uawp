package installtest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUnixInstallerValidatesBeforeAtomicInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	binary := releaseBinary(t)
	archive := unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, binary}})
	server := releaseServer(t, "1.0.0", archive, fmt.Sprintf("%x", sha256.Sum256(archive)))
	dest := t.TempDir()

	result := runUnixInstaller(t, server.URL, dest)

	if result.err != nil {
		t.Fatalf("install failed: %v\n%s", result.err, result.output)
	}
	installed, err := os.ReadFile(filepath.Join(dest, "uawp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, binary) {
		t.Fatal("installed binary differs from verified release binary")
	}
	if !strings.Contains(result.output, filepath.Join(dest, "uawp")) {
		t.Fatalf("output does not name installation path: %s", result.output)
	}
}

func TestUnixInstallerRejectsUnsafeInputsAndPreservesExistingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	valid := unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, []byte("new")}})
	tests := []struct {
		name     string
		archive  []byte
		checksum string
	}{
		{"wrong checksum", valid, strings.Repeat("0", 64)},
		{"missing checksum", valid, ""},
		{"traversal", unixArchive(t, []tarEntry{{"../uawp", tar.TypeReg, []byte("bad")}}), "auto"},
		{"symlink", unixArchive(t, []tarEntry{{"uawp", tar.TypeSymlink, nil}}), "auto"},
		{"duplicate binary", unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, []byte("a")}, {"uawp", tar.TypeReg, []byte("b")}}), "auto"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checksum := test.checksum
			if checksum == "auto" {
				checksum = fmt.Sprintf("%x", sha256.Sum256(test.archive))
			}
			server := releaseServer(t, "1.0.0", test.archive, checksum)
			dest := t.TempDir()
			if err := os.WriteFile(filepath.Join(dest, "uawp"), []byte("old"), 0o700); err != nil {
				t.Fatal(err)
			}
			result := runUnixInstaller(t, server.URL, dest, "--force")
			if result.err == nil {
				t.Fatalf("unsafe input accepted: %s", result.output)
			}
			got, err := os.ReadFile(filepath.Join(dest, "uawp"))
			if err != nil || string(got) != "old" {
				t.Fatalf("existing binary changed: %q err=%v", got, err)
			}
		})
	}
}

func TestUnixInstallerRequiresExplicitForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	binary := releaseBinary(t)
	archive := unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, binary}})
	server := releaseServer(t, "1.0.0", archive, fmt.Sprintf("%x", sha256.Sum256(archive)))
	dest := t.TempDir()
	path := filepath.Join(dest, "uawp")
	if err := os.WriteFile(path, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if result := runUnixInstaller(t, server.URL, dest); result.err == nil {
		t.Fatal("existing binary overwritten without --force")
	}
	if result := runUnixInstaller(t, server.URL, dest, "--force"); result.err != nil {
		t.Fatalf("forced install failed: %v %s", result.err, result.output)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, binary) {
		t.Fatal("forced content differs from verified binary")
	}
}

func TestUnixInstallerRejectsInvalidExecutableAndPreservesExisting(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	archive := unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, []byte("not an executable")}})
	server := releaseServer(t, "1.0.0", archive, fmt.Sprintf("%x", sha256.Sum256(archive)))
	dest := t.TempDir()
	path := filepath.Join(dest, "uawp")
	os.WriteFile(path, []byte("old"), 0o700)
	if result := runUnixInstaller(t, server.URL, dest, "--force"); result.err == nil {
		t.Fatalf("invalid executable installed: %s", result.output)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Fatalf("existing binary changed to %q", got)
	}
}

func TestUnixInstallerRaceDoesNotOverwriteWithoutForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	binary := releaseBinary(t)
	archive := unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, binary}})
	dest := t.TempDir()
	path := filepath.Join(dest, "uawp")
	server := releaseServerWithHook(t, "1.0.0", archive, fmt.Sprintf("%x", sha256.Sum256(archive)), func() {
		_ = os.WriteFile(path, []byte("racing install"), 0o700)
	})
	if result := runUnixInstaller(t, server.URL, dest); result.err == nil {
		t.Fatal("racing installation overwritten without --force")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "racing install" {
		t.Fatalf("racing binary changed to %q", got)
	}
}

func TestUnixInstallerRejectsUnsupportedPlatform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	archive := unixArchive(t, []tarEntry{{"uawp", tar.TypeReg, []byte("new")}})
	server := releaseServer(t, "1.0.0", archive, fmt.Sprintf("%x", sha256.Sum256(archive)))
	dest := t.TempDir()
	root := filepath.Clean(filepath.Join("..", ".."))
	command := exec.Command("sh", filepath.Join(root, "install", "install.sh"), "--version", "1.0.0", "--dest", dest)
	command.Env = append(os.Environ(), "UAWP_INSTALL_TESTING=1", "UAWP_INSTALL_BASE_URL="+server.URL, "UAWP_INSTALL_TEST_PLATFORM=Plan9:mips")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unsupported platform accepted: %s", output)
	}
}

func TestUnixInstallerNetworkFailurePreservesExistingBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix installer")
	}
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	dest := t.TempDir()
	path := filepath.Join(dest, "uawp")
	if err := os.WriteFile(path, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if result := runUnixInstaller(t, url, dest, "--force"); result.err == nil {
		t.Fatal("network failure accepted")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "old" {
		t.Fatalf("existing binary changed to %q", got)
	}
}

type installResult struct {
	output string
	err    error
}

func runUnixInstaller(t *testing.T, baseURL, dest string, extra ...string) installResult {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	args := []string{filepath.Join(root, "install", "install.sh"), "--version", "1.0.0", "--dest", dest}
	args = append(args, extra...)
	command := exec.Command("sh", args...)
	command.Env = append(os.Environ(), "UAWP_INSTALL_TESTING=1", "UAWP_INSTALL_BASE_URL="+baseURL)
	output, err := command.CombinedOutput()
	return installResult{string(output), err}
}

func releaseServer(t *testing.T, version string, archive []byte, checksum string) *httptest.Server {
	return releaseServerWithHook(t, version, archive, checksum, nil)
}

func releaseServerWithHook(t *testing.T, version string, archive []byte, checksum string, hook func()) *httptest.Server {
	t.Helper()
	asset := fmt.Sprintf("uawp_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch filepath.Base(request.URL.Path) {
		case asset:
			if hook != nil {
				hook()
				hook = nil
			}
			_, _ = writer.Write(archive)
		case "uawp_" + version + "_checksums.txt":
			if checksum == "" {
				http.NotFound(writer, request)
				return
			}
			fmt.Fprintf(writer, "%s  %s\n", checksum, asset)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func releaseBinary(t *testing.T) []byte {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	path := filepath.Join(t.TempDir(), "uawp")
	if runtime.GOOS == "windows" {
		path += ".exe"
	}
	ldflags := "-X github.com/vibemaker-community/uawp/internal/buildinfo.Version=1.0.0 -X github.com/vibemaker-community/uawp/internal/buildinfo.Commit=0123456789abcdef0123456789abcdef01234567 -X github.com/vibemaker-community/uawp/internal/buildinfo.BuiltAt=2026-09-23T10:54:23Z"
	command := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", path, "./cmd/uawp")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build release binary: %v\n%s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type tarEntry struct {
	name     string
	typeflag byte
	body     []byte
}

func unixArchive(t *testing.T, custom []tarEntry) []byte {
	t.Helper()
	entries := append([]tarEntry{}, custom...)
	for _, name := range []string{"LICENSE", "NOTICE", "README.md", "THIRD_PARTY_NOTICES.md", "TRADEMARKS.md", "release-metadata.json"} {
		entries = append(entries, tarEntry{name, tar.TypeReg, []byte(name)})
	}
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Typeflag: entry.typeflag, Mode: 0o644, Size: int64(len(entry.body))}
		if entry.name == "uawp" {
			header.Mode = 0o755
		}
		if entry.typeflag != tar.TypeReg {
			header.Size = 0
			header.Linkname = "uawp"
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			_, _ = tw.Write(entry.body)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
