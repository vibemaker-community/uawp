package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type CheckpointRequest struct {
	WorkerID, MilestoneID, Label string
	DecisionReferences           []string
	At                           time.Time
}

var milestonePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func PlanCheckpointAt(root Root, request CheckpointRequest) (plan.Plan, error) {
	worker := strings.TrimSpace(request.WorkerID)
	id := strings.TrimSpace(request.MilestoneID)
	label := strings.TrimSpace(request.Label)
	if worker == "" || label == "" || len(id) > 64 || !milestonePattern.MatchString(id) {
		return plan.Plan{}, fmt.Errorf("invalid checkpoint request")
	}
	ownerBytes, err := os.ReadFile(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		return plan.Plan{}, err
	}
	owner, err := core.DecodeOwnership(strings.NewReader(string(ownerBytes)))
	if err != nil {
		return plan.Plan{}, err
	}
	if owner.Status != core.Active || owner.WorkerID != worker {
		return plan.Plan{}, fmt.Errorf("worker %q is not ACTIVE owner", worker)
	}
	context, err := os.ReadFile(filepath.Join(root.Path(), ".uawp", "CONTEXT.md"))
	if err != nil {
		return plan.Plan{}, err
	}
	stamp := request.At.Format("20060102T150405Z07")
	name := stamp + "-" + id + ".md"
	logical := ".uawp/checkpoints/" + name
	target, err := root.ResolveUAWP("checkpoints/" + name)
	if err != nil {
		return plan.Plan{}, err
	}
	if _, err := os.Lstat(target); err == nil {
		return plan.Plan{}, fmt.Errorf("checkpoint already exists")
	} else if !os.IsNotExist(err) {
		return plan.Plan{}, err
	}
	refs := "none"
	if len(request.DecisionReferences) > 0 {
		refs = strings.Join(request.DecisionReferences, ", ")
	}
	body := fmt.Sprintf("# UAWP Checkpoint\n\n- Milestone: %s\n- Created At: %s\n- Worker ID: %s\n- Agent: %s\n- Context SHA256: %s\n- Decision References: %s\n\n## Context\n\n%s", label, core.FormatTimestamp(request.At), worker, owner.Agent, plan.HashBytes(context), refs, context)
	change := plan.NewFile(logical, 0o600, plan.MissingSHA256, []byte(body))
	inputs := []plan.Input{{Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(ownerBytes)}, {Path: ".uawp/CONTEXT.md", SHA256: plan.HashBytes(context)}}
	return plan.NewForWorkspaceInputsMetadata("checkpoint", root.Path(), []plan.Change{change}, inputs, plan.Metadata{ActorWorkerID: worker}), nil
}
