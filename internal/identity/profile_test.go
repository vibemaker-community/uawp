package identity

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateDisplayNameAcceptsArbitraryLabelsAndTrims(t *testing.T) {
	for _, value := range []string{"Sushi 的 Gemini", "abc123", "🤖"} {
		got, err := ValidateDisplayName("  " + value + "  ")
		if err != nil || got != value {
			t.Fatalf("ValidateDisplayName(%q) = %q, %v", value, got, err)
		}
	}
}

func TestValidateDisplayNameRejectsUnsafeLabels(t *testing.T) {
	for _, value := range []string{"", "   ", "line\nbreak", "bad\x00name", strings.Repeat("界", 101)} {
		if _, err := ValidateDisplayName(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestNewProfileUsesIndependentRandomIdentifiers(t *testing.T) {
	random := bytes.NewReader(append(bytes.Repeat([]byte{0x11}, 16), bytes.Repeat([]byte{0x22}, 16)...))
	profile, err := NewProfile("  duplicate label  ", random)
	if err != nil {
		t.Fatal(err)
	}
	if profile.ProfileID != "profile-11111111111111111111111111111111" {
		t.Fatalf("profile ID = %q", profile.ProfileID)
	}
	if profile.WorkerID != "worker-22222222222222222222222222222222" {
		t.Fatalf("worker ID = %q", profile.WorkerID)
	}
	if profile.DisplayName != "duplicate label" {
		t.Fatalf("display name = %q", profile.DisplayName)
	}
}

func TestNewProfileRejectsShortRandomInput(t *testing.T) {
	if _, err := NewProfile("worker", bytes.NewReader(make([]byte, 20))); err == nil {
		t.Fatal("created profile from short random input")
	}
}
