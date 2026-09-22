package identity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const registrySchemaVersion = "1"

type Registry struct {
	SchemaVersion    string           `json:"schemaVersion"`
	DefaultProfileID string           `json:"defaultProfileID,omitempty"`
	Profiles         []Profile        `json:"profiles"`
	Bindings         []SessionBinding `json:"bindings,omitempty"`
}

func DefaultPath(userConfigDir string) string {
	return filepath.Join(userConfigDir, "uawp", "identity.json")
}

func (r Registry) Selected(profileID string, allowDefault bool) (Profile, error) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" && allowDefault {
		profileID = r.DefaultProfileID
	}
	if profileID == "" {
		return Profile{}, fmt.Errorf("profile selection is required")
	}
	for _, profile := range r.Profiles {
		if profile.ProfileID == profileID {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("profile %q does not exist", profileID)
}

func Load(path string) (Registry, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Registry{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Registry{}, fmt.Errorf("identity registry is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return Registry{}, fmt.Errorf("identity registry permissions must not allow group or world access")
	}
	if runtime.GOOS != "windows" {
		parent, parentErr := os.Stat(filepath.Dir(path))
		if parentErr != nil {
			return Registry{}, parentErr
		}
		if !parent.IsDir() || parent.Mode().Perm()&0o077 != 0 {
			return Registry{}, fmt.Errorf("identity registry directory permissions must be private")
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, err
	}
	if err := rejectDuplicateJSONKeys(content); err != nil {
		return Registry{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var value Registry
	if err := decoder.Decode(&value); err != nil {
		return Registry{}, fmt.Errorf("decode identity registry: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Registry{}, err
	}
	if err := validateRegistry(value); err != nil {
		return Registry{}, err
	}
	return value, nil
}

func Save(path string, value Registry) error {
	if value.SchemaVersion == "" {
		value.SchemaVersion = registrySchemaVersion
	}
	if err := validateRegistry(value); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("identity registry is not a regular file")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(parent)
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("identity registry directory permissions must be private")
		}
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	temporary, err := os.CreateTemp(parent, ".identity-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	if runtime.GOOS != "windows" {
		directory, err := os.Open(parent)
		if err != nil {
			return err
		}
		err = directory.Sync()
		closeErr := directory.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func validateRegistry(value Registry) error {
	if value.SchemaVersion != registrySchemaVersion {
		return fmt.Errorf("unsupported identity registry schema %q", value.SchemaVersion)
	}
	profiles := make(map[string]bool, len(value.Profiles))
	workers := make(map[string]bool, len(value.Profiles))
	for _, profile := range value.Profiles {
		if strings.TrimSpace(profile.ProfileID) == "" || strings.ContainsAny(profile.ProfileID, "\r\n") {
			return fmt.Errorf("invalid profile ID")
		}
		if strings.TrimSpace(profile.WorkerID) == "" || strings.ContainsAny(profile.WorkerID, "\r\n") {
			return fmt.Errorf("invalid Worker ID")
		}
		if _, err := ValidateDisplayName(profile.DisplayName); err != nil {
			return err
		}
		if profiles[profile.ProfileID] || workers[profile.WorkerID] {
			return fmt.Errorf("duplicate profile or Worker ID")
		}
		profiles[profile.ProfileID] = true
		workers[profile.WorkerID] = true
	}
	if value.DefaultProfileID != "" && !profiles[value.DefaultProfileID] {
		return fmt.Errorf("default profile does not exist")
	}
	bindings := map[string]bool{}
	for _, binding := range value.Bindings {
		if binding.Workspace == "" || !filepath.IsAbs(binding.Workspace) || !profiles[binding.ProfileID] || strings.TrimSpace(binding.SessionID) == "" || binding.Generation == 0 {
			return fmt.Errorf("invalid Session binding")
		}
		key := binding.Workspace + "\x00" + binding.ProfileID
		if bindings[key] {
			return fmt.Errorf("duplicate Session binding")
		}
		bindings[key] = true
	}
	return nil
}

func rejectDuplicateJSONKeys(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	if err := inspectJSONValue(decoder); err != nil {
		return fmt.Errorf("invalid identity registry JSON: %w", err)
	}
	return requireJSONEOF(decoder)
}

func inspectJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate key %q", key)
			}
			seen[key] = true
			if err := inspectJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := inspectJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("identity registry contains trailing data")
		}
		return fmt.Errorf("decode identity registry trailing data: %w", err)
	}
	return nil
}
