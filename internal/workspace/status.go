package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
)

type FindingCode string

const (
	CodeReady                    FindingCode = "READY"
	CodeUninitialized            FindingCode = "UNINITIALIZED"
	CodeActiveOwner              FindingCode = "ACTIVE_OWNER"
	CodeUnknownNamespace         FindingCode = "UNKNOWN_NAMESPACE"
	CodeInvalidManifest          FindingCode = "INVALID_MANIFEST"
	CodeUnsupportedVersion       FindingCode = "UNSUPPORTED_VERSION"
	CodeInvalidOwnership         FindingCode = "INVALID_OWNERSHIP"
	CodeIncompleteState          FindingCode = "INCOMPLETE_STATE"
	CodeRecoveryRequired         FindingCode = "RECOVERY_REQUIRED"
	CodeUpgradeRequired          FindingCode = "UPGRADE_REQUIRED"
	CodeUpgradeAvailable         FindingCode = "UPGRADE_AVAILABLE"
	CodeMaintenanceBlockedActive FindingCode = "MAINTENANCE_BLOCKED_ACTIVE"
	CodeRepairAvailable          FindingCode = "REPAIR_AVAILABLE"
	CodeManualRepairRequired     FindingCode = "MANUAL_REPAIR_REQUIRED"
)

type StatusReport struct {
	Code       FindingCode `json:"code"`
	Severity   string      `json:"severity"`
	Observed   string      `json:"observed"`
	Mutated    bool        `json:"mutated"`
	NextAction string      `json:"nextAction"`
}

type DoctorReport struct {
	StatusReport
	Findings []StatusReport `json:"findings"`
}

func Status(root Root) StatusReport {
	return diagnose(root)
}

func Doctor(root Root) DoctorReport {
	primary := diagnose(root)
	findings := []StatusReport{primary}
	inventory, err := Discover(root)
	if err == nil && inventory.Namespace == NamespaceOwned && inventory.Manifest != nil {
		if inventory.StateCompatibility == core.StateUpgradeRequired {
			findings = append(findings, finding(CodeUpgradeAvailable, "warning", fmt.Sprintf("State version %s can be upgraded to %s.", inventory.Manifest.StateVersion, core.CurrentStateVersion), "Preview uawp upgrade."))
		}
		ownership, ownershipErr := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
		if ownershipErr == nil && ownership.Status == core.Active {
			findings = append(findings, finding(CodeMaintenanceBlockedActive, "warning", fmt.Sprintf("Worker %s currently owns the workspace.", ownership.WorkerID), "Release ownership before maintenance."))
		}
		for _, item := range []struct {
			name      string
			generated bool
		}{{"INSTRUCTIONS.md", true}, {"CONTEXT.md", false}, {"DECISIONS.md", false}} {
			path := filepath.Join(root.Path(), ".uawp", item.name)
			if info, statErr := os.Lstat(path); os.IsNotExist(statErr) {
				code, next := CodeManualRepairRequired, "Restore this user-authored state from a checkpoint or backup."
				if item.generated {
					code, next = CodeRepairAvailable, "Preview uawp repair to regenerate the exact versioned template."
				}
				findings = append(findings, finding(code, "error", fmt.Sprintf("Required state file %s is missing.", item.name), next))
			} else if statErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				findings = append(findings, finding(CodeManualRepairRequired, "error", fmt.Sprintf("Required state file %s is unsafe.", item.name), "Resolve the filesystem object manually."))
			}
		}
		if ids, idsErr := ConfiguredAdapterIDs(root); idsErr == nil && len(ids) > 0 {
			if resolutions, resolveErr := ResolveAdapters(root, ids, adapter.RuntimeFacts{}); resolveErr == nil {
				for _, resolution := range resolutions {
					if resolution.Confidence == adapter.Unsupported || resolution.Health != "HEALTHY" {
						findings = append(findings, finding(CodeManualRepairRequired, "error", fmt.Sprintf("Adapter %s requires reviewed repair: %s.", resolution.Provider, resolution.Health), "Resolve native-entry drift or use an explicitly reviewed repair plan."))
					}
				}
			}
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity < findings[j].Severity
		}
		return findings[i].Code < findings[j].Code
	})
	return DoctorReport{StatusReport: primary, Findings: findings}
}

