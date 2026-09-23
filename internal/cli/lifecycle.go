package cli

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/uawp/uawp/internal/core"
	"github.com/uawp/uawp/internal/identity"
	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/workspace"
)

type lifecycleFlags struct {
	workspace, format, approval, profile, worker, session, agent, purpose, contextFile, milestone, label, controller, reason string
	generation                                                                                                               uint64
	nonInteractive                                                                                                           bool
	sessionProvided, generationProvided                                                                                      bool
	refs                                                                                                                     []string
}
type stringList []string

func (s *stringList) String() string     { return fmt.Sprint([]string(*s)) }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func generatedCheckpointID(label string, at time.Time, random io.Reader) (string, error) {
	var suffix [8]byte
	if _, err := io.ReadFull(random, suffix[:]); err != nil {
		return "", fmt.Errorf("generate checkpoint ID: %w", err)
	}
	var slug strings.Builder
	lastHyphen := false
	for _, current := range strings.ToLower(strings.TrimSpace(label)) {
		if current >= 'a' && current <= 'z' || current >= '0' && current <= '9' {
			slug.WriteRune(current)
			lastHyphen = false
		} else if (unicode.IsSpace(current) || current == '-' || current == '_' || current == '/') && slug.Len() > 0 && !lastHyphen {
			slug.WriteByte('-')
			lastHyphen = true
		}
		if slug.Len() >= 30 {
			break
		}
	}
	name := strings.Trim(slug.String(), "-")
	if name == "" {
		name = "checkpoint"
	}
	return at.UTC().Format("20060102t150405") + "-" + name + "-" + hex.EncodeToString(suffix[:]), nil
}

func checkpointApprovalToken(base, id string) string {
	return base + "." + base64.RawURLEncoding.EncodeToString([]byte(id))
}

func checkpointApprovalParts(token string) (string, string, error) {
	separator := strings.LastIndexByte(token, '.')
	if separator < 0 {
		return "", "", fmt.Errorf("checkpoint approval token has no generated ID")
	}
	base, encodedID := token[:separator], token[separator+1:]
	if strings.Count(base, ".") != 1 || encodedID == "" {
		return "", "", fmt.Errorf("checkpoint approval token is malformed")
	}
	rawID, err := base64.RawURLEncoding.DecodeString(encodedID)
	if err != nil || len(rawID) == 0 {
		return "", "", fmt.Errorf("checkpoint approval token has an invalid generated ID")
	}
	return base, string(rawID), nil
}

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
	set.Visit(func(value *flag.Flag) {
		switch value.Name {
		case "session-id":
			f.sessionProvided = true
		case "generation":
			f.generationProvided = true
		}
	})
	f.refs = []string(refs)
	return f, true
}

