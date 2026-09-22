package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/transaction"
)

type transactionSnapshot struct {
	bundle      transaction.Bundle
	observation transaction.Observation
	journalHash string
	bundleDir   string
}

func TransactionStatus(root Root) (transaction.Report, error) {
	snapshot, err := loadTransactionSnapshot(root)
	if err != nil {
		return transaction.Report{Classification: transaction.ManualRequired}, err
	}
	return transaction.Classify(snapshot.bundle, snapshot.observation), nil
}

func PlanTransactionRollbackAt(root Root, controllerID, reason string, at time.Time) (plan.Plan, error) {
	return planTransactionRecovery(root, controllerID, reason, "ROLLBACK", at)
}

func PlanTransactionContinueAt(root Root, controllerID, reason string, at time.Time) (plan.Plan, error) {
	return planTransactionRecovery(root, controllerID, reason, "CONTINUE", at)
}

func planTransactionRecovery(root Root, controllerID, reason, action string, _ time.Time) (plan.Plan, error) {
	if controllerID == "" || reason == "" {
		return plan.Plan{}, fmt.Errorf("controller ID and reason are required")
	}
	snapshot, err := loadTransactionSnapshot(root)
	if err != nil {
		return plan.Plan{}, err
	}
	var changes []plan.Change
	if action == "ROLLBACK" {
		changes, err = transaction.RollbackChanges(snapshot.bundle, snapshot.observation)
	} else {
		changes, err = transaction.ContinueChanges(snapshot.bundle, snapshot.observation)
	}
	if err != nil {
		return plan.Plan{}, err
	}
	metadata := plan.Metadata{ControllerID: controllerID, Reason: reason, TransactionID: snapshot.bundle.Journal.TransactionID, JournalSHA256: snapshot.journalHash, RecoveryAction: action}
	return plan.NewForWorkspaceInputsMetadata("transaction-"+action, root.Path(), changes, nil, metadata), nil
}

