package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func identityRuntime(t *testing.T, input string) (runtime, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	config := t.TempDir()
	var stdout, stderr bytes.Buffer
	randomData := make([]byte, 256)
	for index := range randomData {
		randomData[index] = byte(index)
	}
	return runtime{
		stdin: strings.NewReader(input), stdout: &stdout, stderr: &stderr,
		getwd:         func() (string, error) { return t.TempDir(), nil },
		userConfigDir: func() (string, error) { return config, nil },
		now:           time.Now, random: bytes.NewReader(randomData),
		stdinTTY: true, stdoutTTY: true,
	}, &stdout, &stderr, config
}

func TestIdentityCommandsManageProfilesAsSingleJSONDocuments(t *testing.T) {
	rt, stdout, stderr, _ := identityRuntime(t, "")
	if code := runWithRuntime([]string{"identity", "create", "--name", "Sushi 的 Gemini", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("create code=%d stderr=%s", code, stderr.String())
	}
	var created struct {
		Profile struct {
			ProfileID, WorkerID, DisplayName string
		} `json:"profile"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("create output is not one JSON document: %v: %s", err, stdout.String())
	}
	if !strings.HasPrefix(created.Profile.ProfileID, "profile-") || !strings.HasPrefix(created.Profile.WorkerID, "worker-") || created.Profile.DisplayName != "Sushi 的 Gemini" {
		t.Fatalf("created=%#v", created)
	}

	stdout.Reset()
	if code := runWithRuntime([]string{"identity", "create", "--name", "Sushi 的 Gemini", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("duplicate label code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	if code := runWithRuntime([]string{"identity", "list", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("list code=%d stderr=%s", code, stderr.String())
	}
	var listed struct {
		Profiles []json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil || len(listed.Profiles) != 2 {
		t.Fatalf("list output=%s err=%v", stdout.String(), err)
	}

	stdout.Reset()
	if code := runWithRuntime([]string{"identity", "use", created.Profile.ProfileID, "--format", "json"}, rt); code != exitOK {
		t.Fatalf("use code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	if code := runWithRuntime([]string{"identity", "show", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("show code=%d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(created.Profile.ProfileID)) {
		t.Fatalf("show output=%s", stdout.String())
	}

	stdout.Reset()
	if code := runWithRuntime([]string{"identity", "remove", created.Profile.ProfileID, "--format", "json"}, rt); code != exitOK {
		t.Fatalf("remove code=%d stderr=%s", code, stderr.String())
	}
}

func TestIdentityRemoveBoundProfileWarnsWithoutTouchingWorkspace(t *testing.T) {
	rt, stdout, stderr, config := identityRuntime(t, "")
	if code := runWithRuntime([]string{"identity", "create", "--name", "Rock's Codex", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("create code=%d stderr=%s", code, stderr.String())
	}
	var created struct {
		Profile struct {
			ProfileID string `json:"profileID"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	workspaceDir := t.TempDir()
	marker := filepath.Join(workspaceDir, "keep.txt")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(config, "uawp", "identity.json")
	raw, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	var registry map[string]any
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	registry["bindings"] = []map[string]any{{"workspace": workspaceDir, "profileID": created.Profile.ProfileID, "sessionID": "session-existing", "generation": 1}}
	raw, _ = json.Marshal(registry)
	if err := os.WriteFile(registryPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithRuntime([]string{"identity", "remove", created.Profile.ProfileID, "--format", "json"}, rt); code != exitOK {
		t.Fatalf("remove code=%d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("warning")) {
		t.Fatalf("missing warning: %s", stderr.String())
	}
	content, err := os.ReadFile(marker)
	if err != nil || string(content) != "unchanged" {
		t.Fatalf("workspace touched: %q %v", content, err)
	}
}
