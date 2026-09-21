package adapter

import (
	"sort"

	"github.com/uawp/uawp/internal/core"
)

type EntryState string

const (
	Effective         EntryState = "EFFECTIVE"
	CoLoaded          EntryState = "CO_LOADED"
	FallbackEffective EntryState = "FALLBACK_EFFECTIVE"
	Shadowed          EntryState = "SHADOWED"
	Unavailable       EntryState = "UNAVAILABLE"
	Unknown           EntryState = "UNKNOWN"
	Missing           EntryState = "MISSING"
)

type Confidence string

const (
	Verified    Confidence = "VERIFIED"
	Conditional Confidence = "CONDITIONAL"
	Unsupported Confidence = "UNSUPPORTED"
)

type FileState string

const (
	FileMissing     FileState = "MISSING"
	FileRegular     FileState = "REGULAR"
	FileUnsafe      FileState = "UNSAFE"
	FileBinary      FileState = "BINARY"
	FileInvalidUTF8 FileState = "INVALID_UTF8"
	FileTooLarge    FileState = "TOO_LARGE"
)

type Evidence struct {
	OfficialURLs []string `json:"officialURLs"`
	VerifiedAt   string   `json:"verifiedAt"`
	Versions     string   `json:"versions,omitempty"`
}
type FileFact struct {
	Path    string    `json:"path"`
	State   FileState `json:"state"`
	SHA256  string    `json:"sha256,omitempty"`
	Content []byte    `json:"-"`
	Mode    uint32    `json:"-"`
}
type Snapshot struct {
	Root            string
	Files           map[string]FileFact
	ProviderVersion string
	Options         map[string]string
}
type Candidate struct {
	Path   string     `json:"path"`
	State  EntryState `json:"state"`
	Reason string     `json:"reason,omitempty"`
}
type Route struct {
	Path   string               `json:"path,omitempty"`
	Mode   core.IntegrationMode `json:"mode,omitempty"`
	Target string               `json:"target,omitempty"`
	Create bool                 `json:"create,omitempty"`
}
type Finding struct {
	Code       string `json:"code"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	NextAction string `json:"nextAction,omitempty"`
}
type Resolution struct {
	Provider   string      `json:"provider"`
	Candidates []Candidate `json:"candidates"`
	Route      Route       `json:"route"`
	Confidence Confidence  `json:"confidence"`
	Findings   []Finding   `json:"findings,omitempty"`
	Health     string      `json:"health,omitempty"`
}
type RuntimeFacts struct {
	Versions map[string]string
	Options  map[string]map[string]string
}

type Adapter interface {
	ID() string
	Evidence() Evidence
	Resolve(Snapshot) Resolution
}

func (r Resolution) Canonical() Resolution {
	r.Candidates = append([]Candidate(nil), r.Candidates...)
	r.Findings = append([]Finding(nil), r.Findings...)
	sort.Slice(r.Candidates, func(i, j int) bool { return r.Candidates[i].Path < r.Candidates[j].Path })
	sort.Slice(r.Findings, func(i, j int) bool { return r.Findings[i].Code < r.Findings[j].Code })
	return r
}
