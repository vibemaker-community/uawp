package transaction

import (
	"fmt"
	"sort"

	"github.com/uawp/uawp/internal/plan"
)

func RollbackChanges(bundle Bundle, observation Observation) ([]plan.Change, error) {
	classification := Classify(bundle, observation).Classification
	if classification != RollbackAvailable && classification != BothAvailable {
		return nil, fmt.Errorf("rollback is unavailable: %s", classification)
	}
	var changes []plan.Change
	for index := len(bundle.Journal.Actions) - 1; index >= 0; index-- {
		action := bundle.Journal.Actions[index]
		live := observation.Paths[action.Change.Path]
		if matchesBefore(action.Change, live) {
			continue
		}
		backup := observation.Backups[index]
		var change plan.Change
		switch action.Change.Kind {
		case plan.CreateFile:
			change = plan.NewDeleteFile(action.Change.Path, action.Change.AfterSHA256)
		case plan.CreateDir:
			change = plan.NewDeleteDirectory(action.Change.Path)
		case plan.UpdateFile:
			change = plan.NewUpdateFile(action.Change.Path, action.BackupMode, action.Change.AfterSHA256, backup.Content)
		case plan.DeleteFile:
			change = plan.NewFile(action.Change.Path, action.BackupMode, plan.MissingSHA256, backup.Content)
		case plan.DeleteDir:
			change = plan.NewDirectory(action.Change.Path, action.Change.Mode)
		default:
			return nil, fmt.Errorf("unsupported rollback kind %s", action.Change.Kind)
		}
		changes = append(changes, change.WithSequence((len(bundle.Journal.Actions)-index)*10))
	}
	return changes, nil
}

func ContinueChanges(bundle Bundle, observation Observation) ([]plan.Change, error) {
	classification := Classify(bundle, observation).Classification
	if classification != ContinueAvailable && classification != BothAvailable && classification != TransactionComplete {
		return nil, fmt.Errorf("continuation is unavailable: %s", classification)
	}
	var changes []plan.Change
	for _, action := range bundle.Journal.Actions {
		live := observation.Paths[action.Change.Path]
		if matchesAfter(action.Change, live) {
			continue
		}
		if !matchesBefore(action.Change, live) {
			return nil, fmt.Errorf("path %s is neither before nor after", action.Change.Path)
		}
		change, err := plan.RestoreChange(action.Change)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Sequence < changes[j].Sequence })
	return changes, nil
}
