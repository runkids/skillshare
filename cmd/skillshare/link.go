package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/sourcelink"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// linkScope is what link and unlink need from the global or project config.
type linkScope struct {
	global     bool
	source     string
	targets    []string // skills paths of the active sync targets
	trashDir   string
	configPath string // oplog location
	follow     bool   // follow_source_links
	enable     func() error
	flags      string // appended to suggested commands
}

func loadLinkScope(args []string) (*linkScope, []string, error) {
	// Keep mode flags after -- as positional names or paths.
	var positional []string
	if i := slices.Index(args, "--"); i >= 0 {
		positional, args = args[i:], args[:i]
	}
	mode, rest, err := parseModeArgs(args, "--name")
	if err != nil {
		return nil, nil, err
	}
	rest = append(rest, positional...)
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot determine working directory: %w", err)
	}
	mode = resolveAutoMode(mode, cwd)
	applyModeLabel(mode)

	if mode == modeProject {
		if err := ensureProjectConfig(cwd); err != nil {
			return nil, nil, err
		}
		cfg, err := config.LoadProject(cwd)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load project config: %w", err)
		}
		resolved, _ := config.ResolveValidProjectTargets(cwd, cfg)
		return &linkScope{
			source:     cfg.EffectiveSkillsSource(cwd),
			targets:    config.SkillsTargetPaths(resolved),
			trashDir:   trash.ProjectTrashDir(cwd),
			configPath: config.ProjectConfigPath(cwd),
			follow:     cfg.FollowSourceLinks,
			enable: func() error {
				cfg.FollowSourceLinks = true
				return cfg.Save(cwd)
			},
			flags: " --project",
		}, rest, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &linkScope{
		global:     true,
		source:     cfg.EffectiveSkillsSource(),
		targets:    config.SkillsTargetPaths(cfg.Targets),
		trashDir:   trash.TrashDir(),
		configPath: config.ConfigPath(),
		follow:     cfg.FollowSourceLinks,
		enable: func() error {
			cfg.FollowSourceLinks = true
			return cfg.Save()
		},
	}, rest, nil
}

func cmdLink(args []string) error {
	start := time.Now()
	if wantsHelp(optionArgs(args)) {
		printLinkHelp()
		return nil
	}
	scope, rest, err := loadLinkScope(args)
	if err != nil {
		return err
	}

	var path, name string
	enable := false
	options := true
	for i := 0; i < len(rest); i++ {
		switch arg := rest[i]; {
		case options && arg == "--":
			options = false
		case options && arg == "--name":
			i++
			if i >= len(rest) {
				return fmt.Errorf("--name requires a value")
			}
			name = rest[i]
		case options && arg == "--enable":
			enable = true
		case options && (arg == "--help" || arg == "-h"):
			printLinkHelp()
			return nil
		case options && strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		case path != "":
			return fmt.Errorf("unexpected argument: %s", arg)
		default:
			path = arg
		}
	}
	if path == "" {
		printLinkHelp()
		return fmt.Errorf("path is required")
	}

	res, err := sourcelink.Create(scope.source, scope.targets, path, name)
	if err == nil && enable && !scope.follow {
		// Enabling is part of the request: a failed config write must not
		// leave a link discovery ignores behind a logged success.
		if err = scope.enable(); err != nil {
			_ = sourcelink.Discard(scope.source, filepath.Base(res.Path))
			err = fmt.Errorf("failed to enable follow_source_links: %w", err)
		}
	}
	logLinkOp(scope.configPath, "link", map[string]any{"name": name, "target": path}, start, statusFromErr(err), err)
	if err != nil {
		return fmt.Errorf("cannot link %s: %w", path, err)
	}
	linkName := filepath.Base(res.Path)
	fmt.Printf("%s Linked %s %s\n", ui.StyledMark(ui.MarkOK), linkName, ui.DimText("→ "+utils.FoldHomePath(res.Target)+" · "+res.Kind))
	if res.Warning != "" {
		ui.Warning("%s; skillshare update cannot pull it", res.Warning)
	}

	switch {
	case scope.follow:
	case enable:
		fmt.Printf("%s Set follow_source_links: true %s\n", ui.StyledMark(ui.MarkOK), ui.DimText(utils.FoldHomePath(scope.configPath)))
	default:
		ui.Note(unfollowedLinkHint(linkName, true))
		return nil
	}
	ui.Next("skillshare sync"+scope.flags, "link its skills into your targets")
	return nil
}

