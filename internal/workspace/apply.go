package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/plan"
)

type ApplyOptions struct {
	ApprovedPlanID string
	Clock          func() time.Time
	Failpoint      func(stage string, index int) error
}

type ApplyReport struct {
	PlanID   string
	Applied  []string
	Verified bool
}

type recoveryJournal struct {
	PlanID      string   `json:"planID"`
	StartedAt   string   `json:"startedAt"`
	Completed   []string `json:"completed"`
	RolledBack  []string `json:"rolledBack"`
	Pending     []string `json:"pending"`
	InProgress  string   `json:"inProgress,omitempty"`
	LastFailure string   `json:"lastFailure,omitempty"`
}

func Apply(root Root, value plan.Plan, options ApplyOptions) (ApplyReport, error) {
	if options.ApprovedPlanID == "" || options.ApprovedPlanID != value.ID {
		return ApplyReport{}, fmt.Errorf("explicit approval for plan %s is required", value.ID)
	}
	if value.Workspace != root.Path() {
		return ApplyReport{}, fmt.Errorf("plan workspace %q does not match %q", value.Workspace, root.Path())
	}
	changes := value.Changes()
	if err := verifyInputs(root, value.Inputs()); err != nil {
		return ApplyReport{}, err
	}
	if err := verifyBeforeState(root, changes); err != nil {
		return ApplyReport{}, err
	}
	if len(changes) == 0 {
		return ApplyReport{PlanID: value.ID, Verified: true}, nil
	}

	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	journal := recoveryJournal{PlanID: value.ID, StartedAt: clock().Format(time.RFC3339)}
	for _, change := range changes {
		journal.Pending = append(journal.Pending, change.Path)
	}

	applied := make([]appliedChange, 0, len(changes))
	for index, change := range changes {
		if err := callFailpoint(options, "before-action", index); err != nil {
			return rollback(root, journal, applied, fmt.Errorf("before action %d: %w", index, err))
		}
		target, err := targetForChange(root, change)
		if err != nil {
			return rollback(root, journal, applied, err)
		}
		if index == 0 && change.Kind == plan.CreateDir && change.Path == ".uawp" {
			if err := os.Mkdir(target, os.FileMode(change.Mode)); err != nil {
				return ApplyReport{}, fmt.Errorf("create namespace: %w", err)
			}
			applied = append(applied, appliedChange{path: target, kind: change.Kind, logical: change.Path})
			journal.Completed = append(journal.Completed, change.Path)
			journal.Pending = journal.Pending[1:]
			if err := writeJournal(root, journal); err != nil {
				return rollback(root, journal, applied, err)
			}
			continue
		}
		journal.InProgress = change.Path
		if err := writeJournal(root, journal); err != nil {
			return ApplyReport{}, fmt.Errorf("record pending action %s: %w", change.Path, err)
		}

		switch change.Kind {
		case plan.CreateDir:
			err = os.Mkdir(target, os.FileMode(change.Mode))
		case plan.CreateFile:
			err = writeAtomic(target, change.Content(), os.FileMode(change.Mode), index, options)
		default:
			err = fmt.Errorf("unsupported change kind %q", change.Kind)
		}
		if err != nil {
			return rollback(root, journal, applied, fmt.Errorf("apply %s: %w", change.Path, err))
		}
		entry := appliedChange{path: target, kind: change.Kind, logical: change.Path, afterSHA256: change.AfterSHA256}
		applied = append(applied, entry)
		if err := callFailpoint(options, "after-publish", index); err != nil {
			return ApplyReport{}, fmt.Errorf("publication of %s may require recovery: %w", change.Path, err)
		}
		journal.Completed = append(journal.Completed, change.Path)
		journal.Pending = journal.Pending[1:]
		journal.InProgress = ""
		if err := writeJournal(root, journal); err != nil {
			return ApplyReport{}, fmt.Errorf("publication of %s requires recovery: %w", change.Path, err)
		}
	}

	if err := verifyAfterState(root, changes); err != nil {
		return rollback(root, journal, applied, err)
	}
	journalPath := filepath.Join(root.Path(), ".uawp", "RECOVERY.json")
	if err := os.Remove(journalPath); err != nil && !os.IsNotExist(err) {
		return ApplyReport{}, fmt.Errorf("remove recovery journal: %w", err)
	}
	_ = syncDir(filepath.Dir(journalPath))

	report := ApplyReport{PlanID: value.ID, Verified: true}
	for _, change := range changes {
		report.Applied = append(report.Applied, change.Path)
	}
	return report, nil
}

func verifyInputs(root Root, inputs []plan.Input) error {
	for _, input := range inputs {
		path := filepath.Join(root.Path(), input.Path)
		info, err := os.Lstat(path)
		if input.SHA256 == plan.MissingSHA256 {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("plan drift: project input %s now exists", input.Path)
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("plan drift: project input %s changed", input.Path)
		}
		content, err := os.ReadFile(path)
		if err != nil || plan.HashBytes(content) != input.SHA256 {
			return fmt.Errorf("plan drift: project input %s changed", input.Path)
		}
	}
	return nil
}

