package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type PurgeReport struct {
	ExportPath       string   `json:"exportPath"`
	ArchiveSHA256    string   `json:"archiveSHA256,omitempty"`
	TombstonePath    string   `json:"tombstonePath,omitempty"`
	Verified         bool     `json:"verified"`
	ConsumersRemoved []string `json:"consumersRemoved,omitempty"`
}
type PurgeOptions struct {
	ApplyOptions ApplyOptions
	Failpoint    func(string) error
}
type purgeMarker struct {
	SchemaVersion   string `json:"schemaVersion"`
	PlanID          string `json:"planID"`
	ExportPath      string `json:"exportPath"`
	NamespaceSHA256 string `json:"namespaceSHA256"`
	ArchiveSHA256   string `json:"archiveSHA256,omitempty"`
}

func PlanPurgeAt(root Root, facts adapter.RuntimeFacts, exportPath string, at time.Time) (plan.Plan, PurgeReport, error) {
	report := PurgeReport{ExportPath: exportPath}
	absolute, err := filepath.Abs(exportPath)
	if err != nil {
		return plan.Plan{}, report, err
	}
	report.ExportPath = absolute
	if !strings.HasSuffix(strings.ToLower(absolute), ".tar.gz") {
		return plan.Plan{}, report, fmt.Errorf("purge export must end in .tar.gz")
	}
	inside, _ := filepath.Rel(filepath.Join(root.Path(), ".uawp"), absolute)
	if inside == "." || (!strings.HasPrefix(inside, ".."+string(filepath.Separator)) && inside != "..") {
		return plan.Plan{}, report, fmt.Errorf("purge export must be outside .uawp")
	}
	if _, statErr := os.Lstat(absolute); statErr == nil || !os.IsNotExist(statErr) {
		return plan.Plan{}, report, fmt.Errorf("purge export already exists")
	}
	if matches, _ := filepath.Glob(filepath.Join(root.Path(), ".uawp-purge-*")); len(matches) != 0 {
		return plan.Plan{}, report, fmt.Errorf("purge tombstone requires recovery")
	}
	detach, detachReport, err := PlanUninstallDetachAt(root, facts, at)
	if err != nil {
		return plan.Plan{}, report, err
	}
	report.ConsumersRemoved = detachReport.ConsumersRemoved
	metadata := detach.Metadata()
	metadata.PurgeExportPath = absolute
	metadata.Reason = "purge export=" + absolute
	return plan.NewForWorkspaceInputsMetadata("purge", root.Path(), detach.Changes(), detach.Inputs(), metadata), report, nil
}

