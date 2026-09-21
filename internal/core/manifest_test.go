package core

import (
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
		{"future major", `{"protocol":"UAWP","stateVersion":"2.0.0"}`, true},
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
