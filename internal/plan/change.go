package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type ChangeKind string

const (
	CreateDir  ChangeKind = "CREATE_DIR"
	CreateFile ChangeKind = "CREATE_FILE"
	UpdateFile ChangeKind = "UPDATE_FILE"
	DeleteFile ChangeKind = "DELETE_FILE"
	DeleteDir  ChangeKind = "DELETE_DIR"

	MissingSHA256   = "MISSING"
	DirectorySHA256 = "DIRECTORY"
)

type Change struct {
	Kind         ChangeKind `json:"kind"`
	Path         string     `json:"path"`
	BeforeSHA256 string     `json:"beforeSHA256"`
	AfterSHA256  string     `json:"afterSHA256"`
	Mode         uint32     `json:"mode"`
	Size         int64      `json:"size"`
	Sequence     int        `json:"sequence"`
	content      []byte
}

func NewDirectory(path string, mode uint32) Change {
	return Change{Kind: CreateDir, Path: path, BeforeSHA256: MissingSHA256, AfterSHA256: DirectorySHA256, Mode: mode}
}

func NewFile(path string, mode uint32, before string, content []byte) Change {
	return Change{
		Kind:         CreateFile,
		Path:         path,
		BeforeSHA256: before,
		AfterSHA256:  HashBytes(content),
		Mode:         mode,
		Size:         int64(len(content)),
		content:      append([]byte(nil), content...),
	}
}

func NewUpdateFile(path string, mode uint32, before string, content []byte) Change {
	return Change{Kind: UpdateFile, Path: path, BeforeSHA256: before, AfterSHA256: HashBytes(content), Mode: mode, Size: int64(len(content)), content: append([]byte(nil), content...)}
}

func NewDeleteFile(path, before string) Change {
	return Change{Kind: DeleteFile, Path: path, BeforeSHA256: before, AfterSHA256: MissingSHA256}
}

func NewDeleteDirectory(path string) Change {
	return Change{Kind: DeleteDir, Path: path, BeforeSHA256: DirectorySHA256, AfterSHA256: MissingSHA256}
}

func (c Change) WithSequence(sequence int) Change { c.Sequence = sequence; return c }

func (c Change) Content() []byte {
	return append([]byte(nil), c.content...)
}

type Plan struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
	Workspace string `json:"workspace,omitempty"`
	changes   []Change
	inputs    []Input
	metadata  Metadata
}

type PersistedChange struct {
	Kind         ChangeKind `json:"kind"`
	Path         string     `json:"path"`
	BeforeSHA256 string     `json:"beforeSHA256"`
	AfterSHA256  string     `json:"afterSHA256"`
	Mode         uint32     `json:"mode"`
	Size         int64      `json:"size"`
	Sequence     int        `json:"sequence"`
	Content      []byte     `json:"content,omitempty"`
}

type PersistedPlan struct {
	ID        string            `json:"id"`
	Operation string            `json:"operation"`
	Workspace string            `json:"workspace,omitempty"`
	Changes   []PersistedChange `json:"changes"`
	Inputs    []Input           `json:"inputs"`
	Metadata  Metadata          `json:"metadata"`
}

type Metadata struct {
	ActorWorkerID        string `json:"actorWorkerID,omitempty"`
	Reason               string `json:"reason,omitempty"`
	ControllerID         string `json:"controllerID,omitempty"`
	TransactionID        string `json:"transactionID,omitempty"`
	JournalSHA256        string `json:"journalSHA256,omitempty"`
	RecoveryAction       string `json:"recoveryAction,omitempty"`
	MigrationFrom        string `json:"migrationFrom,omitempty"`
	MigrationTo          string `json:"migrationTo,omitempty"`
	MigrationReceiptPath string `json:"migrationReceiptPath,omitempty"`
	PurgeExportPath      string `json:"purgeExportPath,omitempty"`
}

// Input is a read-only project fact on which an approval depends.
type Input struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func New(operation string, input []Change) Plan {
	return NewForWorkspace(operation, "", input)
}

func NewForWorkspace(operation, workspace string, input []Change) Plan {
	return NewForWorkspaceInputs(operation, workspace, input, nil)
}

func NewForWorkspaceInputs(operation, workspace string, input []Change, inputs []Input) Plan {
	return NewForWorkspaceInputsMetadata(operation, workspace, input, inputs, Metadata{})
}