func diagnose(root Root) StatusReport {
	if HasPendingPurge(root) {
		return finding(CodeRecoveryRequired, "error", "A pre-commit purge record is present.", "Use transaction rollback to clear the pending purge after review.")
	}
	if tombstone, err := findPurgeTombstone(root); err != nil {
		return finding(CodeRecoveryRequired, "error", err.Error(), "Resolve the purge tombstone manually before further mutation.")
	} else if tombstone != "" {
		return finding(CodeRecoveryRequired, "error", fmt.Sprintf("A committed purge tombstone is present at %s.", filepath.Base(tombstone)), "Verify the external export, then continue tombstone cleanup.")
	}
	// A journal is meaningful even when a crash occurred before manifest.json,
	// which is intentionally published last.
	recoveryPath := filepath.Join(root.Path(), ".uawp", "RECOVERY.json")
	if info, err := os.Lstat(recoveryPath); err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		return finding(CodeRecoveryRequired, "error", "A UAWP recovery journal is present.", "Run recovery diagnostics before any mutation.")
	}
	inventory, err := Discover(root)
	if err != nil {
		return finding(CodeInvalidManifest, "error", err.Error(), "Inspect filesystem permissions and retry diagnostics.")
	}
	switch inventory.Namespace {
	case NamespaceAbsent:
		return finding(CodeUninitialized, "info", "No .uawp namespace exists.", "Run uawp init to preview initialization.")
	case NamespaceUnknown:
		return finding(CodeUnknownNamespace, "error", "The .uawp path is not a recognized UAWP namespace.", "Preserve the existing path and resolve the namespace collision manually.")
	case NamespaceInvalid:
		if strings.Contains(inventory.ManifestError, string(core.ErrUnsupportedVersion)) {
			return finding(CodeUnsupportedVersion, "error", inventory.ManifestError, "Use a UAWP version that supports this state version.")
		}
		return finding(CodeInvalidManifest, "error", inventory.ManifestError, "Repair or restore manifest.json before continuing.")
	case NamespaceOwned:
	default:
		return finding(CodeInvalidManifest, "error", fmt.Sprintf("Unknown namespace classification %q.", inventory.Namespace), "Stop and inspect the workspace.")
	}
	if inventory.StateCompatibility == core.StateUnsupported || inventory.StateCompatibility == core.StateFutureMajor {
		return finding(CodeUnsupportedVersion, "error", fmt.Sprintf("State version %s is not supported by this UAWP release.", inventory.Manifest.StateVersion), "Use a UAWP version with an explicit compatibility path for this state version.")
	}
	if inventory.StateCompatibility == core.StateUpgradeRequired {
		return finding(CodeUpgradeRequired, "warning", fmt.Sprintf("State version %s requires upgrade to %s.", inventory.Manifest.StateVersion, core.CurrentStateVersion), "Run uawp upgrade to preview the migration.")
	}

	if _, err := os.Lstat(recoveryPath); err == nil {
		return finding(CodeRecoveryRequired, "error", "A UAWP recovery journal is present.", "Run recovery diagnostics before any mutation.")
	} else if !os.IsNotExist(err) {
		return finding(CodeRecoveryRequired, "error", fmt.Sprintf("Cannot inspect recovery journal: %v", err), "Resolve filesystem access before any mutation.")
	}

	for _, name := range []string{"CONTEXT.md", "ACTIVE_WORKER.md", "DECISIONS.md"} {
		path := filepath.Join(root.Path(), ".uawp", name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			observed := fmt.Sprintf("Required state file %s is missing or not regular.", name)
			if err != nil && !os.IsNotExist(err) {
				observed = fmt.Sprintf("Cannot inspect required state file %s: %v", name, err)
			}
			return finding(CodeIncompleteState, "error", observed, "Restore the required state file before resuming work.")
		}
	}
	checkpoints, err := os.Lstat(filepath.Join(root.Path(), ".uawp", "checkpoints"))
	if err != nil || !checkpoints.IsDir() || checkpoints.Mode()&os.ModeSymlink != 0 {
		return finding(CodeIncompleteState, "error", "Required checkpoints directory is missing or unsafe.", "Restore the required checkpoints directory before resuming work.")
	}

	ownership, err := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
	if err != nil {
		return finding(CodeInvalidOwnership, "error", err.Error(), "Repair ACTIVE_WORKER.md before any substantive write.")
	}
	if ownership.Status == core.Active {
		return finding(CodeActiveOwner, "warning", fmt.Sprintf("Worker %s is ACTIVE via %s.", ownership.WorkerID, ownership.Agent), "Remain read-only unless this worker owns the claim.")
	}
	return finding(CodeReady, "info", fmt.Sprintf("Namespace is valid and ownership is %s.", ownership.Status), "Resume workspace context before acquiring ownership.")
}

func findPurgeTombstone(root Root) (string, error) {
	matches, err := filepath.Glob(filepath.Join(root.Path(), ".uawp-purge-*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("multiple purge tombstones require manual recovery")
	}
	info, err := os.Lstat(matches[0])
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("unsafe purge tombstone")
	}
	file, err := os.Open(filepath.Join(matches[0], "manifest.json"))
	if err != nil {
		return "", fmt.Errorf("unrecognized purge tombstone")
	}
	manifest, decodeErr := core.DecodeManifest(file)
	file.Close()
	if decodeErr != nil || manifest.Protocol != core.ProtocolName {
		return "", fmt.Errorf("unrecognized purge tombstone")
	}
	return matches[0], nil
}

func finding(code FindingCode, severity, observed, next string) StatusReport {
	return StatusReport{Code: code, Severity: severity, Observed: observed, Mutated: false, NextAction: next}
}

func readOwnership(path string) (core.Ownership, error) {
	file, err := os.Open(path)
	if err != nil {
		return core.Ownership{}, fmt.Errorf("read ownership: %w", err)
	}
	defer file.Close()
	ownership, err := core.DecodeOwnership(file)
	if err != nil {
		return core.Ownership{}, fmt.Errorf("invalid ownership state: %w", err)
	}
	return ownership, nil
}
