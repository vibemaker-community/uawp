package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestReleaseBundleAcceptsExactSafeAssets(t *testing.T) {
	dir := validRelease(t, "1.0.0")
	if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err != nil {
		t.Fatal(err)
	}
}

func TestReleaseMetadataFilesDescribeEveryTarget(t *testing.T) {
	dir := t.TempDir()
	if err := writeReleaseMetadata(dir, "1.0.0", testCommit); err != nil {
		t.Fatal(err)
	}
	for _, target := range releaseTargets("1.0.0") {
		data := readFile(t, filepath.Join(dir, target.GOOS+"_"+target.GOARCH+".json"))
		var got releaseMetadata
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Version != "1.0.0" || got.Commit != testCommit || got.GOOS != target.GOOS || got.GOARCH != target.GOARCH {
			t.Fatalf("metadata = %+v", got)
		}
	}
}

func TestReleaseBundleRejectsMissingAndExtraAssets(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{"missing archive", func(t *testing.T, dir string) { removeFile(t, filepath.Join(dir, releaseTargets("1.0.0")[0].Name)) }},
		{"extra archive", func(t *testing.T, dir string) { writeFile(t, filepath.Join(dir, "unexpected.tar.gz"), []byte("x")) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := validRelease(t, "1.0.0")
			test.mutate(t, dir)
			if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err == nil {
				t.Fatal("unsafe asset set accepted")
			}
		})
	}
}

func TestReleaseBundleRejectsInvalidChecksums(t *testing.T) {
	t.Run("duplicate entry", func(t *testing.T) {
		dir := validRelease(t, "1.0.0")
		path := filepath.Join(dir, checksumName("1.0.0"))
		data := readFile(t, path)
		first := strings.SplitN(string(data), "\n", 2)[0]
		writeFile(t, path, append(data, []byte(first+"\n")...))
		if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err == nil {
			t.Fatal("duplicate checksum accepted")
		}
	})

	t.Run("digest mismatch", func(t *testing.T) {
		dir := validRelease(t, "1.0.0")
		path := filepath.Join(dir, releaseTargets("1.0.0")[0].Name)
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("tampered")); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err == nil {
			t.Fatal("checksum mismatch accepted")
		}
	})
}

func TestReleaseBundleRejectsUnsafeTarEntries(t *testing.T) {
	for _, test := range []struct {
		name     string
		entry    string
		typeflag byte
	}{
		{"absolute", "/tmp/uawp", tar.TypeReg},
		{"traversal", "../uawp", tar.TypeReg},
		{"symlink", "link", tar.TypeSymlink},
		{"hardlink", "link", tar.TypeLink},
		{"device", "device", tar.TypeChar},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := validRelease(t, "1.0.0")
			target := releaseTargets("1.0.0")[0]
			writeTar(t, filepath.Join(dir, target.Name), target, archiveEntry{Name: test.entry, Typeflag: test.typeflag})
			writeChecksums(t, dir, "1.0.0")
			if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err == nil {
				t.Fatal("unsafe tar entry accepted")
			}
		})
	}
}

func TestReleaseBundleRejectsWrongBinaryNameAndMetadata(t *testing.T) {
	t.Run("windows binary", func(t *testing.T) {
		dir := validRelease(t, "1.0.0")
		target := releaseTargets("1.0.0")[4]
		writeZip(t, filepath.Join(dir, target.Name), target, "uawp")
		writeChecksums(t, dir, "1.0.0")
		if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err == nil {
			t.Fatal("Windows archive without uawp.exe accepted")
		}
	})

	t.Run("version", func(t *testing.T) {
		dir := validRelease(t, "1.0.0")
		target := releaseTargets("1.0.0")[0]
		target.Version = "9.9.9"
		writeTar(t, filepath.Join(dir, target.Name), target)
		writeChecksums(t, dir, "1.0.0")
		if err := verifyRelease(dir, "1.0.0", testCommit, acceptBinary); err == nil {
			t.Fatal("archive with wrong embedded metadata accepted")
		}
	})
}

func TestNativeExecutableNameUsesWindowsExtension(t *testing.T) {
	if got := nativeExecutableName(releaseMetadata{GOOS: "windows"}); got != "uawp.exe" {
		t.Fatalf("Windows executable name = %q", got)
	}
	if got := nativeExecutableName(releaseMetadata{GOOS: "linux"}); got != "uawp" {
		t.Fatalf("Linux executable name = %q", got)
	}
}

func validRelease(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	for _, target := range releaseTargets(version) {
		target.Commit = testCommit
		path := filepath.Join(dir, target.Name)
		if target.GOOS == "windows" {
			writeZip(t, path, target, "uawp.exe")
		} else {
			writeTar(t, path, target)
		}
	}
	writeChecksums(t, dir, version)
	return dir
}

type archiveEntry struct {
	Name     string
	Typeflag byte
}

func writeTar(t *testing.T, path string, target releaseMetadata, extras ...archiveEntry) {
	t.Helper()
	var compressed bytes.Buffer
	gz, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(gz)
	entries := []archiveEntry{{Name: "uawp", Typeflag: tar.TypeReg}, {Name: "LICENSE", Typeflag: tar.TypeReg}, {Name: "NOTICE", Typeflag: tar.TypeReg}, {Name: "THIRD_PARTY_NOTICES.md", Typeflag: tar.TypeReg}, {Name: "TRADEMARKS.md", Typeflag: tar.TypeReg}, {Name: "README.md", Typeflag: tar.TypeReg}, {Name: "release-metadata.json", Typeflag: tar.TypeReg}}
	entries = append(entries, extras...)
	metadata, _ := json.Marshal(target)
	for _, entry := range entries {
		body := []byte(entry.Name)
		if entry.Name == "release-metadata.json" {
			body = metadata
		}
		header := &tar.Header{Name: entry.Name, Typeflag: entry.Typeflag, Mode: 0o644, Size: int64(len(body))}
		if entry.Name == "uawp" {
			header.Mode = 0o755
		}
		if entry.Typeflag != tar.TypeReg {
			header.Size = 0
			header.Linkname = "uawp"
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if _, err := tw.Write(body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, compressed.Bytes())
}

func writeZip(t *testing.T, path string, target releaseMetadata, binary string) {
	t.Helper()
	var data bytes.Buffer
	zw := zip.NewWriter(&data)
	metadata, _ := json.Marshal(target)
	for _, entry := range []struct {
		name string
		body []byte
	}{{binary, []byte("binary")}, {"LICENSE", []byte("license")}, {"NOTICE", []byte("notice")}, {"THIRD_PARTY_NOTICES.md", []byte("third party")}, {"TRADEMARKS.md", []byte("trademark")}, {"README.md", []byte("readme")}, {"release-metadata.json", metadata}} {
		writer, err := zw.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, data.Bytes())
}

func writeChecksums(t *testing.T, dir, version string) {
	t.Helper()
	var lines []string
	for _, target := range releaseTargets(version) {
		data := readFile(t, filepath.Join(dir, target.Name))
		lines = append(lines, fmt.Sprintf("%x  %s", sha256.Sum256(data), target.Name))
	}
	writeFile(t, filepath.Join(dir, checksumName(version)), []byte(strings.Join(lines, "\n")+"\n"))
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func acceptBinary(_ releaseMetadata, _ []byte) error { return nil }
