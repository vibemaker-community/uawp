package cli

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/uawp/uawp/internal/identity"
)

type sessionOutput struct {
	SchemaVersion string `json:"schemaVersion"`
	Command       string `json:"command"`
	SessionID     string `json:"sessionID"`
	Mutated       bool   `json:"mutated"`
	NextAction    string `json:"nextAction"`
}

func runSession(args []string, rt runtime) int {
	if len(args) == 0 || args[0] != "new" {
		fmt.Fprintln(rt.stderr, "usage: uawp session new")
		return exitUsage
	}
	set := flag.NewFlagSet("session new", flag.ContinueOnError)
	set.SetOutput(rt.stderr)
	format := set.String("format", "text", "output format")
	if err := set.Parse(args[1:]); err != nil || set.NArg() != 0 || (*format != "text" && *format != "json") {
		return exitUsage
	}
	value, err := identity.NewSessionID(rt.random)
	if err != nil {
		fmt.Fprintf(rt.stderr, "generate Session ID: %v\n", err)
		return exitInternal
	}
	result := sessionOutput{SchemaVersion: "1", Command: "session new", SessionID: value, NextAction: "Use this Session ID for one conversation or execution session."}
	if *format == "json" {
		encoder := json.NewEncoder(rt.stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
	} else {
		fmt.Fprintf(rt.stdout, "%s\n%s\n", result.SessionID, result.NextAction)
	}
	return exitOK
}
