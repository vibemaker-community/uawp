package core

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

const releasedOwnership = "# UAWP Active Worker\n\n- Status: RELEASED\n- Worker ID: worker-a\n- Agent: Test Agent\n- Acquired At: 2026-09-21T10:00:00+08:00\n- Released At: 2026-09-21T11:00:00+08:00\n- Purpose: test work\n"

func TestDecodeOwnershipRoundTripAndCRLF(t *testing.T) {
	value, err := DecodeOwnership(strings.NewReader(strings.ReplaceAll(releasedOwnership, "\n", "\r\n")))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeOwnership(value)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != releasedOwnership {
		t.Fatalf("encoded=%q", encoded)
	}
}

func TestDecodeOwnershipRejectsAmbiguousOrInvalidDocuments(t *testing.T) {
	cases := map[string]string{
		"duplicate":               releasedOwnership + "- Status: ACTIVE\n",
		"missing":                 strings.Replace(releasedOwnership, "- Worker ID: worker-a\n", "", 1),
		"unknown status":          strings.Replace(releasedOwnership, "RELEASED", "STALE", 1),
		"conflicting release":     strings.Replace(releasedOwnership, "2026-09-21T11:00:00+08:00", "none", 1),
		"duplicate after heading": releasedOwnership + "\n## Later\n- Worker ID: worker-b\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeOwnership(strings.NewReader(input)); err == nil {
				t.Fatal("accepted invalid document")
			}
		})
	}
	if _, err := DecodeOwnership(bytes.NewReader(bytes.Repeat([]byte("x"), (1<<20)+1))); err == nil {
		t.Fatal("accepted oversized document")
	}
}

func TestEncodeOwnershipRejectsInvalidValue(t *testing.T) {
	if _, err := EncodeOwnership(Ownership{}); err == nil {
		t.Fatal("encoded invalid ownership")
	}
	now := time.Date(2026, 9, 21, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	value := Ownership{Status: Active, WorkerID: "w", Agent: "a", AcquiredAt: now, Purpose: "p"}
	if _, err := EncodeOwnership(value); err != nil {
		t.Fatal(err)
	}
}

func TestEncodeOwnershipRejectsMultilineFields(t *testing.T) {
	now := time.Now()
	for _, value := range []Ownership{
		{Status: Active, WorkerID: "w\n- Status: RELEASED", Agent: "a", AcquiredAt: now, Purpose: "p"},
		{Status: Active, WorkerID: "w", Agent: "a\rbroken", AcquiredAt: now, Purpose: "p"},
		{Status: Active, WorkerID: "w", Agent: "a", AcquiredAt: now, Purpose: "p\ninjected"},
	} {
		if _, err := EncodeOwnership(value); err == nil {
			t.Fatal("encoded multiline field")
		}
	}
}
