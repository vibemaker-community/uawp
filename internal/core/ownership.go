package core

import (
	"regexp"
	"strings"
	"time"
)

type OwnershipStatus string

const (
	Active   OwnershipStatus = "ACTIVE"
	Released OwnershipStatus = "RELEASED"
)

type Ownership struct {
	Status     OwnershipStatus
	WorkerID   string
	SessionID  string
	Generation uint64
	Agent      string
	AcquiredAt time.Time
	ReleasedAt *time.Time
	Purpose    string
}

type Actor struct {
	WorkerID   string
	SessionID  string
	Generation uint64
}

func ValidateOwnership(value Ownership) error {
	if value.Status != Active && value.Status != Released {
		return newDomainError(ErrInvalidState, "unknown ownership status %q", value.Status)
	}
	if strings.TrimSpace(value.WorkerID) == "" {
		return newDomainError(ErrInvalidState, "worker ID is required")
	}
	sessionID := strings.TrimSpace(value.SessionID)
	if strings.EqualFold(sessionID, "none") {
		return newDomainError(ErrInvalidState, "session ID uses reserved value none")
	}
	if value.Status == Active && (sessionID == "" || value.Generation == 0) {
		return newDomainError(ErrInvalidState, "ACTIVE ownership requires a session ID and positive generation")
	}
	if value.Status == Released && ((sessionID == "") != (value.Generation == 0)) {
		return newDomainError(ErrInvalidState, "RELEASED ownership session ID and generation must both be present or absent")
	}
	if strings.TrimSpace(value.Agent) == "" {
		return newDomainError(ErrInvalidState, "agent is required")
	}
	if strings.TrimSpace(value.Purpose) == "" {
		return newDomainError(ErrInvalidState, "purpose is required")
	}
	for name, field := range map[string]string{"worker ID": value.WorkerID, "session ID": value.SessionID, "agent": value.Agent, "purpose": value.Purpose} {
		if strings.ContainsAny(field, "\r\n") || len(field) > 4096 {
			return newDomainError(ErrInvalidState, "%s must be a single line of at most 4096 bytes", name)
		}
	}
	if value.AcquiredAt.IsZero() {
		return newDomainError(ErrInvalidState, "acquisition time is required")
	}

	switch value.Status {
	case Active:
		if value.ReleasedAt != nil {
			return newDomainError(ErrInvalidState, "ACTIVE ownership cannot have a release time")
		}
	case Released:
		if value.ReleasedAt == nil {
			return newDomainError(ErrInvalidState, "RELEASED ownership requires a release time")
		}
		if value.ReleasedAt.Before(value.AcquiredAt) {
			return newDomainError(ErrInvalidState, "release time precedes acquisition time")
		}
	}
	return nil
}

func ValidateActiveActor(current Ownership, actor Actor) error {
	if err := ValidateOwnership(current); err != nil {
		return err
	}
	workerID := strings.TrimSpace(actor.WorkerID)
	sessionID := strings.TrimSpace(actor.SessionID)
	if current.Status != Active {
		return newDomainError(ErrInvalidState, "ownership is not ACTIVE")
	}
	if workerID == "" || sessionID == "" || actor.Generation == 0 {
		return newDomainError(ErrInvalidState, "complete ACTIVE actor tuple is required")
	}
	if current.WorkerID != workerID || current.SessionID != sessionID || current.Generation != actor.Generation {
		return newDomainError(ErrInvalidState, "actor does not own the ACTIVE claim")
	}
	return nil
}

var timezoneSuffix = regexp.MustCompile(`(?:Z|[+-][0-9]{2}:[0-9]{2})$`)

func ParseTimestamp(value string) (time.Time, error) {
	if !timezoneSuffix.MatchString(value) {
		return time.Time{}, newDomainError(ErrInvalidState, "timestamp must include an explicit timezone")
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, newDomainError(ErrInvalidState, "invalid RFC 3339 timestamp %q: %v", value, err)
	}
	return parsed, nil
}

func FormatTimestamp(value time.Time) string {
	return value.Format(time.RFC3339)
}
