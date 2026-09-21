package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type ResumeOutcome string

const (
	ResumeAcquireAvailable ResumeOutcome = "ACQUIRE_AVAILABLE"
	ResumeOwnedByCaller    ResumeOutcome = "OWNED_BY_CALLER"
	ResumeBlockedByOther   ResumeOutcome = "BLOCKED_BY_OTHER"
)

type ResumeReport struct {
	Outcome    ResumeOutcome  `json:"outcome"`
	Owner      core.Ownership `json:"owner"`
	NextAction string         `json:"nextAction"`
}

func PlanAcquireAt(root Root, request core.AcquireRequest) (plan.Plan, error) {
	return planOwnershipTransition(root, "acquire", strings.TrimSpace(request.WorkerID), func(current core.Ownership) (core.Ownership, error) { return core.Acquire(current, request) })
}

func PlanReleaseAt(root Root, request core.ReleaseRequest) (plan.Plan, error) {
	return planOwnershipTransition(root, "release", strings.TrimSpace(request.WorkerID), func(current core.Ownership) (core.Ownership, error) { return core.Release(current, request) })
}

func planOwnershipTransition(root Root, operation, actor string, transition func(core.Ownership) (core.Ownership, error)) (plan.Plan, error) {
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
	after, err := core.EncodeOwnership(next)
	if err != nil {
		return plan.Plan{}, err
	}
	change := plan.NewUpdateFile(".uawp/ACTIVE_WORKER.md", 0o600, plan.HashBytes(before), after)
	return plan.NewForWorkspaceInputsMetadata(operation, root.Path(), []plan.Change{change}, nil, plan.Metadata{ActorWorkerID: actor}), nil
}

func Resume(root Root, workerID string) (ResumeReport, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return ResumeReport{}, fmt.Errorf("worker ID is required")
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
	if owner.WorkerID == workerID {
		return ResumeReport{Outcome: ResumeOwnedByCaller, Owner: owner, NextAction: "Resume work and revalidate ownership before shared writes."}, nil
	}
	return ResumeReport{Outcome: ResumeBlockedByOther, Owner: owner, NextAction: "Remain read-only while another worker is ACTIVE."}, nil
}
