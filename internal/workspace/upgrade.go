package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/migration"
	"github.com/uawp/uawp/internal/plan"
)

type UpgradeReport struct {
	From     string               `json:"from"`
	To       string               `json:"to"`
	Adapters []adapter.Resolution `json:"adapters,omitempty"`
}

func PlanUpgrade(root Root, facts adapter.RuntimeFacts) (plan.Plan, UpgradeReport, error) {
	return PlanUpgradeAt(root, facts, time.Now())
}

func PlanUpgradeAt(root Root, facts adapter.RuntimeFacts, at time.Time) (plan.Plan, UpgradeReport, error) {
	inventory, err := Discover(root)
	if err != nil {
		return plan.Plan{}, UpgradeReport{}, err
	}
	if inventory.Namespace != NamespaceOwned || inventory.Manifest == nil {
		return plan.Plan{}, UpgradeReport{}, fmt.Errorf("upgrade requires an owned UAWP namespace")
	}
	report := UpgradeReport{From: inventory.Manifest.StateVersion, To: core.CurrentStateVersion}
	if inventory.StateCompatibility == core.StateCurrent {
		return plan.NewForWorkspace("upgrade", root.Path(), nil), report, nil
	}
	if inventory.StateCompatibility != core.StateUpgradeRequired {
		return plan.Plan{}, report, fmt.Errorf("state version %s has no supported upgrade path", inventory.Manifest.StateVersion)
	}
	if _, statErr := os.Lstat(filepath.Join(root.Path(), ".uawp", "RECOVERY.json")); statErr == nil || !os.IsNotExist(statErr) {
		return plan.Plan{}, report, fmt.Errorf("workspace recovery must be resolved before upgrade")
	}
	ownership, err := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		return plan.Plan{}, report, err
	}
	if ownership.Status != core.Released {
		return plan.Plan{}, report, fmt.Errorf("upgrade requires RELEASED ownership")
	}
	ids, err := ConfiguredAdapterIDs(root)
	if err != nil {
		return plan.Plan{}, report, err
	}
	if len(ids) > 0 {
		report.Adapters, err = ResolveAdapters(root, ids, facts)
		if err != nil {
			return plan.Plan{}, report, err
		}
		for _, resolution := range report.Adapters {
			if resolution.Confidence == adapter.Unsupported || resolution.Health != "HEALTHY" {
				return plan.Plan{}, report, fmt.Errorf("adapter %s is not healthy for upgrade: %s", resolution.Provider, resolution.Health)
			}
		}
	}
	manifest, manifestBytes, err := readAdapterManifest(root)
	if err != nil {
		return plan.Plan{}, report, err
	}
	inputs := []plan.Input{{Path: ".uawp/manifest.json", SHA256: plan.HashBytes(manifestBytes)}}
	for _, relative := range []string{".uawp/migrations", ".uawp/recovery"} {
		info, statErr := os.Lstat(filepath.Join(root.Path(), filepath.FromSlash(relative)))
		snapshot := plan.MissingSHA256
		if statErr == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return plan.Plan{}, report, fmt.Errorf("unsafe upgrade directory %s", relative)
			}
			snapshot = plan.DirectorySHA256
		} else if !os.IsNotExist(statErr) {
			return plan.Plan{}, report, statErr
		}
		inputs = append(inputs, plan.Input{Path: relative, SHA256: snapshot})
	}
	for _, id := range ids {
		_, nativeInputs, snapshotErr := adapterSnapshot(root, id, facts)
		if snapshotErr != nil {
			return plan.Plan{}, report, snapshotErr
		}
		inputs = append(inputs, nativeInputs...)
	}
	inputs = uniqueUpgradeInputs(inputs)
	registry, err := migration.NewRegistry([]migration.Step{migration.V1_0ToV1_1{}})
	if err != nil {
		return plan.Plan{}, report, err
	}
	steps, err := registry.Resolve(manifest.StateVersion, core.CurrentStateVersion)
	if err != nil {
		return plan.Plan{}, report, err
	}
	var changes []plan.Change
	context := migration.Context{Manifest: manifest, Inputs: inputs, GeneratedAt: at}
	for _, step := range steps {
		stepChanges, stepErr := step.Plan(context)
		if stepErr != nil {
			return plan.Plan{}, report, stepErr
		}
		changes = append(changes, stepChanges...)
		context.Manifest.StateVersion = step.To()
	}
	receiptName := fmt.Sprintf("%s-%s-to-%s-%s.json", at.UTC().Format("20060102T150405Z"), report.From, report.To, plan.HashBytes([]byte(report.From + "\x00" + report.To + "\x00" + at.UTC().Format(time.RFC3339Nano)))[:8])
	receiptPath := ".uawp/migrations/" + receiptName
	receiptTarget, err := root.ResolveUAWP(strings.TrimPrefix(receiptPath, ".uawp/"))
	if err != nil {
		return plan.Plan{}, report, err
	}
	if _, err := os.Lstat(receiptTarget); err == nil || !os.IsNotExist(err) {
		return plan.Plan{}, report, fmt.Errorf("migration receipt collision at %s", receiptPath)
	}
	metadata := plan.Metadata{MigrationFrom: report.From, MigrationTo: report.To, MigrationReceiptPath: receiptPath}
	return plan.NewForWorkspaceInputsMetadata("upgrade", root.Path(), changes, inputs, metadata), report, nil
}

func VerifyUpgrade(root Root, facts adapter.RuntimeFacts) error {
	inventory, err := Discover(root)
	if err != nil {
		return err
	}
	if inventory.Namespace != NamespaceOwned || inventory.Manifest == nil || inventory.StateCompatibility != core.StateCurrent {
		return fmt.Errorf("workspace is not at current state version")
	}
	if err := (migration.V1_0ToV1_1{}).Verify(migration.Context{Manifest: *inventory.Manifest}); err != nil {
		return err
	}
	ids, err := ConfiguredAdapterIDs(root)
	if err != nil {
		return err
	}
	resolutions, err := ResolveAdapters(root, ids, facts)
	if err != nil {
		return err
	}
	for _, resolution := range resolutions {
		if resolution.Confidence == adapter.Unsupported || resolution.Health != "HEALTHY" {
			return fmt.Errorf("adapter %s failed upgrade verification", resolution.Provider)
		}
	}
	return nil
}

func uniqueUpgradeInputs(values []plan.Input) []plan.Input {
	byPath := make(map[string]string, len(values))
	for _, value := range values {
		byPath[value.Path] = value.SHA256
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]plan.Input, 0, len(paths))
	for _, path := range paths {
		result = append(result, plan.Input{Path: path, SHA256: byPath[path]})
	}
	return result
}

type migrationReceipt struct {
	SchemaVersion   string            `json:"schemaVersion"`
	From            string            `json:"from"`
	To              string            `json:"to"`
	ApprovedPlanID  string            `json:"approvedPlanID"`
	CompletedAt     string            `json:"completedAt"`
	ResultingHashes map[string]string `json:"resultingHashes"`
}

func encodeMigrationReceipt(value migrationReceipt) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