func cmdUnlink(args []string) error {
	start := time.Now()
	if wantsHelp(optionArgs(args)) {
		printUnlinkHelp()
		return nil
	}
	scope, rest, err := loadLinkScope(args)
	if err != nil {
		return err
	}

	var name string
	options := true
	for _, arg := range rest {
		switch {
		case options && arg == "--":
			options = false
		case options && (arg == "--help" || arg == "-h"):
			printUnlinkHelp()
			return nil
		case options && strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		case name != "":
			return fmt.Errorf("unexpected argument: %s", arg)
		default:
			name = arg
		}
	}
	if name == "" {
		printUnlinkHelp()
		return fmt.Errorf("link name is required")
	}

	opArgs := map[string]any{"name": name}
	err = sourcelink.Remove(scope.source, scope.trashDir, name)
	if err != nil {
		logLinkOp(scope.configPath, "unlink", opArgs, start, statusFromErr(err), err)
		return fmt.Errorf("cannot unlink %s: %w", name, err)
	}
	name = strings.TrimRight(name, `/\`)
	status := "ok"
	if scope.global {
		store, metadataErr := install.LoadMetadataWithMigration(scope.source, "")
		if metadataErr == nil && store.ForgetUnlinkedName(name) {
			metadataErr = store.Save(scope.source)
		}
		if metadataErr != nil {
			status = "partial"
			err = fmt.Errorf("Unlinked, but could not update metadata: %w", metadataErr)
			ui.Warning("%v", err)
		}
	}
	logLinkOp(scope.configPath, "unlink", opArgs, start, status, err)
	fmt.Printf("%s Unlinked %s %s\n", ui.StyledMark(ui.MarkOK), name, ui.DimText("→ trash, kept 7 days; its target is untouched"))
	ui.Next(
		"skillshare sync"+scope.flags, "remove its skills from your targets",
		"skillshare trash restore "+name+scope.flags, "undo",
	)
	return nil
}

func logLinkOp(cfgPath, op string, args map[string]any, start time.Time, status string, cmdErr error) {
	e := oplog.NewEntry(op, status, time.Since(start))
	e.Args = args
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func printLinkHelp() {
	printHelp("skillshare link <path> [options]", "Link a folder, such as your own skills checkout, directly under the skills source.\nWith follow_source_links on, skillshare lists and syncs the skills inside it.\nThe link is refused when the folder is the source or a parent of it, sits\ninside the source, overlaps a sync target, or is not a directory.\nOn Windows a junction is created, or a directory symlink when that fails.",
		helpGroup{title: "Options", rows: []helpRow{
			{"--name <name>", "Link name in the source (default: _<folder name>)"},
			{"--enable", "Also set follow_source_links: true"},
			{"-p, --project", "Use project-level config in current directory"},
			{"-g, --global", "Use global config (~/.config/skillshare)"},
			{"--", "End options; allow a path starting with -"},
		}},
		helpExamples(
			helpRow{"skillshare link ~/dev/my-skills", "Link as _my-skills"},
			helpRow{"skillshare link ~/dev/my-skills --enable", "Link and turn on following"},
			helpRow{"skillshare link ../team --name _team -p", "Link into the project source"},
			helpRow{"skillshare link -- -checkout", "Link a path starting with -"},
		),
	)
}

func printUnlinkHelp() {
	printHelp("skillshare unlink <name> [options]", "Remove a link created by skillshare link, or any first-level link in the\nskills source. Only the link moves to trash; the folder it points at is kept.\nIn global mode it also removes the tracked-repository record kept for that name.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-p, --project", "Use project-level config in current directory"},
			{"-g, --global", "Use global config (~/.config/skillshare)"},
			{"--", "End options; allow a name starting with -"},
		}},
		helpExamples(
			helpRow{"skillshare unlink _my-skills", "Remove the link"},
			helpRow{"skillshare unlink _team -p", "Remove a project source link"},
			helpRow{"skillshare unlink -- -local", "Remove a link whose name starts with -"},
		),
	)
}

// optionArgs returns the arguments before a -- terminator, so help is shown
// before any config is loaded or created and a name after -- is left alone.
func optionArgs(args []string) []string {
	if i := slices.Index(args, "--"); i >= 0 {
		return args[:i]
	}
	return args
}
