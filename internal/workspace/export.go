package workspace

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/plan"
)

type ExportEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}
type NamespaceSnapshot struct {
	Entries     []ExportEntry
	content     map[string][]byte
	fingerprint string
}
type ExportManifest struct {
	SchemaVersion string        `json:"schemaVersion"`
	Protocol      string        `json:"protocol"`
	StateVersion  string        `json:"stateVersion"`
	Entries       []ExportEntry `json:"entries"`
	ArchiveSHA256 string        `json:"archiveSHA256,omitempty"`
}

func SnapshotNamespace(root Root) (NamespaceSnapshot, error) {
	base := filepath.Join(root.Path(), ".uawp")
	info, err := os.Lstat(base)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return NamespaceSnapshot{}, fmt.Errorf("unsafe UAWP namespace")
	}
	snapshot := NamespaceSnapshot{content: map[string][]byte{}}
	err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == base {
			return nil
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("unsafe export entry %s", relative)
		}
		value := ExportEntry{Path: relative, Mode: uint32(info.Mode().Perm())}
		if info.IsDir() {
			value.Kind = "directory"
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Kind = "file"
			value.Size = int64(len(content))
			value.SHA256 = plan.HashBytes(content)
			snapshot.content[relative] = content
		}
		snapshot.Entries = append(snapshot.Entries, value)
		return nil
	})
	if err != nil {
		return NamespaceSnapshot{}, err
	}
	sort.Slice(snapshot.Entries, func(i, j int) bool { return snapshot.Entries[i].Path < snapshot.Entries[j].Path })
	encoded, _ := json.Marshal(snapshot.Entries)
	snapshot.fingerprint = plan.HashBytes(encoded)
	return snapshot, nil
}

func CreateVerifiedExport(root Root, destination string, snapshot NamespaceSnapshot) (ExportManifest, error) {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return ExportManifest{}, err
	}
	if !strings.HasSuffix(strings.ToLower(absolute), ".tar.gz") {
		return ExportManifest{}, fmt.Errorf("export destination must end in .tar.gz")
	}
	inside, _ := filepath.Rel(filepath.Join(root.Path(), ".uawp"), absolute)
	if inside == "." || (!strings.HasPrefix(inside, ".."+string(filepath.Separator)) && inside != "..") {
		return ExportManifest{}, fmt.Errorf("export destination must be outside .uawp")
	}
	if _, err := os.Lstat(absolute); err == nil || !os.IsNotExist(err) {
		return ExportManifest{}, fmt.Errorf("export destination already exists")
	}
	current, err := SnapshotNamespace(root)
	if err != nil {
		return ExportManifest{}, err
	}
	if current.fingerprint != snapshot.fingerprint {
		return ExportManifest{}, fmt.Errorf("namespace changed after export preview")
	}
	manifest := ExportManifest{SchemaVersion: "1", Protocol: "UAWP", Entries: append([]ExportEntry(nil), snapshot.Entries...)}
	if inventory, discoverErr := Discover(root); discoverErr == nil && inventory.Manifest != nil {
		manifest.StateVersion = inventory.Manifest.StateVersion
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ExportManifest{}, err
	}
	manifestBytes = append(manifestBytes, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(absolute), ".uawp-export-*.tmp")
	if err != nil {
		return ExportManifest{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	gz := gzip.NewWriter(temporary)
	gz.Header.ModTime = time.Unix(0, 0)
	gz.Header.OS = 255
	tw := tar.NewWriter(gz)
	writeEntry := func(name string, mode int64, content []byte, directory bool) error {
		header := &tar.Header{Name: name, Mode: mode, ModTime: time.Unix(0, 0), AccessTime: time.Unix(0, 0), ChangeTime: time.Unix(0, 0), Uid: 0, Gid: 0}
		if directory {
			header.Typeflag = tar.TypeDir
			if !strings.HasSuffix(header.Name, "/") {
				header.Name += "/"
			}
		} else {
			header.Typeflag = tar.TypeReg
			header.Size = int64(len(content))
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !directory {
			_, err := tw.Write(content)
			return err
		}
		return nil
	}
	if err := writeEntry("export-manifest.json", 0o600, manifestBytes, false); err != nil {
		temporary.Close()
		return ExportManifest{}, err
	}
	for _, entry := range snapshot.Entries {
		if err := writeEntry(".uawp/"+entry.Path, int64(entry.Mode), snapshot.content[entry.Path], entry.Kind == "directory"); err != nil {
			temporary.Close()
			return ExportManifest{}, err
		}
	}
	if err := tw.Close(); err != nil {
		temporary.Close()
		return ExportManifest{}, err
	}
	if err := gz.Close(); err != nil {
		temporary.Close()
		return ExportManifest{}, err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return ExportManifest{}, err
	}
	if err := temporary.Close(); err != nil {
		return ExportManifest{}, err
	}
	if err := verifyExportArchive(temporaryPath, manifest); err != nil {
		return ExportManifest{}, err
	}
	archive, err := os.ReadFile(temporaryPath)
	if err != nil {
		return ExportManifest{}, err
	}
	manifest.ArchiveSHA256 = plan.HashBytes(archive)
	if err := os.Link(temporaryPath, absolute); err != nil {
		return ExportManifest{}, fmt.Errorf("publish export without overwrite: %w", err)
	}
	if err := syncDir(filepath.Dir(absolute)); err != nil {
		return ExportManifest{}, err
	}
	return manifest, nil
}

func verifyExportArchive(path string, manifest ExportManifest) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.IsAbs(header.Name) || strings.Contains(header.Name, "..") || seen[header.Name] {
			return fmt.Errorf("unsafe or duplicate archive entry %s", header.Name)
		}
		seen[header.Name] = true
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			if header.Name != "export-manifest.json" {
				relative := strings.TrimPrefix(header.Name, ".uawp/")
				for _, entry := range manifest.Entries {
					if entry.Path == relative && entry.SHA256 != plan.HashBytes(content) {
						return fmt.Errorf("archive hash mismatch for %s", relative)
					}
				}
			}
		}
	}
	if !seen["export-manifest.json"] {
		return fmt.Errorf("export manifest is missing")
	}
	return nil
}
