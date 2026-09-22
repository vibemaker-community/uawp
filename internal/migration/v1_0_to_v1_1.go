package migration

import (
	"fmt"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

type V1_0ToV1_1 struct{}

func (V1_0ToV1_1) From() string { return "1.0.0" }
func (V1_0ToV1_1) To() string   { return "1.1.0" }

func (step V1_0ToV1_1) Plan(context Context) ([]plan.Change, error) {
	if context.Manifest.Protocol != core.ProtocolName || context.Manifest.StateVersion != step.From() {
		return nil, fmt.Errorf("migration requires %s state version %s", core.ProtocolName, step.From())
	}
	inputs := make(map[string]string, len(context.Inputs))
	for _, input := range context.Inputs {
		inputs[input.Path] = input.SHA256
	}
	manifestHash, ok := inputs[".uawp/manifest.json"]
	if !ok || manifestHash == plan.MissingSHA256 || manifestHash == plan.DirectorySHA256 {
		return nil, fmt.Errorf("migration manifest input is missing")
	}
	var changes []plan.Change
	for index, path := range []string{".uawp/migrations", ".uawp/recovery"} {
		snapshot, ok := inputs[path]
		if !ok {
			return nil, fmt.Errorf("migration input %s is missing", path)
		}
		switch snapshot {
		case plan.MissingSHA256:
			changes = append(changes, plan.NewDirectory(path, 0o700).WithSequence((index+1)*10))
		case plan.DirectorySHA256:
		default:
			return nil, fmt.Errorf("migration input %s is not a directory", path)
		}
	}
	target := context.Manifest
	target.StateVersion = step.To()
	encoded, err := core.EncodeManifest(target)
	if err != nil {
		return nil, err
	}
	changes = append(changes, plan.NewUpdateFile(".uawp/manifest.json", 0o600, manifestHash, encoded).WithSequence(100))
	return changes, nil
}

func (step V1_0ToV1_1) Verify(context Context) error {
	if context.Manifest.Protocol != core.ProtocolName || context.Manifest.StateVersion != step.To() {
		return fmt.Errorf("migration target state is not %s", step.To())
	}
	return nil
}
