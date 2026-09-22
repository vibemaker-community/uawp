package core

import (
	"math"
	"strings"
	"time"
)

type AcquireRequest struct {
	WorkerID, SessionID, Agent, Purpose string
	At                                  time.Time
}
type ReleaseRequest struct {
	Actor Actor
	At    time.Time
}

func Acquire(current Ownership, request AcquireRequest) (Ownership, error) {
	if err := ValidateOwnership(current); err != nil {
		return Ownership{}, err
	}
	if current.Status != Released {
		return Ownership{}, newDomainError(ErrInvalidState, "ownership is already ACTIVE")
	}
	if current.Generation == math.MaxUint64 {
		return Ownership{}, newDomainError(ErrInvalidState, "ownership generation overflow")
	}
	next := Ownership{Status: Active, WorkerID: strings.TrimSpace(request.WorkerID), SessionID: strings.TrimSpace(request.SessionID), Generation: current.Generation + 1, Agent: strings.TrimSpace(request.Agent), AcquiredAt: request.At, Purpose: strings.TrimSpace(request.Purpose)}
	if err := ValidateOwnership(next); err != nil {
		return Ownership{}, err
	}
	return next, nil
}

func Release(current Ownership, request ReleaseRequest) (Ownership, error) {
	if err := ValidateOwnership(current); err != nil {
		return Ownership{}, err
	}
	if err := ValidateActiveActor(current, request.Actor); err != nil {
		return Ownership{}, err
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
