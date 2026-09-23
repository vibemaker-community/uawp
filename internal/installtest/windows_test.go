package installtest

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPowerShellInstallerParses(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil && runtime.GOOS == "windows" {
		pwsh, err = exec.LookPath("powershell")
	}
	if err != nil {
		t.Skip("PowerShell is verified on the native Windows CI runner")
	}
	root := filepath.Clean(filepath.Join("..", ".."))
	command := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", "$errors=$null; [System.Management.Automation.Language.Parser]::ParseFile($args[0],[ref]$null,[ref]$errors) > $null; if($errors.Count){exit 1}", filepath.Join(root, "install", "install.ps1"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell syntax: %v %s", err, output)
	}
}
