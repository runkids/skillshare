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
	case "memory":
		return cmdExtrasMemory(rest)
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
	fmt.Println(`Usage: skillshare extras <command> [options]

Manage non-skill resources (rules, commands, prompts, etc.).

Commands:
  init <name>        Create a new extra (a folder, or one file with --file [--as])
  list               List all configured extras and sync status (interactive TUI)
  remove <name>      Remove an extra resource type
  collect <name>     Collect local files from a target into extras source
  source [path]      Show or set the global extras_source directory
  memory <command>   Manage shared Markdown memory notes

Operating on an existing extra (flags on 'extras <name>'):
  skillshare extras <name> --mode <mode>           Change sync mode
  skillshare extras <name> --flatten               Enable flatten
  skillshare extras <name> --no-flatten            Disable flatten
  skillshare extras <name> --add-target <path>     Add a target
  skillshare extras <name> --remove-target <path>  Remove a target

Options:
  --project, -p      Use project-mode extras (.skillshare/)
  --global, -g       Use global extras (~/.config/skillshare/)
  --help, -h         Show this help

Source directory resolution (per extra):
  1. Per-extra "source" field in config.yaml
  2. Global "extras_source" in config.yaml
  3. Default: <skills_source>/extras/<name>/`)
}

func printExtraHelp(name string) {
	fmt.Printf(`Usage: skillshare extras %s [options]

Options:
  --mode <mode>             Change sync mode: merge, copy, symlink, or import (single-file extras)
  --target <path>           Select a target for --mode
  --add-target <path>       Add a target directory
  --as <filename>           Target filename for a single-file --add-target
  --remove-target <path>    Detach a target
  --prune                  Restore/remove managed files when detaching
  --flatten / --no-flatten  Change directory-extra flattening
  --project, -p            Use project mode
  --global, -g             Use global mode
  --help, -h               Show this help
`, name)
}