func lifecycleArgsWithWorkspace(args []string, rt runtime) ([]string, error) {
	for index, arg := range args {
		if arg == "--workspace" || arg == "-workspace" || strings.HasPrefix(arg, "--workspace=") || strings.HasPrefix(arg, "-workspace=") {
			if (arg == "--workspace" || arg == "-workspace") && index+1 >= len(args) {
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

func lifecycleRequestedFormat(args []string) string {
	for index, arg := range args {
		if (arg == "--format" || arg == "-format") && index+1 < len(args) {
			return args[index+1]
		}
		for _, prefix := range []string{"--format=", "-format="} {
			if strings.HasPrefix(arg, prefix) {
				return strings.TrimPrefix(arg, prefix)
			}
		}
	}
	return "text"
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
	requestedFormat := lifecycleRequestedFormat(args)
	f, ok := parseLifecycle("resume", args, rt.stderr)
	if !ok {
		return writeLifecycleFailure(rt, requestedFormat, "resume", f.workspace, codeInvalidArguments, "Invalid resume arguments.", exitUsage)
	}
	if f.profile != "" && f.worker != "" {
		return writeLifecycleFailure(rt, f.format, "resume", f.workspace, codeProfileAmbiguous, "Choose either --profile or --worker-id, not both.", exitUsage)
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		return writeLifecycleFailure(rt, f.format, "resume", f.workspace, codeWorkspaceInvalid, err.Error(), exitInternal)
	}
	surface := selectSurface(f.format, rt.stdinTTY, rt.stdoutTTY, f.nonInteractive)
	if surface == surfaceHuman && f.worker == "" && !f.sessionProvided && !f.generationProvided && f.approval == "" {
		return runGuidedResume(root, f, rt)
	}
	actor, actorErr := resolveMachineActor(f, rt)
	if actorErr != nil {
		code := codeIdentityRequired
		if f.worker != "" && f.session == "" {
			code = codeSessionRequired
		}
		if f.profile != "" {
			code = codeProfileAmbiguous
		}
		return writeLifecycleFailure(rt, f.format, "resume", root.Path(), code, actorErr.Error(), exitUsage)
	}
	report, err := workspace.Resume(root, actor)
	if err != nil {
		return writeLifecycleFailure(rt, f.format, "resume", root.Path(), codeWorkspaceInvalid, err.Error(), exitInvalidState)
	}
	writeOutput(rt.stdout, f.format, resumeOutput(root, actor, report))
	return exitOK
}

func writeLifecycleFailure(rt runtime, format, command, workspacePath, code, next string, exitCode int) int {
	if format == "json" {
		writeOutput(rt.stdout, format, commandOutput{SchemaVersion: "1", Command: command, Workspace: workspacePath, Code: code, NextAction: next})
	} else {
		fmt.Fprintln(rt.stderr, next)
	}
	return exitCode
}

func resumeOutput(root workspace.Root, actor core.Actor, report workspace.ResumeReport) commandOutput {
	code := ""
	switch report.Outcome {
	case workspace.ResumeBlockedBySession:
		code = codeActiveOtherSession
	case workspace.ResumeBlockedByOther:
		code = codeActiveOtherWorker
	}
	return commandOutput{SchemaVersion: "1", Command: "resume", Workspace: root.Path(), Lifecycle: report, WorkerID: actor.WorkerID, SessionID: actor.SessionID, OwnershipGeneration: actor.Generation, Code: code, NextAction: report.NextAction}
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
		writeOutput(rt.stdout, f.format, resumeOutput(root, actor, report))
		return exitOK
	}
	value, err := workspace.PlanAcquireAt(root, core.AcquireRequest{WorkerID: profile.WorkerID, SessionID: actor.SessionID, Agent: profile.DisplayName, Purpose: "Resume work", At: rt.now()})
	if err != nil {
		fmt.Fprintln(rt.stderr, err)
		return exitInvalidState
	}
	fmt.Fprintf(rt.stdout, "Ownership preview for %s: %s will become ACTIVE for Session %s. Continue? [y/N] ", root.Path(), profile.DisplayName, actor.SessionID)
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
			fmt.Fprintf(rt.stdout, "  %s\n--- before ---\n%s\n--- after ---\n%s\n", change.Path, change.Before, change.After)
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
	requestedFormat := lifecycleRequestedFormat(args)
	f, ok := parseLifecycle(command, args, rt.stderr)
	if !ok {
		return writeLifecycleFailure(rt, requestedFormat, command, f.workspace, codeInvalidArguments, "Invalid lifecycle arguments.", exitUsage)
	}
	if f.profile != "" && f.worker != "" {
		return writeLifecycleFailure(rt, f.format, command, f.workspace, codeProfileAmbiguous, "Choose either --profile or --worker-id, not both.", exitUsage)
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		return writeLifecycleFailure(rt, f.format, command, f.workspace, codeWorkspaceInvalid, err.Error(), exitInternal)
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
	autoCheckpointID := false
	approvalValue := f.approval
	if command == "checkpoint" && f.milestone == "" && f.approval != "" {
		approvalValue, f.milestone, err = checkpointApprovalParts(f.approval)
		if err != nil {
			return writeLifecycleFailure(rt, f.format, command, root.Path(), codeApprovalDrift, err.Error()+"; request a new preview.", exitApprovalRequired)
		}
		autoCheckpointID = true
	}
	if f.approval != "" {
		generatedAt, approvedHash, err = parseApprovalToken(approvalValue)
		if err != nil {
			return writeLifecycleFailure(rt, f.format, command, root.Path(), codeApprovalDrift, "Approval token is malformed; request a new preview.", exitApprovalRequired)
		}
	}
	var registry identity.Registry
	var registryPath string
	var selectedProfile identity.Profile
	actor := core.Actor{WorkerID: f.worker, SessionID: f.session, Generation: f.generation}
	useBinding := command != "recover" && surface == surfaceHuman && f.worker == "" && !f.sessionProvided && !f.generationProvided && f.approval == ""
	if useBinding {
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
			code := codeIdentityRequired
			if f.worker != "" && f.session == "" {
				code = codeSessionRequired
			}
			if f.profile != "" {
				code = codeProfileAmbiguous
			}
			return writeLifecycleFailure(rt, f.format, command, root.Path(), code, err.Error(), exitUsage)
		}
		if f.profile != "" {
			registry, registryPath, err = loadIdentityRegistry(rt)
			if err != nil {
				return writeLifecycleFailure(rt, f.format, command, root.Path(), codeIdentityRequired, err.Error(), exitInternal)
			}
			selectedProfile, err = registry.Selected(f.profile, false)
			if err != nil {
				return writeLifecycleFailure(rt, f.format, command, root.Path(), codeProfileAmbiguous, err.Error(), exitUsage)
			}
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
		if command == "checkpoint" && f.label == "" {
			f.label, err = readPrompt("Checkpoint name: ")
		}
		if err == nil && (command == "acquire" || command == "handoff") && f.purpose == "" {
			f.purpose, err = readPrompt("Purpose: ")
		}
		if err != nil {
			fmt.Fprintln(rt.stderr, err)
			return exitUsage
		}
	}
	if command == "checkpoint" && f.milestone == "" {
		f.milestone, err = generatedCheckpointID(f.label, generatedAt, rt.random)
		if err != nil {
			return writeLifecycleFailure(rt, f.format, command, root.Path(), codeInvalidArguments, err.Error(), exitInternal)
		}
		autoCheckpointID = true
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
		if f.approval != "" {
			writeOutput(rt.stdout, f.format, commandOutput{SchemaVersion: "1", Command: command, Workspace: root.Path(), WorkerID: actor.WorkerID, SessionID: actor.SessionID, OwnershipGeneration: actor.Generation, Code: codeApprovalDrift, NextAction: "Workspace or actor state changed; request a new preview."})
			return exitApprovalRequired
		}
		code := codeWorkspaceInvalid
		if command != "acquire" && command != "recover" {
			if report, resumeErr := workspace.Resume(root, actor); resumeErr == nil {
				if report.Outcome == workspace.ResumeBlockedBySession {
					code = codeActiveOtherSession
				}
				if report.Outcome == workspace.ResumeBlockedByOther {
					code = codeActiveOtherWorker
				}
			}
		}
		return writeLifecycleFailure(rt, f.format, command, root.Path(), code, err.Error(), exitInvalidState)
	}
	token := approvalToken(generatedAt, value.ID)
	if command == "checkpoint" && autoCheckpointID {
		token = checkpointApprovalToken(token, f.milestone)
	}
	metadata := value.Metadata()
	result := commandOutput{SchemaVersion: "1", Command: command, Workspace: root.Path(), PlanID: token, Changes: value.Changes(), Metadata: metadata, Preview: reviewableChanges(root, value), WorkerID: metadata.ActorWorkerID, SessionID: metadata.ActorSessionID, OwnershipGeneration: metadata.OwnershipGeneration, Code: codeApprovalRequired, NextAction: "Review the plan and rerun with --approve " + token}
	if command == "checkpoint" {
		result.CheckpointID = f.milestone
	}
	if f.approval == "" {
		presentation := mutationPresentation{Command: command, Plan: value, Result: result, Exceptional: command == "recover"}
		if surface == surfaceHuman && command != "recover" {
			return presentMutation(rt, surface, presentation, func(candidate plan.Plan) (bool, error) {
				report, applyErr := workspace.Apply(root, candidate, workspace.ApplyOptions{ApprovedPlanID: candidate.ID})
				if applyErr != nil {
					return false, applyErr
				}
				if (command == "handoff" || command == "release") && registryPath != "" && selectedProfile.ProfileID != "" {
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
		result.Code = codeApprovalDrift
		result.NextAction = "Workspace or actor state changed; request a new preview."
		writeOutput(rt.stdout, f.format, result)
		return exitApprovalRequired
	}
	report, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		return writeLifecycleFailure(rt, f.format, command, root.Path(), codeApprovalDrift, err.Error(), exitInvalidState)
	}
	result.Mutated = len(report.Applied) > 0
	result.Code = ""
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
