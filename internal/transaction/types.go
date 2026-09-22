package transaction

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/plan"
)

const (
	JournalSchemaVersion = "1"
	PointerSchemaVersion = "1"
	ReceiptSchemaVersion = "1"
	MaxRecordBytes       = 8 << 20
	PhasePreparing       = "PREPARING"
	PhaseApplying        = "APPLYING"
	PhaseRecovering      = "RECOVERING"
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ActionState string

const (
	Pending          ActionState = "PENDING"
	InProgress       ActionState = "IN_PROGRESS"
	Applied          ActionState = "APPLIED"
	Verified         ActionState = "VERIFIED"
	RolledBack       ActionState = "ROLLED_BACK"
	RollbackVerified ActionState = "ROLLBACK_VERIFIED"
)

type Action struct {
	Index        int                  `json:"index"`
	Change       plan.PersistedChange `json:"change"`
	State        ActionState          `json:"state"`
	BackupPath   string               `json:"backupPath,omitempty"`
	BackupSHA256 string               `json:"backupSHA256,omitempty"`
}

type Journal struct {
	SchemaVersion string   `json:"schemaVersion"`
	TransactionID string   `json:"transactionID"`
	Operation     string   `json:"operation"`
	PlanID        string   `json:"planID"`
	Workspace     string   `json:"workspace"`
	Phase         string   `json:"phase"`
	StartedAt     string   `json:"startedAt"`
	Actions       []Action `json:"actions"`
}

type LivePointer struct {
	SchemaVersion string `json:"schemaVersion"`
	TransactionID string `json:"transactionID"`
	Operation     string `json:"operation"`
	PlanID        string `json:"planID"`
	Workspace     string `json:"workspace"`
	Phase         string `json:"phase"`
}

type Result string

const (
	ResultCompleted  Result = "COMPLETED"
	ResultRolledBack Result = "ROLLED_BACK"
)

type Receipt struct {
	SchemaVersion string            `json:"schemaVersion"`
	TransactionID string            `json:"transactionID"`
	Operation     string            `json:"operation"`
	PlanID        string            `json:"planID"`
	CompletedAt   string            `json:"completedAt"`
	Result        Result            `json:"result"`
	Hashes        map[string]string `json:"hashes"`
}

type Bundle struct {
	Pointer LivePointer
	Journal Journal
	Plan    plan.PersistedPlan
	Receipt *Receipt
}

func (b Bundle) Validate() error {
	if err := b.Pointer.Validate(); err != nil {
		return fmt.Errorf("pointer: %w", err)
	}
	if err := b.Journal.Validate(); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	if _, err := plan.Restore(b.Plan); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	if b.Pointer.TransactionID != b.Journal.TransactionID || b.Pointer.Operation != b.Journal.Operation || b.Pointer.PlanID != b.Journal.PlanID || b.Pointer.Workspace != b.Journal.Workspace || b.Pointer.Phase != b.Journal.Phase {
		return fmt.Errorf("pointer and journal disagree")
	}
	if b.Plan.ID != b.Journal.PlanID || b.Plan.Operation != b.Journal.Operation || b.Plan.Workspace != b.Journal.Workspace {
		return fmt.Errorf("journal and plan disagree")
	}
	if len(b.Journal.Actions) != len(b.Plan.Changes) {
		return fmt.Errorf("journal action count does not match plan")
	}
	for index := range b.Journal.Actions {
		if !reflect.DeepEqual(b.Journal.Actions[index].Change, b.Plan.Changes[index]) {
			return fmt.Errorf("journal action %d does not match plan", index)
		}
	}
	if b.Receipt != nil {
		if err := b.Receipt.Validate(); err != nil {
			return fmt.Errorf("receipt: %w", err)
		}
		if b.Receipt.TransactionID != b.Journal.TransactionID || b.Receipt.Operation != b.Journal.Operation || b.Receipt.PlanID != b.Journal.PlanID {
			return fmt.Errorf("receipt and journal disagree")
		}
	}
	return nil
}

func (j Journal) Validate() error {
	if j.SchemaVersion != JournalSchemaVersion {
		return fmt.Errorf("unsupported journal schema %q", j.SchemaVersion)
	}
	if err := validateIdentity(j.TransactionID, j.Operation, j.PlanID, j.Workspace, j.Phase); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, j.StartedAt); err != nil {
		return fmt.Errorf("invalid journal start time")
	}
	seen := map[string]bool{}
	for index, action := range j.Actions {
		if action.Index != index {
			return fmt.Errorf("action index %d is not canonical", action.Index)
		}
		if seen[action.Change.Path] {
			return fmt.Errorf("duplicate action path %q", action.Change.Path)
		}
		seen[action.Change.Path] = true
		if _, err := plan.RestoreChange(action.Change); err != nil {
			return fmt.Errorf("action %d: %w", index, err)
		}
		if !validActionState(action.State) {
			return fmt.Errorf("invalid action state %q", action.State)
		}
		needsBackup := action.Change.Kind == plan.UpdateFile || action.Change.Kind == plan.DeleteFile
		if needsBackup && (action.BackupPath == "" || !hashPattern.MatchString(action.BackupSHA256)) {
			return fmt.Errorf("action %d lacks verified backup", index)
		}
		if action.BackupPath != "" && !validBackupPath(action.BackupPath) {
			return fmt.Errorf("invalid backup path %q", action.BackupPath)
		}
		if action.BackupPath == "" && action.BackupSHA256 != "" {
			return fmt.Errorf("backup hash without path")
		}
	}
	return nil
}

