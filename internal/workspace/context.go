package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

const maxContextBytes = 1 << 20

func PlanContextSync(root Root, actor core.Actor, content []byte) (plan.Plan, error) {
	if status := Status(root); status.Code != CodeActiveOwner {
		return plan.Plan{}, fmt.Errorf("workspace blocks context sync: %s", status.Observed)
	}
	if len(content) == 0 || len(content) > maxContextBytes {
		return plan.Plan{}, fmt.Errorf("context must contain 1 to 1048576 bytes")
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
	if err := core.ValidateActiveActor(owner, actor); err != nil {
		return plan.Plan{}, err
	}
	contextPath := filepath.Join(root.Path(), ".uawp", "CONTEXT.md")
	before, err := os.ReadFile(contextPath)
	if err != nil {
		return plan.Plan{}, err
	}
	inputs := []plan.Input{{Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(ownerBytes)}}
	meta := plan.Metadata{ActorWorkerID: strings.TrimSpace(actor.WorkerID), ActorSessionID: strings.TrimSpace(actor.SessionID), OwnershipGeneration: actor.Generation}
	if string(before) == string(content) {
		return plan.NewForWorkspaceInputsMetadata("context-sync", root.Path(), nil, inputs, meta), nil
	}
	change := plan.NewUpdateFile(".uawp/CONTEXT.md", 0o600, plan.HashBytes(before), content)
	return plan.NewForWorkspaceInputsMetadata("context-sync", root.Path(), []plan.Change{change}, inputs, meta), nil
}
