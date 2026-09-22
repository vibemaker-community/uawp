package core

import (
	"strings"
	"time"
)

type RecoveryRequest struct {
	ControllerID, Reason string
	At                   time.Time
}
type RecoveryAudit struct {
	ControllerID, Reason, OldWorkerID, OldSessionID, OldAgent string
	OldGeneration                                             uint64
	OldAcquiredAt, RecoveredAt                                time.Time
}

func RecoverStaleOwnership(current Ownership, request RecoveryRequest) (Ownership, RecoveryAudit, error) {
	if err := ValidateOwnership(current); err != nil {
		return Ownership{}, RecoveryAudit{}, err
	}
	controller, reason := strings.TrimSpace(request.ControllerID), strings.TrimSpace(request.Reason)
	if current.Status != Active || controller == "" || reason == "" || request.At.IsZero() || request.At.Before(current.AcquiredAt) {
		return Ownership{}, RecoveryAudit{}, newDomainError(ErrInvalidState, "invalid stale recovery")
	}
	next := current
	next.Status = Released
	released := request.At
	next.ReleasedAt = &released
	if err := ValidateOwnership(next); err != nil {
		return Ownership{}, RecoveryAudit{}, err
	}
	audit := RecoveryAudit{ControllerID: controller, Reason: reason, OldWorkerID: current.WorkerID, OldSessionID: current.SessionID, OldGeneration: current.Generation, OldAgent: current.Agent, OldAcquiredAt: current.AcquiredAt, RecoveredAt: request.At}
	return next, audit, nil
}
