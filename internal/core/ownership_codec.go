package core

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const maxOwnershipBytes = 1 << 20

var ownershipFields = map[string]bool{
	"Status": true, "Worker ID": true, "Agent": true,
	"Session ID": true, "Generation": true, "Acquired At": true,
	"Released At": true, "Purpose": true,
}

func DecodeOwnership(reader io.Reader) (Ownership, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maxOwnershipBytes+1))
	if err != nil {
		return Ownership{}, newDomainError(ErrInvalidState, "read ownership: %v", err)
	}
	if len(content) > maxOwnershipBytes {
		return Ownership{}, newDomainError(ErrInvalidState, "ownership document exceeds 1 MiB")
	}
	fields := make(map[string]string, len(ownershipFields))
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
		if !ok {
			return Ownership{}, newDomainError(ErrInvalidState, "malformed ownership field")
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ownershipFields[key] {
			return Ownership{}, newDomainError(ErrInvalidState, "unknown ownership field %q", key)
		}
		if _, exists := fields[key]; exists {
			return Ownership{}, newDomainError(ErrInvalidState, "duplicate ownership field %q", key)
		}
		fields[key] = value
	}
	if err := scanner.Err(); err != nil {
		return Ownership{}, newDomainError(ErrInvalidState, "scan ownership: %v", err)
	}
	for key := range ownershipFields {
		if _, ok := fields[key]; !ok {
			return Ownership{}, newDomainError(ErrInvalidState, "missing ownership field %q", key)
		}
	}
	acquired, err := ParseTimestamp(fields["Acquired At"])
	if err != nil {
		return Ownership{}, err
	}
	generation, err := strconv.ParseUint(fields["Generation"], 10, 64)
	if err != nil {
		return Ownership{}, newDomainError(ErrInvalidState, "invalid ownership generation %q", fields["Generation"])
	}
	sessionID := fields["Session ID"]
	if strings.EqualFold(sessionID, "none") {
		sessionID = ""
	}
	value := Ownership{Status: OwnershipStatus(fields["Status"]), WorkerID: fields["Worker ID"], SessionID: sessionID, Generation: generation, Agent: fields["Agent"], AcquiredAt: acquired, Purpose: fields["Purpose"]}
	if released := fields["Released At"]; released != "" && !strings.EqualFold(released, "none") {
		parsed, err := ParseTimestamp(released)
		if err != nil {
			return Ownership{}, err
		}
		value.ReleasedAt = &parsed
	}
	if err := ValidateOwnership(value); err != nil {
		return Ownership{}, err
	}
	return value, nil
}

func EncodeOwnership(value Ownership) ([]byte, error) {
	if err := ValidateOwnership(value); err != nil {
		return nil, err
	}
	released := "none"
	if value.ReleasedAt != nil {
		released = FormatTimestamp(*value.ReleasedAt)
	}
	sessionID := value.SessionID
	if sessionID == "" {
		sessionID = "none"
	}
	return []byte(fmt.Sprintf("# UAWP Active Worker\n\n- Status: %s\n- Worker ID: %s\n- Session ID: %s\n- Generation: %d\n- Agent: %s\n- Acquired At: %s\n- Released At: %s\n- Purpose: %s\n", value.Status, value.WorkerID, sessionID, value.Generation, value.Agent, FormatTimestamp(value.AcquiredAt), released, value.Purpose)), nil
}
