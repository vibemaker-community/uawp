package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
)

type ResumeOutcome string

const (
	ResumeAcquireAvailable  ResumeOutcome = "ACQUIRE_AVAILABLE"
	ResumeOwnedByCaller     ResumeOutcome = "OWNED_BY_CALLER"
	ResumeBlockedByOther    ResumeOutcome = "BLOCKED_BY_OTHER"
	ResumeBlockedBySession  ResumeOutcome = "BLOCKED_BY_OTHER_SESSION"
	ResumeExpiredGeneration ResumeOutcome = "EXPIRED_GENERATION"
)

type ResumeReport struct {
	Outcome    ResumeOutcome  `json:"outcome"`
	Owner      core.Ownership `json:"owner"`
	NextAction string         `json:"nextAction"`
}

func PlanAcquireAt(root Root, request core.AcquireRequest) (plan.Plan, error) {
	actor := core.Actor{WorkerID: strings.TrimSpace(request.WorkerID), SessionID: strings.TrimSpace(request.SessionID)}
	return planOwnershipTransition(root, "acquire", actor, func(current core.Ownership) (core.Ownership, error) { return core.Acquire(current, request) })
}

func PlanReleaseAt(root Root, request core.ReleaseRequest) (plan.Plan, error) {
	return planOwnershipTransition(root, "release", request.Actor, func(current core.Ownership) (core.Ownership, error) { return core.Release(current, request) })
}

func planOwnershipTransition(root Root, operation string, actor core.Actor, transition func(core.Ownership) (core.Ownership, error)) (plan.Plan, error) {
	status := Status(root)
	if status.Code != CodeReady && status.Code != CodeActiveOwner {
		return plan.Plan{}, fmt.Errorf("workspace blocks %s: %s", operation, status.Observed)
	}
	path := filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md")
	before, err := os.ReadFile(path)
	if err != nil {
		return plan.Plan{}, err
	}
	current, err := core.DecodeOwnership(strings.NewReader(string(before)))
	if err != nil {
		return plan.Plan{}, err
	}
	next, err := transition(current)
	if err != nil {
		return plan.Plan{}, err
	}
	if actor.Generation == 0 {
		actor.Generation = next.Generation
	}
	after, err := core.EncodeOwnership(next)
	if err != nil {
		return plan.Plan{}, err
	}
	change := plan.NewUpdateFile(".uawp/ACTIVE_WORKER.md", 0o600, plan.HashBytes(before), after)
	inputs := []plan.Input{{Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(before)}}
	metadata := plan.Metadata{ActorWorkerID: strings.TrimSpace(actor.WorkerID), ActorSessionID: strings.TrimSpace(actor.SessionID), OwnershipGeneration: actor.Generation}
	return plan.NewForWorkspaceInputsMetadata(operation, root.Path(), []plan.Change{change}, inputs, metadata), nil
}

func Resume(root Root, actor core.Actor) (ResumeReport, error) {
	actor.WorkerID = strings.TrimSpace(actor.WorkerID)
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	if actor.WorkerID == "" || actor.SessionID == "" {
		return ResumeReport{}, fmt.Errorf("worker ID and session ID are required")
	}
	status := Status(root)
	if status.Code != CodeReady && status.Code != CodeActiveOwner {
		return ResumeReport{}, fmt.Errorf("workspace is not resumable: %s", status.Observed)
	}
	owner, err := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		return ResumeReport{}, err
	}
	if owner.Status == core.Released {
		return ResumeReport{Outcome: ResumeAcquireAvailable, Owner: owner, NextAction: "Preview ownership acquisition."}, nil
	}
	if owner.WorkerID != actor.WorkerID {
		return ResumeReport{Outcome: ResumeBlockedByOther, Owner: owner, NextAction: "Remain read-only while another worker is ACTIVE."}, nil
	}
	if owner.SessionID != actor.SessionID {
		return ResumeReport{Outcome: ResumeBlockedBySession, Owner: owner, NextAction: "Remain read-only while another Session is ACTIVE."}, nil
	}
	if owner.Generation != actor.Generation {
		return ResumeReport{Outcome: ResumeExpiredGeneration, Owner: owner, NextAction: "Refresh ownership; this Session generation is no longer current."}, nil
	}
	if owner.WorkerID == actor.WorkerID {
		return ResumeReport{Outcome: ResumeOwnedByCaller, Owner: owner, NextAction: "Resume work and revalidate ownership before shared writes."}, nil
	}
	return ResumeReport{Outcome: ResumeBlockedByOther, Owner: owner, NextAction: "Remain read-only while another worker is ACTIVE."}, nil
}
