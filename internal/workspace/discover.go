package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
)

const maxManifestBytes = 1 << 20

type NamespaceKind string

const (
	NamespaceAbsent  NamespaceKind = "ABSENT"
	NamespaceOwned   NamespaceKind = "OWNED"
	NamespaceUnknown NamespaceKind = "UNKNOWN"
	NamespaceInvalid NamespaceKind = "INVALID"
)

type Inventory struct {
	Namespace          NamespaceKind
	Manifest           *core.Manifest
	ManifestError      string
	RootContextPresent bool
	NativeFiles        []string
}

func ProjectInputs(root Root) ([]plan.Input, error) {
	var inputs []plan.Input
	for _, name := range []string{"context.md", "AGENTS.md", "CLAUDE.md"} {
		path := filepath.Join(root.Path(), name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			inputs = append(inputs, plan.Input{Path: name, SHA256: plan.MissingSHA256})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect project input %s: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("project input %s is not a regular file", name)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, plan.Input{Path: name, SHA256: plan.HashBytes(content)})
	}
	return inputs, nil
}

func Discover(root Root) (Inventory, error) {
	inventory := Inventory{}
	if _, err := os.Lstat(filepath.Join(root.Path(), "context.md")); err == nil {
		inventory.RootContextPresent = true
	} else if !os.IsNotExist(err) {
		return Inventory{}, fmt.Errorf("inspect project context.md: %w", err)
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Lstat(filepath.Join(root.Path(), name)); err == nil {
			inventory.NativeFiles = append(inventory.NativeFiles, name)
		} else if !os.IsNotExist(err) {
			return Inventory{}, fmt.Errorf("inspect %s: %w", name, err)
		}
	}

	namespace := filepath.Join(root.Path(), ".uawp")
	info, err := os.Lstat(namespace)
	if os.IsNotExist(err) {
		inventory.Namespace = NamespaceAbsent
		return inventory, nil
	}
	if err != nil {
		return Inventory{}, fmt.Errorf("inspect .uawp namespace: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		inventory.Namespace = NamespaceUnknown
		return inventory, nil
	}

	manifestPath := filepath.Join(namespace, "manifest.json")
	manifestInfo, err := os.Lstat(manifestPath)
	if err == nil && (!manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0) {
		inventory.Namespace = NamespaceInvalid
		inventory.ManifestError = "manifest.json is not a regular file"
		return inventory, nil
	}
	manifestFile, err := os.Open(manifestPath)
	if os.IsNotExist(err) {
		inventory.Namespace = NamespaceUnknown
		return inventory, nil
	}
	if err != nil {
		return Inventory{}, fmt.Errorf("open namespace manifest: %w", err)
	}
	defer manifestFile.Close()

	limited := io.LimitReader(manifestFile, maxManifestBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return Inventory{}, fmt.Errorf("read namespace manifest: %w", err)
	}
	if len(content) > maxManifestBytes {
		inventory.Namespace = NamespaceInvalid
		inventory.ManifestError = "manifest exceeds 1 MiB"
		return inventory, nil
	}
	manifest, err := core.DecodeManifest(bytesReader(content))
	if err != nil {
		inventory.Namespace = NamespaceInvalid
		inventory.ManifestError = err.Error()
		return inventory, nil
	}
	inventory.Namespace = NamespaceOwned
	inventory.Manifest = &manifest
	return inventory, nil
}

func bytesReader(content []byte) io.Reader {
	return &readOnlyBytes{content: content}
}

type readOnlyBytes struct {
	content []byte
	offset  int
}

func (r *readOnlyBytes) Read(target []byte) (int, error) {
	if r.offset >= len(r.content) {
		return 0, io.EOF
	}
	n := copy(target, r.content[r.offset:])
	r.offset += n
	return n, nil
}