func ApplyPurge(root Root, value plan.Plan, options PurgeOptions) (PurgeReport, error) {
	report := PurgeReport{ExportPath: value.Metadata().PurgeExportPath}
	if value.Operation != "purge" || report.ExportPath == "" {
		return report, fmt.Errorf("invalid purge plan")
	}
	if _, err := Apply(root, value, options.ApplyOptions); err != nil {
		return report, err
	}
	if err := VerifyUninstallDetached(root); err != nil {
		return report, err
	}
	unlock, err := acquireTransactionLock(root, false)
	if err != nil {
		return report, err
	}
	if unlock != nil {
		defer unlock()
	}
	snapshot, err := SnapshotNamespace(root)
	if err != nil {
		return report, err
	}
	if err := verifyReleasedInput(root, value); err != nil {
		return report, err
	}
	markerPath := filepath.Join(root.Path(), ".uawp", "PURGE.json")
	marker := purgeMarker{SchemaVersion: "1", PlanID: value.ID, ExportPath: report.ExportPath, NamespaceSHA256: snapshot.fingerprint}
	markerBytes, _ := json.MarshalIndent(marker, "", "  ")
	markerBytes = append(markerBytes, '\n')
	if err := writeExclusiveSynced(markerPath, markerBytes, 0o600); err != nil {
		return report, err
	}
	if options.Failpoint != nil {
		if err := options.Failpoint("after-marker"); err != nil {
			return report, err
		}
	}
	exported, err := CreateVerifiedExport(root, report.ExportPath, snapshot)
	if err != nil {
		return report, err
	}
	report.ArchiveSHA256 = exported.ArchiveSHA256
	marker.ArchiveSHA256 = exported.ArchiveSHA256
	markerBytes, _ = json.MarshalIndent(marker, "", "  ")
	markerBytes = append(markerBytes, '\n')
	if err := writeReplaceAtomic(markerPath, markerBytes, 0o600); err != nil {
		return report, err
	}
	if options.Failpoint != nil {
		if err := options.Failpoint("after-export"); err != nil {
			return report, err
		}
	}
	current, err := SnapshotNamespace(root)
	if err != nil {
		return report, err
	}
	if current.fingerprint != snapshot.fingerprint {
		return report, fmt.Errorf("namespace changed after verified export; purge stopped")
	}
	tombstone := filepath.Join(root.Path(), ".uawp-purge-"+value.ID[:16])
	if _, err := os.Lstat(tombstone); err == nil || !os.IsNotExist(err) {
		return report, fmt.Errorf("purge tombstone collision")
	}
	if err := os.Rename(filepath.Join(root.Path(), ".uawp"), tombstone); err != nil {
		return report, err
	}
	if err := syncDir(root.Path()); err != nil {
		return report, err
	}
	report.TombstonePath = tombstone
	if options.Failpoint != nil {
		if err := options.Failpoint("after-rename"); err != nil {
			return report, fmt.Errorf("purge committed and requires continuation: %w", err)
		}
	}
	if err := os.RemoveAll(tombstone); err != nil {
		return report, fmt.Errorf("purge committed; tombstone cleanup failed: %w", err)
	}
	if err := syncDir(root.Path()); err != nil {
		return report, err
	}
	report.Verified = true
	report.TombstonePath = ""
	return report, nil
}

func PlanPurgeCleanupAt(root Root, controllerID, reason string, at time.Time) (plan.Plan, error) {
	_ = at
	if controllerID == "" || reason == "" {
		return plan.Plan{}, fmt.Errorf("controller ID and reason are required")
	}
	tombstone, err := findPurgeTombstone(root)
	if err != nil || tombstone == "" {
		return plan.Plan{}, fmt.Errorf("purge tombstone is unavailable")
	}
	marker, err := readPurgeMarker(filepath.Join(tombstone, "PURGE.json"))
	if err != nil {
		return plan.Plan{}, err
	}
	exported, err := VerifyExportFile(marker.ExportPath)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("verify purge export: %w", err)
	}
	if exported.ArchiveSHA256 != marker.ArchiveSHA256 {
		return plan.Plan{}, fmt.Errorf("verify purge export: archive hash changed")
	}
	tombstoneSnapshot, err := snapshotDirectory(tombstone)
	if err != nil || tombstoneSnapshot.fingerprint != marker.NamespaceSHA256 {
		return plan.Plan{}, fmt.Errorf("purge tombstone differs from exported namespace")
	}
	markerRaw, _ := os.ReadFile(filepath.Join(tombstone, "PURGE.json"))
	metadata := plan.Metadata{ControllerID: controllerID, Reason: reason, TransactionID: filepath.Base(tombstone), RecoveryAction: "CONTINUE", PurgeExportPath: marker.ExportPath, PurgeArchiveSHA256: marker.ArchiveSHA256, PurgeMarkerSHA256: plan.HashBytes(markerRaw)}
	return plan.NewForWorkspaceInputsMetadata("purge-cleanup", root.Path(), nil, nil, metadata), nil
}

