package releasecontract

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var actionReference = regexp.MustCompile(`(?m)^\s*-?\s*uses:\s*([^\s@]+)@([^\s#]+)`)

func TestWorkflowSecurityContract(t *testing.T) {
	root := repositoryRoot(t)
	files, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("workflow discovery: files=%v err=%v", files, err)
	}
	for _, filename := range files {
		filename := filename
		t.Run(filepath.Base(filename), func(t *testing.T) {
			data, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if strings.Contains(text, "pull_request_target") {
				t.Fatal("pull_request_target is prohibited")
			}
			if !strings.Contains(text, "permissions:\n  contents: read") {
				t.Fatal("workflow must default to contents: read")
			}
			for _, match := range actionReference.FindAllStringSubmatch(text, -1) {
				if strings.HasPrefix(match[1], "./") {
					continue
				}
				if ok, _ := regexp.MatchString(`^[0-9a-f]{40}$`, match[2]); !ok {
					t.Errorf("action %s must use a full commit SHA, got %s", match[1], match[2])
				}
			}
			for _, prohibited := range []string{"self-hosted", "xlarge", "larger", "github.event.pull_request.title", "github.event.pull_request.body", "github.head_ref", "github.ref_name"} {
				if strings.Contains(strings.ToLower(text), strings.ToLower(prohibited)) {
					t.Errorf("workflow contains prohibited value %q", prohibited)
				}
			}
			assertJobTimeouts(t, text)
			assertWritePermissionsAreReleaseOnly(t, filepath.Base(filename), text)
		})
	}
}

func TestSecurityWorkflowsUsePinnedAnalyzers(t *testing.T) {
	root := repositoryRoot(t)
	want := map[string]string{
		"codeql.yml":            "github/codeql-action/init@1c5b675653bb5c22dbe9b12b556ec555138e09fd",
		"dependency-review.yml": "actions/dependency-review-action@2031cfc080254a8a887f58cffee85186f0e49e48",
		"govulncheck.yml":       "golang/govulncheck-action@032d45514ae346b1db93c04b0c90b841c370344f",
	}
	for name, action := range want {
		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), "uses: "+action) {
			t.Errorf("%s does not use pinned analyzer %s", name, action)
		}
	}
}

func TestWritePermissionBoundary(t *testing.T) {
	for _, permission := range []string{"contents: write", "id-token: write", "attestations: write"} {
		t.Run(permission, func(t *testing.T) {
			unsafe := "permissions:\n  contents: read\njobs:\n  build:\n    permissions:\n      " + permission + "\n    timeout-minutes: 1\n"
			if violations := writePermissionViolations("release.yml", unsafe); len(violations) == 0 {
				t.Fatalf("non-publish job accepted %s", permission)
			}

			safe := "permissions:\n  contents: read\njobs:\n  publish:\n    permissions:\n      " + permission + "\n    timeout-minutes: 1\n"
			if violations := writePermissionViolations("release.yml", safe); len(violations) != 0 {
				t.Fatalf("publish job rejected %s: %v", permission, violations)
			}
		})
	}

	pullRequest := "on:\n  pull_request:\npermissions:\n  contents: read\njobs:\n  publish:\n    permissions:\n      contents: write\n    timeout-minutes: 1\n"
	if violations := writePermissionViolations("release.yml", pullRequest); len(violations) == 0 {
		t.Fatal("pull-request publish job accepted write permission")
	}
}

func TestNativeMatrixUsesExactSupportedRunners(t *testing.T) {
	root := repositoryRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"macos-15", "macos-15-intel", "ubuntu-24.04", "ubuntu-24.04-arm", "windows-2025"}
	var got []string
	for _, runner := range want {
		if strings.Contains(string(data), "runner: "+runner) {
			got = append(got, runner)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("native runners = %v, want %v", got, want)
	}
}

func assertJobTimeouts(t *testing.T, workflow string) {
	t.Helper()
	inJobs := false
	jobHasTimeout := true
	jobName := ""
	for _, line := range strings.Split(workflow, "\n") {
		if line == "jobs:" {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(line, ":") {
			if jobName != "" && !jobHasTimeout {
				t.Errorf("job %s has no timeout-minutes", jobName)
			}
			jobName = strings.TrimSuffix(strings.TrimSpace(line), ":")
			jobHasTimeout = false
		}
		if jobName != "" && strings.HasPrefix(strings.TrimSpace(line), "timeout-minutes:") {
			jobHasTimeout = true
		}
	}
	if jobName != "" && !jobHasTimeout {
		t.Errorf("job %s has no timeout-minutes", jobName)
	}
}

func assertWritePermissionsAreReleaseOnly(t *testing.T, filename, workflow string) {
	t.Helper()
	for _, violation := range writePermissionViolations(filename, workflow) {
		t.Error(violation)
	}
}

func writePermissionViolations(filename, workflow string) []string {
	prohibited := map[string]bool{
		"contents: write":     true,
		"id-token: write":     true,
		"attestations: write": true,
	}
	isPullRequest := strings.Contains(workflow, "pull_request:")
	inJobs := false
	jobName := ""
	var violations []string
	for _, line := range strings.Split(workflow, "\n") {
		if line == "jobs:" {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(line, ":") {
			jobName = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		permission := strings.TrimSpace(line)
		if !prohibited[permission] {
			continue
		}
		if filename != "release.yml" || jobName != "publish" {
			violations = append(violations, permission+" is allowed only in release.yml job publish")
		}
		if isPullRequest {
			violations = append(violations, "pull-request workflow must not request "+permission)
		}
	}
	return violations
}
