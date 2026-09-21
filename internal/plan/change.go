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

func (c Change) Content() []byte {
	return append([]byte(nil), c.content...)
}

type Plan struct {
	ID        string `json:"id"`
	Operation string `json:"operation"`
	Workspace string `json:"workspace,omitempty"`
	changes   []Change
}

func New(operation string, input []Change) Plan {
	return NewForWorkspace(operation, "", input)
}

func NewForWorkspace(operation, workspace string, input []Change) Plan {
	changes := cloneChanges(input)
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	canonical, err := json.Marshal(struct {
		Operation string   `json:"operation"`
		Workspace string   `json:"workspace,omitempty"`
		Changes   []Change `json:"changes"`
	}{operation, workspace, changes})
	if err != nil {
		panic(fmt.Sprintf("canonical plan encoding failed: %v", err))
	}
	return Plan{ID: HashBytes(canonical), Operation: operation, Workspace: workspace, changes: changes}
}

func (p Plan) Changes() []Change {
	return cloneChanges(p.changes)
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
