package core

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
)

const ProtocolName = "UAWP"

var stateVersionPattern = regexp.MustCompile(`^1\.[0-9]+\.[0-9]+$`)
var integrationIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type IntegrationMode string

const (
	Direct       IntegrationMode = "DIRECT"
	Import       IntegrationMode = "IMPORT"
	ManagedBlock IntegrationMode = "MANAGED_BLOCK"
)

type IntegrationArtifact struct {
	ID                   string                       `json:"id"`
	Path                 string                       `json:"path"`
	Mode                 IntegrationMode              `json:"mode"`
	Target               string                       `json:"target"`
	Consumers            []string                     `json:"consumers"`
	ConsumerFacts        map[string]map[string]string `json:"consumerFacts,omitempty"`
	CreatedFile          bool                         `json:"createdFile"`
	Inserted             bool                         `json:"inserted,omitempty"`
	ArtifactSHA256       string                       `json:"artifactSHA256"`
	OutsideContentSHA256 string                       `json:"outsideContentSHA256,omitempty"`
}

// Manifest establishes that a .uawp namespace belongs to UAWP and identifies
// its state compatibility version.
type Manifest struct {
	Protocol     string                `json:"protocol"`
	StateVersion string                `json:"stateVersion"`
	Integrations []IntegrationArtifact `json:"integrations,omitempty"`
}

// DecodeManifest strictly decodes one manifest object. Duplicate and unknown
// fields are rejected so ownership and version meaning cannot be shadowed.
func DecodeManifest(r io.Reader) (Manifest, error) {
	decoder := json.NewDecoder(r)

	start, err := decoder.Token()
	if err != nil {
		return Manifest{}, newDomainError(ErrInvalidState, "decode manifest: %v", err)
	}
	if delimiter, ok := start.(json.Delim); !ok || delimiter != '{' {
		return Manifest{}, newDomainError(ErrInvalidState, "manifest must be a JSON object")
	}

	var manifest Manifest
	seen := make(map[string]struct{}, 2)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return Manifest{}, newDomainError(ErrInvalidState, "decode manifest field: %v", err)
		}
		key, ok := token.(string)
		if !ok {
			return Manifest{}, newDomainError(ErrInvalidState, "manifest field name is not a string")
		}
		if _, exists := seen[key]; exists {
			return Manifest{}, newDomainError(ErrInvalidState, "duplicate manifest field %q", key)
		}
		seen[key] = struct{}{}

		switch key {
		case "protocol":
			var value string
			if err := decoder.Decode(&value); err != nil {
				return Manifest{}, newDomainError(ErrInvalidState, "manifest field %q must be a string: %v", key, err)
			}
			manifest.Protocol = value
		case "stateVersion":
			var value string
			if err := decoder.Decode(&value); err != nil {
				return Manifest{}, newDomainError(ErrInvalidState, "manifest field %q must be a string: %v", key, err)
			}
			manifest.StateVersion = value
		case "integrations":
			if err := decoder.Decode(&manifest.Integrations); err != nil {
				return Manifest{}, newDomainError(ErrInvalidState, "decode integrations: %v", err)
			}
		default:
			var discarded json.RawMessage
			_ = decoder.Decode(&discarded)
			return Manifest{}, newDomainError(ErrInvalidState, "unknown manifest field %q", key)
		}
	}

	end, err := decoder.Token()
	if err != nil {
		return Manifest{}, newDomainError(ErrInvalidState, "close manifest object: %v", err)
	}
	if delimiter, ok := end.(json.Delim); !ok || delimiter != '}' {
		return Manifest{}, newDomainError(ErrInvalidState, "manifest object is not closed")
	}
	if token, err := decoder.Token(); err != io.EOF {
		if err != nil {
			return Manifest{}, newDomainError(ErrInvalidState, "decode trailing manifest data: %v", err)
		}
		return Manifest{}, newDomainError(ErrInvalidState, "unexpected trailing manifest value %v", token)
	}

	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// ValidateManifest checks namespace ownership and supported state versions.
