package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/vibemaker-community/uawp/internal/buildinfo"
)

type releaseMetadata struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	GOOS    string `json:"goos"`
	GOARCH  string `json:"goarch"`
	Name    string `json:"-"`
}

type binaryRunner func(releaseMetadata, []byte) error

func main() {
	dist := flag.String("dist", "dist", "directory containing release assets")
	version := flag.String("version", "", "release version without leading v")
	commit := flag.String("commit", "", "full release commit")
	metadataDir := flag.String("generate-metadata", "", "write per-target release metadata to this directory")
	flag.Parse()
	if flag.NArg() != 0 || *version == "" || *commit == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts [--dist DIR | --generate-metadata DIR] --version VERSION --commit COMMIT")
		os.Exit(2)
	}
	if *metadataDir != "" {
		if err := writeReleaseMetadata(*metadataDir, strings.TrimPrefix(*version, "v"), *commit); err != nil {
			fmt.Fprintf(os.Stderr, "write release metadata: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if err := verifyRelease(*dist, strings.TrimPrefix(*version, "v"), *commit, executeNativeBinary); err != nil {
		fmt.Fprintf(os.Stderr, "release verification failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("verified UAWP %s release assets in %s\n", strings.TrimPrefix(*version, "v"), *dist)
}

func writeReleaseMetadata(dir, version, commit string) error {
	if version == "" || commit == "" {
		return errors.New("version and commit are required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, target := range releaseTargets(version) {
		target.Commit = commit
		data, err := json.Marshal(target)
		if err != nil {
			return err
		}
		filename := filepath.Join(dir, target.GOOS+"_"+target.GOARCH+".json")
		if err := os.WriteFile(filename, append(data, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func checksumName(version string) string { return "uawp_" + version + "_checksums.txt" }

func releaseTargets(version string) []releaseMetadata {
	targets := []releaseMetadata{
		{Version: version, GOOS: "darwin", GOARCH: "arm64"},
		{Version: version, GOOS: "darwin", GOARCH: "amd64"},
		{Version: version, GOOS: "linux", GOARCH: "amd64"},
		{Version: version, GOOS: "linux", GOARCH: "arm64"},
		{Version: version, GOOS: "windows", GOARCH: "amd64"},
	}
	for index := range targets {
		extension := ".tar.gz"
		if targets[index].GOOS == "windows" {
			extension = ".zip"
		}
		targets[index].Name = fmt.Sprintf("uawp_%s_%s_%s%s", version, targets[index].GOOS, targets[index].GOARCH, extension)
	}
	return targets
}

func verifyRelease(dist, version, commit string, runner binaryRunner) error {
	if version == "" || commit == "" {
		return errors.New("version and commit are required")
	}
	targets := releaseTargets(version)
	expected := map[string]bool{checksumName(version): true}
	for _, target := range targets {
		expected[target.Name] = true
	}
	entries, err := os.ReadDir(dist)
	if err != nil {
		return err
	}
	if len(entries) != len(expected) {
		return fmt.Errorf("asset count %d, want %d", len(entries), len(expected))
	}
	for _, entry := range entries {
		if entry.IsDir() || !expected[entry.Name()] {
			return fmt.Errorf("unexpected release asset %q", entry.Name())
		}
	}

	checksums, err := parseChecksums(filepath.Join(dist, checksumName(version)))
	if err != nil {
		return err
	}
	if len(checksums) != len(targets) {
		return fmt.Errorf("checksum entry count %d, want %d", len(checksums), len(targets))
	}
	for _, target := range targets {
		target.Commit = commit
		wantDigest, ok := checksums[target.Name]
		if !ok {
			return fmt.Errorf("missing checksum for %s", target.Name)
		}
		archive, err := os.ReadFile(filepath.Join(dist, target.Name))
		if err != nil {
			return err
		}
		gotDigest := sha256.Sum256(archive)
		if !bytes.Equal(gotDigest[:], wantDigest) {
			return fmt.Errorf("checksum mismatch for %s", target.Name)
		}
		binary, metadata, err := inspectArchive(target, archive)
		if err != nil {
			return fmt.Errorf("%s: %w", target.Name, err)
		}
		if metadata.Version != version || metadata.Commit != commit || metadata.GOOS != target.GOOS || metadata.GOARCH != target.GOARCH {
			return fmt.Errorf("metadata mismatch: got %+v", metadata)
		}
		if target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH {
			if err := runner(target, binary); err != nil {
				return fmt.Errorf("native binary: %w", err)
			}
		}
	}
	return nil
}

func parseChecksums(filename string) (map[string][]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte)
	for lineNumber, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		parts := strings.Split(line, "  ")
		if len(parts) != 2 || len(parts[0]) != 64 || parts[1] == "" || filepath.Base(parts[1]) != parts[1] {
			return nil, fmt.Errorf("invalid checksum line %d", lineNumber+1)
		}
		if _, exists := result[parts[1]]; exists {
			return nil, fmt.Errorf("duplicate checksum for %s", parts[1])
		}
		digest, err := hex.DecodeString(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid checksum line %d: %w", lineNumber+1, err)
		}
		result[parts[1]] = digest
	}
	return result, nil
}

func inspectArchive(target releaseMetadata, data []byte) ([]byte, releaseMetadata, error) {
	if target.GOOS == "windows" {
		return inspectZip(data, "uawp.exe")
	}
	return inspectTar(data, "uawp")
}

func inspectTar(data []byte, binaryName string) ([]byte, releaseMetadata, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, releaseMetadata{}, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := make(map[string][]byte)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, releaseMetadata{}, err
		}
		if !safeArchiveName(header.Name) || header.Typeflag != tar.TypeReg {
			return nil, releaseMetadata{}, fmt.Errorf("unsafe archive entry %q type %d", header.Name, header.Typeflag)
		}
		if _, duplicate := files[header.Name]; duplicate {
			return nil, releaseMetadata{}, fmt.Errorf("duplicate archive entry %q", header.Name)
		}
		body, err := io.ReadAll(io.LimitReader(tr, 128<<20))
		if err != nil {
			return nil, releaseMetadata{}, err
		}
		if int64(len(body)) != header.Size {
			return nil, releaseMetadata{}, fmt.Errorf("truncated archive entry %q", header.Name)
		}
		files[header.Name] = body
	}
	return validateArchiveFiles(files, binaryName)
}

func inspectZip(data []byte, binaryName string) ([]byte, releaseMetadata, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, releaseMetadata{}, err
	}
	files := make(map[string][]byte)
	for _, file := range zr.File {
		if !safeArchiveName(file.Name) || !file.Mode().IsRegular() || file.UncompressedSize64 > 128<<20 {
			return nil, releaseMetadata{}, fmt.Errorf("unsafe archive entry %q", file.Name)
		}
		if _, duplicate := files[file.Name]; duplicate {
			return nil, releaseMetadata{}, fmt.Errorf("duplicate archive entry %q", file.Name)
		}
		reader, err := file.Open()
		if err != nil {
			return nil, releaseMetadata{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, 128<<20))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, releaseMetadata{}, readErr
		}
		if closeErr != nil {
			return nil, releaseMetadata{}, closeErr
		}
		if uint64(len(body)) != file.UncompressedSize64 {
			return nil, releaseMetadata{}, fmt.Errorf("truncated archive entry %q", file.Name)
		}
		files[file.Name] = body
	}
	return validateArchiveFiles(files, binaryName)
}

func safeArchiveName(name string) bool {
	return name != "" && name == path.Base(name) && path.Clean(name) == name && !path.IsAbs(name) && !strings.Contains(name, "\\") && !strings.Contains(name, ":")
}

func validateArchiveFiles(files map[string][]byte, binaryName string) ([]byte, releaseMetadata, error) {
	required := []string{binaryName, "LICENSE", "NOTICE", "README.md", "THIRD_PARTY_NOTICES.md", "TRADEMARKS.md", "release-metadata.json"}
	sort.Strings(required)
	actual := make([]string, 0, len(files))
	for name := range files {
		actual = append(actual, name)
	}
	sort.Strings(actual)
	if strings.Join(actual, "\x00") != strings.Join(required, "\x00") {
		return nil, releaseMetadata{}, fmt.Errorf("archive entries %v, want %v", actual, required)
	}
	var metadata releaseMetadata
	decoder := json.NewDecoder(bytes.NewReader(files["release-metadata.json"]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return nil, releaseMetadata{}, fmt.Errorf("release metadata: %w", err)
	}
	return files[binaryName], metadata, nil
}

func executeNativeBinary(target releaseMetadata, binary []byte) error {
	dir, err := os.MkdirTemp("", "uawp-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	filename := filepath.Join(dir, nativeExecutableName(target))
	if err := os.WriteFile(filename, binary, 0o700); err != nil {
		return err
	}
	output, err := exec.Command(filename, "version", "--format", "json").Output()
	if err != nil {
		return err
	}
	var info buildinfo.Info
	if err := json.Unmarshal(output, &info); err != nil {
		return err
	}
	if info.Version != target.Version || info.Commit != target.Commit {
		return fmt.Errorf("version output %+v does not match release", info)
	}
	return nil
}

func nativeExecutableName(target releaseMetadata) string {
	if target.GOOS == "windows" {
		return "uawp.exe"
	}
	return "uawp"
}
