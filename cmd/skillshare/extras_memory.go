package main

import (
	"fmt"

	"skillshare/internal/memory"
)

type memoryOptions struct {
	command, path, from, version, search, mode string
	json                                       bool
}

func cmdExtrasMemory(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 || hasFlag(rest, "--help") || hasFlag(rest, "-h") {
		printMemoryHelp()
		return nil
	}
	opts := memoryOptions{command: rest[0]}
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case "--json":
			opts.json = true
		case "--from", "--version", "--search", "--update-mode":
			flag := rest[i]
			if i+1 == len(rest) {
				return fmt.Errorf("%s requires a value", flag)
			}
			i++
			switch flag {
			case "--from":
				opts.from = rest[i]
			case "--version":
				opts.version = rest[i]
			case "--search":
				opts.search = rest[i]
			case "--update-mode":
				opts.mode = rest[i]
			}
		default:
			if opts.path != "" || len(rest[i]) > 0 && rest[i][0] == '-' {
				return fmt.Errorf("unexpected argument: %s", rest[i])
			}
			opts.path = rest[i]
		}
	}
	switch opts.command {
	case "init", "list", "instructions":
		if opts.path != "" || opts.from != "" || opts.version != "" {
			return fmt.Errorf("%s does not accept a note or write options", opts.command)
		}
	case "show":
		if opts.path == "" || opts.from != "" || opts.version != "" {
			return fmt.Errorf("show requires a note path")
		}
	case "write":
		if opts.path == "" || opts.from == "" {
			return fmt.Errorf("write requires a note path and --from <file|->")
		}
	case "delete":
		if opts.path == "" || opts.version == "" || opts.from != "" {
			return fmt.Errorf("delete requires a note path and --version <hash>")
		}
	default:
		return fmt.Errorf("unknown memory command: %s", opts.command)
	}
	if opts.search != "" && opts.command != "list" {
		return fmt.Errorf("--search is only supported by list")
	}
	if opts.mode != "" && opts.command != "instructions" {
		return fmt.Errorf("--update-mode is only supported by instructions")
	}
	if opts.mode, err = memory.ParseMode(opts.mode); err != nil {
		return err
	}
	return runMemory(mode, opts)
}

func printMemoryHelp() {
	printHelp("skillshare extras memory <command> [options]", "Manage user-owned Markdown notes under extras/memory.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"init", "Create the memory extra with INDEX.md and LEARNED.md"},
			{"list [--search <text>]", "List notes, searching names and content"},
			{"show <note.md>", "Read a note; --json includes its version"},
			{"write <note.md> --from <file|-> [--version <hash>]", ""},
			{"", "Create a note or update the last read version"},
			{"instructions [--update-mode passive|active]", ""},
			{"", "Print guidance to add to your agent instructions"},
			{"delete <note.md> --version <hash>", ""},
			{"", "Back up and delete the last read version"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--json", "Emit JSON"},
			{"-g, --global", "Use global extras"},
			{"-p, --project", "Use project extras"},
		}},
		helpNotes("Notes",
			"An empty --version creates a new note. To update, use the version from show --json.",
			"Changes are backed up; a stale version is rejected. Native automatic memory is separate.",
			"Both modes read INDEX.md at the start of each task. --update-mode passive (default) points out facts worth keeping and saves them only when asked; active saves them here instead of the tool's own memory and proposes ones it is unsure of.",
		),
	)
}
