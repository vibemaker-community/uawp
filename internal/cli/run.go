package cli

import (
	"fmt"
	"io"
)

// Run executes the UAWP command and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(stdout, "uawp dev")
		return 0
	}

	fmt.Fprintln(stderr, "usage: uawp <command>")
	return 2
}
