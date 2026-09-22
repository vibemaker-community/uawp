package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/identity"
	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/workspace"
)

type lifecycleFlags struct {
	workspace, format, approval, profile, worker, session, agent, purpose, contextFile, milestone, label, controller, reason string
	generation                                                                                                               uint64
	nonInteractive                                                                                                           bool
	refs                                                                                                                     []string
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
	set.StringVar(&f.profile, "profile", "", "local Worker profile ID")
	set.StringVar(&f.worker, "worker-id", "", "worker ID")
	set.StringVar(&f.session, "session-id", "", "conversation or execution Session ID")
	set.Uint64Var(&f.generation, "generation", 0, "ownership generation")
	set.BoolVar(&f.nonInteractive, "non-interactive", false, "disable prompts")
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

func lifecycleArgsWithWorkspace(args []string, rt runtime) ([]string, error) {
	for index, arg := range args {
		if arg == "--workspace" || strings.HasPrefix(arg, "--workspace=") {
			if arg == "--workspace" && index+1 >= len(args) {
				return args, nil
			}
			return args, nil
		}
	}
	value, err := rt.getwd()
	if err != nil {
		return nil, err
	}
	return append(append([]string(nil), args...), "--workspace", value), nil
}

func runResume(args []string, stdout, stderr io.Writer) int {
	return runResumeWithRuntime(args, defaultRuntime(stdout, stderr))
}

func runResumeWithRuntime(args []string, rt runtime) int {
	var err error
	args, err = lifecycleArgsWithWorkspace(args, rt)
	if err != nil {
		fmt.Fprintf(rt.stderr, "resolve current workspace: %v\n", err)
		return exitInternal
	}
	f, ok := parseLifecycle("resume", args, rt.stderr)
	if !ok {
		return exitUsage
	}
	if f.profile != "" && f.worker != "" {
		fmt.Fprintln(rt.stderr, "choose either --profile or --worker-id, not both")
		return exitUsage
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		return exitInternal
	}
	surface := selectSurface(f.format, rt.stdinTTY, rt.stdoutTTY, f.nonInteractive)
	if surface == surfaceHuman && f.worker == "" {
		return runGuidedResume(root, f, rt)
	}
	actor, actorErr := resolveMachineActor(f, rt)
	if actorErr != nil {
		fmt.Fprintln(rt.stderr, actorErr)
		return exitUsage
	}
	report, err := workspace.Resume(root, actor)
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInvalidState
	}
	writeOutput(rt.stdout, f.format, commandOutput{SchemaVersion: "1", Command: "resume", Workspace: root.Path(), Lifecycle: report, NextAction: report.NextAction})
	return exitOK
}

func resolveMachineActor(f lifecycleFlags, rt runtime) (core.Actor, error) {
	if f.worker != "" {
		if f.session == "" {
			return core.Actor{}, fmt.Errorf("--session-id is required with --worker-id")
		}
		return core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation}, nil
	}
	if f.profile == "" || f.session == "" {
		return core.Actor{}, fmt.Errorf("explicit --worker-id and --session-id, or --profile and --session-id, are required")
	}
	registry, _, err := loadIdentityRegistry(rt)
	if err != nil {
		return core.Actor{}, err
	}
	profile, err := registry.Selected(f.profile, false)
	if err != nil {
		return core.Actor{}, err
	}
	return core.Actor{WorkerID: profile.WorkerID, SessionID: f.session, Generation: f.generation}, nil
}

