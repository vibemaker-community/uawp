package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/uawp/uawp/internal/identity"
)

type identityOutput struct {
	SchemaVersion string             `json:"schemaVersion"`
	Command       string             `json:"command"`
	Profile       *identity.Profile  `json:"profile,omitempty"`
	Profiles      []identity.Profile `json:"profiles,omitempty"`
	Mutated       bool               `json:"mutated"`
	NextAction    string             `json:"nextAction"`
}

func loadIdentityRegistry(rt runtime) (identity.Registry, string, error) {
	config, err := rt.userConfigDir()
	if err != nil {
		return identity.Registry{}, "", err
	}
	path := identity.DefaultPath(config)
	value, err := identity.Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return identity.Registry{SchemaVersion: "1", Profiles: []identity.Profile{}}, path, nil
	}
	return value, path, err
}

func writeIdentityOutput(writer io.Writer, format string, result identityOutput) {
	if format == "json" {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
		return
	}
	if result.Profile != nil {
		fmt.Fprintf(writer, "%s (%s, %s)\n", result.Profile.DisplayName, result.Profile.ProfileID, result.Profile.WorkerID)
	}
	for _, profile := range result.Profiles {
		fmt.Fprintf(writer, "%s (%s, %s)\n", profile.DisplayName, profile.ProfileID, profile.WorkerID)
	}
	fmt.Fprintln(writer, result.NextAction)
}

func runIdentity(args []string, rt runtime) int {
	if len(args) == 0 {
		return identityUsage(rt.stderr)
	}
	command := args[0]
	operationArgs := args[1:]
	positional := ""
	if (command == "use" || command == "remove") && len(operationArgs) > 0 && operationArgs[0] != "" && operationArgs[0][0] != '-' {
		positional = operationArgs[0]
		operationArgs = operationArgs[1:]
	}
	set := flag.NewFlagSet("identity "+command, flag.ContinueOnError)
	set.SetOutput(rt.stderr)
	format := set.String("format", "text", "output format")
	name := set.String("name", "", "display name")
	if err := set.Parse(operationArgs); err != nil || (*format != "text" && *format != "json") {
		return exitUsage
	}
	registry, path, err := loadIdentityRegistry(rt)
	if err != nil {
		fmt.Fprintf(rt.stderr, "load identity registry: %v\n", err)
		return exitInternal
	}
	result := identityOutput{SchemaVersion: "1", Command: "identity " + command}
	switch command {
	case "create":
		if set.NArg() != 0 || *name == "" {
			return identityUsage(rt.stderr)
		}
		profile, createErr := identity.NewProfile(*name, rt.random)
		if createErr != nil {
			fmt.Fprintln(rt.stderr, createErr)
			return exitUsage
		}
		registry.Profiles = append(registry.Profiles, profile)
		if registry.DefaultProfileID == "" {
			registry.DefaultProfileID = profile.ProfileID
		}
		if err = identity.Save(path, registry); err != nil {
			fmt.Fprintf(rt.stderr, "save identity registry: %v\n", err)
			return exitInternal
		}
		result.Profile, result.Mutated, result.NextAction = &profile, true, "Use this profile with UAWP lifecycle commands."
	case "list":
		if set.NArg() != 0 || *name != "" {
			return identityUsage(rt.stderr)
		}
		result.Profiles, result.NextAction = registry.Profiles, "Choose a profile with uawp identity use PROFILE_ID."
	case "show":
		if set.NArg() != 0 || *name != "" {
			return identityUsage(rt.stderr)
		}
		profile, selectErr := registry.Selected("", true)
		if selectErr != nil {
			fmt.Fprintln(rt.stderr, selectErr)
			return exitInvalidState
		}
		result.Profile, result.NextAction = &profile, "This is the default local Worker profile."
	case "use":
		if set.NArg() != 0 || positional == "" || *name != "" {
			return identityUsage(rt.stderr)
		}
		profile, selectErr := registry.Selected(positional, false)
		if selectErr != nil {
			fmt.Fprintln(rt.stderr, selectErr)
			return exitUsage
		}
		registry.DefaultProfileID = profile.ProfileID
		if err = identity.Save(path, registry); err != nil {
			fmt.Fprintf(rt.stderr, "save identity registry: %v\n", err)
			return exitInternal
		}
		result.Profile, result.Mutated, result.NextAction = &profile, true, "Default profile updated."
	case "remove":
		if set.NArg() != 0 || positional == "" || *name != "" {
			return identityUsage(rt.stderr)
		}
		profileID := positional
		profile, selectErr := registry.Selected(profileID, false)
		if selectErr != nil {
			fmt.Fprintln(rt.stderr, selectErr)
			return exitUsage
		}
		bound := false
		for _, binding := range registry.Bindings {
			if binding.ProfileID == profileID {
				bound = true
			}
		}
		if bound && selectSurface(*format, rt.stdinTTY, rt.stdoutTTY, false) == surfaceHuman {
			fmt.Fprint(rt.stdout, "This profile has local Session bindings. Remove it? [y/N] ")
			approved, confirmErr := confirm(rt.stdin)
			if confirmErr != nil {
				return exitInternal
			}
			if !approved {
				result.Profile, result.NextAction = &profile, "Removal cancelled."
				writeIdentityOutput(rt.stdout, *format, result)
				return exitOK
			}
		}
		if bound {
			fmt.Fprintln(rt.stderr, "warning: removed local Session bindings; no Workspace was modified")
		}
		profiles := registry.Profiles[:0]
		for _, item := range registry.Profiles {
			if item.ProfileID != profileID {
				profiles = append(profiles, item)
			}
		}
		registry.Profiles = profiles
		bindings := registry.Bindings[:0]
		for _, item := range registry.Bindings {
			if item.ProfileID != profileID {
				bindings = append(bindings, item)
			}
		}
		registry.Bindings = bindings
		if registry.DefaultProfileID == profileID {
			registry.DefaultProfileID = ""
			if len(profiles) > 0 {
				registry.DefaultProfileID = profiles[0].ProfileID
			}
		}
		if err = identity.Save(path, registry); err != nil {
			fmt.Fprintf(rt.stderr, "save identity registry: %v\n", err)
			return exitInternal
		}
		result.Profile, result.Mutated, result.NextAction = &profile, true, "Profile removed; Workspaces were not modified."
	default:
		return identityUsage(rt.stderr)
	}
	writeIdentityOutput(rt.stdout, *format, result)
	return exitOK
}

func identityUsage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: uawp identity <create --name NAME|list|use PROFILE_ID|show|remove PROFILE_ID>")
	return exitUsage
}
