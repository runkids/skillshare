package main

import "fmt"

func cmdExtras(args []string) error {
	if len(args) == 0 {
		printExtrasHelp()
		return nil
	}

	sub := args[0]
	rest := args[1:]

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
	case "mode":
		return cmdExtrasMode(rest)
	case "--help", "-h":
		printExtrasHelp()
		return nil
	default:
		// Shorthand: skillshare extras <name> --mode/--flatten/--no-flatten
		if hasFlag(args, "--mode") || hasFlag(args, "--flatten") || hasFlag(args, "--no-flatten") {
			return cmdExtrasMode(args)
		}
		return fmt.Errorf("unknown extras subcommand: %s (run 'skillshare extras --help')", sub)
	}
}

func printExtrasHelp() {
	fmt.Println(`Usage: skillshare extras <command> [options]

Manage non-skill resources (rules, commands, prompts, etc.).

Commands:
  init <name>        Create a new extra resource type
  list               List all configured extras and sync status (interactive TUI)
  remove <name>      Remove an extra resource type
  collect <name>     Collect local files from a target into extras source
  source [path]      Show or set the global extras_source directory
  mode <name>        Change sync mode or flatten setting of an extra's target

Shortcuts:
  skillshare extras <name> --mode <mode>       Change sync mode
  skillshare extras <name> --flatten           Enable flatten
  skillshare extras <name> --no-flatten        Disable flatten

Options:
  --project, -p      Use project-mode extras (.skillshare/)
  --global, -g       Use global extras (~/.config/skillshare/)
  --help, -h         Show this help

Source directory resolution (per extra):
  1. Per-extra "source" field in config.yaml
  2. Global "extras_source" in config.yaml
  3. Default: <skills_source>/extras/<name>/`)
}
