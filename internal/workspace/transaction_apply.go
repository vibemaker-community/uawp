package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/transaction"
)

type durableTransaction struct {
	root      Root
	id        string
	namespace string
	bundleDir string
	journal   transaction.Journal
	bootstrap bool
	metadata  plan.Metadata
}

func prepareDurableTransaction(root Root, value plan.Plan, options ApplyOptions) (*durableTransaction, error) {
	if err := callFailpoint(options, "before-bundle", -1); err != nil {
		return nil, err
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	started := clock()
	idHash := plan.HashBytes([]byte(value.ID + "\x00" + started.Format(time.RFC3339Nano)))
	id := "txn-" + idHash[:32]
	namespace := filepath.Join(root.Path(), ".uawp")
	storageNamespace := namespace
	bootstrap := false
	if _, err := os.Lstat(namespace); os.IsNotExist(err) {
		bootstrap = true
		storageNamespace = filepath.Join(root.Path(), ".uawp-bootstrap-"+id)
		if err := os.Mkdir(storageNamespace, 0o700); err != nil {
			return nil, fmt.Errorf("create bootstrap namespace: %w", err)
		}
	} else if err != nil {
		return nil, err
	}

	recoveryRoot := filepath.Join(storageNamespace, "recovery")
	bundleDir := filepath.Join(recoveryRoot, id)
	cleanupPath := bundleDir
	if bootstrap {
		cleanupPath = storageNamespace
	}
	cleanup := func() { _ = os.RemoveAll(cleanupPath) }
	backupDir := filepath.Join(bundleDir, "backups")
	if info, err := os.Lstat(recoveryRoot); os.IsNotExist(err) {
		if err := os.Mkdir(recoveryRoot, 0o700); err != nil {
			cleanup()
			return nil, fmt.Errorf("create recovery root: %w", err)
		}
	} else if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		cleanup()
		return nil, fmt.Errorf("unsafe recovery root")
	}
	for _, directory := range []string{bundleDir, backupDir} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			cleanup()
			return nil, fmt.Errorf("create recovery directory: %w", err)
		}
	}

	persisted := value.Persisted()
	encodedPlan, err := transaction.EncodePlan(persisted)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := writeExclusiveSynced(filepath.Join(bundleDir, "plan.json"), encodedPlan, 0o600); err != nil {
		cleanup()
		return nil, err
	}
	if err := callFailpoint(options, "after-plan-sync", -1); err != nil {
		cleanup()
		return nil, err
	}

	actions := make([]transaction.Action, len(persisted.Changes))
	for index, persistedChange := range persisted.Changes {
		action := transaction.Action{Index: index, Change: persistedChange, State: transaction.Pending}
		if persistedChange.Kind == plan.UpdateFile || persistedChange.Kind == plan.DeleteFile {
			change, err := plan.RestoreChange(persistedChange)
			if err != nil {
				cleanup()
				return nil, err
			}
			target, err := targetForChange(root, change)
			if err != nil {
				cleanup()
				return nil, err
			}
			content, mode, err := readRegularBounded(target, transaction.MaxRecordBytes)
			if err != nil || plan.HashBytes(content) != persistedChange.BeforeSHA256 {
				cleanup()
				return nil, fmt.Errorf("cannot stage verified backup for %s", persistedChange.Path)
			}
			action.BackupPath = fmt.Sprintf("backups/%04d.bak", index)
			action.BackupSHA256 = plan.HashBytes(content)
			action.BackupMode = uint32(mode.Perm())
			if err := writeExclusiveSynced(filepath.Join(bundleDir, filepath.FromSlash(action.BackupPath)), content, 0o600); err != nil {
				cleanup()
				return nil, err
			}
		}
		actions[index] = action
	}
	if err := syncDir(backupDir); err != nil {
		cleanup()
		return nil, err
	}
	if err := callFailpoint(options, "after-backup-sync", -1); err != nil {
		cleanup()
		return nil, err
	}
	if bootstrap {
		for index := range actions {
			if actions[index].Change.Kind == plan.CreateDir && (actions[index].Change.Path == ".uawp" || actions[index].Change.Path == ".uawp/recovery") {
				actions[index].State = transaction.Verified
			}
		}
	}
	journal := transaction.Journal{SchemaVersion: transaction.JournalSchemaVersion, TransactionID: id, Operation: value.Operation, PlanID: value.ID, Workspace: root.Path(), Phase: transaction.PhaseApplying, StartedAt: started.Format(time.RFC3339), Actions: actions}
	encodedJournal, err := transaction.EncodeJournal(journal)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := writeExclusiveSynced(filepath.Join(bundleDir, "journal.json"), encodedJournal, 0o600); err != nil {
		cleanup()
		return nil, err
	}
	pointer := transaction.LivePointer{SchemaVersion: transaction.PointerSchemaVersion, TransactionID: id, Operation: value.Operation, PlanID: value.ID, Workspace: root.Path(), Phase: transaction.PhaseApplying}
	encodedPointer, err := transaction.EncodeLivePointer(pointer)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := writeExclusiveSynced(filepath.Join(storageNamespace, "RECOVERY.json"), encodedPointer, 0o600); err != nil {
		cleanup()
		return nil, err
	}
	if bootstrap {
		if err := callFailpoint(options, "before-bootstrap-rename", -1); err != nil {
			cleanup()
			return nil, err
		}
		if _, err := os.Lstat(namespace); !os.IsNotExist(err) {
			cleanup()
			return nil, fmt.Errorf("namespace appeared during bootstrap")
		}
		if err := os.Rename(storageNamespace, namespace); err != nil {
			cleanup()
			return nil, fmt.Errorf("publish bootstrap namespace: %w", err)
		}
		if err := syncDir(root.Path()); err != nil {
			return nil, err
		}
		bundleDir = filepath.Join(namespace, "recovery", id)
	}
	if err := callFailpoint(options, "after-pointer-sync", -1); err != nil {
		return nil, err
	}
	return &durableTransaction{root: root, id: id, namespace: namespace, bundleDir: bundleDir, journal: journal, bootstrap: bootstrap, metadata: value.Metadata()}, nil
}