func verifyBeforeState(root Root, changes []plan.Change) error {
	for _, change := range changes {
		target, err := targetForChange(root, change)
		if err != nil {
			return err
		}
		info, err := os.Lstat(target)
		if change.BeforeSHA256 == plan.MissingSHA256 {
			if err == nil {
				return fmt.Errorf("plan drift: %s now exists (%s)", change.Path, info.Mode())
			}
			if !os.IsNotExist(err) {
				return fmt.Errorf("inspect %s: %w", change.Path, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("plan drift: inspect %s: %w", change.Path, err)
		}
		content, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("fingerprint %s: %w", change.Path, err)
		}
		if plan.HashBytes(content) != change.BeforeSHA256 {
			return fmt.Errorf("plan drift: fingerprint changed for %s", change.Path)
		}
	}
	return nil
}

func verifyAfterState(root Root, changes []plan.Change) error {
	for _, change := range changes {
		target, err := targetForChange(root, change)
		if err != nil {
			return err
		}
		info, err := os.Lstat(target)
		if err != nil {
			return fmt.Errorf("verify %s: %w", change.Path, err)
		}
		if change.Kind == plan.CreateDir {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("verify %s: not a real directory", change.Path)
			}
			continue
		}
		content, err := os.ReadFile(target)
		if err != nil || plan.HashBytes(content) != change.AfterSHA256 {
			return fmt.Errorf("verify %s: content mismatch", change.Path)
		}
	}
	return nil
}

func targetForChange(root Root, change plan.Change) (string, error) {
	if change.Path == ".uawp" {
		return filepath.Join(root.Path(), ".uawp"), nil
	}
	if !strings.HasPrefix(change.Path, ".uawp/") {
		return "", fmt.Errorf("change escapes UAWP namespace: %s", change.Path)
	}
	return root.ResolveUAWP(strings.TrimPrefix(change.Path, ".uawp/"))
}

func writeAtomic(target string, content []byte, mode os.FileMode, index int, options ApplyOptions) error {
	temporary, err := os.CreateTemp(filepath.Dir(target), ".uawp-tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := callFailpoint(options, "temp-write", index); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := callFailpoint(options, "before-rename", index); err != nil {
		return err
	}
	// Link is an atomic create-if-absent publication on supported filesystems.
	// Unlike Rename it cannot replace a competing file created after preview.
	if err := os.Link(temporaryPath, target); err != nil {
		return err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return err
	}
	return syncDir(filepath.Dir(target))
}

func callFailpoint(options ApplyOptions, stage string, index int) error {
	if options.Failpoint == nil {
		return nil
	}
	return options.Failpoint(stage, index)
}

func writeJournal(root Root, journal recoveryJournal) error {
	content, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	path := filepath.Join(root.Path(), ".uawp", "RECOVERY.json")
	return writeReplaceAtomic(path, content, 0o600)
}

// writeReplaceAtomic is reserved for UAWP's transaction journal. Regular
// planned state files use writeAtomic, which intentionally never replaces.
func writeReplaceAtomic(target string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(target), ".uawp-journal-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return err
	}
	return syncDir(filepath.Dir(target))
}

type appliedChange struct {
	path        string
	kind        plan.ChangeKind
	logical     string
	afterSHA256 string
}

func rollback(root Root, journal recoveryJournal, applied []appliedChange, cause error) (ApplyReport, error) {
	journal.LastFailure = cause.Error()
	// Never follow a namespace replacement during rollback. Leave the journal
	// visible for human recovery instead of risking a path outside the root.
	namespace := filepath.Join(root.Path(), ".uawp")
	info, namespaceErr := os.Lstat(namespace)
	if namespaceErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ApplyReport{}, fmt.Errorf("%w; automatic rollback refused: namespace changed", cause)
	}
	journalPath := filepath.Join(root.Path(), ".uawp", "RECOVERY.json")
	_ = os.Remove(journalPath)
	for index := len(applied) - 1; index >= 0; index-- {
		entry := applied[index]
		if entry.kind == plan.CreateFile {
			content, err := os.ReadFile(entry.path)
			if err != nil || plan.HashBytes(content) != entry.afterSHA256 {
				journal.LastFailure += "; rollback refused for changed " + entry.logical
				journal.Pending = append([]string{entry.logical}, journal.Pending...)
				continue
			}
		}
		if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) {
			journal.LastFailure += "; rollback " + entry.logical + ": " + err.Error()
			journal.Pending = append([]string{entry.logical}, journal.Pending...)
			continue
		}
		journal.RolledBack = append(journal.RolledBack, entry.logical)
	}
	journal.Completed = nil
	if _, err := os.Stat(filepath.Join(root.Path(), ".uawp")); err == nil {
		if len(journal.Pending) == 0 {
			_ = os.Remove(journalPath)
		} else {
			_ = writeJournal(root, journal)
		}
	}
	return ApplyReport{}, cause
}

func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		// Windows does not provide the Unix directory fsync contract. File data is
		// synced before publication; directory durability is best-effort here.
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
