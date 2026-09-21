package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uawp/uawp/internal/core"
)

type FindingCode string

const (
	CodeReady              FindingCode = "READY"
	CodeUninitialized      FindingCode = "UNINITIALIZED"
	CodeActiveOwner        FindingCode = "ACTIVE_OWNER"
	CodeUnknownNamespace   FindingCode = "UNKNOWN_NAMESPACE"
	CodeInvalidManifest    FindingCode = "INVALID_MANIFEST"
	CodeUnsupportedVersion FindingCode = "UNSUPPORTED_VERSION"
	CodeInvalidOwnership   FindingCode = "INVALID_OWNERSHIP"
	CodeIncompleteState    FindingCode = "INCOMPLETE_STATE"
	CodeRecoveryRequired   FindingCode = "RECOVERY_REQUIRED"
)

type StatusReport struct {
	Code       FindingCode `json:"code"`
	Severity   string      `json:"severity"`
	Observed   string      `json:"observed"`
	Mutated    bool        `json:"mutated"`
	NextAction string      `json:"nextAction"`
}

type DoctorReport = StatusReport

func Status(root Root) StatusReport {
	return diagnose(root)
}

func Doctor(root Root) DoctorReport {
	return diagnose(root)
}

func diagnose(root Root) StatusReport {
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

func finding(code FindingCode, severity, observed, next string) StatusReport {
	return StatusReport{Code: code, Severity: severity, Observed: observed, Mutated: false, NextAction: next}
}

func readOwnership(path string) (core.Ownership, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return core.Ownership{}, fmt.Errorf("read ownership: %w", err)
	}
	fields := make(map[string]string)
	recognized := map[string]bool{"Status": true, "Worker ID": true, "Agent": true, "Acquired At": true, "Released At": true, "Purpose": true}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if recognized[key] && fields[key] != "" {
			return core.Ownership{}, fmt.Errorf("duplicate ownership field %q", key)
		}
		fields[key] = strings.TrimSpace(value)
	}
	acquired, err := core.ParseTimestamp(fields["Acquired At"])
	if err != nil {
		return core.Ownership{}, fmt.Errorf("invalid ownership acquisition time: %w", err)
	}
	ownership := core.Ownership{
		Status:     core.OwnershipStatus(fields["Status"]),
		WorkerID:   fields["Worker ID"],
		Agent:      fields["Agent"],
		AcquiredAt: acquired,
		Purpose:    fields["Purpose"],
	}
	releasedValue := fields["Released At"]
	if releasedValue != "" && !strings.EqualFold(releasedValue, "none") {
		released, err := core.ParseTimestamp(releasedValue)
		if err != nil {
			return core.Ownership{}, fmt.Errorf("invalid ownership release time: %w", err)
		}
		ownership.ReleasedAt = &released
	}
	if err := core.ValidateOwnership(ownership); err != nil {
		return core.Ownership{}, fmt.Errorf("invalid ownership state: %w", err)
	}
	return ownership, nil
}
