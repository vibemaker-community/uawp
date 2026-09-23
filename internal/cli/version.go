package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/vibemaker-community/uawp/internal/buildinfo"
)

func runVersion(args []string, stdout, stderr io.Writer) int {
	set := flag.NewFlagSet("version", flag.ContinueOnError)
	set.SetOutput(stderr)
	format := set.String("format", "text", "output format: text or json")
	if err := set.Parse(args); err != nil || set.NArg() != 0 {
		return exitUsage
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "unsupported format %q; use text or json\n", *format)
		return exitUsage
	}

	info := buildinfo.Current()
	if *format == "json" {
		if err := json.NewEncoder(stdout).Encode(info); err != nil {
			fmt.Fprintf(stderr, "write version: %v\n", err)
			return exitInternal
		}
		return exitOK
	}

	builtAt := info.BuiltAt
	if builtAt == "" {
		builtAt = "unknown"
	}
	if _, err := fmt.Fprintf(stdout, "uawp %s\ncommit: %s\nbuilt at: %s\n", info.Version, info.Commit, builtAt); err != nil {
		fmt.Fprintf(stderr, "write version: %v\n", err)
		return exitInternal
	}
	return exitOK
}
