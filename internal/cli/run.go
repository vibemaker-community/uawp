package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/uawp/uawp/internal/adapter"
	"github.com/uawp/uawp/internal/plan"
	"github.com/uawp/uawp/internal/workspace"
)

const (
	exitOK               = 0
	exitUsage            = 2
	exitUnsafe           = 3
	exitInvalidState     = 4
	exitApprovalRequired = 5
	exitInternal         = 10
)

type commandOutput struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Command       string                   `json:"command"`
	Workspace     string                   `json:"workspace"`
	PlanID        string                   `json:"planID,omitempty"`
	Findings      []workspace.StatusReport `json:"findings"`
	Adapters      []adapter.Resolution     `json:"adapters,omitempty"`
	Lifecycle     any                      `json:"lifecycle,omitempty"`
	Metadata      plan.Metadata            `json:"metadata,omitempty"`
	Preview       []previewChange          `json:"preview,omitempty"`
	Changes       []plan.Change            `json:"changes"`
	Mutated       bool                     `json:"mutated"`
	NextAction    string                   `json:"nextAction"`
}

type previewChange struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(stdout, "uawp dev")
		return exitOK
	}
	if len(args) == 0 {
		return usage(stderr)
	}
	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "adapter":
		return runAdapter(args[1:], stdout, stderr)
	case "status", "doctor":
		return runDiagnostic(args[0], args[1:], stdout, stderr)
	case "resume":
		return runResume(args[1:], stdout, stderr)
	case "acquire", "release", "sync", "checkpoint", "handoff", "recover":
		return runLifecycleMutation(args[0], args[1:], stdout, stderr)
	default:
		return usage(stderr)
	}
}

func runInit(args []string, stdout, stderr io.Writer) int {
	workspacePath, format, approval, ok := parseFlags("init", args, stderr, true)
	if !ok {
		return exitUsage
	}
	root, err := workspace.OpenRoot(workspacePath)
	if err != nil {
		fmt.Fprintf(stderr, "open workspace: %v\n", err)
		return exitInternal
	}

	generatedAt := time.Now()
	approvedHash := ""
	if approval != "" {
		generatedAt, approvedHash, err = parseApprovalToken(approval)
		if err != nil {
			fmt.Fprintf(stderr, "approval rejected: %v\n", err)
			return exitApprovalRequired
		}
	}
	value, err := workspace.PlanInitAt(root, generatedAt)
	if err != nil {
		fmt.Fprintf(stderr, "unsafe to initialize: %v\n", err)
		return exitUnsafe
	}
	token := approvalToken(generatedAt, value.ID)
	result := commandOutput{SchemaVersion: "1", Command: "init", Workspace: root.Path(), PlanID: token, Changes: value.Changes(), NextAction: "Review the plan and rerun init with --approve " + token}
	if approval == "" {
		writeOutput(stdout, format, result)
		return exitApprovalRequired
	}
	if approvedHash != value.ID || approval != token {
		fmt.Fprintln(stderr, "approval rejected: current plan differs from approved plan")
		return exitApprovalRequired
	}
	report, err := workspace.Apply(root, value, workspace.ApplyOptions{ApprovedPlanID: value.ID})
	if err != nil {
		fmt.Fprintf(stderr, "apply failed: %v\n", err)
		return exitInvalidState
	}
	result.Mutated = len(report.Applied) > 0
	result.NextAction = "Run uawp status to verify workspace readiness."
	writeOutput(stdout, format, result)
	return exitOK
}

