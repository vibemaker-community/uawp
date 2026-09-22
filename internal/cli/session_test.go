package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionNewReturnsOneJSONDocumentWithoutWorkspaceAccess(t *testing.T) {
	rt, stdout, stderr, _ := identityRuntime(t, "")
	rt.getwd = func() (string, error) { t.Fatal("session new accessed workspace"); return "", nil }
	if code := runWithRuntime([]string{"session", "new", "--format", "json"}, rt); code != exitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	var result struct {
		SchemaVersion, Command, SessionID, NextAction string
		Mutated                                       bool
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("not one JSON document: %v: %s", err, stdout.String())
	}
	if result.SchemaVersion != "1" || result.Command != "session new" || !strings.HasPrefix(result.SessionID, "session-") || result.Mutated {
		t.Fatalf("result=%#v", result)
	}
	if bytes.Count(stdout.Bytes(), []byte("{")) != 1 {
		t.Fatalf("multiple JSON documents: %s", stdout.String())
	}
}
