package core

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

const ProtocolName = "UAWP"

var stateVersionPattern = regexp.MustCompile(`^1\.[0-9]+\.[0-9]+$`)

// Manifest establishes that a .uawp namespace belongs to UAWP and identifies
// its state compatibility version.
type Manifest struct {
	Protocol     string `json:"protocol"`
	StateVersion string `json:"stateVersion"`
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

		var value string
		if err := decoder.Decode(&value); err != nil {
			return Manifest{}, newDomainError(ErrInvalidState, "manifest field %q must be a string: %v", key, err)
		}
		switch key {
		case "protocol":
			manifest.Protocol = value
		case "stateVersion":
			manifest.StateVersion = value
		default:
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
	return nil
}

// EncodeManifest returns the canonical indented representation used by init.
func EncodeManifest(manifest Manifest) ([]byte, error) {
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return append(data, '\n'), nil
}
