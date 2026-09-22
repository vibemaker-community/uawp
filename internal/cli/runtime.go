package cli

import (
	"crypto/rand"
	"io"
	"os"
	"time"
)

type cliSurface string

const (
	surfaceHuman   cliSurface = "human"
	surfaceMachine cliSurface = "machine"
)

type runtime struct {
	stdin         io.Reader
	stdout        io.Writer
	stderr        io.Writer
	getwd         func() (string, error)
	userConfigDir func() (string, error)
	now           func() time.Time
	random        io.Reader
	stdinTTY      bool
	stdoutTTY     bool
}

func defaultRuntime(stdout, stderr io.Writer) runtime {
	return runtime{
		stdin: os.Stdin, stdout: stdout, stderr: stderr,
		getwd: os.Getwd, userConfigDir: os.UserConfigDir,
		now: time.Now, random: rand.Reader,
		stdinTTY: isTerminalFile(os.Stdin), stdoutTTY: isTerminalWriter(stdout),
	}
}

func isTerminalFile(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func isTerminalWriter(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && isTerminalFile(file)
}

func selectSurface(format string, inputTTY, outputTTY, nonInteractive bool) cliSurface {
	if format == "text" && inputTTY && outputTTY && !nonInteractive {
		return surfaceHuman
	}
	return surfaceMachine
}

func runWithRuntime(args []string, rt runtime) int {
	if len(args) == 1 && args[0] == "version" {
		return runVersion(rt.stdout)
	}
	if len(args) == 0 {
		return usage(rt.stderr)
	}
	switch args[0] {
	case "identity":
		return runIdentity(args[1:], rt)
	case "session":
		return runSession(args[1:], rt)
	case "init":
		return runInitWithRuntime(args[1:], rt)
	case "adapter":
		return runAdapter(args[1:], rt.stdout, rt.stderr)
	case "status", "doctor":
		return runDiagnostic(args[0], args[1:], rt.stdout, rt.stderr)
	case "resume":
		return runResumeWithRuntime(args[1:], rt)
	case "acquire", "release", "sync", "checkpoint", "handoff", "recover":
		return runLifecycleMutationWithRuntime(args[0], args[1:], rt)
	case "upgrade", "repair", "uninstall":
		return runMaintenance(args[0], args[1:], rt.stdout, rt.stderr)
	case "transaction":
		return runTransaction(args[1:], rt.stdout, rt.stderr)
	default:
		return usage(rt.stderr)
	}
}

func runVersion(stdout io.Writer) int {
	_, _ = io.WriteString(stdout, "uawp dev\n")
	return exitOK
}