func ValidateManifest(manifest Manifest) error {
	if manifest.Protocol != ProtocolName {
		return newDomainError(ErrUnknownNamespace, "protocol must be %q, got %q", ProtocolName, manifest.Protocol)
	}
	if !stateVersionPattern.MatchString(manifest.StateVersion) {
		return newDomainError(ErrUnsupportedVersion, "unsupported state version %q", manifest.StateVersion)
	}
	seenIDs, seenPaths, seenConsumers := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for i := range manifest.Integrations {
		artifact := manifest.Integrations[i]
		if err := validateIntegration(artifact); err != nil {
			return newDomainError(ErrInvalidState, "integration %d: %v", i, err)
		}
		if seenIDs[artifact.ID] {
			return newDomainError(ErrInvalidState, "duplicate integration id %q", artifact.ID)
		}
		if seenPaths[artifact.Path] {
			return newDomainError(ErrInvalidState, "duplicate integration path %q", artifact.Path)
		}
		seenIDs[artifact.ID], seenPaths[artifact.Path] = true, true
		for _, consumer := range artifact.Consumers {
			if seenConsumers[consumer] {
				return newDomainError(ErrInvalidState, "consumer %q is registered by multiple artifacts", consumer)
			}
			seenConsumers[consumer] = true
		}
	}
	return nil
}

func validateIntegration(a IntegrationArtifact) error {
	if !integrationIDPattern.MatchString(a.ID) {
		return fmt.Errorf("invalid id %q", a.ID)
	}
	for name, value := range map[string]string{"path": a.Path, "target": a.Target} {
		if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
			return fmt.Errorf("invalid %s %q", name, value)
		}
	}
	if a.Mode != Direct && a.Mode != Import && a.Mode != ManagedBlock {
		return fmt.Errorf("invalid mode %q", a.Mode)
	}
	if !sha256Pattern.MatchString(a.ArtifactSHA256) {
		return fmt.Errorf("invalid artifact hash")
	}
	if a.OutsideContentSHA256 != "" && !sha256Pattern.MatchString(a.OutsideContentSHA256) {
		return fmt.Errorf("invalid outside-content hash")
	}
	if len(a.Consumers) == 0 {
		return fmt.Errorf("at least one consumer is required")
	}
	seen := map[string]bool{}
	for _, consumer := range a.Consumers {
		if !integrationIDPattern.MatchString(consumer) || seen[consumer] {
			return fmt.Errorf("invalid or duplicate consumer %q", consumer)
		}
		seen[consumer] = true
	}
	for consumer := range a.ConsumerFacts {
		if !seen[consumer] {
			return fmt.Errorf("facts provided for unregistered consumer %q", consumer)
		}
	}
	return nil
}

func (a *IntegrationArtifact) UnmarshalJSON(data []byte) error {
	type alias IntegrationArtifact
	var decoded alias
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&decoded); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := rejectDuplicateObjectKeys(data, &keys); err != nil {
		return err
	}
	*a = IntegrationArtifact(decoded)
	return nil
}

func rejectDuplicateObjectKeys(data []byte, target *map[string]json.RawMessage) error {
	d := json.NewDecoder(bytesReaderCore(data))
	token, err := d.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("must be object")
	}
	*target = map[string]json.RawMessage{}
	for d.More() {
		keyToken, err := d.Token()
		if err != nil {
			return err
		}
		key := keyToken.(string)
		if _, ok := (*target)[key]; ok {
			return fmt.Errorf("duplicate field %q", key)
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return err
		}
		(*target)[key] = raw
	}
	_, err = d.Token()
	return err
}

type coreBytesReader struct {
	data   []byte
	offset int
}

func bytesReaderCore(data []byte) io.Reader { return &coreBytesReader{data: data} }
func (r *coreBytesReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

// EncodeManifest returns the canonical indented representation used by init.
func EncodeManifest(manifest Manifest) ([]byte, error) {
	manifest.Integrations = append([]IntegrationArtifact(nil), manifest.Integrations...)
	for i := range manifest.Integrations {
		manifest.Integrations[i].Consumers = append([]string(nil), manifest.Integrations[i].Consumers...)
		sort.Strings(manifest.Integrations[i].Consumers)
	}
	sort.Slice(manifest.Integrations, func(i, j int) bool { return manifest.Integrations[i].ID < manifest.Integrations[j].ID })
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return append(data, '\n'), nil
}