func (p LivePointer) Validate() error {
	if p.SchemaVersion != PointerSchemaVersion {
		return fmt.Errorf("unsupported pointer schema %q", p.SchemaVersion)
	}
	return validateIdentity(p.TransactionID, p.Operation, p.PlanID, p.Workspace, p.Phase)
}

func (r Receipt) Validate() error {
	if r.SchemaVersion != ReceiptSchemaVersion {
		return fmt.Errorf("unsupported receipt schema %q", r.SchemaVersion)
	}
	if !identifierPattern.MatchString(r.TransactionID) || r.Operation == "" || !hashPattern.MatchString(r.PlanID) {
		return fmt.Errorf("invalid receipt identity")
	}
	if _, err := time.Parse(time.RFC3339, r.CompletedAt); err != nil {
		return fmt.Errorf("invalid receipt completion time")
	}
	if r.Result != ResultCompleted && r.Result != ResultRolledBack {
		return fmt.Errorf("invalid receipt result %q", r.Result)
	}
	keys := make([]string, 0, len(r.Hashes))
	for key := range r.Hashes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !validLogicalPath(key) || !hashPattern.MatchString(r.Hashes[key]) {
			return fmt.Errorf("invalid receipt hash for %q", key)
		}
	}
	return nil
}

func CanTransition(from, to ActionState) bool {
	switch from {
	case Pending:
		return to == InProgress
	case InProgress:
		return to == Applied || to == RolledBack
	case Applied:
		return to == Verified || to == RolledBack
	case Verified:
		return to == RolledBack
	case RolledBack:
		return to == RollbackVerified
	default:
		return false
	}
}

func validActionState(state ActionState) bool {
	return state == Pending || state == InProgress || state == Applied || state == Verified || state == RolledBack || state == RollbackVerified
}

func validateIdentity(transactionID, operation, planID, workspace, phase string) error {
	if !identifierPattern.MatchString(transactionID) || operation == "" || !hashPattern.MatchString(planID) {
		return fmt.Errorf("invalid transaction identity")
	}
	if !filepath.IsAbs(workspace) {
		return fmt.Errorf("workspace must be absolute")
	}
	if phase != PhasePreparing && phase != PhaseApplying && phase != PhaseRecovering {
		return fmt.Errorf("invalid transaction phase %q", phase)
	}
	return nil
}

func validBackupPath(value string) bool {
	return strings.HasPrefix(value, "backups/") && validLogicalPath(value) && strings.Count(value, "/") == 1
}

func validLogicalPath(value string) bool {
	return value != "" && !strings.Contains(value, `\`) && !strings.HasPrefix(value, "/") && path.Clean(value) == value && value != "." && !strings.HasPrefix(value, "../")
}