func HasPurgeTombstone(root Root) bool {
	value, err := findPurgeTombstone(root)
	if err != nil || value == "" {
		return false
	}
	marker, err := readPurgeMarker(filepath.Join(value, "PURGE.json"))
	if err != nil || filepath.Base(value) != ".uawp-purge-"+marker.PlanID[:16] {
		return false
	}
	exported, err := VerifyExportFile(marker.ExportPath)
	if err != nil || exported.ArchiveSHA256 != marker.ArchiveSHA256 {
		return false
	}
	snapshot, err := snapshotDirectory(value)
	return err == nil && snapshot.fingerprint == marker.NamespaceSHA256
}

func ApplyPurgeCleanup(root Root, value plan.Plan, approved string) error {
	if value.Operation != "purge-cleanup" || approved != value.ID {
		return fmt.Errorf("exact purge cleanup approval is required")
	}
	tombstone, err := findPurgeTombstone(root)
	if err != nil || filepath.Base(tombstone) != value.Metadata().TransactionID {
		return fmt.Errorf("purge tombstone evidence changed")
	}
	marker, err := readPurgeMarker(filepath.Join(tombstone, "PURGE.json"))
	if err != nil || marker.ExportPath != value.Metadata().PurgeExportPath {
		return fmt.Errorf("purge marker evidence changed")
	}
	markerRaw, _ := os.ReadFile(filepath.Join(tombstone, "PURGE.json"))
	if plan.HashBytes(markerRaw) != value.Metadata().PurgeMarkerSHA256 {
		return fmt.Errorf("purge marker changed")
	}
	exported, err := VerifyExportFile(marker.ExportPath)
	if err != nil {
		return err
	}
	if exported.ArchiveSHA256 != value.Metadata().PurgeArchiveSHA256 || marker.ArchiveSHA256 != exported.ArchiveSHA256 {
		return fmt.Errorf("purge export evidence changed")
	}
	snapshot, err := snapshotDirectory(tombstone)
	if err != nil || snapshot.fingerprint != marker.NamespaceSHA256 {
		return fmt.Errorf("purge tombstone differs from exported namespace")
	}
	if err := os.RemoveAll(tombstone); err != nil {
		return err
	}
	return syncDir(root.Path())
}

func readPurgeMarker(path string) (purgeMarker, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return purgeMarker{}, err
	}
	var marker purgeMarker
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil {
		return marker, err
	}
	if marker.SchemaVersion != "1" || len(marker.PlanID) < 16 || marker.ExportPath == "" || marker.NamespaceSHA256 == "" {
		return marker, fmt.Errorf("invalid purge marker")
	}
	return marker, nil
}

func HasPendingPurge(root Root) bool {
	_, err := os.Lstat(filepath.Join(root.Path(), ".uawp", "PURGE.json"))
	return err == nil
}

func PlanPurgeAbortAt(root Root, controllerID, reason string, at time.Time) (plan.Plan, error) {
	_ = at
	if controllerID == "" || reason == "" {
		return plan.Plan{}, fmt.Errorf("controller ID and reason are required")
	}
	path := filepath.Join(root.Path(), ".uawp", "PURGE.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return plan.Plan{}, err
	}
	if _, err := readPurgeMarker(path); err != nil {
		return plan.Plan{}, err
	}
	change := plan.NewDeleteFile(".uawp/PURGE.json", plan.HashBytes(raw))
	return plan.NewForWorkspaceInputsMetadata("purge-abort", root.Path(), []plan.Change{change}, nil, plan.Metadata{ControllerID: controllerID, Reason: reason, RecoveryAction: "ROLLBACK"}), nil
}

func verifyReleasedInput(root Root, value plan.Plan) error {
	for _, input := range value.Inputs() {
		if input.Path == ".uawp/ACTIVE_WORKER.md" {
			if err := verifyInputs(root, []plan.Input{input}); err != nil {
				return err
			}
			ownership, err := readOwnership(filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"))
			if err != nil || ownership.Status != core.Released {
				return fmt.Errorf("purge requires unchanged RELEASED ownership")
			}
			return nil
		}
	}
	return fmt.Errorf("purge plan does not bind ownership")
}
