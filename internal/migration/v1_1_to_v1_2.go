package migration

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type V1_1ToV1_2 struct{}

func (V1_1ToV1_2) From() string { return "1.1.0" }
func (V1_1ToV1_2) To() string   { return "1.2.0" }

func (step V1_1ToV1_2) Plan(context Context) ([]plan.Change, error) {
	if context.Manifest.Protocol != core.ProtocolName || context.Manifest.StateVersion != step.From() {
		return nil, fmt.Errorf("migration requires %s state version %s", core.ProtocolName, step.From())
	}
	const path = ".uawp/ACTIVE_WORKER.md"
	before, ok := context.Files[path]
	if !ok {
		return nil, fmt.Errorf("migration ownership snapshot is missing")
	}
	inputHash := ""
	for _, input := range context.Inputs {
		if input.Path == path {
			inputHash = input.SHA256
			break
		}
	}
	if inputHash == "" || inputHash != plan.HashBytes(before) {
		return nil, fmt.Errorf("migration ownership input does not match snapshot")
	}
	legacy, err := decodeV1_1Ownership(before)
	if err != nil {
		return nil, err
	}
	if legacy.Status != core.Released {
		return nil, fmt.Errorf("upgrade requires RELEASED ownership")
	}
	after, err := core.EncodeOwnership(legacy)
	if err != nil {
		return nil, err
	}
	return []plan.Change{plan.NewUpdateFile(path, 0o600, inputHash, after).WithSequence(80)}, nil
}

func (step V1_1ToV1_2) Verify(context Context) error {
	if context.Manifest.Protocol != core.ProtocolName || context.Manifest.StateVersion != step.To() {
		return fmt.Errorf("migration target state is not %s", step.To())
	}
	return nil
}

func decodeV1_1Ownership(content []byte) (core.Ownership, error) {
	allowed := map[string]bool{
		"Status": true, "Worker ID": true, "Agent": true,
		"Acquired At": true, "Released At": true, "Purpose": true,
	}
	fields := make(map[string]string, len(allowed))
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		key, value, found := strings.Cut(strings.TrimPrefix(line, "- "), ":")
		if !found {
			return core.Ownership{}, fmt.Errorf("malformed legacy ownership field")
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !allowed[key] {
			return core.Ownership{}, fmt.Errorf("unknown legacy ownership field %q", key)
		}
		if _, duplicate := fields[key]; duplicate {
			return core.Ownership{}, fmt.Errorf("duplicate legacy ownership field %q", key)
		}
		fields[key] = value
	}
	if err := scanner.Err(); err != nil {
		return core.Ownership{}, fmt.Errorf("scan legacy ownership: %w", err)
	}
	for key := range allowed {
		if _, ok := fields[key]; !ok {
			return core.Ownership{}, fmt.Errorf("missing legacy ownership field %q", key)
		}
	}
	acquired, err := core.ParseTimestamp(fields["Acquired At"])
	if err != nil {
		return core.Ownership{}, err
	}
	value := core.Ownership{Status: core.OwnershipStatus(fields["Status"]), WorkerID: fields["Worker ID"], Agent: fields["Agent"], AcquiredAt: acquired, Purpose: fields["Purpose"]}
	if released := fields["Released At"]; released != "" && !strings.EqualFold(released, "none") {
		parsed, parseErr := core.ParseTimestamp(released)
		if parseErr != nil {
			return core.Ownership{}, parseErr
		}
		value.ReleasedAt = &parsed
	}
	for name, field := range map[string]string{"worker ID": value.WorkerID, "agent": value.Agent, "purpose": value.Purpose} {
		if strings.TrimSpace(field) == "" || strings.ContainsAny(field, "\r\n") {
			return core.Ownership{}, fmt.Errorf("invalid legacy ownership %s", name)
		}
	}
	switch value.Status {
	case core.Active:
		if value.ReleasedAt != nil {
			return core.Ownership{}, fmt.Errorf("legacy ACTIVE ownership cannot have a release time")
		}
	case core.Released:
		if value.ReleasedAt == nil || value.ReleasedAt.Before(value.AcquiredAt) {
			return core.Ownership{}, fmt.Errorf("invalid legacy RELEASED ownership time")
		}
	default:
		return core.Ownership{}, fmt.Errorf("unknown legacy ownership status %q", value.Status)
	}
	return value, nil
}
