package cli

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/workspace"
)

type maintenanceFlags struct {
	workspace, format, approval, controller, reason, export string
	purge                                                   bool
	selected                                                []string
}

func parseMaintenance(command string, args []string, stderr io.Writer) (maintenanceFlags, bool) {
	var f maintenanceFlags
	var selected stringList
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&f.workspace, "workspace", "", "workspace root")
	set.StringVar(&f.format, "format", "text", "output format")
	set.StringVar(&f.approval, "approve", "", "approved plan ID")
	set.StringVar(&f.controller, "controller-id", "", "Human Controller ID")
	set.StringVar(&f.reason, "reason", "", "auditable reason")
	set.StringVar(&f.export, "export", "", "external .tar.gz export path")
	set.BoolVar(&f.purge, "purge-state", false, "purge state after verified export")
	set.BoolVar(&f.purge, "purge", false, "deprecated alias for --purge-state")
	set.Var(&selected, "select", "repair item")
	if set.Parse(args) != nil || set.NArg() != 0 || f.workspace == "" || (f.format != "text" && f.format != "json") {
		return f, false
	}
	f.selected = []string(selected)
	return f, true
}

func runMaintenance(command string, args []string, stdout, stderr io.Writer) int {
	f, ok := parseMaintenance(command, args, stderr)
	if !ok {
		return exitUsage
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInternal
	}
	generatedAt := time.Now()
	approvedHash := ""
	if f.approval != "" {
		generatedAt, approvedHash, err = parseApprovalToken(f.approval)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitApprovalRequired
		}
	}
	var value plan.Plan
	var findings []workspace.StatusReport
	var report any
	switch command {
	case "upgrade":
		value, report, err = workspace.PlanUpgradeAt(root, adapter.RuntimeFacts{}, generatedAt)
	case "repair":
		value, findings, err = workspace.PlanRepairAt(root, adapter.RuntimeFacts{}, f.selected, generatedAt)
		report = findings
	case "uninstall":
		if f.purge {
			if f.export == "" {
				fmt.Fprintln(stderr, "--export is required with --purge")
				return exitUsage
			}
			value, report, err = workspace.PlanPurgeAt(root, adapter.RuntimeFacts{}, f.export, generatedAt)
		} else {
			value, report, err = workspace.PlanUninstallDetachAt(root, adapter.RuntimeFacts{}, generatedAt)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInvalidState
	}
	return previewOrApplyMaintenance(command, root, f, generatedAt, approvedHash, value, findings, report, stdout, stderr)
}

func runTransaction(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return exitUsage
	}
	action := args[0]
	if action != "status" && action != "rollback" && action != "continue" {
		return exitUsage
	}
	f, ok := parseMaintenance("transaction "+action, args[1:], stderr)
	if !ok {
		return exitUsage
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInternal
	}
	if action == "status" {
		if f.approval != "" {
			return exitUsage
		}
		report, statusErr := workspace.TransactionStatus(root)
		if statusErr != nil {
			if workspace.HasPurgeTombstone(root) {
				result := commandOutput{SchemaVersion: "1", Command: "transaction status", Workspace: root.Path(), Classification: "TRANSACTION_COMPLETE", NextAction: "Continue verified purge tombstone cleanup."}
				writeOutput(stdout, f.format, result)
				return exitOK
			}
			fmt.Fprintln(stderr, statusErr)
			return exitInvalidState
		}
		result := commandOutput{SchemaVersion: "1", Command: "transaction status", Workspace: root.Path(), Classification: report, NextAction: "Choose an evidence-supported transaction recovery action."}
		writeOutput(stdout, f.format, result)
		return exitOK
	}
	if f.controller == "" || f.reason == "" {
		fmt.Fprintln(stderr, "--controller-id and --reason are required")
		return exitUsage
	}
	generatedAt := time.Now()
	approvedHash := ""
	if f.approval != "" {
		generatedAt, approvedHash, err = parseApprovalToken(f.approval)
		if err != nil {
			return exitApprovalRequired
		}
	}
	var value plan.Plan
	if action == "rollback" {
		value, err = workspace.PlanTransactionRollbackAt(root, f.controller, f.reason, generatedAt)
	} else {
		value, err = workspace.PlanTransactionContinueAt(root, f.controller, f.reason, generatedAt)
		if err != nil && workspace.HasPurgeTombstone(root) {
			value, err = workspace.PlanPurgeCleanupAt(root, f.controller, f.reason, generatedAt)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInvalidState
	}
	return previewOrApplyTransaction(action, root, f, generatedAt, approvedHash, value, stdout, stderr)
}

