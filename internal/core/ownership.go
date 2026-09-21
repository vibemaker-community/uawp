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
	Agent      string
	AcquiredAt time.Time
	ReleasedAt *time.Time
	Purpose    string
}

func ValidateOwnership(value Ownership) error {
	if value.Status != Active && value.Status != Released {
		return newDomainError(ErrInvalidState, "unknown ownership status %q", value.Status)
	}
	if strings.TrimSpace(value.WorkerID) == "" {
		return newDomainError(ErrInvalidState, "worker ID is required")
	}
	if strings.TrimSpace(value.Agent) == "" {
		return newDomainError(ErrInvalidState, "agent is required")
	}
	if strings.TrimSpace(value.Purpose) == "" {
		return newDomainError(ErrInvalidState, "purpose is required")
	}
	for name, field := range map[string]string{"worker ID": value.WorkerID, "agent": value.Agent, "purpose": value.Purpose} {
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
