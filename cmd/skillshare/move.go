package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// moveOptions holds the parsed arguments of `skillshare move`.
type moveOptions struct {
	names  []string // skills or folders; at least one
	dest   string   // last positional, relative to the skills source
	force  bool
	dryRun bool
	json   bool
}

// parseMoveArgs parses the arguments left after the mode flags. The second
// result asks for the help page.
func parseMoveArgs(args []string) (*moveOptions, bool, error) {
	opts := &moveOptions{}
	var positional []string
	for i, arg := range args {
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		switch {
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--json":
			opts.json = true
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-") && arg != "-":
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) < 2 {
		return nil, true, fmt.Errorf("usage: skillshare move <skill|folder>... <dest-folder>")
	}
	opts.names, opts.dest = positional[:len(positional)-1], positional[len(positional)-1]
	return opts, false, nil
}

func cmdMove(args []string) error {
	start := time.Now()
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	opts, showHelp, err := parseMoveArgs(rest)
	if showHelp {
		printMoveHelp()
		return err
	}
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}
	mode = resolveAutoMode(mode, cwd)
	applyModeLabel(mode)

	// Auto-initializing a project prints; --json stdout must stay the payload.
	quiet := newJSONUISuppressor(opts.json)
	var m *moveMode
	if mode == modeProject {
		m, err = projectMoveMode(cwd)
	} else {
		m, err = globalMoveMode()
	}
	quiet.Flush()
	if err != nil {
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}
	return runMove(opts, m, start)
}

func printMoveHelp() {
	printHelp("skillshare move <skill|folder>... <dest-folder> [options]",
		"Move installed skills, or whole folders, to another folder of the source.\nTheir install records, audit acceptances and (project mode) lockfile pin move with them,\nso nothing is downloaded again. Run 'skillshare sync' afterwards to rename the Target links.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"<skill|folder>", "Skill path, flat name or unique base name, or a folder of the source"},
			{"<dest-folder>", "Folder relative to the source; created when missing, \".\" for the root"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"-n, --dry-run", "Preview without making changes"},
			{"-f, --force", "Move even when a Target would get two skills with one name"},
			{"--json", "Machine-readable output"},
			{"-p, --project", "Use project-level config in current directory"},
			{"-g, --global", "Use global config (~/.config/skillshare)"},
		}},
		helpExamples(
			helpRow{"skillshare move react-best-practices frontend", "Move one skill into frontend/"},
			helpRow{"skillshare move a b c grp", "Move several skills"},
			helpRow{"skillshare move frontend archive", "Move a folder: it becomes archive/frontend"},
			helpRow{"skillshare move grp/demo .", "Move a skill back to the source root"},
			helpRow{"skillshare move demo grp -n", "Preview the move"},
		),
		helpNotes("Not moved", "Skills of a tracked repo (_name) and anything at or below a followed source link.", "A folder holding a tracked repo is refused whole; nothing in it moves."),
	)
}