func previewOrApplyMaintenance(command string, root workspace.Root, f maintenanceFlags, at time.Time, approvedHash string, value plan.Plan, findings []workspace.StatusReport, detail any, stdout, stderr io.Writer) int {
	token := approvalToken(at, value.ID)
	result := commandOutput{SchemaVersion: "1", Command: command, Workspace: root.Path(), PlanID: token, Findings: findings, Lifecycle: detail, Changes: value.Changes(), Metadata: value.Metadata(), Preview: reviewableChanges(root, value), NextAction: "Review the plan and rerun with --approve " + token}
	if len(value.Changes()) == 0 && f.approval == "" && !(command == "uninstall" && f.purge) {
		result.PlanID = ""
		result.NextAction = "No maintenance changes are required."
		writeOutput(stdout, f.format, result)
		return exitOK
	}
	if f.approval == "" {
		writeOutput(stdout, f.format, result)
		return exitApprovalRequired
	}
	if approvedHash != value.ID || f.approval != token {
		return exitApprovalRequired
	}
	if command == "uninstall" && f.purge {
		purge, err := workspace.ApplyPurge(root, value, workspace.PurgeOptions{ApplyOptions: workspace.ApplyOptions{ApprovedPlanID: value.ID}})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitInvalidState
		}
		result.Lifecycle = purge
		result.Mutated = true
	} else {
		applied, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
		if err != nil {
			fmt.Fprintln(stderr, err)
			return exitInvalidState
		}
		result.Mutated = len(applied.Applied) > 0
	}
	if command == "upgrade" {
		if err := workspace.VerifyUpgrade(root, adapter.RuntimeFacts{}); err != nil {
			fmt.Fprintln(stderr, err)
			return exitInvalidState
		}
	}
	if command == "uninstall" && !f.purge {
		if err := workspace.VerifyUninstallDetached(root); err != nil {
			fmt.Fprintln(stderr, err)
			return exitInvalidState
		}
	}
	result.NextAction = "Maintenance applied and verified."
	writeOutput(stdout, f.format, result)
	return exitOK
}

func previewOrApplyTransaction(action string, root workspace.Root, f maintenanceFlags, at time.Time, approvedHash string, value plan.Plan, stdout, stderr io.Writer) int {
	token := approvalToken(at, value.ID)
	result := commandOutput{SchemaVersion: "1", Command: "transaction " + action, Workspace: root.Path(), PlanID: token, Changes: value.Changes(), Metadata: value.Metadata(), Preview: reviewableChanges(root, value), NextAction: "Review recovery evidence and rerun with --approve " + token}
	if f.approval == "" {
		writeOutput(stdout, f.format, result)
		return exitApprovalRequired
	}
	if approvedHash != value.ID || f.approval != token {
		return exitApprovalRequired
	}
	var applied workspace.ApplyReport
	var err error
	if value.Operation == "purge-cleanup" {
		err = workspace.ApplyPurgeCleanup(root, value, value.ID)
		applied.Verified = err == nil
	} else {
		applied, err = workspace.ApplyTransactionRecovery(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInvalidState
	}
	result.Mutated = len(applied.Applied) > 0 || value.Operation == "purge-cleanup"
	result.NextAction = "Transaction recovery applied and verified."
	writeOutput(stdout, f.format, result)
	return exitOK
}
