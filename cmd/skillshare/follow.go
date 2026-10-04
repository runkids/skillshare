package main

import (
	"fmt"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/ui"
)

type followOptions struct {
	name     string
	to       string
	local    bool
	keepLink bool
	json     bool
}

func cmdFollow(args []string) error   { return cmdFollowToggle(args, true) }
func cmdUnfollow(args []string) error { return cmdFollowToggle(args, false) }

func cmdFollowToggle(args []string, follow bool) error {
	start := time.Now()
	action := "unfollow"
	if follow {
		action = "follow"
	}

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	var opts followOptions
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch {
		case arg == "--help" || arg == "-h":
			printFollowHelp(action)
			return nil
		case arg == "--local":
			opts.local = true
		case arg == "--json":
			opts.json = true
		case arg == "--keep-link" && !follow:
			opts.keepLink = true
		case arg == "--to" && follow:
			if i+1 >= len(rest) {
				return fmt.Errorf("--to requires a directory")
			}
			i++
			opts.to = rest[i]
		case len(arg) > 0 && arg[0] == '-':
			return fmt.Errorf("unknown flag: %s", arg)
		case opts.name != "":
			return fmt.Errorf("%s takes one name", action)
		default:
			opts.name = arg
		}
	}
	if opts.name == "" {
		return fmt.Errorf("usage: skillshare %s <name>", action)
	}

	fail := func(err error) error {
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}
	if !sourcewalk.ValidEntryName(opts.name) {
		return fail(fmt.Errorf("%q is not a first-level entry name: no path separators, globs, or absolute paths", opts.name))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fail(fmt.Errorf("cannot determine working directory: %w", err))
	}
	mode = resolveAutoMode(mode, cwd)
	applyModeLabel(mode)
	fc, err := resolveFollowContext(mode, cwd)
	if err != nil {
		return fail(err)
	}

	var result any
	var changed, hint bool
	if follow {
		r, err := runFollow(fc, opts)
		if err != nil {
			return fail(err)
		}
		result, changed, hint = r, r.changed(), r.changed()
		if !opts.json {
			renderFollow(r)
		}
	} else {
		r, err := runUnfollow(fc, opts)
		if err != nil {
			return fail(err)
		}
		result, changed, hint = r, r.changed(), r.leftDiscovery()
		if !opts.json {
			renderUnfollow(r)
		}
	}

	if changed {
		e := oplog.NewEntry(action, "ok", time.Since(start))
		e.Args = map[string]any{"name": opts.name, "local": opts.local}
		if opts.to != "" {
			e.Args["to"] = opts.to
		}
		if opts.keepLink {
			e.Args["keep_link"] = true
		}
		oplog.Write(fc.cfgPath, oplog.OpsFile, e)
	}
	if opts.json {
		return writeJSON(result)
	}
	if hint {
		note := "apply the change"
		if !follow {
			note = "prune the entry's managed links"
		}
		ui.Next("skillshare sync", note)
	}
	return nil
}

// followContext is the resolved skills source of one follow or unfollow.
type followContext struct {
	source  string
	cfgPath string
	// snapshot classifies the declarations as they are on disk now.
	snapshot func() *sourcewalk.FollowSet
}

func resolveFollowContext(mode runMode, cwd string) (*followContext, error) {
	if mode == modeProject {
		runtime, err := loadProjectRuntime(cwd)
		if err != nil {
			return nil, fmt.Errorf("failed to load project config: %w", err)
		}
		return &followContext{
			source:   runtime.sourcePath,
			cfgPath:  config.ProjectConfigPath(cwd),
			snapshot: func() *sourcewalk.FollowSet { return skillFollowSet(runtime.sourcePath, runtime.targets, cwd) },
		}, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &followContext{
		source:   cfg.EffectiveSkillsSource(),
		cfgPath:  config.ConfigPath(),
		snapshot: func() *sourcewalk.FollowSet { return globalSkillFollowSet(cfg) },
	}, nil
}

func printFollowHelp(action string) {
	if action == "follow" {
		printHelp("skillshare follow <name> [options]",
			"Declare a first-level link in the skills source so discovery follows it.\n"+
				"Adds <name> to .skillfollow and, in a Git source, the anchored ignore line.",
			helpGroup{title: "Arguments", rows: []helpRow{
				{"<name>", "First-level entry in the source (\"_repo\" for a tracked repo, otherwise a group)"},
			}},
			helpGroup{title: "Options", rows: []helpRow{
				{"--to <dir>", "Create the link to <dir> first (a junction on Windows)"},
				{"--local", "Write .skillfollow.local and ignore it in Git"},
				{"-p, --project", "Use the project skills source"},
				{"-g, --global", "Use the global skills source"},
				{"--json", "Output as JSON"},
			}},
			helpNotes("See also", "skillshare unfollow", "skillshare doctor"),
		)
		return
	}
	printHelp("skillshare unfollow <name> [options]",
		"Stop following a first-level link: remove <name> from every declaration file\n"+
			"and remove the link itself. The link's target is never touched.",
		helpGroup{title: "Arguments", rows: []helpRow{
			{"<name>", "Declared first-level entry"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--local", "Remove the name from .skillfollow.local only"},
			{"--keep-link", "Keep the link and its ignore line"},
			{"-p, --project", "Use the project skills source"},
			{"-g, --global", "Use the global skills source"},
			{"--json", "Output as JSON"},
		}},
		helpNotes("See also", "skillshare follow"),
	)
}
