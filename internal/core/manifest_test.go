package core

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeManifest(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid", `{"protocol":"UAWP","stateVersion":"1.0.0"}`, false},
		{"wrong owner", `{"protocol":"other","stateVersion":"1.0.0"}`, true},
		{"future major is structurally valid", `{"protocol":"UAWP","stateVersion":"2.0.0"}`, false},
		{"malformed version", `{"protocol":"UAWP","stateVersion":"v1"}`, true},
		{"duplicate protocol", `{"protocol":"UAWP","protocol":"other","stateVersion":"1.0.0"}`, true},
		{"unknown field", `{"protocol":"UAWP","stateVersion":"1.0.0","x":1}`, true},
		{"trailing value", `{"protocol":"UAWP","stateVersion":"1.0.0"} {}`, true},
		{"missing protocol", `{"stateVersion":"1.0.0"}`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeManifest(strings.NewReader(tt.input))
			if tt.wantErr && err == nil {
				t.Fatalf("DecodeManifest() = %#v, want error", got)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("DecodeManifest() error = %v", err)
			}
		})
	}
}

func TestManifestCompatibilityIsSeparateFromStructure(t *testing.T) {
	for version, want := range map[string]StateCompatibility{
		"1.0.0":  StateUpgradeRequired,
		"1.1.0":  StateUpgradeRequired,
		"1.2.0":  StateCurrent,
		"1.99.0": StateUnsupported,
		"2.0.0":  StateFutureMajor,
	} {
		manifest, err := DecodeManifest(strings.NewReader(`{"protocol":"UAWP","stateVersion":"` + version + `"}`))
		if err != nil {
			t.Fatalf("DecodeManifest(%s): %v", version, err)
		}
		if got := ClassifyStateVersion(manifest.StateVersion); got != want {
			t.Fatalf("compatibility(%s) = %s, want %s", version, got, want)
		}
	}
}

func TestManifestIntegrationRoundTripCanonicalizesCopies(t *testing.T) {
	manifest := Manifest{
		Protocol: ProtocolName, StateVersion: "1.0.0",
		Integrations: []IntegrationArtifact{
			{ID: "z-entry", Path: "CLAUDE.md", Mode: Import, Target: ".uawp/INSTRUCTIONS.md", Consumers: []string{"claude", "alpha"}, CreatedFile: true, Inserted: true, ArtifactSHA256: strings.Repeat("a", 64)},
			{ID: "a-entry", Path: "AGENTS.md", Mode: ManagedBlock, Target: ".uawp/INSTRUCTIONS.md", Consumers: []string{"workbuddy", "codex"}, ArtifactSHA256: strings.Repeat("b", 64), OutsideContentSHA256: strings.Repeat("c", 64)},
		},
	}
	before := append([]string(nil), manifest.Integrations[0].Consumers...)
	encoded, err := EncodeManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, manifest.Integrations[0].Consumers) {
		t.Fatal("EncodeManifest mutated caller")
	}
	if bytes.Index(encoded, []byte(`"id": "a-entry"`)) > bytes.Index(encoded, []byte(`"id": "z-entry"`)) {
		t.Fatal("artifacts not canonical")
	}
	got, err := DecodeManifest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if got.Integrations[0].ID != "a-entry" || !reflect.DeepEqual(got.Integrations[0].Consumers, []string{"codex", "workbuddy"}) || !got.Integrations[1].Inserted {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestManifestIntegrationValidation(t *testing.T) {
	valid := `{"protocol":"UAWP","stateVersion":"1.0.0","integrations":[{"id":"entry","path":"AGENTS.md","mode":"MANAGED_BLOCK","target":".uawp/INSTRUCTIONS.md","consumers":["codex"],"createdFile":false,"artifactSHA256":"` + strings.Repeat("a", 64) + `"}]}`
	tests := []struct{ name, input string }{
		{"unknown nested field", strings.Replace(valid, `"artifactSHA256"`, `"extra":true,"artifactSHA256"`, 1)},
		{"duplicate nested field", strings.Replace(valid, `"id":"entry"`, `"id":"entry","id":"other"`, 1)},
		{"absolute path", strings.Replace(valid, `"AGENTS.md"`, `"/AGENTS.md"`, 1)},
		{"traversal path", strings.Replace(valid, `"AGENTS.md"`, `"../AGENTS.md"`, 1)},
		{"backslash path", strings.Replace(valid, `"AGENTS.md"`, `"dir\\AGENTS.md"`, 1)},
		{"bad mode", strings.Replace(valid, `"MANAGED_BLOCK"`, `"COPY"`, 1)},
		{"bad hash", strings.Replace(valid, strings.Repeat("a", 64), "abc", 1)},
		{"empty consumer", strings.Replace(valid, `"codex"`, `""`, 1)},
		{"duplicate consumer", strings.Replace(valid, `["codex"]`, `["codex","codex"]`, 1)},
		{"duplicate artifact id", strings.Replace(valid, `]}`, `,{"id":"entry","path":"CLAUDE.md","mode":"IMPORT","target":".uawp/INSTRUCTIONS.md","consumers":["claude"],"createdFile":false,"artifactSHA256":"`+strings.Repeat("b", 64)+`"}]}`, 1)},
		{"consumer on multiple artifacts", strings.Replace(valid, `]}`, `,{"id":"other","path":"CLAUDE.md","mode":"IMPORT","target":".uawp/INSTRUCTIONS.md","consumers":["codex"],"createdFile":false,"artifactSHA256":"`+strings.Repeat("b", 64)+`"}]}`, 1)},
		{"facts for unknown consumer", strings.Replace(valid, `"createdFile":false`, `"consumerFacts":{"claude":{"providerVersion":"2.1.277"}},"createdFile":false`, 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := DecodeManifest(strings.NewReader(tt.input)); err == nil {
				t.Fatalf("DecodeManifest() = %#v, want error", got)
			}
		})
	}
	if _, err := DecodeManifest(strings.NewReader(valid)); err != nil {
		t.Fatalf("valid integration: %v", err)
	}
}
