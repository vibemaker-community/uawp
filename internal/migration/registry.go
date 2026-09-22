package migration

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type Context struct {
	Manifest    core.Manifest
	Inputs      []plan.Input
	GeneratedAt time.Time
}

type Step interface {
	From() string
	To() string
	Plan(Context) ([]plan.Change, error)
	Verify(Context) error
}

type Registry struct {
	bySource map[string]Step
}

func NewRegistry(steps []Step) (Registry, error) {
	ordered := append([]Step(nil), steps...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].From() == ordered[j].From() {
			return ordered[i].To() < ordered[j].To()
		}
		return ordered[i].From() < ordered[j].From()
	})
	registry := Registry{bySource: make(map[string]Step, len(ordered))}
	for _, step := range ordered {
		if step == nil {
			return Registry{}, fmt.Errorf("migration step is nil")
		}
		from, err := parseVersion(step.From())
		if err != nil {
			return Registry{}, fmt.Errorf("invalid migration source %q: %w", step.From(), err)
		}
		to, err := parseVersion(step.To())
		if err != nil {
			return Registry{}, fmt.Errorf("invalid migration target %q: %w", step.To(), err)
		}
		if compareVersion(from, to) >= 0 {
			return Registry{}, fmt.Errorf("migration %s -> %s is not forward", step.From(), step.To())
		}
		if previous, exists := registry.bySource[step.From()]; exists {
			return Registry{}, fmt.Errorf("ambiguous migrations from %s to %s and %s", step.From(), previous.To(), step.To())
		}
		registry.bySource[step.From()] = step
	}
	for source := range registry.bySource {
		seen := map[string]bool{}
		current := source
		for {
			if seen[current] {
				return Registry{}, fmt.Errorf("migration cycle at %s", current)
			}
			seen[current] = true
			step, exists := registry.bySource[current]
			if !exists {
				break
			}
			current = step.To()
		}
	}
	return registry, nil
}

func (r Registry) Resolve(from, to string) ([]Step, error) {
	fromVersion, err := parseVersion(from)
	if err != nil {
		return nil, fmt.Errorf("invalid source version %q: %w", from, err)
	}
	toVersion, err := parseVersion(to)
	if err != nil {
		return nil, fmt.Errorf("invalid target version %q: %w", to, err)
	}
	if compareVersion(fromVersion, toVersion) > 0 {
		return nil, fmt.Errorf("downgrade from %s to %s is unsupported", from, to)
	}
	if from == to {
		return nil, nil
	}
	var result []Step
	current := from
	seen := map[string]bool{}
	for current != to {
		if seen[current] {
			return nil, fmt.Errorf("migration cycle while resolving %s to %s", from, to)
		}
		seen[current] = true
		step, exists := r.bySource[current]
		if !exists {
			return nil, fmt.Errorf("no migration path from %s to %s", from, to)
		}
		nextVersion, _ := parseVersion(step.To())
		if compareVersion(nextVersion, toVersion) > 0 {
			return nil, fmt.Errorf("migration path from %s skips target %s", current, to)
		}
		result = append(result, step)
		current = step.To()
	}
	return result, nil
}

func parseVersion(value string) ([3]int, error) {
	var result [3]int
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("expected semantic version")
	}
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return result, fmt.Errorf("invalid numeric component")
		}
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed < 0 {
			return result, fmt.Errorf("invalid numeric component")
		}
		result[index] = parsed
	}
	return result, nil
}

func compareVersion(left, right [3]int) int {
	for index := range left {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}
