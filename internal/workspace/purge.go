package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/plan"
)

type PurgeReport struct {
	ExportPath       string   `json:"exportPath"`
	ArchiveSHA256    string   `json:"archiveSHA256,omitempty"`
	TombstonePath    string   `json:"tombstonePath,omitempty"`
	Verified         bool     `json:"verified"`
	ConsumersRemoved []string `json:"consumersRemoved,omitempty"`
}
type PurgeOptions struct{ ApplyOptions ApplyOptions }

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
	snapshot, err := SnapshotNamespace(root)
	if err != nil {
		return report, err
	}
	exported, err := CreateVerifiedExport(root, report.ExportPath, snapshot)
	if err != nil {
		return report, err
	}
	report.ArchiveSHA256 = exported.ArchiveSHA256
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
