package transaction

import (
	"fmt"

	"github.com/uawp/uawp/internal/plan"
)

type Classification string

const (
	RollbackAvailable   Classification = "ROLLBACK_AVAILABLE"
	ContinueAvailable   Classification = "CONTINUE_AVAILABLE"
	BothAvailable       Classification = "ROLLBACK_OR_CONTINUE_AVAILABLE"
	ManualRequired      Classification = "MANUAL_RECOVERY_REQUIRED"
	TransactionComplete Classification = "TRANSACTION_COMPLETE"
)

type PathKind string

const (
	PathMissing   PathKind = "MISSING"
	PathFile      PathKind = "FILE"
	PathDirectory PathKind = "DIRECTORY"
	PathUnsafe    PathKind = "UNSAFE"
)

type PathState struct {
	Kind   PathKind
	SHA256 string
}

type BackupState struct {
	Valid   bool
	SHA256  string
	Mode    uint32
	Content []byte
}

type Observation struct {
	Paths   map[string]PathState
	Inputs  map[string]string
	Backups map[int]BackupState
}

type Evidence struct {
	Path     string `json:"path"`
	Observed string `json:"observed"`
	Before   bool   `json:"before"`
	After    bool   `json:"after"`
	Backup   bool   `json:"backup"`
}

type Report struct {
	Classification Classification `json:"classification"`
	Evidence       []Evidence     `json:"evidence"`
}

func Classify(bundle Bundle, observation Observation) Report {
	if err := bundle.Validate(); err != nil {
		return Report{Classification: ManualRequired}
	}
	rollbackSafe, continueSafe, allAfter := true, true, true
	actionPaths := map[string]bool{}
	report := Report{}
	for index, action := range bundle.Journal.Actions {
		actionPaths[action.Change.Path] = true
		live, ok := observation.Paths[action.Change.Path]
		if !ok {
			live = PathState{Kind: PathUnsafe}
		}
		before := matchesBefore(action.Change, live)
		after := matchesAfter(action.Change, live)
		backupOK := true
		if action.Change.Kind == plan.UpdateFile || action.Change.Kind == plan.DeleteFile {
			backup := observation.Backups[index]
			backupOK = backup.Valid && backup.SHA256 == action.BackupSHA256 && backup.Mode == action.BackupMode
		}
		report.Evidence = append(report.Evidence, Evidence{Path: action.Change.Path, Observed: fmt.Sprintf("%s:%s", live.Kind, live.SHA256), Before: before, After: after, Backup: backupOK})
		rollbackSafe = rollbackSafe && (before || after) && backupOK
		continueSafe = continueSafe && (before || after) && backupOK
		allAfter = allAfter && after
	}
	for _, input := range bundle.Plan.Inputs {
		if actionPaths[input.Path] {
			continue
		}
		if observed, ok := observation.Inputs[input.Path]; !ok || observed != input.SHA256 {
			continueSafe = false
			rollbackSafe = false
		}
	}
	if allAfter && len(bundle.Journal.Actions) > 0 {
		report.Classification = TransactionComplete
	} else if rollbackSafe && continueSafe {
		report.Classification = BothAvailable
	} else if rollbackSafe {
		report.Classification = RollbackAvailable
	} else if continueSafe {
		report.Classification = ContinueAvailable
	} else {
		report.Classification = ManualRequired
	}
	return report
}

func matchesBefore(change plan.PersistedChange, live PathState) bool {
	switch change.Kind {
	case plan.CreateDir, plan.CreateFile:
		return live.Kind == PathMissing
	case plan.UpdateFile, plan.DeleteFile:
		return live.Kind == PathFile && live.SHA256 == change.BeforeSHA256
	case plan.DeleteDir:
		return live.Kind == PathDirectory
	default:
		return false
	}
}

func matchesAfter(change plan.PersistedChange, live PathState) bool {
	switch change.Kind {
	case plan.CreateDir:
		return live.Kind == PathDirectory
	case plan.CreateFile, plan.UpdateFile:
		return live.Kind == PathFile && live.SHA256 == change.AfterSHA256
	case plan.DeleteFile, plan.DeleteDir:
		return live.Kind == PathMissing
	default:
		return false
	}
}
