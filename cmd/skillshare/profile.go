package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
)

var profileValueFlags = []string{"--description", "--add-include", "--remove-include", "--add-exclude", "--remove-exclude"}

type profileOptions struct {
	action                                          string
	names                                           []string
	mode                                            runMode
	json, dryRun, noSync, none, discard, allowEmpty bool
	mutation                                        profile.Mutation
}

func parseProfileArgs(args []string) (profileOptions, error) {
	o := profileOptions{action: "list"}
	mode, rest, err := parseModeArgs(args, profileValueFlags...)
	if err != nil {
		return o, err
	}
	o.mode = mode
	var positionals []string
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if a == "--" {
			positionals = append(positionals, rest[i+1:]...)
			break
		}
		switch a {
		case "--help", "-h":
			o.action = "help"
			return o, nil
		case "--json":
			o.json = true
		case "--dry-run", "-n":
			o.dryRun = true
		case "--no-sync":
			o.noSync = true
		case "--none":
			o.none = true
		case "--discard-edits":
			o.discard = true
		case "--allow-empty":
			o.allowEmpty = true
		default:
			if slices.Contains(profileValueFlags, a) {
				if i+1 == len(rest) {
					return o, fmt.Errorf("%s requires a value", a)
				}
				i++
				value := rest[i]
				switch a {
				case "--description":
					o.mutation.Description = &value
				case "--add-include":
					o.mutation.AddInclude = append(o.mutation.AddInclude, value)
				case "--remove-include":
					o.mutation.RemoveInclude = append(o.mutation.RemoveInclude, value)
				case "--add-exclude":
					o.mutation.AddExclude = append(o.mutation.AddExclude, value)
				case "--remove-exclude":
					o.mutation.RemoveExclude = append(o.mutation.RemoveExclude, value)
				}
			} else if strings.HasPrefix(a, "-") {
				return o, fmt.Errorf("unknown profile flag %s", a)
			} else {
				positionals = append(positionals, a)
			}
		}
	}
	if len(positionals) > 0 {
		o.action, o.names = positionals[0], positionals[1:]
	}
	patterns := len(o.mutation.AddInclude)+len(o.mutation.RemoveInclude)+len(o.mutation.AddExclude)+len(o.mutation.RemoveExclude) > 0 || o.mutation.Description != nil
	if patterns && o.action != "create" && o.action != "edit" {
		return o, fmt.Errorf("definition flags require profile create or edit")
	}
	if o.action != "use" && (o.dryRun || o.noSync || o.none || o.discard || o.allowEmpty) {
		return o, fmt.Errorf("switch flags require profile use")
	}
	want := 1
	switch o.action {
	case "list":
		want = 0
	case "show", "create", "edit", "delete":
	case "rename":
		want = 2
	case "use":
		if len(o.names) > 1 || o.none && len(o.names) != 0 || o.noSync && o.discard {
			return o, fmt.Errorf("use takes one name or --none; --no-sync cannot discard edits")
		}
		return o, nil
	default:
		return o, fmt.Errorf("unknown profile subcommand %q", o.action)
	}
	if len(o.names) != want {
		return o, fmt.Errorf("profile %s requires %d name(s)", o.action, want)
	}
	return o, nil
}

func profileSkipsMaintenance(args []string) bool {
	// A use refusal can follow startup. Skip directory/config maintenance and
	// the network update check for every use, as well as reads and parse errors.
	o, err := parseProfileArgs(args)
	return err != nil || slices.Contains([]string{"help", "list", "show", "use"}, o.action)
}

func profileScope(o profileOptions) (profileapply.Scope, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return profileapply.Scope{}, err
	}
	mode := resolveAutoMode(o.mode, cwd)
	applyModeLabel(mode)
	scope := profileapply.Scope{}
	if mode == modeProject {
		scope.ProjectRoot = cwd
	}
	return scope, nil
}

func printProfileHelp() {
	printHelp("skillshare profile <command> [-g|-p]", "Named skill selections (separate from audit --profile).",
		helpGroup{title: "Commands", rows: []helpRow{
			{"list [--json]", "List definitions and local active state (default)"},
			{"show <name> [--json]", "Show patterns, selected skills and per-target counts"},
			{"create <name>", "Create an empty selection; add patterns with --add-include"},
			{"edit <name>", "Edit description and include/exclude patterns"},
			{"rename <old> <new>", "Rename a definition and its local active state"},
			{"delete <name>", "Delete an inactive definition"},
			{"use <name> | --none", "Record and safely sync eligible skill targets"},
		}},
		helpGroup{title: "Definition options", rows: []helpRow{
			{"--description <text>", "Description"}, {"--add-include <pattern>", "Append include (repeatable)"},
			{"--remove-include <pattern>", "Remove exact include (repeatable)"}, {"--add-exclude <pattern>", "Append exclude (repeatable)"},
			{"--remove-exclude <pattern>", "Remove exact exclude (repeatable)"},
		}},
		helpGroup{title: "Use options", rows: []helpRow{
			{"--dry-run, -n", "Preview without writes"}, {"--no-sync", "Record pending selection only"},
			{"--allow-empty", "Allow an empty selection over enabled source skills"},
			{"--discard-edits", "Discard listed edited copies only after verified snapshots"}, {"--json", "Machine-readable output"},
		}})
}
