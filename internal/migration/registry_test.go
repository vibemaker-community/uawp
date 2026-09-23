package migration

import (
	"testing"

	"github.com/vibemaker-community/uawp/internal/plan"
)

type testStep struct{ from, to string }

func (s testStep) From() string                        { return s.from }
func (s testStep) To() string                          { return s.to }
func (s testStep) Plan(Context) ([]plan.Change, error) { return nil, nil }
func (s testStep) Verify(Context) error                { return nil }

func TestRegistryResolvesDeterministicForwardChain(t *testing.T) {
	registry, err := NewRegistry([]Step{
		testStep{from: "1.1.0", to: "1.2.0"},
		testStep{from: "1.0.0", to: "1.1.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := registry.Resolve("1.0.0", "1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0].From() != "1.0.0" || steps[1].To() != "1.2.0" {
		t.Fatalf("Resolve() = %#v", steps)
	}
	current, err := registry.Resolve("1.2.0", "1.2.0")
	if err != nil || len(current) != 0 {
		t.Fatalf("current resolve = %#v, %v", current, err)
	}
}

func TestRegistryRejectsInvalidGraphs(t *testing.T) {
	tests := map[string][]Step{
		"duplicate edge": {testStep{"1.0.0", "1.1.0"}, testStep{"1.0.0", "1.1.0"}},
		"branch":         {testStep{"1.0.0", "1.1.0"}, testStep{"1.0.0", "1.2.0"}},
		"cycle":          {testStep{"1.0.0", "1.1.0"}, testStep{"1.1.0", "1.0.0"}},
		"downgrade":      {testStep{"1.1.0", "1.0.0"}},
		"malformed":      {testStep{"one", "1.0.0"}},
	}
	for name, steps := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRegistry(steps); err == nil {
				t.Fatal("NewRegistry accepted invalid graph")
			}
		})
	}
}

func TestRegistryRejectsMissingPathAndDowngradeRequest(t *testing.T) {
	registry, err := NewRegistry([]Step{testStep{"1.0.0", "1.1.0"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range [][2]string{{"1.0.0", "1.2.0"}, {"1.1.0", "1.0.0"}} {
		if _, err := registry.Resolve(request[0], request[1]); err == nil {
			t.Fatalf("Resolve(%q, %q) succeeded", request[0], request[1])
		}
	}
}