func NewForWorkspaceInputsMetadata(operation, workspace string, input []Change, inputs []Input, metadata Metadata) Plan {
	changes := cloneChanges(input)
	explicitSequence := false
	for index := range changes {
		if changes[index].Sequence != 0 {
			explicitSequence = true
		}
	}
	inputs = append([]Input(nil), inputs...)
	if explicitSequence {
		sort.Slice(changes, func(i, j int) bool { return changes[i].Sequence < changes[j].Sequence })
	} else {
		sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Path < inputs[j].Path })
	canonical, err := json.Marshal(struct {
		Operation string   `json:"operation"`
		Workspace string   `json:"workspace,omitempty"`
		Changes   []Change `json:"changes"`
		Inputs    []Input  `json:"inputs"`
		Metadata  Metadata `json:"metadata"`
	}{operation, workspace, changes, inputs, metadata})
	if err != nil {
		panic(fmt.Sprintf("canonical plan encoding failed: %v", err))
	}
	return Plan{ID: HashBytes(canonical), Operation: operation, Workspace: workspace, changes: changes, inputs: inputs, metadata: metadata}
}
func (p Plan) Inputs() []Input    { return append([]Input(nil), p.inputs...) }
func (p Plan) Metadata() Metadata { return p.metadata }

func (p Plan) Changes() []Change {
	return cloneChanges(p.changes)
}

func (p Plan) Persisted() PersistedPlan {
	result := PersistedPlan{ID: p.ID, Operation: p.Operation, Workspace: p.Workspace, Inputs: p.Inputs(), Metadata: p.Metadata()}
	for _, change := range p.Changes() {
		result.Changes = append(result.Changes, PersistedChange{
			Kind: change.Kind, Path: change.Path, BeforeSHA256: change.BeforeSHA256,
			AfterSHA256: change.AfterSHA256, Mode: change.Mode, Size: change.Size,
			Sequence: change.Sequence, Content: change.Content(),
		})
	}
	return result
}

func Restore(persisted PersistedPlan) (Plan, error) {
	if persisted.ID == "" || persisted.Operation == "" {
		return Plan{}, fmt.Errorf("persisted plan identity is incomplete")
	}
	changes := make([]Change, len(persisted.Changes))
	for index, value := range persisted.Changes {
		change, err := RestoreChange(value)
		if err != nil {
			return Plan{}, fmt.Errorf("persisted change %d: %w", index, err)
		}
		changes[index] = change
	}
	restored := NewForWorkspaceInputsMetadata(persisted.Operation, persisted.Workspace, changes, persisted.Inputs, persisted.Metadata)
	if restored.ID != persisted.ID {
		return Plan{}, fmt.Errorf("persisted plan ID mismatch")
	}
	return restored, nil
}

func RestoreChange(value PersistedChange) (Change, error) {
	if value.Path == "" || value.Size < 0 {
		return Change{}, fmt.Errorf("invalid path or size")
	}
	change := Change{
		Kind: value.Kind, Path: value.Path, BeforeSHA256: value.BeforeSHA256,
		AfterSHA256: value.AfterSHA256, Mode: value.Mode, Size: value.Size,
		Sequence: value.Sequence, content: append([]byte(nil), value.Content...),
	}
	switch change.Kind {
	case CreateDir:
		if change.BeforeSHA256 != MissingSHA256 || change.AfterSHA256 != DirectorySHA256 || change.Size != 0 || len(change.content) != 0 {
			return Change{}, fmt.Errorf("invalid directory change")
		}
	case CreateFile, UpdateFile:
		if change.Size != int64(len(change.content)) || change.AfterSHA256 != HashBytes(change.content) {
			return Change{}, fmt.Errorf("content does not match size or after hash")
		}
	case DeleteFile:
		if change.AfterSHA256 != MissingSHA256 || change.Size != 0 || len(change.content) != 0 {
			return Change{}, fmt.Errorf("invalid delete change")
		}
	case DeleteDir:
		if change.BeforeSHA256 != DirectorySHA256 || change.AfterSHA256 != MissingSHA256 || change.Size != 0 || len(change.content) != 0 {
			return Change{}, fmt.Errorf("invalid directory deletion")
		}
	default:
		return Change{}, fmt.Errorf("unsupported change kind %q", change.Kind)
	}
	return change, nil
}

func (p Plan) Preview() string {
	var output strings.Builder
	fmt.Fprintf(&output, "plan %s operation=%s\n", p.ID, p.Operation)
	for _, change := range p.changes {
		fmt.Fprintf(&output, "%s %s before=%s after=%s %d bytes mode=%#o\n",
			change.Kind, change.Path, change.BeforeSHA256, change.AfterSHA256, change.Size, change.Mode)
	}
	return output.String()
}

func HashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func cloneChanges(input []Change) []Change {
	output := make([]Change, len(input))
	for i, change := range input {
		output[i] = change
		output[i].content = append([]byte(nil), change.content...)
	}
	return output
}
