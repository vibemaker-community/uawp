package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vibemaker-community/uawp/internal/adapter"
	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
	state "github.com/vibemaker-community/uawp/templates/state"
)

func PlanRepairAt(root Root, facts adapter.RuntimeFacts, selected []string, at time.Time) (plan.Plan, []StatusReport, error) {
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
	allowNative := len(selected) == 0
	for _, value := range selected {
		if value == string(CodeRepairAvailable) || value == "INSTRUCTIONS.md" {
			allowGenerated = true
		}
		if value == "native" {
			allowNative = true
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
	ids, err := ConfiguredAdapterIDs(root)
	if err != nil {
		return plan.Plan{}, findings, err
	}
	resolutions, err := ResolveAdapters(root, ids, facts)
	if err != nil {
		return plan.Plan{}, findings, err
	}
	routes := map[string]adapter.Route{}
	for _, resolution := range resolutions {
		routes[resolution.Provider] = resolution.Route
		_, bound, snapErr := adapterSnapshot(root, resolution.Provider, facts)
		if snapErr != nil {
			return plan.Plan{}, findings, snapErr
		}
		inputs = append(inputs, bound...)
	}
	instructions := filepath.Join(root.Path(), ".uawp", "INSTRUCTIONS.md")
	if _, statErr := os.Lstat(instructions); os.IsNotExist(statErr) {
		inputs = append(inputs, plan.Input{Path: ".uawp/INSTRUCTIONS.md", SHA256: plan.MissingSHA256})
		if allowGenerated {
			changes = append(changes, plan.NewFile(".uawp/INSTRUCTIONS.md", 0o600, plan.MissingSHA256, state.Instructions))
		}
	} else if statErr != nil {
		return plan.Plan{}, findings, statErr
	}
	for _, artifact := range inventory.Manifest.Integrations {
		if !allowNative {
			continue
		}
		if artifact.Mode == core.Direct {
			continue
		}
		routeMatches := true
		for _, consumer := range artifact.Consumers {
			route := routes[consumer]
			if route.Path != artifact.Path || route.Mode != artifact.Mode || route.Target != artifact.Target {
				routeMatches = false
			}
		}
		if !routeMatches {
			continue
		}
		path := filepath.Join(root.Path(), filepath.FromSlash(artifact.Path))
		before, readErr := os.ReadFile(path)
		missing := os.IsNotExist(readErr)
		if missing && !artifact.CreatedFile {
			continue
		}
		if readErr != nil && !missing {
			continue
		}
		var content []byte
		content = append([]byte(nil), before...)
		switch artifact.Mode {
		case core.ManagedBlock:
			var meta adapter.BlockMeta
			content, meta, err = adapter.UpsertManagedBlock(content, adapter.BlockSpec{ArtifactID: artifact.ID, Target: artifact.Target, Consumers: artifact.Consumers, Body: adapter.BridgeBody()})
			if err == nil && meta.OutsideSHA256 != artifact.OutsideContentSHA256 {
				err = fmt.Errorf("outside content drift")
			}
		case core.Import:
			for _, target := range ownedImportTargets(artifact) {
				content, _, err = adapter.UpsertImport(content, target)
				if err != nil {
					break
				}
			}
		}
		exact := artifact.Mode == core.ManagedBlock || plan.HashBytes(content) == artifact.ArtifactSHA256
		if err != nil || !exact || string(content) == string(before) {
			continue
		}
		beforeHash := plan.MissingSHA256
		if !missing {
			beforeHash = plan.HashBytes(before)
		}
		inputs = append(inputs, plan.Input{Path: artifact.Path, SHA256: beforeHash})
		if missing {
			changes = append(changes, plan.NewFile(artifact.Path, 0o600, beforeHash, content))
		} else {
			changes = append(changes, plan.NewUpdateFile(artifact.Path, 0o600, beforeHash, content))
		}
	}
	return plan.NewForWorkspaceInputs("repair", root.Path(), changes, uniqueUpgradeInputs(inputs)), findings, nil
}
