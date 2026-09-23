package installtest

import (
	"archive/zip"
	"bytes"
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

func TestPowerShellInstallerParses(t *testing.T) {
	pwsh := powerShell(t)
	if pwsh == "" {
		return
	}
	root := filepath.Clean(filepath.Join("..", ".."))
	command := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", "$tokens=$null; $errors=$null; [System.Management.Automation.Language.Parser]::ParseFile($env:UAWP_PS_PARSE_FILE,[ref]$tokens,[ref]$errors) > $null; if($errors.Count){$errors | ForEach-Object { Write-Error $_.Message }; exit 1}")
	command.Env = append(os.Environ(), "UAWP_PS_PARSE_FILE="+filepath.Join(root, "install", "install.ps1"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell syntax: %v %s", err, output)
	}
}

func TestPowerShellInstallerBehavior(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows installer behavior runs on windows-2025")
	}
	pwsh := powerShell(t)
	binary := releaseBinary(t)
	valid := windowsArchive(t, binary)
	t.Run("valid force and preservation", func(t *testing.T) {
		server := windowsReleaseServer(t, valid, fmt.Sprintf("%x", sha256.Sum256(valid)), nil)
		dest := t.TempDir()
		if output, err := runPowerShellInstaller(pwsh, server.URL, dest, false); err != nil {
			t.Fatalf("valid install: %v\n%s", err, output)
		}
		if output, err := runPowerShellInstaller(pwsh, server.URL, dest, false); err == nil {
			t.Fatalf("existing install overwritten: %s", output)
		}
		if output, err := runPowerShellInstaller(pwsh, server.URL, dest, true); err != nil {
			t.Fatalf("forced install: %v\n%s", err, output)
		}
	})
	for _, test := range []struct {
		name     string
		archive  []byte
		checksum string
	}{
		{"bad checksum", valid, strings.Repeat("0", 64)},
		{"traversal", windowsArchive(t, binary, "../escape"), "auto"},
		{"duplicate", windowsArchive(t, binary, "uawp.exe"), "auto"},
		{"invalid executable", windowsArchive(t, []byte("invalid")), "auto"},
	} {
		t.Run(test.name, func(t *testing.T) {
			checksum := test.checksum
			if checksum == "auto" {
				checksum = fmt.Sprintf("%x", sha256.Sum256(test.archive))
			}
			server := windowsReleaseServer(t, test.archive, checksum, nil)
			dest := t.TempDir()
			path := filepath.Join(dest, "uawp.exe")
			_ = os.WriteFile(path, []byte("old"), 0o600)
			if output, err := runPowerShellInstaller(pwsh, server.URL, dest, true); err == nil {
				t.Fatalf("unsafe archive accepted: %s", output)
			}
			got, _ := os.ReadFile(path)
			if string(got) != "old" {
				t.Fatal("existing binary changed")
			}
		})
	}
	t.Run("arrival race", func(t *testing.T) {
		dest := t.TempDir()
		path := filepath.Join(dest, "uawp.exe")
		server := windowsReleaseServer(t, valid, fmt.Sprintf("%x", sha256.Sum256(valid)), func() { _ = os.WriteFile(path, []byte("racing"), 0o600) })
		if output, err := runPowerShellInstaller(pwsh, server.URL, dest, false); err == nil {
			t.Fatalf("race overwritten: %s", output)
		}
		got, _ := os.ReadFile(path)
		if string(got) != "racing" {
			t.Fatal("racing binary changed")
		}
	})
}

func powerShell(t *testing.T) string {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	if err != nil && runtime.GOOS == "windows" {
		pwsh, err = exec.LookPath("powershell")
	}
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Fatal("PowerShell is required on the Windows gate")
		}
		t.Skip("PowerShell syntax is verified on the native Windows gate")
		return ""
	}
	return pwsh
}

func runPowerShellInstaller(pwsh, baseURL, dest string, force bool) (string, error) {
	root := filepath.Clean(filepath.Join("..", ".."))
	args := []string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(root, "install", "install.ps1"), "-Version", "1.0.0", "-Destination", dest}
	if force {
		args = append(args, "-Force")
	}
	command := exec.Command(pwsh, args...)
	command.Env = append(os.Environ(), "UAWP_INSTALL_TESTING=1", "UAWP_INSTALL_BASE_URL="+baseURL)
	output, err := command.CombinedOutput()
	return string(output), err
}

func windowsReleaseServer(t *testing.T, archive []byte, checksum string, hook func()) *httptest.Server {
	t.Helper()
	asset := "uawp_1.0.0_windows_amd64.zip"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch filepath.Base(request.URL.Path) {
		case asset:
			if hook != nil {
				hook()
				hook = nil
			}
			_, _ = writer.Write(archive)
		case "uawp_1.0.0_checksums.txt":
			fmt.Fprintf(writer, "%s  %s\n", checksum, asset)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func windowsArchive(t *testing.T, binary []byte, extras ...string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	entries := []struct {
		name string
		body []byte
	}{{"uawp.exe", binary}, {"LICENSE", []byte("license")}, {"NOTICE", []byte("notice")}, {"README.md", []byte("readme")}, {"THIRD_PARTY_NOTICES.md", []byte("third party")}, {"TRADEMARKS.md", []byte("trademark")}, {"release-metadata.json", []byte(`{"version":"1.0.0"}`)}}
	for _, extra := range extras {
		entries = append(entries, struct {
			name string
			body []byte
		}{extra, []byte("extra")})
	}
	for _, entry := range entries {
		part, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
