// Package buildinfo exposes deterministic metadata injected at link time.
package buildinfo

import "strings"

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = ""
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
}

func Current() Info {
	version := strings.TrimPrefix(Version, "v")
	if version == "" {
		version = "dev"
	}
	commit := Commit
	if commit == "" {
		commit = "unknown"
	}
	return Info{Version: version, Commit: commit, BuiltAt: BuiltAt}
}
