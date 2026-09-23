package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
)

func PlanStaleRecoveryAt(root Root, request core.RecoveryRequest) (plan.Plan, error) {
	if status := Status(root); status.Code != CodeActiveOwner {
		return plan.Plan{}, fmt.Errorf("stale recovery requires ACTIVE ownership: %s", status.Observed)
	}
	ownerBytes, err := os.ReadFile(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		return plan.Plan{}, err
	}
	owner, err := core.DecodeOwnership(strings.NewReader(string(ownerBytes)))
	if err != nil {
		return plan.Plan{}, err
	}
	next, audit, err := core.RecoverStaleOwnership(owner, request)
	if err != nil {
		return plan.Plan{}, err
	}
	nextBytes, err := core.EncodeOwnership(next)
	if err != nil {
		return plan.Plan{}, err
	}
	decisionsPath := filepath.Join(root.Path(), ".uawp", "DECISIONS.md")
	decisions, err := os.ReadFile(decisionsPath)
	if err != nil {
		return plan.Plan{}, err
	}
	entry := fmt.Sprintf("\n## Human Controller stale-claim recovery — %s\n\n- Controller ID: %s\n- Reason: %s\n- Old Worker ID: %s\n- Old Session ID: %s\n- Old Generation: %d\n- Old Agent: %s\n- Old Acquired At: %s\n- Recovered At: %s\n", core.FormatTimestamp(audit.RecoveredAt), audit.ControllerID, audit.Reason, audit.OldWorkerID, audit.OldSessionID, audit.OldGeneration, audit.OldAgent, core.FormatTimestamp(audit.OldAcquiredAt), core.FormatTimestamp(audit.RecoveredAt))
	changes := []plan.Change{
		plan.NewUpdateFile(".uawp/DECISIONS.md", 0o600, plan.HashBytes(decisions), append(append([]byte(nil), decisions...), []byte(entry)...)).WithSequence(10),
		plan.NewUpdateFile(".uawp/ACTIVE_WORKER.md", 0o600, plan.HashBytes(ownerBytes), nextBytes).WithSequence(20),
	}
	inputs := []plan.Input{{Path: ".uawp/DECISIONS.md", SHA256: plan.HashBytes(decisions)}, {Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(ownerBytes)}}
	meta := plan.Metadata{Reason: audit.Reason, ControllerID: audit.ControllerID}
	return plan.NewForWorkspaceInputsMetadata("stale-recovery", root.Path(), changes, inputs, meta), nil
}