func runGuidedResume(root workspace.Root, f lifecycleFlags, rt runtime) int {
	registry, path, err := loadIdentityRegistry(rt)
	if err != nil {
		fmt.Fprintf(rt.stderr, "load identity registry: %v\n", err)
		return exitInternal
	}
	confirmationInput := rt.stdin
	if len(registry.Profiles) == 0 {
		fmt.Fprint(rt.stdout, "Create a local Worker identity (format example: Zhang San's Codex): ")
		reader := bufio.NewReader(rt.stdin)
		label, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			fmt.Fprintln(rt.stderr, "a display name is required")
			return exitUsage
		}
		label = strings.TrimSpace(label)
		confirmationInput = reader
		profile, createErr := identity.NewProfile(label, rt.random)
		if createErr != nil {
			fmt.Fprintln(rt.stderr, createErr)
			return exitUsage
		}
		registry.Profiles = append(registry.Profiles, profile)
		registry.DefaultProfileID = profile.ProfileID
		if err := identity.Save(path, registry); err != nil {
			fmt.Fprintf(rt.stderr, "save identity registry: %v\n", err)
			return exitInternal
		}
	}
	profile, err := registry.Selected(f.profile, true)
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitUsage
	}
	actor := core.Actor{WorkerID: profile.WorkerID}
	if binding, found := registry.Binding(root.Path(), profile.ProfileID); found {
		actor.SessionID, actor.Generation = binding.SessionID, binding.Generation
	} else {
		actor.SessionID, err = identity.NewSessionID(rt.random)
		if err != nil {
			fmt.Fprintf(rt.stderr, "generate Session ID: %v\n", err)
			return exitInternal
		}
	}
	report, err := workspace.Resume(root, actor)
	if err != nil {
		fmt.Fprintf(rt.stderr, "%v; stale ACTIVE ownership requires Human Controller recovery\n", err)
		return exitInvalidState
	}
	if report.Outcome != workspace.ResumeAcquireAvailable {
		if f.format == "text" {
			fmt.Fprintf(rt.stdout, "Resume outcome: %s\n", report.Outcome)
		}
		writeOutput(rt.stdout, f.format, commandOutput{SchemaVersion: "1", Command: "resume", Workspace: root.Path(), Lifecycle: report, NextAction: report.NextAction})
		return exitOK
	}
	value, err := workspace.PlanAcquireAt(root, core.AcquireRequest{WorkerID: profile.WorkerID, SessionID: actor.SessionID, Agent: profile.DisplayName, Purpose: "Resume work", At: rt.now()})
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInvalidState
	}
	fmt.Fprintf(rt.stdout, "Ownership preview: %s will become ACTIVE for Session %s. Continue? [y/N] ", profile.DisplayName, actor.SessionID)
	approved, err := confirm(confirmationInput)
	if err != nil {
		fmt.Fprintf(rt.stderr, "confirmation failed: %v\n", err)
		return exitInternal
	}
	if !approved {
		fmt.Fprintln(rt.stdout, "Acquisition cancelled; Workspace remains read-only.")
		return exitOK
	}
	if _, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID}); err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInvalidState
	}
	generation := value.Metadata().OwnershipGeneration
	registry.UpsertBinding(identity.SessionBinding{Workspace: root.Path(), ProfileID: profile.ProfileID, SessionID: actor.SessionID, Generation: generation})
	if err := identity.Save(path, registry); err != nil {
		fmt.Fprintf(rt.stderr, "Workspace ownership is ACTIVE as worker=%s session=%s generation=%d, but local binding save failed: %v; rerun resume with the exact tuple\n", profile.WorkerID, actor.SessionID, generation, err)
		return exitInternal
	}
	fmt.Fprintf(rt.stdout, "Ownership acquired: worker=%s session=%s generation=%d\n", profile.WorkerID, actor.SessionID, generation)
	return exitOK
}

func runLifecycleMutation(command string, args []string, stdout, stderr io.Writer) int {
	return runLifecycleMutationWithRuntime(command, args, defaultRuntime(stdout, stderr))
}

type mutationPresentation struct {
	Command     string
	Plan        plan.Plan
	Result      commandOutput
	Exceptional bool
}

