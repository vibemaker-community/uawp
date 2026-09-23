package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vibemaker-community/uawp/internal/adapter"
	"github.com/vibemaker-community/uawp/internal/core"
	"github.com/vibemaker-community/uawp/internal/plan"
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
		effectiveFacts := withPersistedFacts(facts, manifest, id)
		snapshot, _, err := adapterSnapshot(root, id, effectiveFacts)
		if err != nil {
			return nil, err
		}
		injectRegisteredPath(&snapshot, manifest, id)
		resolution := a.Resolve(snapshot)
		out = append(out, diagnoseRegisteredAdapter(snapshot, resolution, manifest, id))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}

// ConfiguredAdapterIDs returns the unique, persisted consumers of native
// integration artifacts. It reports configuration, not live provider use.
func ConfiguredAdapterIDs(root Root) ([]string, error) {
	manifest, _, err := readAdapterManifest(root)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, artifact := range manifest.Integrations {
		for _, id := range artifact.Consumers {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func PlanAdapterAddAt(root Root, id string, facts adapter.RuntimeFacts, acknowledged []string, at time.Time) (plan.Plan, adapter.Resolution, error) {
	a, err := adapter.Lookup(id)
	if err != nil {
		return plan.Plan{}, adapter.Resolution{}, err
	}
	return planAdapterAddAt(root, id, a, facts, acknowledged, at)
}

func planAdapterAddAt(root Root, id string, a adapter.Adapter, facts adapter.RuntimeFacts, acknowledged []string, at time.Time) (plan.Plan, adapter.Resolution, error) {
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
	var changes []plan.Change
	registered := consumerArtifactIndexes(manifest.Integrations, id)
	if len(registered) > 1 {
		return plan.Plan{}, resolution, fmt.Errorf("adapter %s is registered by multiple artifacts", id)
	}
	if len(registered) == 1 && manifest.Integrations[registered[0]].Path == resolution.Route.Path {
		if err := verifyRegisteredArtifact(snapshot, resolution, manifest.Integrations[registered[0]], id); err != nil {
			return plan.Plan{}, resolution, err
		}
		return plan.NewForWorkspace("adapter-add", root.Path(), nil), resolution, nil
	}
	if len(registered) == 1 {
		change, oldInputs, detachErr := detachConsumer(root, &manifest, registered[0], id)
		if detachErr != nil {
			return plan.Plan{}, resolution, detachErr
		}
		inputs = append(inputs, oldInputs...)
		if change != nil {
			changes = append(changes, *change)
		}
	}
	idx := artifactIndex(manifest.Integrations, resolution.Route.Path)
	var existing *core.IntegrationArtifact
	if idx >= 0 {
		existing = &manifest.Integrations[idx]
	}
	fact := snapshot.Files[resolution.Route.Path]
	before := fact.Content
	missing := fact.State == adapter.FileMissing
	mode := fact.Mode
	if resolution.Route.Mode == core.Direct {
		if resolution.Route.Path != resolution.Route.Target {
			return plan.Plan{}, resolution, fmt.Errorf("DIRECT route path and target must match")
		}
		before, err = os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(resolution.Route.Target)))
		if err != nil {
			return plan.Plan{}, resolution, fmt.Errorf("read DIRECT target: %w", err)
		}
		missing = false
		inputs = append(inputs, plan.Input{Path: resolution.Route.Target, SHA256: plan.HashBytes(before)})
	}
	if mode == 0 {
		mode = 0o600
	}
	consumers := []string{id}
	created := resolution.Route.Create
	outsideHash := plan.HashBytes(before)
	inserted := resolution.Route.Mode == core.ManagedBlock
	var insertedImports []string
	if existing != nil {
		insertedImports = append(insertedImports, existing.InsertedImports...)
	}
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
		if resolution.Route.Create && hasFinding(resolution.Findings, "CLAUDE_CREATION_CHANGES_SELECTION") {
			var preservationInserted bool
			after, preservationInserted, err = adapter.UpsertImport(after, "AGENTS.md")
			if err != nil {
				return plan.Plan{}, resolution, err
			}
			if preservationInserted {
				insertedImports = appendUniqueString(insertedImports, "AGENTS.md")
			}
		}
		after, inserted, err = adapter.UpsertImport(after, resolution.Route.Target)
		if err != nil {
			return plan.Plan{}, resolution, err
		}
		if inserted {
			insertedImports = appendUniqueString(insertedImports, resolution.Route.Target)
		}
	case core.Direct:
		created = false
		inserted = false
	default:
		return plan.Plan{}, resolution, fmt.Errorf("unsupported integration mode %q", resolution.Route.Mode)
	}
	if existing != nil && existing.Inserted {
		inserted = true
	}
	consumerFacts := map[string]map[string]string{}
	if existing != nil {
		for consumer, values := range existing.ConsumerFacts {
			consumerFacts[consumer] = cloneStringMap(values)
		}
	}
	if values := flattenFacts(facts, id); len(values) > 0 {
		consumerFacts[id] = values
	}
	artifact := core.IntegrationArtifact{ID: adapter.ArtifactID(resolution.Route.Path), Path: resolution.Route.Path, Mode: resolution.Route.Mode, Target: resolution.Route.Target, Consumers: consumers, ConsumerFacts: consumerFacts, CreatedFile: created, Inserted: inserted, InsertedImports: insertedImports, ArtifactSHA256: plan.HashBytes(after), OutsideContentSHA256: outsideHash}
	if idx >= 0 {
		manifest.Integrations[idx] = artifact
	} else {
		manifest.Integrations = append(manifest.Integrations, artifact)
	}
	encoded, err := core.EncodeManifest(manifest)
	if err != nil {
		return plan.Plan{}, resolution, err
	}
	if resolution.Route.Mode != core.Direct && string(after) != string(before) {
		changes = append(changes, nativeChange(resolution.Route.Path, missing, before, after, mode))
	}
	for i := range changes {
		changes[i] = changes[i].WithSequence(i + 1)
	}
	changes = append(changes, plan.NewUpdateFile(".uawp/manifest.json", 0o600, plan.HashBytes(manifestBytes), encoded).WithSequence(len(changes)+1))
	return plan.NewForWorkspaceInputsMetadata("adapter-add", root.Path(), changes, inputs, plan.Metadata{Reason: "adapter=" + id + " facts=" + factsFingerprint(facts) + " generated=" + at.Format(time.RFC3339Nano)}), resolution, nil
}

