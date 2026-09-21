package workspace

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStatusReportMatrix(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root Root)
		code  FindingCode
	}{
		{"uninitialized", func(t *testing.T, root Root) {}, CodeUninitialized},
		{"released ready", initializeFixture, CodeReady},
		{"active owner", func(t *testing.T, root Root) {
			initializeFixture(t, root)
			writeOwnership(t, root, "ACTIVE", "none")
		}, CodeActiveOwner},
		{"unknown namespace", func(t *testing.T, root Root) {
			mustMkdir(t, filepath.Join(root.Path(), ".uawp"))
		}, CodeUnknownNamespace},
		{"malformed manifest", func(t *testing.T, root Root) {
			mustMkdir(t, filepath.Join(root.Path(), ".uawp"))
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "manifest.json"), "{")
		}, CodeInvalidManifest},
		{"unsupported version", func(t *testing.T, root Root) {
			mustMkdir(t, filepath.Join(root.Path(), ".uawp"))
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "manifest.json"), `{"protocol":"UAWP","stateVersion":"2.0.0"}`)
		}, CodeUnsupportedVersion},
		{"missing state", func(t *testing.T, root Root) {
			initializeFixture(t, root)
			if err := os.Remove(filepath.Join(root.Path(), ".uawp", "CONTEXT.md")); err != nil {
				t.Fatal(err)
			}
		}, CodeIncompleteState},
		{"malformed ownership", func(t *testing.T, root Root) {
			initializeFixture(t, root)
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"), "not ownership")
		}, CodeInvalidOwnership},
		{"recovery journal", func(t *testing.T, root Root) {
			initializeFixture(t, root)
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "RECOVERY.json"), `{}`)
		}, CodeRecoveryRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := openTempRoot(t)
			tt.setup(t, root)
			before := snapshotTree(t, root.Path())
			report := Status(root)
			after := snapshotTree(t, root.Path())
			if report.Code != tt.code {
				t.Fatalf("Status().Code = %q, want %q; report=%#v", report.Code, tt.code, report)
			}
			if report.Mutated || report.NextAction == "" || report.Observed == "" || report.Severity == "" {
				t.Fatalf("incomplete status report: %#v", report)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("Status mutated workspace\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

func TestDoctorReportMatrixIsReadOnly(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(t *testing.T, root Root)
		code  FindingCode
	}{
		{"ready", initializeFixture, CodeReady},
		{"recovery", func(t *testing.T, root Root) {
			initializeFixture(t, root)
			mustWrite(t, filepath.Join(root.Path(), ".uawp", "RECOVERY.json"), `{}`)
		}, CodeRecoveryRequired},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := openTempRoot(t)
			tt.setup(t, root)
			before := snapshotTree(t, root.Path())
			report := Doctor(root)
			after := snapshotTree(t, root.Path())
			if report.Code != tt.code || report.Mutated || report.NextAction == "" {
				t.Fatalf("Doctor() = %#v, want code %q and read-only guidance", report, tt.code)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("Doctor mutated workspace\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

func initializeFixture(t *testing.T, root Root) {
	t.Helper()
	value, err := PlanInit(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(root, value, ApplyOptions{ApprovedPlanID: value.ID}); err != nil {
		t.Fatal(err)
	}
}

func writeOwnership(t *testing.T, root Root, status, releasedAt string) {
	t.Helper()
	content := fmt.Sprintf("# UAWP Active Worker\n\n- Status: %s\n- Worker ID: worker-a\n- Agent: Test Agent\n- Acquired At: 2026-09-21T10:00:00+08:00\n- Released At: %s\n- Purpose: test diagnostics\n", status, releasedAt)
	mustWrite(t, filepath.Join(root.Path(), ".uawp", "ACTIVE_WORKER.md"), content)
}

type fileSnapshot struct {
	Mode fs.FileMode
	Size int64
	Hash [32]byte
}

func snapshotTree(t *testing.T, root string) map[string]fileSnapshot {
	t.Helper()
	result := map[string]fileSnapshot{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fileSnapshot{Mode: info.Mode(), Size: info.Size()}
		if info.Mode().IsRegular() {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Hash = sha256.Sum256(content)
		}
		result[relative] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
