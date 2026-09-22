package identity

import (
	"bytes"
	"testing"
)

func TestNewSessionIDUsesSixteenRandomBytes(t *testing.T) {
	got, err := NewSessionID(bytes.NewReader(bytes.Repeat([]byte{0xab}, 16)))
	if err != nil {
		t.Fatal(err)
	}
	if got != "session-abababababababababababababababab" {
		t.Fatalf("session ID = %q", got)
	}
}

func TestNewSessionIDRejectsShortRandomInput(t *testing.T) {
	if _, err := NewSessionID(bytes.NewReader(make([]byte, 15))); err == nil {
		t.Fatal("created Session ID from short random input")
	}
}
