package identity

import (
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Profile struct {
	ProfileID   string `json:"profileID"`
	WorkerID    string `json:"workerID"`
	DisplayName string `json:"displayName"`
}

func ValidateDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("display name is required")
	}
	if utf8.RuneCountInString(value) > 100 {
		return "", fmt.Errorf("display name exceeds 100 Unicode code points")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("display name contains a control character")
		}
	}
	return value, nil
}

func NewProfile(displayName string, random io.Reader) (Profile, error) {
	name, err := ValidateDisplayName(displayName)
	if err != nil {
		return Profile{}, err
	}
	profileID, err := randomID("profile-", random)
	if err != nil {
		return Profile{}, fmt.Errorf("generate profile ID: %w", err)
	}
	workerID, err := randomID("worker-", random)
	if err != nil {
		return Profile{}, fmt.Errorf("generate Worker ID: %w", err)
	}
	return Profile{ProfileID: profileID, WorkerID: workerID, DisplayName: name}, nil
}

func randomID(prefix string, random io.Reader) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(random, value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}
