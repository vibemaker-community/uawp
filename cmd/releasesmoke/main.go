// Command releasesmoke exercises an extracted UAWP binary through a complete
// workspace lifecycle. It is used only by release verification workflows.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type preview struct {
	PlanID string `json:"planID"`
}

func main() {
	binary := flag.String("binary", "", "path to the extracted native uawp binary")
	workspace := flag.String("workspace", "", "empty or brownfield smoke-test workspace")
	flag.Parse()
	if *binary == "" || *workspace == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "--binary and --workspace are required")
		os.Exit(2)
	}
	if err := smoke(*binary, *workspace); err != nil {
		fmt.Fprintln(os.Stderr, "release smoke failed:", err)
		os.Exit(1)
	}
	fmt.Println("packaged lifecycle smoke passed")
}

func smoke(binary, workspace string) error {
	before, err := projectHashes(workspace)
	if err != nil {
		return err
	}
	contextPath := filepath.Join(filepath.Dir(workspace), "uawp-release-smoke-context.md")
	if err := os.WriteFile(contextPath, []byte("# Release smoke context\n\nVerified.\n"), 0o600); err != nil {
		return err
	}
	defer os.Remove(contextPath)

	if err := previewApply(binary, []string{"init", "--workspace", workspace, "--format", "json", "--non-interactive"}); err != nil {
		return err
	}
	actor := []string{"--workspace", workspace, "--worker-id", "release-worker", "--session-id", "release-session", "--format", "json", "--non-interactive"}
	if err := previewApply(binary, append([]string{"acquire"}, append(actor, "--agent", "Release Smoke", "--purpose", "verify packaged lifecycle")...)); err != nil {
		return err
	}
	if _, err := run(binary, []string{"status", "--workspace", workspace, "--format", "json"}, 0); err != nil {
		return err
	}
	owned := append(actor, "--generation", "1")
	if _, err := run(binary, append([]string{"resume"}, owned...), 0); err != nil {
		return err
	}
	if err := previewApply(binary, append([]string{"sync"}, append(owned, "--context-file", contextPath)...)); err != nil {
		return err
	}
	if err := previewApply(binary, append([]string{"checkpoint"}, append(owned, "--milestone-id", "release-smoke", "--label", "Release smoke")...)); err != nil {
		return err
	}
	if err := previewApply(binary, append([]string{"handoff"}, append(owned, "--purpose", "release smoke complete", "--context-file", contextPath)...)); err != nil {
		return err
	}
	if err := previewApplyOrNoop(binary, []string{"uninstall", "--workspace", workspace, "--format", "json"}); err != nil {
		return err
	}
	after, err := projectHashes(workspace)
	if err != nil {
		return err
	}
	if len(before) != len(after) {
		return fmt.Errorf("project file set changed")
	}
	for name, digest := range before {
		if after[name] != digest {
			return fmt.Errorf("project file %s changed", name)
		}
	}
	return nil
}

func projectHashes(workspace string) (map[string][32]byte, error) {
	result := make(map[string][32]byte)
	err := filepath.WalkDir(workspace, func(filename string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(workspace, filename)
		if err != nil {
			return err
		}
		if relative == ".uawp" || strings.HasPrefix(relative, ".uawp"+string(filepath.Separator)) {
			if entry.IsDir() && relative == ".uawp" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(filename)
			if err != nil {
				return err
			}
			result[relative] = sha256.Sum256(data)
		}
		return nil
	})
	return result, err
}

func previewApplyOrNoop(binary string, args []string) error {
	command := exec.Command(binary, args...)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 5 {
		return fmt.Errorf("%s exit failed: %s", args[0], output)
	}
	var result preview
	if err := json.Unmarshal(output, &result); err != nil || result.PlanID == "" {
		return fmt.Errorf("%s preview returned invalid plan: %w", args[0], err)
	}
	_, err = run(binary, append(args, "--approve", result.PlanID), 0)
	return err
}

func previewApply(binary string, args []string) error {
	output, err := run(binary, args, 5)
	if err != nil {
		return err
	}
	var result preview
	if err := json.Unmarshal(output, &result); err != nil || result.PlanID == "" {
		return fmt.Errorf("%s preview returned invalid plan: %w", args[0], err)
	}
	_, err = run(binary, append(args, "--approve", result.PlanID), 0)
	return err
}

func run(binary string, args []string, wantCode int) ([]byte, error) {
	command := exec.Command(binary, args...)
	output, err := command.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}
	if code != wantCode {
		return nil, fmt.Errorf("%s exit %d, want %d: %s", args[0], code, wantCode, output)
	}
	return output, nil
}
