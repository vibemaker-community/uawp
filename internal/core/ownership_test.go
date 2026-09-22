package core

import (
	"testing"
	"time"
)

func TestValidateOwnership(t *testing.T) {
	acquired := time.Date(2026, 9, 21, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	released := acquired.Add(time.Hour)
	before := acquired.Add(-time.Minute)
	tests := []struct {
		name    string
		value   Ownership
		wantErr bool
	}{
		{"active", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "Agent", AcquiredAt: acquired, Purpose: "test"}, false},
		{"active cannot be released", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "Agent", AcquiredAt: acquired, ReleasedAt: &released, Purpose: "test"}, true},
		{"released needs time", Ownership{Status: Released, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired, Purpose: "test"}, true},
		{"released", Ownership{Status: Released, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired, ReleasedAt: &released, Purpose: "test"}, false},
		{"release before acquire", Ownership{Status: Released, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired, ReleasedAt: &before, Purpose: "test"}, true},
		{"unknown status", Ownership{Status: "STALE", WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired, Purpose: "test"}, true},
		{"missing purpose", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "Agent", AcquiredAt: acquired}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOwnership(tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("ValidateOwnership() succeeded, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateOwnership() error = %v", err)
			}
		})
	}
}

func TestValidateOwnershipRequiresConsistentSessionTuple(t *testing.T) {
	at := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	released := at.Add(time.Hour)
	tests := []struct {
		name    string
		value   Ownership
		wantErr bool
	}{
		{"active tuple", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "Agent A", AcquiredAt: at, Purpose: "work"}, false},
		{"active missing session", Ownership{Status: Active, WorkerID: "worker-a", Generation: 1, Agent: "Agent A", AcquiredAt: at, Purpose: "work"}, true},
		{"active zero generation", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", AcquiredAt: at, Purpose: "work"}, true},
		{"bootstrap released", Ownership{Status: Released, WorkerID: "uawp-bootstrap", Agent: "UAWP", AcquiredAt: at, ReleasedAt: &released, Purpose: "Initialize"}, false},
		{"released audit tuple", Ownership{Status: Released, WorkerID: "worker-a", SessionID: "session-a", Generation: 2, Agent: "Agent A", AcquiredAt: at, ReleasedAt: &released, Purpose: "work"}, false},
		{"released split tuple", Ownership{Status: Released, WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", AcquiredAt: at, ReleasedAt: &released, Purpose: "work"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOwnership(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v wantErr=%t", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOwnershipRejectsReservedNoneSession(t *testing.T) {
	at := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	for _, session := range []string{"none", "NONE", " None "} {
		value := Ownership{Status: Active, WorkerID: "worker-a", SessionID: session, Generation: 1, Agent: "Agent", AcquiredAt: at, Purpose: "work"}
		if err := ValidateOwnership(value); err == nil {
			t.Fatalf("accepted reserved Session ID %q", session)
		}
	}
}

func TestTimestampRoundTripPreservesOffset(t *testing.T) {
	want := time.Date(2026, 9, 21, 22, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	encoded := FormatTimestamp(want)
	got, err := ParseTimestamp(encoded)
	if err != nil {
		t.Fatal(err)
	}
	_, wantOffset := want.Zone()
	_, gotOffset := got.Zone()
	if !got.Equal(want) || gotOffset != wantOffset {
		t.Fatalf("got %s offset %d, want %s offset %d", got, gotOffset, want, wantOffset)
	}
}

func TestParseTimestampRejectsMissingTimezone(t *testing.T) {
	if _, err := ParseTimestamp("2026-09-21T22:30:00"); err == nil {
		t.Fatal("ParseTimestamp() accepted timestamp without timezone")
	}
}
