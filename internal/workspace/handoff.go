package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type HandoffRequest struct {
	Actor        core.Actor
	FinalContext []byte
	Purpose      string
	At           time.Time
}

func PlanHandoffAt(root Root, request HandoffRequest) (plan.Plan, error) {
	if status := Status(root); status.Code != CodeActiveOwner {
		return plan.Plan{}, fmt.Errorf("workspace blocks handoff: %s", status.Observed)
	}
	worker := strings.TrimSpace(request.Actor.WorkerID)
	purpose := strings.TrimSpace(request.Purpose)
	if worker == "" || purpose == "" || len(request.FinalContext) == 0 || len(request.FinalContext) > maxContextBytes {
		return plan.Plan{}, fmt.Errorf("invalid handoff request")
	}
	ownerPath := filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md")
	ownerBytes, err := os.ReadFile(ownerPath)
	if err != nil {
		return plan.Plan{}, err
	}
	owner, err := core.DecodeOwnership(strings.NewReader(string(ownerBytes)))
	if err != nil {
		return plan.Plan{}, err
	}
	next, err := core.Release(owner, core.ReleaseRequest{Actor: request.Actor, At: request.At})
	if err != nil {
		return plan.Plan{}, err
	}
	nextBytes, err := core.EncodeOwnership(next)
	if err != nil {
		return plan.Plan{}, err
	}
	contextPath := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	beforeContext, err := os.ReadFile(contextPath)
	if err != nil {
		return plan.Plan{}, err
	}
	changes := []plan.Change{
		plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(beforeContext), request.FinalContext).WithSequence(10),
		plan.NewUpdateFile(".uawp/ACTIVE_WORKER.md", 0o600, plan.HashBytes(ownerBytes), nextBytes).WithSequence(20),
	}
	inputs := []plan.Input{{Path: ".uawp/CONTEXT.md", SHA256: plan.HashBytes(beforeContext)}, {Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(ownerBytes)}}
	return plan.NewForWorkspaceInputsMetadata("handoff", root.Path(), changes, inputs, plan.Metadata{ActorWorkerID: worker, ActorSessionID: strings.TrimSpace(request.Actor.SessionID), OwnershipGeneration: request.Actor.Generation, Reason: purpose}), nil
}