func presentMutation(rt runtime, surface cliSurface, value mutationPresentation, apply func(plan.Plan) (bool, error)) int {
	if surface == surfaceHuman && !value.Exceptional {
		fmt.Fprintf(rt.stdout, "%s preview for %s:\n", value.Command, value.Result.Workspace)
		for _, change := range value.Result.Preview {
			fmt.Fprintf(rt.stdout, "  %s\n", change.Path)
		}
		fmt.Fprint(rt.stdout, "Apply these changes? [y/N] ")
		approved, err := confirm(rt.stdin)
		if err != nil {
			fmt.Fprintf(rt.stderr, "confirmation failed: %v\n", err)
			return exitInternal
		}
		if !approved {
			fmt.Fprintln(rt.stdout, "Cancelled; no changes were made.")
			return exitOK
		}
		mutated, err := apply(value.Plan)
		if err != nil {
			fmt.Fprintln(rt.stderr, err)
			return exitInvalidState
		}
		value.Result.PlanID, value.Result.Mutated, value.Result.NextAction = "", mutated, "Operation applied and verified."
		writeOutput(rt.stdout, "text", value.Result)
		return exitOK
	}
	writeOutput(rt.stdout, value.ResultFormat(), value.Result)
	return exitApprovalRequired
}

func (value mutationPresentation) ResultFormat() string {
	if value.Result.PlanID == "" {
		return "text"
	}
	return "json"
}

