package workspace

import (
	"fmt"
	"strings"
	"time"

	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
	state "github.com/vibemaker-community/uawp/templates/state"
)

func PlanInit(root Root) (plan.Plan, error) {
	return planInitAt(root, time.Now())
}

// PlanInitAt reconstructs the exact initialization plan for an approved time.
func PlanInitAt(root Root, generatedAt time.Time) (plan.Plan, error) {
	return planInitAt(root, generatedAt)
}

func planInitAt(root Root, now time.Time) (plan.Plan, error) {
	inventory, err := Discover(root)
	if err != nil {
		return plan.Plan{}, err
	}
	switch inventory.Namespace {
	case NamespaceOwned:
		report := Status(root)
		if report.Code != CodeReady && report.Code != CodeActiveOwner {
			return plan.Plan{}, fmt.Errorf("cannot initialize incomplete UAWP namespace: %s", report.Observed)
		}
		return plan.NewForWorkspace("init", root.Path(), nil), nil
	case NamespaceUnknown, NamespaceInvalid:
		return plan.Plan{}, fmt.Errorf("cannot initialize %s namespace: %s", inventory.Namespace, inventory.ManifestError)
	case NamespaceAbsent:
	default:
		return plan.Plan{}, fmt.Errorf("unrecognized namespace classification %q", inventory.Namespace)
	}

	manifest, err := core.EncodeManifest(core.Manifest{Protocol: core.ProtocolName, StateVersion: core.CurrentStateVersion})
	if err != nil {
		return plan.Plan{}, err
	}
	timestamp := core.FormatTimestamp(now)
	activeWorker := []byte(strings.ReplaceAll(string(state.ActiveWorker), "{{TIMESTAMP}}", timestamp))
	releasedAt := now
	if err := core.ValidateOwnership(core.Ownership{
		Status: core.Released, WorkerID: "uawp-bootstrap", Agent: "UAWP",
		AcquiredAt: now, ReleasedAt: &releasedAt, Purpose: "Initialize UAWP workspace state",
	}); err != nil {
		return plan.Plan{}, err
	}

	changes := []plan.Change{
		plan.NewDirectory(".uawp", 0o700),
		plan.NewDirectory(".uawp/checkpoints", 0o700),
		plan.NewDirectory(".uawp/migrations", 0o700),
		plan.NewDirectory(".uawp/recovery", 0o700),
		plan.NewFile(".uawp/manifest.json", 0o600, plan.MissingSHA256, manifest),
		plan.NewFile(".uawp/CONTEXT.md", 0o600, plan.MissingSHA256, state.Context),
		plan.NewFile(".uawp/ACTIVE_WORKER.md", 0o600, plan.MissingSHA256, activeWorker),
		plan.NewFile(".uawp/DECISIONS.md", 0o600, plan.MissingSHA256, state.Decisions),
		plan.NewFile(".uawp/INSTRUCTIONS.md", 0o600, plan.MissingSHA256, state.Instructions),
	}
	inputs, err := ProjectInputs(root)
	if err != nil {
		return plan.Plan{}, err
	}
	return plan.NewForWorkspaceInputs("init", root.Path(), changes, inputs), nil
}