func ApplyTransactionRecovery(root Root, value plan.Plan, options ApplyOptions) (ApplyReport, error) {
	if options.ApprovedPlanID == "" || options.ApprovedPlanID != value.ID || value.Workspace != root.Path() {
		return ApplyReport{}, fmt.Errorf("exact recovery plan approval is required")
	}
	snapshot, err := loadTransactionSnapshot(root)
	if err != nil {
		return ApplyReport{}, err
	}
	metadata := value.Metadata()
	if metadata.TransactionID != snapshot.bundle.Journal.TransactionID || metadata.JournalSHA256 != snapshot.journalHash || (metadata.RecoveryAction != "ROLLBACK" && metadata.RecoveryAction != "CONTINUE") {
		return ApplyReport{}, fmt.Errorf("recovery plan no longer matches transaction evidence")
	}
	lockPath := filepath.Join(root.Path(), ".uawp", "TRANSACTION.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return ApplyReport{}, fmt.Errorf("another UAWP transaction is active: %w", err)
	}
	_ = lock.Close()
	defer os.Remove(lockPath)
	if err := verifyBeforeState(root, value.Changes()); err != nil {
		return ApplyReport{}, err
	}
	for index, change := range value.Changes() {
		target, err := targetForChange(root, change)
		if err != nil {
			return ApplyReport{}, err
		}
		switch change.Kind {
		case plan.CreateDir:
			err = os.Mkdir(target, os.FileMode(change.Mode))
		case plan.CreateFile:
			err = writeAtomic(target, change.Content(), os.FileMode(change.Mode), index, options)
		case plan.UpdateFile:
			err = writeUpdate(target, change.BeforeSHA256, change.Content(), os.FileMode(change.Mode), index, options)
		case plan.DeleteFile, plan.DeleteDir:
			err = os.Remove(target)
		default:
			err = fmt.Errorf("unsupported recovery change %s", change.Kind)
		}
		if err != nil {
			return ApplyReport{}, err
		}
		if err := verifyAfterState(root, []plan.Change{change}); err != nil {
			return ApplyReport{}, err
		}
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	result := transaction.ResultCompleted
	if metadata.RecoveryAction == "ROLLBACK" {
		result = transaction.ResultRolledBack
	}
	receipt := transaction.Receipt{SchemaVersion: transaction.ReceiptSchemaVersion, TransactionID: snapshot.bundle.Journal.TransactionID, Operation: snapshot.bundle.Journal.Operation, PlanID: snapshot.bundle.Journal.PlanID, CompletedAt: clock().Format(time.RFC3339), Result: result, Hashes: map[string]string{}}
	encodedReceipt, err := transaction.EncodeReceipt(receipt)
	if err != nil {
		return ApplyReport{}, err
	}
	receiptPath := filepath.Join(snapshot.bundleDir, "receipt.json")
	if _, err := os.Lstat(receiptPath); os.IsNotExist(err) {
		err = writeExclusiveSynced(receiptPath, encodedReceipt, 0o600)
	} else {
		err = writeReplaceAtomic(receiptPath, encodedReceipt, 0o600)
	}
	if err != nil {
		return ApplyReport{}, err
	}
	pointerPath := filepath.Join(root.Path(), ".uawp", "RECOVERY.json")
	if err := os.Remove(pointerPath); err != nil {
		return ApplyReport{}, err
	}
	if err := syncDir(filepath.Dir(pointerPath)); err != nil {
		return ApplyReport{}, err
	}
	paths := make([]string, 0, len(value.Changes()))
	for _, change := range value.Changes() {
		paths = append(paths, change.Path)
	}
	return ApplyReport{PlanID: value.ID, Applied: paths, Verified: true}, nil
}

func loadTransactionSnapshot(root Root) (transactionSnapshot, error) {
	pointerPath := filepath.Join(root.Path(), ".uawp", "RECOVERY.json")
	pointerFile, err := os.Open(pointerPath)
	if err != nil {
		return transactionSnapshot{}, fmt.Errorf("open recovery pointer: %w", err)
	}
	pointer, err := transaction.DecodeLivePointer(pointerFile)
	pointerFile.Close()
	if err != nil {
		return transactionSnapshot{}, err
	}
	if pointer.Workspace != root.Path() {
		return transactionSnapshot{}, fmt.Errorf("recovery pointer belongs to another workspace")
	}
	bundleRelative := "recovery/" + pointer.TransactionID
	bundleDir, err := root.ResolveUAWP(bundleRelative)
	if err != nil {
		return transactionSnapshot{}, err
	}
	journalRaw, err := os.ReadFile(filepath.Join(bundleDir, "journal.json"))
	if err != nil {
		return transactionSnapshot{}, err
	}
	journal, err := transaction.DecodeJournal(bytesReader(journalRaw))
	if err != nil {
		return transactionSnapshot{}, err
	}
	planFile, err := os.Open(filepath.Join(bundleDir, "plan.json"))
	if err != nil {
		return transactionSnapshot{}, err
	}
	persisted, err := transaction.DecodePlan(planFile)
	planFile.Close()
	if err != nil {
		return transactionSnapshot{}, err
	}
	bundle := transaction.Bundle{Pointer: pointer, Journal: journal, Plan: persisted}
	if receiptFile, receiptErr := os.Open(filepath.Join(bundleDir, "receipt.json")); receiptErr == nil {
		receipt, decodeErr := transaction.DecodeReceipt(receiptFile)
		receiptFile.Close()
		if decodeErr != nil {
			return transactionSnapshot{}, decodeErr
		}
		bundle.Receipt = &receipt
	} else if !os.IsNotExist(receiptErr) {
		return transactionSnapshot{}, receiptErr
	}
	if err := bundle.Validate(); err != nil {
		return transactionSnapshot{}, err
	}
	observation := transaction.Observation{Paths: map[string]transaction.PathState{}, Inputs: map[string]string{}, Backups: map[int]transaction.BackupState{}}
	for index, action := range journal.Actions {
		change, _ := plan.RestoreChange(action.Change)
		target, err := targetForChange(root, change)
		if err != nil {
			return transactionSnapshot{}, err
		}
		observation.Paths[action.Change.Path] = observeRecoveryPath(target)
		if action.BackupPath != "" {
			backupPath := filepath.Join(bundleDir, filepath.FromSlash(action.BackupPath))
			content, mode, readErr := readRegularBounded(backupPath, transaction.MaxRecordBytes)
			observation.Backups[index] = transaction.BackupState{Valid: readErr == nil && plan.HashBytes(content) == action.BackupSHA256 && uint32(mode.Perm()) == action.BackupMode, SHA256: plan.HashBytes(content), Mode: uint32(mode.Perm()), Content: content}
		}
	}
	for _, input := range persisted.Inputs {
		if _, changed := observation.Paths[input.Path]; changed {
			continue
		}
		target, err := logicalTarget(root, input.Path)
		if err != nil {
			return transactionSnapshot{}, err
		}
		state := observeRecoveryPath(target)
		switch state.Kind {
		case transaction.PathMissing:
			observation.Inputs[input.Path] = plan.MissingSHA256
		case transaction.PathFile:
			observation.Inputs[input.Path] = state.SHA256
		default:
			observation.Inputs[input.Path] = "UNSAFE"
		}
	}
	return transactionSnapshot{bundle: bundle, observation: observation, journalHash: plan.HashBytes(journalRaw), bundleDir: bundleDir}, nil
}

func observeRecoveryPath(target string) transaction.PathState {
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return transaction.PathState{Kind: transaction.PathMissing}
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		return transaction.PathState{Kind: transaction.PathUnsafe}
	}
	if info.IsDir() {
		return transaction.PathState{Kind: transaction.PathDirectory}
	}
	if !info.Mode().IsRegular() || info.Size() > transaction.MaxRecordBytes {
		return transaction.PathState{Kind: transaction.PathUnsafe}
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return transaction.PathState{Kind: transaction.PathUnsafe}
	}
	return transaction.PathState{Kind: transaction.PathFile, SHA256: plan.HashBytes(content)}
}

func logicalTarget(root Root, logical string) (string, error) {
	if logical == ".uawp" {
		return filepath.Join(root.Path(), ".uawp"), nil
	}
	if len(logical) > len(".uawp/") && logical[:len(".uawp/")] == ".uawp/" {
		return root.ResolveUAWP(logical[len(".uawp/"):])
	}
	return root.resolveNative(logical)
}