func runLifecycleMutationWithRuntime(command string, args []string, rt runtime) int {
	var err error
	args, err = lifecycleArgsWithWorkspace(args, rt)
	if err != nil {
		fmt.Fprintf(rt.stderr, "resolve current workspace: %v\n", err)
		return exitInternal
	}
	f, ok := parseLifecycle(command, args, rt.stderr)
	if !ok {
		return exitUsage
	}
	if f.profile != "" && f.worker != "" {
		fmt.Fprintln(rt.stderr, "choose either --profile or --worker-id, not both")
		return exitUsage
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInternal
	}
	surface := selectSurface(f.format, rt.stdinTTY, rt.stdoutTTY, f.nonInteractive)
	if surface == surfaceHuman {
		rt.stdin = bufio.NewReader(rt.stdin)
		if (command == "sync" || command == "handoff") && f.contextFile == "" {
			fmt.Fprintln(rt.stderr, "--context-file PATH is required; terminal confirmation input is never used as context")
			return exitUsage
		}
		if (command == "sync" || command == "handoff") && f.contextFile == "-" {
			fmt.Fprintln(rt.stderr, "--context-file - cannot share interactive stdin with confirmation; use a file or --non-interactive")
			return exitUsage
		}
	}
	generatedAt := rt.now()
	approvedHash := ""
	if f.approval != "" {
		generatedAt, approvedHash, err = parseApprovalToken(f.approval)
		if err != nil {
			return exitApprovalRequired
		}
	}
	var registry identity.Registry
	var registryPath string
	var selectedProfile identity.Profile
	actor := core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation}
	if command != "recover" && surface == surfaceHuman && f.worker == "" {
		registry, registryPath, err = loadIdentityRegistry(rt)
		if err != nil {
			fmt.Fprintln(rt.stderr, err)
			return exitInternal
		}
		selectedProfile, err = registry.Selected(f.profile, true)
		if err != nil {
			fmt.Fprintln(rt.stderr, "run uawp resume first to create/select an identity")
			return exitUsage
		}
		binding, found := registry.Binding(root.Path(), selectedProfile.ProfileID)
		if !found {
			fmt.Fprintln(rt.stderr, "run uawp resume first to acquire this Workspace")
			return exitInvalidState
		}
		actor = core.Actor{WorkerID: selectedProfile.WorkerID, SessionID: binding.SessionID, Generation: binding.Generation}
	} else if command != "recover" {
		actor, err = resolveMachineActor(f, rt)
		if err != nil {
			fmt.Fprintln(rt.stderr, err)
			return exitUsage
		}
	}
	readPrompt := func(label string) (string, error) {
		fmt.Fprint(rt.stdout, label)
		line, readErr := rt.stdin.(*bufio.Reader).ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return "", readErr
		}
		value := strings.TrimSpace(line)
		if value == "" {
			return "", fmt.Errorf("a value is required")
		}
		return value, nil
	}
	if surface == surfaceHuman {
		if command == "checkpoint" && f.milestone == "" {
			f.milestone, err = readPrompt("Milestone ID: ")
		}
		if err == nil && command == "checkpoint" && f.label == "" {
			f.label, err = readPrompt("Milestone label: ")
		}
		if err == nil && (command == "acquire" || command == "handoff") && f.purpose == "" {
			f.purpose, err = readPrompt("Purpose: ")
		}
		if err != nil {
			fmt.Fprintln(rt.stderr, err)
			return exitUsage
		}
	}
	var value plan.Plan
	switch command {
	case "acquire":
		agent := f.agent
		if agent == "" {
			agent = selectedProfile.DisplayName
		}
		value, err = workspace.PlanAcquireAt(root, core.AcquireRequest{WorkerID: actor.WorkerID, SessionID: actor.SessionID, Agent: agent, Purpose: f.purpose, At: generatedAt})
	case "release":
		value, err = workspace.PlanReleaseAt(root, core.ReleaseRequest{Actor: actor, At: generatedAt})
	case "recover":
		value, err = workspace.PlanStaleRecoveryAt(root, core.RecoveryRequest{ControllerID: f.controller, Reason: f.reason, At: generatedAt})
	case "sync", "handoff":
		var content []byte
		if f.contextFile == "-" {
			content, err = io.ReadAll(rt.stdin)
		} else if f.contextFile != "" {
			content, err = os.ReadFile(f.contextFile)
		} else {
			err = fmt.Errorf("--context-file is required")
		}
		if err == nil {
			if command == "sync" {
				value, err = workspace.PlanContextSync(root, actor, content)
			} else {
				value, err = workspace.PlanHandoffAt(root, workspace.HandoffRequest{Actor: actor, FinalContext: content, Purpose: f.purpose, At: generatedAt})
			}
		}
	case "checkpoint":
		value, err = workspace.PlanCheckpointAt(root, workspace.CheckpointRequest{Actor: actor, MilestoneID: f.milestone, Label: f.label, DecisionReferences: f.refs, At: generatedAt})
	}
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInvalidState
	}
	token := approvalToken(generatedAt, value.ID)
	result := commandOutput{SchemaVersion: "1", Command: command, Workspace: root.Path(), PlanID: token, Changes: value.Changes(), Metadata: value.Metadata(), Preview: reviewableChanges(root, value), NextAction: "Review the plan and rerun with --approve " + token}
	if f.approval == "" {
		presentation := mutationPresentation{Command: command, Plan: value, Result: result, Exceptional: command == "recover"}
		if surface == surfaceHuman && command != "recover" {
			return presentMutation(rt, surface, presentation, func(candidate plan.Plan) (bool, error) {
				report, applyErr := workspace.Apply(root, candidate, workspace.ApplyOptions{ApprovedPlanID: candidate.ID})
				if applyErr != nil {
					return false, applyErr
				}
				if command == "handoff" || command == "release" {
					registry.RemoveBinding(root.Path(), selectedProfile.ProfileID)
					if saveErr := identity.Save(registryPath, registry); saveErr != nil {
						return len(report.Applied) > 0, fmt.Errorf("Workspace release verified but local binding cleanup failed: %w", saveErr)
					}
				}
				return len(report.Applied) > 0, nil
			})
		}
		writeOutput(rt.stdout, f.format, result)
		return exitApprovalRequired
	}
	if approvedHash != value.ID || f.approval != token {
		return exitApprovalRequired
	}
	report, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInvalidState
	}
	result.Mutated = len(report.Applied) > 0
	result.NextAction = "Operation applied and verified."
	writeOutput(rt.stdout, f.format, result)
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
