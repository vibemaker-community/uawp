package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/vibemaker-community/uawp/internal/adapter"
	"github.com/vibemaker-community/uawp/internal/plan"
	"github.com/vibemaker-community/uawp/internal/workspace"
)

var launchAdapterIDs = []string{"claude-code", "codex", "workbuddy"}

type adapterFlags struct {
	workspace, format, approval, version                       string
	instructionFiles, directAgentsSupport, providerEnvironment string
	fallbackFilenames, workingDirectory                        string
	acknowledged                                               []string
}

func runAdapter(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return adapterUsage(stderr)
	}
	action := args[0]
	if action != "list" && action != "add" && action != "remove" {
		return adapterUsage(stderr)
	}
	id := ""
	flagArgs := args[1:]
	if action != "list" {
		if len(flagArgs) == 0 || strings.HasPrefix(flagArgs[0], "-") {
			return adapterUsage(stderr)
		}
		id, flagArgs = flagArgs[0], flagArgs[1:]
		if _, err := adapter.Lookup(id); err != nil {
			fmt.Fprintln(stderr, err)
			return exitUsage
		}
	}
	f, ok := parseAdapterFlags(action, flagArgs, stderr)
	if !ok {
		return exitUsage
	}
	root, err := workspace.OpenRoot(f.workspace)
	if err != nil {
		fmt.Fprintf(stderr, "open workspace: %v\n", err)
		return exitInternal
	}
	facts := adapterFacts(id, f)
	if action == "list" {
		resolutions, err := workspace.ResolveAdapters(root, launchAdapterIDs, adapter.RuntimeFacts{})
		if err != nil {
			return writeAdapterError(stderr, err)
		}
		writeOutput(stdout, f.format, commandOutput{SchemaVersion: "1", Command: "adapter list", Workspace: root.Path(), Adapters: resolutions, NextAction: adapterNextAction(resolutions)})
		return exitOK
	}

	generatedAt := time.Now()
	approvedHash := ""
	if f.approval != "" {
		generatedAt, approvedHash, err = parseApprovalToken(f.approval)
		if err != nil {
			fmt.Fprintf(stderr, "approval rejected: %v\n", err)
			return exitApprovalRequired
		}
	}
	var value plan.Plan
	var resolution adapter.Resolution
	if action == "add" {
		value, resolution, err = workspace.PlanAdapterAddAt(root, id, facts, f.acknowledged, generatedAt)
	} else {
		value, err = workspace.PlanAdapterRemoveAt(root, id, facts, generatedAt)
		if err == nil {
			resolved, resolveErr := workspace.ResolveAdapters(root, []string{id}, facts)
			if resolveErr == nil && len(resolved) == 1 {
				resolution = resolved[0]
			}
		}
	}
	if err != nil {
		return writeAdapterError(stderr, err)
	}
	token := approvalToken(generatedAt, value.ID)
	result := commandOutput{SchemaVersion: "1", Command: "adapter " + action, Workspace: root.Path(), PlanID: token, Adapters: []adapter.Resolution{resolution}, Changes: value.Changes(), Preview: reviewableChanges(root, value), NextAction: "Review the plan and rerun with --approve " + token}
	if f.approval == "" {
		writeOutput(stdout, f.format, result)
		return exitApprovalRequired
	}
	if approvedHash != value.ID || f.approval != token {
		fmt.Fprintln(stderr, "approval rejected: current plan differs from approved plan")
		return exitApprovalRequired
	}
	report, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		fmt.Fprintf(stderr, "apply failed: %v\n", err)
		return exitInvalidState
	}
	if action == "add" {
		err = workspace.VerifyAdapterRoute(root, id, facts)
	} else {
		err = workspace.VerifyAdapterRemoved(root, id)
	}
	if err != nil {
		fmt.Fprintf(stderr, "post-apply adapter verification failed: %v\n", err)
		return exitInvalidState
	}
	result.Mutated = len(report.Applied) > 0
	result.NextAction = "Adapter operation applied and verified."
	writeOutput(stdout, f.format, result)
	return exitOK
}

func parseAdapterFlags(action string, args []string, stderr io.Writer) (adapterFlags, bool) {
	var f adapterFlags
	var ack stringList
	set := flag.NewFlagSet("adapter "+action, flag.ContinueOnError)
	set.SetOutput(stderr)
	set.StringVar(&f.workspace, "workspace", "", "workspace root")
	set.StringVar(&f.format, "format", "text", "output format")
	set.StringVar(&f.approval, "approve", "", "approved plan ID")
	set.StringVar(&f.version, "provider-version", "", "observed provider version")
	set.StringVar(&f.instructionFiles, "instruction-files", "", "observed instructionFiles setting")
	set.StringVar(&f.directAgentsSupport, "direct-agents-support", "", "observed direct AGENTS.md support")
	set.StringVar(&f.providerEnvironment, "provider-environment", "", "observed provider environment")
	set.StringVar(&f.fallbackFilenames, "fallback-filenames", "", "observed Codex fallback filenames")
	set.StringVar(&f.workingDirectory, "working-directory", "", "observed Codex working directory")
	set.Var(&ack, "acknowledge", "finding code acknowledged after review")
	if set.Parse(args) != nil || set.NArg() != 0 || f.workspace == "" || (f.format != "text" && f.format != "json") {
		return f, false
	}
	if action == "list" && (f.approval != "" || f.version != "" || f.instructionFiles != "" || f.directAgentsSupport != "" || f.providerEnvironment != "" || f.fallbackFilenames != "" || f.workingDirectory != "" || len(ack) > 0) {
		fmt.Fprintln(stderr, "adapter list accepts only --workspace and --format")
		return f, false
	}
	f.acknowledged = []string(ack)
	return f, true
}

func adapterFacts(id string, f adapterFlags) adapter.RuntimeFacts {
	options := map[string]string{}
	if f.instructionFiles != "" {
		options["instructionFiles"] = f.instructionFiles
	}
	if f.directAgentsSupport != "" {
		options["directAgentsSupport"] = f.directAgentsSupport
	}
	if f.providerEnvironment != "" {
		options["providerEnvironment"] = f.providerEnvironment
	}
	if f.fallbackFilenames != "" {
		options["fallbackFilenames"] = f.fallbackFilenames
	}
	if f.workingDirectory != "" {
		options["workingDirectory"] = f.workingDirectory
	}
	return adapter.RuntimeFacts{Versions: map[string]string{id: f.version}, Options: map[string]map[string]string{id: options}}
}

func writeAdapterError(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	if strings.Contains(err.Error(), "unsafe native entry") {
		return exitUnsafe
	}
	return exitInvalidState
}

func adapterNextAction(resolutions []adapter.Resolution) string {
	for _, resolution := range resolutions {
		if resolution.Health != "HEALTHY" {
			for _, finding := range resolution.Findings {
				if finding.NextAction != "" {
					return finding.NextAction
				}
			}
			return "Review adapter findings before mutation."
		}
	}
	return "No adapter action is required."
}

func adapterUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: uawp adapter <list|add PROVIDER|remove PROVIDER> --workspace PATH")
	return exitUsage
}