func runDiagnostic(command string, args []string, stdout, stderr io.Writer) int {
	workspacePath, format, _, ok := parseFlags(command, args, stderr, false)
	if !ok {
		return exitUsage
	}
	root, err := workspace.OpenRoot(workspacePath)
	if err != nil {
		fmt.Fprintf(stderr, "open workspace: %v\n", err)
		return exitInternal
	}
	report := workspace.Status(root)
	if command == "doctor" {
		report = workspace.Doctor(root)
	}
	result := commandOutput{SchemaVersion: "1", Command: command, Workspace: root.Path(), Findings: []workspace.StatusReport{report}, NextAction: report.NextAction}
	if report.Code == workspace.CodeReady || report.Code == workspace.CodeActiveOwner {
		ids, idsErr := workspace.ConfiguredAdapterIDs(root)
		if idsErr != nil {
			fmt.Fprintf(stderr, "adapter diagnostics failed: %v\n", idsErr)
			return exitInvalidState
		}
		result.Adapters, err = workspace.ResolveAdapters(root, ids, adapter.RuntimeFacts{})
		if err != nil {
			return writeAdapterError(stderr, err)
		}
		if next := adapterNextAction(result.Adapters); len(result.Adapters) > 0 && next != "No adapter action is required." {
			result.NextAction = next
		}
	}
	writeOutput(stdout, format, result)
	switch report.Code {
	case workspace.CodeUnknownNamespace:
		return exitUnsafe
	case workspace.CodeInvalidManifest, workspace.CodeUnsupportedVersion, workspace.CodeInvalidOwnership, workspace.CodeIncompleteState, workspace.CodeRecoveryRequired:
		return exitInvalidState
	default:
		return exitOK
	}
}

func parseFlags(command string, args []string, stderr io.Writer, allowApproval bool) (string, string, string, bool) {
	set := flag.NewFlagSet(command, flag.ContinueOnError)
	set.SetOutput(stderr)
	workspacePath := set.String("workspace", "", "workspace root")
	format := set.String("format", "text", "output format")
	approval := set.String("approve", "", "approved plan ID")
	if err := set.Parse(args); err != nil || set.NArg() != 0 {
		return "", "", "", false
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "unsupported format %q; use text or json\n", *format)
		return "", "", "", false
	}
	if *workspacePath == "" {
		fmt.Fprintln(stderr, "--workspace is required")
		return "", "", "", false
	}
	if !allowApproval && *approval != "" {
		fmt.Fprintf(stderr, "--approve is not supported by %s\n", command)
		return "", "", "", false
	}
	return *workspacePath, *format, *approval, true
}

func writeOutput(writer io.Writer, format string, result commandOutput) {
	if format == "json" {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
		return
	}
	fmt.Fprintf(writer, "observed:\n  workspace: %s\n", result.Workspace)
	for _, finding := range result.Findings {
		fmt.Fprintf(writer, "  %s: %s\n", finding.Code, finding.Observed)
	}
	for _, resolution := range result.Adapters {
		fmt.Fprintf(writer, "  adapter %s: entry=%s mode=%s confidence=%s health=%s\n", resolution.Provider, resolution.Route.Path, resolution.Route.Mode, resolution.Confidence, resolution.Health)
		for _, finding := range resolution.Findings {
			fmt.Fprintf(writer, "    %s: %s\n", finding.Code, finding.Message)
		}
	}
	fmt.Fprintln(writer, "planned:")
	if result.PlanID != "" {
		fmt.Fprintf(writer, "  plan ID: %s\n", result.PlanID)
	}
	for _, change := range result.Changes {
		fmt.Fprintf(writer, "  %s %s before=%s after=%s %d bytes\n", change.Kind, change.Path, change.BeforeSHA256, change.AfterSHA256, change.Size)
	}
	fmt.Fprintf(writer, "changed:\n  mutated: %t\nnext:\n  %s\n", result.Mutated, result.NextAction)
}

func approvalToken(generatedAt time.Time, hash string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(generatedAt.Format(time.RFC3339Nano))) + "." + hash
}

func parseApprovalToken(token string) (time.Time, string, error) {
	encodedTime, hash, ok := strings.Cut(token, ".")
	if !ok || hash == "" {
		return time.Time{}, "", errors.New("malformed plan ID")
	}
	rawTime, err := base64.RawURLEncoding.DecodeString(encodedTime)
	if err != nil {
		return time.Time{}, "", errors.New("malformed plan timestamp")
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, string(rawTime))
	if err != nil {
		return time.Time{}, "", errors.New("invalid plan timestamp")
	}
	return generatedAt, hash, nil
}

func usage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: uawp <version|init|adapter|status|doctor|resume|acquire|release|sync|checkpoint|handoff|recover>")
	return exitUsage
}
