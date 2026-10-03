package main

import "fmt"

func cmdExtras(args []string) error {
	if len(args) == 0 {
		printExtrasHelp()
		return nil
	}

	sub := args[0]
	rest := args[1:]

	// Shorthand operations on `extras <name>` must win over subcommand names so
	// valid extras named "list", "source", or "remove" remain operable.
	if hasFlag(args, "--add-target") {
		return cmdExtrasAddTarget(args)
	}
	if hasFlag(args, "--remove-target") {
		return cmdExtrasRemoveTarget(args)
	}
	if sub != "init" && (hasFlag(args, "--mode") || hasFlag(args, "--flatten") || hasFlag(args, "--no-flatten")) {
		return cmdExtrasMode(args)
	}

	switch sub {
	case "init":
		return cmdExtrasInit(rest)
	case "list", "ls":
		return cmdExtrasList(rest)
	case "remove", "rm":
		return cmdExtrasRemove(rest)
	case "collect":
		return cmdExtrasCollect(rest)
	case "source":
		return cmdExtrasSource(rest)
	case "--help", "-h":
		printExtrasHelp()
		return nil
	default:
		if len(rest) == 0 || hasFlag(rest, "--help") || hasFlag(rest, "-h") {
			printExtraHelp(sub)
			return nil
		}
		return fmt.Errorf("unknown extras subcommand: %s (run 'skillshare extras --help')", sub)
	}
}

func printExtrasHelp() {
	printHelp("skillshare extras <command> [options]",
		"Manage non-skill resources (rules, commands, prompts, etc.).",
		helpGroup{title: "Commands", rows: []helpRow{
			{"init <name>", "Create a new extra (a folder, or one file with --file [--as])"},
			{"list", "List all configured extras and sync status (interactive TUI)"},
			{"remove <name>", "Remove an extra resource type"},
			{"collect <name>", "Collect local files from a target into extras source"},
			{"source [path]", "Show or set the global extras_source directory"},
		}},
		helpGroup{title: "Change an existing extra", rows: []helpRow{
			{"<name> --mode <mode>", "Change sync mode"},
			{"<name> --flatten", "Enable flatten"},
			{"<name> --no-flatten", "Disable flatten"},
			{"<name> --add-target <path>", "Add a target"},
			{"<name> --remove-target <path>", "Remove a target"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-p, --project", "Use project-mode extras (.skillshare/)"},
			{"-g, --global", "Use global extras (~/.config/skillshare/)"},
			{"-h, --help", "Show this help"},
		}},
		helpNotes("Source directory, per extra",
			"1. Per-extra \"source\" field in config.yaml",
			"2. Global \"extras_source\" in config.yaml",
			"3. Default: <skills_source>/extras/<name>/",
		),
	)
}

func printExtraHelp(name string) {
	printHelp("skillshare extras "+name+" [options]", "",
		helpGroup{title: "Options", rows: []helpRow{
			{"--mode <mode>", "Change sync mode: merge, copy, symlink, or import (single-file extras)"},
			{"--target <path>", "Select a target for --mode"},
			{"--add-target <path>", "Add a target directory"},
			{"--as <filename>", "Target filename for a single-file --add-target"},
			{"--remove-target <path>", "Detach a target"},
			{"--prune", "Restore/remove managed files when detaching"},
			{"--flatten / --no-flatten", "Change directory-extra flattening"},
			{"-p, --project", "Use project mode"},
			{"-g, --global", "Use global mode"},
		}},
	)
}
