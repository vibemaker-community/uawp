package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
	state "github.com/uawp/uawp/templates/state"
)

func PlanRepairAt(root Root, facts adapter.RuntimeFacts, selected []string, at time.Time) (plan.Plan, []StatusReport, error) {
	_ = facts
	_ = at
	inventory, err := Discover(root)
	if err != nil {
		return plan.Plan{}, nil, err
	}
	if inventory.Namespace != NamespaceOwned || inventory.Manifest == nil || inventory.StateCompatibility != core.StateCurrent {
		return plan.Plan{}, nil, fmt.Errorf("repair requires a current owned UAWP namespace")
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), ".uawp", "RECOVERY.json")); statErr == nil || !os.IsNotExist(statErr) {
		return plan.Plan{}, nil, fmt.Errorf("transaction recovery must be resolved before repair")
	}
	ownershipPath := filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md")
	ownership, err := readOwnership(ownershipPath)
	if err != nil {
		return plan.Plan{}, nil, err
	}
	if ownership.Status != core.Released {
		return plan.Plan{}, nil, fmt.Errorf("repair requires RELEASED ownership")
	}
	doctor := Doctor(root)
	findings := append([]StatusReport(nil), doctor.Findings...)
	allowGenerated := len(selected) == 0
	for _, value := range selected {
		if value == string(CodeRepairAvailable) || value == "INSTRUCTIONS.md" {
			allowGenerated = true
		}
	}
	var changes []plan.Change
	manifestBytes, err := os.ReadFile(filepath.Join(root.Path(), ".uawp", "manifest.json"))
	if err != nil {
		return plan.Plan{}, findings, err
	}
	ownershipBytes, err := os.ReadFile(ownershipPath)
	if err != nil {
		return plan.Plan{}, findings, err
	}
	inputs := []plan.Input{{Path: ".uawp/manifest.json", SHA256: plan.HashBytes(manifestBytes)}, {Path: ".uawp/ACTIVE_WORKER.md", SHA256: plan.HashBytes(ownershipBytes)}}
	instructions := filepath.Join(root.Path(), ".uawp", "INSTRUCTIONS.md")
	if _, statErr := os.Lstat(instructions); os.IsNotExist(statErr) {
		inputs = append(inputs, plan.Input{Path: ".uawp/INSTRUCTIONS.md", SHA256: plan.MissingSHA256})
		if allowGenerated {
			changes = append(changes, plan.NewFile(".uawp/INSTRUCTIONS.md", 0o600, plan.MissingSHA256, state.Instructions))
		}
	} else if statErr != nil {
		return plan.Plan{}, findings, statErr
	}
	return plan.NewForWorkspaceInputs("repair", root.Path(), changes, inputs), findings, nil
}