func (d *durableTransaction) transition(index int, state transaction.ActionState) error {
	current := d.journal.Actions[index].State
	if !transaction.CanTransition(current, state) {
		return fmt.Errorf("invalid action transition %s -> %s", current, state)
	}
	d.journal.Actions[index].State = state
	encoded, err := transaction.EncodeJournal(d.journal)
	if err != nil {
		return err
	}
	return writeReplaceAtomic(filepath.Join(d.bundleDir, "journal.json"), encoded, 0o600)
}

func (d *durableTransaction) complete(changes []plan.Change, options ApplyOptions) error {
	hashes := map[string]string{}
	for _, change := range changes {
		if change.AfterSHA256 != plan.MissingSHA256 && change.AfterSHA256 != plan.DirectorySHA256 {
			hashes[change.Path] = change.AfterSHA256
		}
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	if d.metadata.MigrationReceiptPath != "" {
		if err := d.writeMigrationReceipt(changes, clock()); err != nil {
			return err
		}
	}
	receipt := transaction.Receipt{SchemaVersion: transaction.ReceiptSchemaVersion, TransactionID: d.id, Operation: d.journal.Operation, PlanID: d.journal.PlanID, CompletedAt: clock().Format(time.RFC3339), Result: transaction.ResultCompleted, Hashes: hashes}
	encoded, err := transaction.EncodeReceipt(receipt)
	if err != nil {
		return err
	}
	if err := writeExclusiveSynced(filepath.Join(d.bundleDir, "receipt.json"), encoded, 0o600); err != nil {
		return err
	}
	if err := callFailpoint(options, "after-receipt-sync", -1); err != nil {
		return err
	}
	pointerPath := filepath.Join(d.namespace, "RECOVERY.json")
	if err := os.Remove(pointerPath); err != nil {
		return err
	}
	if err := syncDir(d.namespace); err != nil {
		return err
	}
	return callFailpoint(options, "after-pointer-clear", -1)
}

func (d *durableTransaction) writeMigrationReceipt(changes []plan.Change, completedAt time.Time) error {
	hashes := map[string]string{}
	for _, change := range changes {
		if change.AfterSHA256 != plan.MissingSHA256 && change.AfterSHA256 != plan.DirectorySHA256 {
			hashes[change.Path] = change.AfterSHA256
		}
	}
	content, err := encodeMigrationReceipt(migrationReceipt{SchemaVersion: "1", From: d.metadata.MigrationFrom, To: d.metadata.MigrationTo, ApprovedPlanID: d.journal.PlanID, CompletedAt: completedAt.Format(time.RFC3339), ResultingHashes: hashes})
	if err != nil {
		return err
	}
	relative := d.metadata.MigrationReceiptPath
	if !strings.HasPrefix(relative, ".uawp/") {
		return fmt.Errorf("invalid migration receipt path")
	}
	target, err := d.root.ResolveUAWP(strings.TrimPrefix(relative, ".uawp/"))
	if err != nil {
		return err
	}
	if existing, readErr := os.ReadFile(target); readErr == nil {
		if string(existing) == string(content) {
			return nil
		}
		return fmt.Errorf("migration receipt collision at %s", relative)
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	return writeExclusiveSynced(target, content, 0o600)
}

func writeExclusiveSynced(target string, content []byte, mode os.FileMode) error {
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return syncDir(filepath.Dir(target))
}

func readRegularBounded(target string, limit int64) ([]byte, os.FileMode, error) {
	info, err := os.Lstat(target)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, 0, fmt.Errorf("unsafe or oversized backup source")
	}
	content, err := os.ReadFile(target)
	return content, info.Mode(), err
}