func PlanAdapterRemoveAt(root Root, id string, facts adapter.RuntimeFacts, at time.Time) (plan.Plan, error) {
	manifest, manifestBytes, err := readAdapterManifest(root)
	if err != nil {
		return plan.Plan{}, err
	}
	var changes []plan.Change
	var inputs []plan.Input
	registered := consumerArtifactIndexes(manifest.Integrations, id)
	if len(registered) == 0 {
		return plan.NewForWorkspace("adapter-remove", root.Path(), nil), nil
	}
	if len(registered) > 1 {
		return plan.Plan{}, fmt.Errorf("adapter %s is registered by multiple artifacts", id)
	}
	change, entryInputs, err := detachConsumer(root, &manifest, registered[0], id)
	if err != nil {
		return plan.Plan{}, err
	}
	inputs = append(inputs, entryInputs...)
	if change != nil {
		changes = append(changes, *change)
	}
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
			snapshot, _, snapErr := adapterSnapshot(root, id, withPersistedFacts(facts, manifest, id))
			if snapErr != nil {
				return snapErr
			}
			return verifyRegisteredArtifact(snapshot, res[0], art, id)
		}
	}
	return fmt.Errorf("adapter %s is not registered through its effective entry", id)
}

func VerifyAdapterRemoved(root Root, id string) error {
	ids, err := ConfiguredAdapterIDs(root)
	if err != nil {
		return err
	}
	if contains(ids, id) {
		return fmt.Errorf("adapter %s remains registered", id)
	}
	return nil
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
func consumerArtifactIndexes(items []core.IntegrationArtifact, consumer string) []int {
	var indexes []int
	for i := range items {
		if contains(items[i].Consumers, consumer) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func detachConsumer(root Root, manifest *core.Manifest, index int, id string) (*plan.Change, []plan.Input, error) {
	art := manifest.Integrations[index]
	remaining := removeConsumer(art.Consumers, id)
	if art.Mode == core.Direct {
		if len(remaining) == 0 {
			manifest.Integrations = append(manifest.Integrations[:index], manifest.Integrations[index+1:]...)
		} else {
			art.Consumers = remaining
			delete(art.ConsumerFacts, id)
			manifest.Integrations[index] = art
		}
		return nil, nil, nil
	}
	snapshot, err := adapter.DiscoverSnapshot(root.Path(), []string{art.Path}, "", nil)
	if err != nil {
		return nil, nil, err
	}
	inputs, err := entryInputsFromSnapshot(snapshot)
	if err != nil {
		return nil, nil, err
	}
	fact := snapshot.Files[art.Path]
	before, after := fact.Content, fact.Content
	if len(remaining) == 0 && art.CreatedFile && plan.HashBytes(before) == art.ArtifactSHA256 {
		after = nil
	} else if art.Mode == core.ManagedBlock {
		after, err = adapter.RemoveManagedBlock(before, adapter.BlockSpec{ArtifactID: art.ID, Target: art.Target, Consumers: art.Consumers, Body: adapter.BridgeBody()})
	} else if art.Mode == core.Import && len(remaining) == 0 {
		ownedImports := ownedImportTargets(art)
		for _, target := range ownedImports {
			after, _, err = adapter.RemoveImport(after, target)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		return nil, nil, err
	}
	if len(remaining) > 0 {
		if art.Mode == core.ManagedBlock {
			var meta adapter.BlockMeta
			after, meta, err = adapter.UpsertManagedBlock(after, adapter.BlockSpec{ArtifactID: art.ID, Target: art.Target, Consumers: remaining, Body: adapter.BridgeBody()})
			if err != nil {
				return nil, nil, err
			}
			art.OutsideContentSHA256 = meta.OutsideSHA256
		}
		art.Consumers = remaining
		delete(art.ConsumerFacts, id)
		art.ArtifactSHA256 = plan.HashBytes(after)
		manifest.Integrations[index] = art
	} else {
		manifest.Integrations = append(manifest.Integrations[:index], manifest.Integrations[index+1:]...)
	}
	if len(after) == 0 && art.CreatedFile {
		change := plan.NewDeleteFile(art.Path, plan.HashBytes(before))
		return &change, inputs, nil
	}
	if string(after) != string(before) {
		mode := fact.Mode
		if mode == 0 {
			mode = 0o600
		}
		change := plan.NewUpdateFile(art.Path, mode, plan.HashBytes(before), after)
		return &change, inputs, nil
	}
	return nil, inputs, nil
}

func ownedImportTargets(art core.IntegrationArtifact) []string {
	if len(art.InsertedImports) > 0 {
		return append([]string(nil), art.InsertedImports...)
	}
	if !art.Inserted {
		return nil
	}
	targets := []string{art.Target}
	legacyConditional := []byte("@AGENTS.md\n@" + art.Target)
	if art.CreatedFile && art.Path == "CLAUDE.md" && plan.HashBytes(legacyConditional) == art.ArtifactSHA256 {
		targets = append([]string{"AGENTS.md"}, targets...)
	}
	return targets
}
func contains(items []string, want string) bool {
	for _, v := range items {
		if v == want {
			return true
		}
	}
	return false
}
func appendUniqueString(items []string, value string) []string {
	if contains(items, value) {
		return items
	}
	return append(items, value)
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
func hasFinding(findings []adapter.Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
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

func flattenFacts(facts adapter.RuntimeFacts, id string) map[string]string {
	values := map[string]string{}
	if version := facts.Versions[id]; version != "" {
		values["providerVersion"] = version
	}
	for key, value := range facts.Options[id] {
		if value != "" {
			values[key] = value
		}
	}
	return values
}

func withPersistedFacts(facts adapter.RuntimeFacts, manifest core.Manifest, id string) adapter.RuntimeFacts {
	merged := adapter.RuntimeFacts{Versions: map[string]string{}, Options: map[string]map[string]string{id: {}}}
	for _, art := range manifest.Integrations {
		if !contains(art.Consumers, id) {
			continue
		}
		for key, value := range art.ConsumerFacts[id] {
			if key == "providerVersion" {
				merged.Versions[id] = value
			} else {
				merged.Options[id][key] = value
			}
		}
	}
	if value := facts.Versions[id]; value != "" {
		merged.Versions[id] = value
	}
	for key, value := range facts.Options[id] {
		if value != "" {
			merged.Options[id][key] = value
		}
	}
	return merged
}

func cloneStringMap(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func verifyRegisteredArtifact(snapshot adapter.Snapshot, resolution adapter.Resolution, art core.IntegrationArtifact, id string) error {
	if resolution.Route.Path != art.Path || resolution.Route.Mode != art.Mode || resolution.Route.Target != art.Target {
		return fmt.Errorf("adapter %s effective entry drifted from registered artifact", id)
	}
	switch art.Mode {
	case core.Direct:
		content, err := os.ReadFile(filepath.Join(snapshot.Root, filepath.FromSlash(art.Target)))
		if err != nil || plan.HashBytes(content) != art.ArtifactSHA256 {
			return fmt.Errorf("adapter %s DIRECT target drifted", id)
		}
	case core.ManagedBlock:
		fact, ok := snapshot.Files[art.Path]
		if !ok || fact.State != adapter.FileRegular {
			return fmt.Errorf("adapter %s registered entry is unavailable", id)
		}
		if _, err := adapter.RemoveManagedBlock(fact.Content, adapter.BlockSpec{ArtifactID: art.ID, Target: art.Target, Consumers: art.Consumers, Body: adapter.BridgeBody()}); err != nil {
			return fmt.Errorf("adapter %s managed bridge is invalid: %w", id, err)
		}
	case core.Import:
		fact, ok := snapshot.Files[art.Path]
		if !ok || fact.State != adapter.FileRegular {
			return fmt.Errorf("adapter %s registered entry is unavailable", id)
		}
		_, removed, err := adapter.RemoveImport(fact.Content, art.Target)
		if err != nil || !removed {
			return fmt.Errorf("adapter %s import bridge is invalid", id)
		}
	default:
		return fmt.Errorf("adapter %s has unsupported registered mode %q", id, art.Mode)
	}
	return nil
}

func diagnoseRegisteredAdapter(snapshot adapter.Snapshot, resolution adapter.Resolution, manifest core.Manifest, id string) adapter.Resolution {
	registered := consumerArtifactIndexes(manifest.Integrations, id)
	if len(registered) == 0 {
		return resolution
	}
	if len(registered) > 1 {
		resolution.Health = "INTEGRATION_DRIFT"
		resolution.Findings = append(resolution.Findings, adapter.Finding{Code: "DUPLICATE_REGISTRATION", Severity: "error", Message: "The adapter is registered through multiple integration artifacts.", NextAction: "Repair the manifest before adapter mutation."})
		return resolution.Canonical()
	}
	art := manifest.Integrations[registered[0]]
	if resolution.Route.Path != art.Path || resolution.Route.Mode != art.Mode || resolution.Route.Target != art.Target {
		resolution.Health = "ENTRY_DRIFT"
		if !hasFinding(resolution.Findings, "ENTRY_DRIFT") {
			resolution.Findings = append(resolution.Findings, adapter.Finding{Code: "ENTRY_DRIFT", Severity: "error", Message: "The effective provider entry differs from the registered UAWP route.", NextAction: "Preview a reviewed adapter migration to the effective entry."})
		}
		return resolution.Canonical()
	}
	if err := verifyRegisteredArtifact(snapshot, resolution, art, id); err != nil {
		resolution.Health = "INTEGRATION_DRIFT"
		resolution.Findings = append(resolution.Findings, adapter.Finding{Code: "INTEGRATION_DRIFT", Severity: "error", Message: err.Error(), NextAction: "Repair the registered bridge before adapter mutation."})
	}
	return resolution.Canonical()
}
