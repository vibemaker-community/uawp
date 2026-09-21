package core

import (
	"strings"
	"time"
)

type AcquireRequest struct {
	WorkerID, Agent, Purpose string
	At                       time.Time
}
type ReleaseRequest struct {
	WorkerID string
	At       time.Time
}

func Acquire(current Ownership, request AcquireRequest) (Ownership, error) {
	if err := ValidateOwnership(current); err != nil {
		return Ownership{}, err
	}
	if current.Status != Released {
		return Ownership{}, newDomainError(ErrInvalidState, "ownership is already ACTIVE")
	}
	next := Ownership{Status: Active, WorkerID: strings.TrimSpace(request.WorkerID), Agent: strings.TrimSpace(request.Agent), AcquiredAt: request.At, Purpose: strings.TrimSpace(request.Purpose)}
	if err := ValidateOwnership(next); err != nil {
		return Ownership{}, err
	}
	return next, nil
}

func Release(current Ownership, request ReleaseRequest) (Ownership, error) {
	if err := ValidateOwnership(current); err != nil {
		return Ownership{}, err
	}
	workerID := strings.TrimSpace(request.WorkerID)
	if current.Status != Active {
		return Ownership{}, newDomainError(ErrInvalidState, "ownership is not ACTIVE")
	}
	if workerID == "" || workerID != current.WorkerID {
		return Ownership{}, newDomainError(ErrInvalidState, "worker %q does not own the ACTIVE claim", workerID)
	}
	next := current
	next.Status = Released
	released := request.At
	next.ReleasedAt = &released
	if err := ValidateOwnership(next); err != nil {
		return Ownership{}, err
	}
	return next, nil
}
