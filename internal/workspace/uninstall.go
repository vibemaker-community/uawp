package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type UninstallReport struct {
	ConsumersRemoved []string `json:"consumersRemoved"`
	ArtifactsRemoved []string `json:"artifactsRemoved"`
	StatePreserved   bool     `json:"statePreserved"`
}

func PlanUninstallDetachAt(root Root, facts adapter.RuntimeFacts, at time.Time) (plan.Plan, UninstallReport, error) {
	report := UninstallReport{StatePreserved: true}
	inventory, err := Discover(root)
	if err != nil {
		return plan.Plan{}, report, err
	}
	if inventory.Namespace != NamespaceOwned || inventory.Manifest == nil || inventory.StateCompatibility != core.StateCurrent {
		return plan.Plan{}, report, fmt.Errorf("uninstall requires a current owned UAWP namespace")
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), ".uawp", "RECOVERY.json")); statErr == nil || !os.IsNotExist(statErr) {
		return plan.Plan{}, report, fmt.Errorf("transaction recovery must be resolved before uninstall")
	}
	ownership, err := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		return plan.Plan{}, report, err
	}
	if ownership.Status != core.Released {
		return plan.Plan{}, report, fmt.Errorf("uninstall requires RELEASED ownership")
	}
	manifest, manifestBytes, err := readAdapterManifest(root)
	if err != nil {
		return plan.Plan{}, report, err
	}
	ids, err := ConfiguredAdapterIDs(root)
	if err != nil {
		return plan.Plan{}, report, err
	}
	if len(ids) == 0 {
		return plan.NewForWorkspace("uninstall-detach", root.Path(), nil), report, nil
	}
	resolutions, err := ResolveAdapters(root, ids, facts)
	if err != nil {
		return plan.Plan{}, report, err
	}
	for _, resolution := range resolutions {
		if resolution.Confidence == adapter.Unsupported || resolution.Health != "HEALTHY" {
			return plan.Plan{}, report, fmt.Errorf("adapter %s is not safely detachable: %s", resolution.Provider, resolution.Health)
		}
	}
	var changes []plan.Change
	var inputs []plan.Input
	for _, artifact := range manifest.Integrations {
		report.ArtifactsRemoved = append(report.ArtifactsRemoved, artifact.Path)
		report.ConsumersRemoved = append(report.ConsumersRemoved, artifact.Consumers...)
		if artifact.Mode == core.Direct {
			continue
		}
		snapshot, snapshotErr := adapter.DiscoverSnapshot(root.Path(), []string{artifact.Path}, "", nil)
		if snapshotErr != nil {
			return plan.Plan{}, report, snapshotErr
		}
		entryInputs, inputErr := entryInputsFromSnapshot(snapshot)
		if inputErr != nil {
			return plan.Plan{}, report, inputErr
		}
		inputs = append(inputs, entryInputs...)
		fact := snapshot.Files[artifact.Path]
		before := fact.Content
		var after []byte
		if artifact.CreatedFile && plan.HashBytes(before) == artifact.ArtifactSHA256 {
			after = nil
		} else {
			after = append([]byte(nil), before...)
			switch artifact.Mode {
			case core.ManagedBlock:
				after, err = adapter.RemoveManagedBlock(before, adapter.BlockSpec{ArtifactID: artifact.ID, Target: artifact.Target, Consumers: artifact.Consumers, Body: adapter.BridgeBody()})
			case core.Import:
				for _, target := range ownedImportTargets(artifact) {
					var removed bool
					after, removed, err = adapter.RemoveImport(after, target)
					if err == nil && !removed {
						err = fmt.Errorf("owned import %s is missing", target)
					}
					if err != nil {
						break
					}
				}
			default:
				err = fmt.Errorf("unsupported integration mode %s", artifact.Mode)
			}
			if err != nil {
				return plan.Plan{}, report, err
			}
		}
		if len(after) == 0 && artifact.CreatedFile {
			changes = append(changes, plan.NewDeleteFile(artifact.Path, plan.HashBytes(before)))
			continue
		}
		if string(after) != string(before) {
			mode := fact.Mode
			if mode == 0 {
				mode = 0o600
			}
			changes = append(changes, plan.NewUpdateFile(artifact.Path, mode, plan.HashBytes(before), after))
		}
	}
	sort.Strings(report.ConsumersRemoved)
	report.ConsumersRemoved = uniqueStrings(report.ConsumersRemoved)
	sort.Strings(report.ArtifactsRemoved)
	manifest.Integrations = nil
	encoded, err := core.EncodeManifest(manifest)
	if err != nil {
		return plan.Plan{}, report, err
	}
	for index := range changes {
		changes[index] = changes[index].WithSequence(index + 1)
	}
	changes = append(changes, plan.NewUpdateFile(".uawp/manifest.json", 0o600, plan.HashBytes(manifestBytes), encoded).WithSequence(len(changes)+1))
	metadata := plan.Metadata{Reason: "full-detach facts=" + factsFingerprint(facts) + " generated=" + at.Format(time.RFC3339Nano)}
	return plan.NewForWorkspaceInputsMetadata("uninstall-detach", root.Path(), changes, uniqueUpgradeInputs(inputs), metadata), report, nil
}

func VerifyUninstallDetached(root Root) error {
	ids, err := ConfiguredAdapterIDs(root)
	if err != nil {
		return err
	}
	if len(ids) != 0 {
		return fmt.Errorf("configured adapter consumers remain: %v", ids)
	}
	return nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := []string{values[0]}
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
