package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/workspace"
)

type lifecycleFlags struct {
	workspace, format, approval, worker, session, agent, purpose, contextFile, milestone, label, controller, reason string
	generation                                                                                                      uint64
	refs                                                                                                            []string
}
type stringList []string

func (s *stringList) String() string     { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func parseLifecycle(command string, args []string, stderr io.Writer) (lifecycleFlags, bool) {
	var f lifecycleFlags
	var refs stringList
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&f.workspace, "workspace", "", "workspace root")
	set.StringVar(&f.format, "format", "text", "output format")
	set.StringVar(&f.approval, "approve", "", "approved plan ID")
	set.StringVar(&f.worker, "worker-id", "", "worker ID")
	set.StringVar(&f.session, "session-id", "", "conversation or execution Session ID")
	set.Uint64Var(&f.generation, "generation", 0, "ownership generation")
	set.StringVar(&f.agent, "agent", "", "agent label")
	set.StringVar(&f.purpose, "purpose", "", "operation purpose")
	set.StringVar(&f.contextFile, "context-file", "", "context input file")
	set.StringVar(&f.milestone, "milestone-id", "", "milestone ID")
	set.StringVar(&f.label, "label", "", "milestone label")
	set.Var(&refs, "decision-ref", "decision reference")
	set.StringVar(&f.controller, "controller-id", "", "Human Controller ID")
	set.StringVar(&f.reason, "reason", "", "recovery reason")
	if set.Parse(args) != nil || set.NArg() != 0 || f.workspace == "" || (f.format != "text" && f.format != "json") {
		return f, false
	}
	f.refs = []string(refs)
	return f, true
}

func runResume(args []string, stdout, stderr io.Writer) int {
	f, ok := parseLifecycle("resume", args, stderr)
	if !ok {
		return exitUsage
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		return exitInternal
	}
	report, err := workspace.Resume(root, core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInvalidState
	}
	writeOutput(stdout, f.format, commandOutput{SchemaVersion: "1", Command: "resume", Workspace: root.Path(), Lifecycle: report, NextAction: report.NextAction})
	return exitOK
}

func runLifecycleMutation(command string, args []string, stdout, stderr io.Writer) int {
	f, ok := parseLifecycle(command, args, stderr)
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
			return exitApprovalRequired
		}
	}
	var value plan.Plan
	switch command {
	case "acquire":
		value, err = workspace.PlanAcquireAt(root, core.AcquireRequest{WorkerID: f.worker, SessionID: f.session, Agent: f.agent, Purpose: f.purpose, At: generatedAt})
	case "release":
		value, err = workspace.PlanReleaseAt(root, core.ReleaseRequest{Actor: core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation}, At: generatedAt})
	case "recover":
		value, err = workspace.PlanStaleRecoveryAt(root, core.RecoveryRequest{ControllerID: f.controller, Reason: f.reason, At: generatedAt})
	case "sync", "handoff":
		var content []byte
		if f.contextFile == "-" {
			content, err = io.ReadAll(os.Stdin)
		} else if f.contextFile != "" {
			content, err = os.ReadFile(f.contextFile)
		} else {
			err = fmt.Errorf("--context-file is required")
		}
		if err == nil {
			actor := core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation}
			if command == "sync" {
				value, err = workspace.PlanContextSync(root, actor, content)
			} else {
				value, err = workspace.PlanHandoffAt(root, workspace.HandoffRequest{Actor: actor, FinalContext: content, Purpose: f.purpose, At: generatedAt})
			}
		}
	case "checkpoint":
		value, err = workspace.PlanCheckpointAt(root, workspace.CheckpointRequest{Actor: core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation}, MilestoneID: f.milestone, Label: f.label, DecisionReferences: f.refs, At: generatedAt})
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInvalidState
	}
	token := approvalToken(generatedAt, value.ID)
	result := commandOutput{SchemaVersion: "1", Command: command, Workspace: root.Path(), PlanID: token, Changes: value.Changes(), Metadata: value.Metadata(), Preview: reviewableChanges(root, value), NextAction: "Review the plan and rerun with --approve " + token}
	if f.approval == "" {
		writeOutput(stdout, f.format, result)
		return exitApprovalRequired
	}
	if approvedHash != value.ID || f.approval != token {
		return exitApprovalRequired
	}
	report, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitInvalidState
	}
	result.Mutated = len(report.Applied) > 0
	result.NextAction = "Operation applied and verified."
	writeOutput(stdout, f.format, result)
	return exitOK
}

func reviewableChanges(root workspace.Root, value plan.Plan) []previewChange {
	var result []previewChange
	for _, change := range value.Changes() {
		before := "<missing>"
		if raw, err := os.ReadFile(filepath.Join(root.Path(), filepath.FromSlash(change.Path))); err == nil {
			before = string(raw)
		}
		after := "<directory>"
		if change.Kind != plan.CreateDir {
			after = string(change.Content())
		}
		result = append(result, previewChange{Path: change.Path, Before: before, After: after})
	}
	return result
}
