package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

func ResolveAdapters(root Root, ids []string, facts adapter.RuntimeFacts) ([]adapter.Resolution, error) {
	manifest, _, err := readAdapterManifest(root)
	if err != nil {
		return nil, err
	}
	out := make([]adapter.Resolution, 0, len(ids))
	for _, id := range ids {
		a, err := adapter.Lookup(id)
		if err != nil {
			return nil, err
		}
		snapshot, _, err := adapterSnapshot(root, id, facts)
		if err != nil {
			return nil, err
		}
		injectRegisteredPath(&snapshot, manifest, id)
		out = append(out, a.Resolve(snapshot))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}

func PlanAdapterAddAt(root Root, id string, facts adapter.RuntimeFacts, acknowledged []string, at time.Time) (plan.Plan, adapter.Resolution, error) {
	a, err := adapter.Lookup(id)
	if err != nil {
		return plan.Plan{}, adapter.Resolution{}, err
	}
	snapshot, inputs, err := adapterSnapshot(root, id, facts)
	if err != nil {
		return plan.Plan{}, adapter.Resolution{}, err
	}
	manifest, manifestBytes, err := readAdapterManifest(root)
	if err != nil {
		return plan.Plan{}, adapter.Resolution{}, err
	}
	injectRegisteredPath(&snapshot, manifest, id)
	resolution := a.Resolve(snapshot)
	if resolution.Confidence == adapter.Unsupported {
		return plan.Plan{}, resolution, fmt.Errorf("adapter %s is unsupported in the observed configuration", id)
	}
	if resolution.Confidence == adapter.Conditional && !acknowledgesAll(resolution.Findings, acknowledged) {
		return plan.Plan{}, resolution, fmt.Errorf("conditional adapter route requires acknowledgement")
	}
	idx := artifactIndex(manifest.Integrations, resolution.Route.Path)
	var existing *core.IntegrationArtifact
	if idx >= 0 {
		existing = &manifest.Integrations[idx]
		if contains(existing.Consumers, id) {
			if err := verifyRegisteredArtifact(snapshot, resolution, *existing, id); err != nil {
				return plan.Plan{}, resolution, err
			}
			return plan.NewForWorkspace("adapter-add", root.Path(), nil), resolution, nil
		}
	}
	fact := snapshot.Files[resolution.Route.Path]
	before := fact.Content
	mode := fact.Mode
	if mode == 0 {
		mode = 0o600
	}
	consumers := []string{id}
	created := resolution.Route.Create
	outsideHash := plan.HashBytes(before)
	if existing != nil {
		if existing.Mode != resolution.Route.Mode || existing.Target != resolution.Route.Target {
			return plan.Plan{}, resolution, fmt.Errorf("existing artifact conflicts with resolved route")
		}
		consumers = append(append([]string(nil), existing.Consumers...), id)
		created = existing.CreatedFile
	}
	sort.Strings(consumers)
	after := before
	switch resolution.Route.Mode {
	case core.ManagedBlock:
		if existing != nil {
			after, err = adapter.RemoveManagedBlock(before, adapter.BlockSpec{ArtifactID: existing.ID, Target: existing.Target, Consumers: existing.Consumers, Body: adapter.BridgeBody()})
			if err != nil {
				return plan.Plan{}, resolution, err
			}
		}
		var meta adapter.BlockMeta
		after, meta, err = adapter.UpsertManagedBlock(after, adapter.BlockSpec{ArtifactID: adapter.ArtifactID(resolution.Route.Path), Target: resolution.Route.Target, Consumers: consumers, Body: adapter.BridgeBody()})
		if err != nil {
			return plan.Plan{}, resolution, err
		}
		outsideHash = meta.OutsideSHA256
	case core.Import:
		var changed bool
		after, changed, err = adapter.UpsertImport(before, resolution.Route.Target)
		_ = changed
		if err != nil {
			return plan.Plan{}, resolution, err
		}
	default:
		return plan.Plan{}, resolution, fmt.Errorf("unsupported integration mode %q", resolution.Route.Mode)
	}
	artifact := core.IntegrationArtifact{ID: adapter.ArtifactID(resolution.Route.Path), Path: resolution.Route.Path, Mode: resolution.Route.Mode, Target: resolution.Route.Target, Consumers: consumers, CreatedFile: created, ArtifactSHA256: plan.HashBytes(after), OutsideContentSHA256: outsideHash}
	if idx >= 0 {
		manifest.Integrations[idx] = artifact
	} else {
		manifest.Integrations = append(manifest.Integrations, artifact)
	}
	encoded, err := core.EncodeManifest(manifest)
	if err != nil {
		return plan.Plan{}, resolution, err
	}
	changes := []plan.Change{nativeChange(resolution.Route.Path, fact.State == adapter.FileMissing, before, after, mode).WithSequence(1), plan.NewUpdateFile(".uawp/manifest.json", 0o600, plan.HashBytes(manifestBytes), encoded).WithSequence(2)}
	return plan.NewForWorkspaceInputsMetadata("adapter-add", root.Path(), changes, inputs, plan.Metadata{Reason: "adapter=" + id + " facts=" + factsFingerprint(facts) + " generated=" + at.Format(time.RFC3339Nano)}), resolution, nil
}

func PlanAdapterRemoveAt(root Root, id string, facts adapter.RuntimeFacts, at time.Time) (plan.Plan, error) {
	manifest, manifestBytes, err := readAdapterManifest(root)
	if err != nil {
		return plan.Plan{}, err
	}
	var changes []plan.Change
	var inputs []plan.Input
	kept := make([]core.IntegrationArtifact, 0, len(manifest.Integrations))
	found := false
	for _, art := range manifest.Integrations {
		if !contains(art.Consumers, id) {
			kept = append(kept, art)
			continue
		}
		found = true
		snapshot, err := adapter.DiscoverSnapshot(root.Path(), []string{art.Path}, "", nil)
		if err != nil {
			return plan.Plan{}, err
		}
		entryInputs, inputErr := entryInputsFromSnapshot(snapshot)
		if inputErr != nil {
			return plan.Plan{}, inputErr
		}
		inputs = append(inputs, entryInputs...)
		fact := snapshot.Files[art.Path]
		before := fact.Content
		remaining := removeConsumer(art.Consumers, id)
		after := before
		if art.Mode == core.ManagedBlock {
			after, err = adapter.RemoveManagedBlock(before, adapter.BlockSpec{ArtifactID: art.ID, Target: art.Target, Consumers: art.Consumers, Body: adapter.BridgeBody()})
		} else if art.Mode == core.Import {
			after, _, err = adapter.RemoveImport(before, art.Target)
		}
		if err != nil {
			return plan.Plan{}, err
		}
		if len(remaining) > 0 {
			if art.Mode == core.ManagedBlock {
				var meta adapter.BlockMeta
				after, meta, err = adapter.UpsertManagedBlock(after, adapter.BlockSpec{ArtifactID: art.ID, Target: art.Target, Consumers: remaining, Body: adapter.BridgeBody()})
				art.OutsideContentSHA256 = meta.OutsideSHA256
				if err != nil {
					return plan.Plan{}, err
				}
			} else if art.Mode == core.Import {
				after, _, err = adapter.UpsertImport(after, art.Target)
				if err != nil {
					return plan.Plan{}, err
				}
			}
			art.Consumers = remaining
			art.ArtifactSHA256 = plan.HashBytes(after)
			kept = append(kept, art)
		}
		if len(after) == 0 && art.CreatedFile {
			changes = append(changes, plan.NewDeleteFile(art.Path, plan.HashBytes(before)).WithSequence(len(changes)+1))
		} else if string(after) != string(before) {
			mode := fact.Mode
			if mode == 0 {
				mode = 0o600
			}
			changes = append(changes, plan.NewUpdateFile(art.Path, mode, plan.HashBytes(before), after).WithSequence(len(changes)+1))
		}
	}
	if !found {
		return plan.NewForWorkspace("adapter-remove", root.Path(), nil), nil
	}
	manifest.Integrations = kept
	encoded, err := core.EncodeManifest(manifest)
	if err != nil {
		return plan.Plan{}, err
	}
	changes = append(changes, plan.NewUpdateFile(".uawp/manifest.json", 0o600, plan.HashBytes(manifestBytes), encoded).WithSequence(len(changes)+1))
	return plan.NewForWorkspaceInputsMetadata("adapter-remove", root.Path(), changes, inputs, plan.Metadata{Reason: "adapter=" + id + " facts=" + factsFingerprint(facts) + " generated=" + at.Format(time.RFC3339Nano)}), nil
}

func VerifyAdapterRoute(root Root, id string, facts adapter.RuntimeFacts) error {
	res, err := ResolveAdapters(root, []string{id}, facts)
	if err != nil {
		return err
	}
	manifest, _, err := readAdapterManifest(root)
	if err != nil {
		return err
	}
	for _, art := range manifest.Integrations {
		if art.Path == res[0].Route.Path && contains(art.Consumers, id) {
			snapshot, _, snapErr := adapterSnapshot(root, id, facts)
			if snapErr != nil {
				return snapErr
			}
			return verifyRegisteredArtifact(snapshot, res[0], art, id)
		}
	}
	return fmt.Errorf("adapter %s is not registered through its effective entry", id)
}

func adapterSnapshot(root Root, id string, facts adapter.RuntimeFacts) (adapter.Snapshot, []plan.Input, error) {
	paths := adapter.CandidatePaths(id, facts)
	version := ""
	options := map[string]string{}
	if facts.Versions != nil {
		version = facts.Versions[id]
	}
	if facts.Options != nil {
		for k, v := range facts.Options[id] {
			options[k] = v
		}
	}
	s, err := adapter.DiscoverSnapshot(root.Path(), paths, version, options)
	if err != nil {
		return adapter.Snapshot{}, nil, err
	}
	inputs, err := entryInputsFromSnapshot(s)
	if err != nil {
		return adapter.Snapshot{}, nil, err
	}
	return s, inputs, nil
}
func entryInputsFromSnapshot(s adapter.Snapshot) ([]plan.Input, error) {
	out := make([]plan.Input, 0, len(s.Files))
	for path, f := range s.Files {
		hash := plan.MissingSHA256
		switch f.State {
		case adapter.FileRegular:
			hash = f.SHA256
		case adapter.FileMissing:
		default:
			return nil, fmt.Errorf("unsafe native entry %s: %s", path, f.State)
		}
		out = append(out, plan.Input{Path: path, SHA256: hash})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
func readAdapterManifest(root Root) (core.Manifest, []byte, error) {
	content, err := os.ReadFile(filepath.Join(root.Path(), ".uawp", "manifest.json"))
	if err != nil {
		return core.Manifest{}, nil, err
	}
	m, err := core.DecodeManifest(bytesReader(content))
	return m, content, err
}
func injectRegisteredPath(s *adapter.Snapshot, m core.Manifest, id string) {
	for _, a := range m.Integrations {
		if contains(a.Consumers, id) {
			s.Options["registeredPath"] = a.Path
			return
		}
	}
}
func artifactIndex(items []core.IntegrationArtifact, path string) int {
	for i := range items {
		if items[i].Path == path {
			return i
		}
	}
	return -1
}
func contains(items []string, want string) bool {
	for _, v := range items {
		if v == want {
			return true
		}
	}
	return false
}
func removeConsumer(items []string, id string) []string {
	out := []string{}
	for _, v := range items {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
func acknowledgesAll(findings []adapter.Finding, ack []string) bool {
	for _, f := range findings {
		if f.Severity == "warning" && !contains(ack, f.Code) {
			return false
		}
	}
	return true
}
func nativeChange(path string, missing bool, before, after []byte, mode uint32) plan.Change {
	if missing {
		return plan.NewFile(path, mode, plan.MissingSHA256, after)
	}
	return plan.NewUpdateFile(path, mode, plan.HashBytes(before), after)
}

func factsFingerprint(facts adapter.RuntimeFacts) string {
	encoded, err := json.Marshal(facts)
	if err != nil {
		panic(fmt.Sprintf("encode runtime facts: %v", err))
	}
	return plan.HashBytes(encoded)
}

func verifyRegisteredArtifact(snapshot adapter.Snapshot, resolution adapter.Resolution, art core.IntegrationArtifact, id string) error {
	if resolution.Route.Path != art.Path || resolution.Route.Mode != art.Mode || resolution.Route.Target != art.Target {
		return fmt.Errorf("adapter %s effective entry drifted from registered artifact", id)
	}
	fact, ok := snapshot.Files[art.Path]
	if !ok || fact.State != adapter.FileRegular {
		return fmt.Errorf("adapter %s registered entry is unavailable", id)
	}
	if plan.HashBytes(fact.Content) != art.ArtifactSHA256 {
		return fmt.Errorf("adapter %s registered entry content drifted", id)
	}
	switch art.Mode {
	case core.ManagedBlock:
		if _, err := adapter.RemoveManagedBlock(fact.Content, adapter.BlockSpec{ArtifactID: art.ID, Target: art.Target, Consumers: art.Consumers, Body: adapter.BridgeBody()}); err != nil {
			return fmt.Errorf("adapter %s managed bridge is invalid: %w", id, err)
		}
	case core.Import:
		_, removed, err := adapter.RemoveImport(fact.Content, art.Target)
		if err != nil || !removed {
			return fmt.Errorf("adapter %s import bridge is invalid", id)
		}
	default:
		return fmt.Errorf("adapter %s has unsupported registered mode %q", id, art.Mode)
	}
	return nil
}
