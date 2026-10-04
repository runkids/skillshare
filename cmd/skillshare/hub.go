package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/hub"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdHub(args []string) error {
	if len(args) == 0 {
		printHubHelp()
		return nil
	}

	subcmd := args[0]
	subargs := args[1:]

	// For subcommands that support --project/--global, parse mode
	switch subcmd {
	case "index":
		return cmdHubIndex(subargs)
	case "add":
		mode, rest, err := parseModeArgs(subargs)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		mode = resolveAutoMode(mode, cwd)
		applyModeLabel(mode)
		return cmdHubAdd(rest, mode, cwd)
	case "list", "ls":
		if wantsHelp(subargs) {
			printHubHelp()
			return nil
		}
		mode, _, err := parseModeArgs(subargs)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		mode = resolveAutoMode(mode, cwd)
		applyModeLabel(mode)
		return cmdHubList(mode, cwd)
	case "remove", "rm":
		if wantsHelp(subargs) {
			printHubHelp()
			return nil
		}
		mode, rest, err := parseModeArgs(subargs)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		mode = resolveAutoMode(mode, cwd)
		applyModeLabel(mode)
		return cmdHubRemove(rest, mode, cwd)
	case "default":
		mode, rest, err := parseModeArgs(subargs)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		mode = resolveAutoMode(mode, cwd)
		applyModeLabel(mode)
		return cmdHubDefault(rest, mode, cwd)
	case "help", "-h", "--help":
		printHubHelp()
		return nil
	default:
		return fmt.Errorf("unknown hub subcommand: %s\nRun 'skillshare hub help' for usage", subcmd)
	}
}

func cmdHubIndex(args []string) error {
	// Parse mode flags first
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	// Auto-detect mode
	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	var sourcePath string
	var outputPath string
	var full bool
	var auditSkills bool

	// Parse remaining arguments
	i := 0
	for i < len(rest) {
		arg := rest[i]
		key, val, hasEq := strings.Cut(arg, "=")
		switch {
		case key == "--source" || key == "-s":
			if hasEq {
				sourcePath = strings.TrimSpace(val)
			} else if i+1 >= len(rest) {
				return fmt.Errorf("--source requires a value")
			} else {
				i++
				sourcePath = strings.TrimSpace(rest[i])
			}
		case key == "--output" || key == "-o":
			if hasEq {
				outputPath = strings.TrimSpace(val)
			} else if i+1 >= len(rest) {
				return fmt.Errorf("--output requires a value")
			} else {
				i++
				outputPath = strings.TrimSpace(rest[i])
			}
		case key == "--full":
			full = true
		case key == "--audit":
			auditSkills = true
		case key == "--help" || key == "-h":
			printHubIndexHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			return fmt.Errorf("unexpected argument: %s", arg)
		}
		i++
	}

	// Resolve source path
	if sourcePath == "" {
		resolved, err := resolveSourcePath(mode, cwd)
		if err != nil {
			return err
		}
		sourcePath = resolved
	}

	// Resolve output path
	if outputPath == "" {
		outputPath = sourcePath + "/skillshare-hub.json"
	}

	start := time.Now()
	sp := ui.StartSpinner("Scanning source directory...")
	follow := configuredSkillFollowSet(sourcePath)
	if mode == modeProject {
		rt, err := loadProjectRuntime(cwd)
		if err != nil {
			return err
		}
		follow = skillFollowSet(sourcePath, rt.targets, cwd)
	}
	idx, err := hub.BuildIndexWithOptions(sourcePath, full, auditSkills, ssync.DiscoveryOptions{Follow: follow})
	sp.Stop()
	if err != nil {
		return fmt.Errorf("failed to build index: %w", err)
	}
	if err := hub.WriteIndex(outputPath, idx); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	var contents string
	switch {
	case full && auditSkills:
		contents = "full + audit (metadata and risk scores included)"
	case full:
		contents = "full (metadata included)"
	case auditSkills:
		contents = "audit (risk scores included)"
	default:
		contents = "minimal (name, description, source only)"
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Wrote %s with %s", shortenPath(outputPath), plural(len(idx.Skills), "skill")), time.Since(start))
	ui.Note(contents)

	return nil
}

// resolveSourcePath determines the source directory based on mode.
func resolveSourcePath(mode runMode, cwd string) (string, error) {
	if mode == modeProject {
		rt, err := loadProjectRuntime(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to load project config: %w", err)
		}
		return rt.sourcePath, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return cfg.EffectiveSkillsSource(), nil
}

func printHubHelp() {
	printHelp("skillshare hub <subcommand> [options]", "Manage skill hubs — saved hub sources for search.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"add <url>", "Save a hub source (--label to set name)"},
			{"list", "List saved hubs (* marks default)"},
			{"remove <label>", "Remove a saved hub"},
			{"default [label]", "Show or set the default hub (--reset to clear)"},
			{"index", "Build an index.json from source skills"},
		}},
		helpNotes("Notes",
			"Run 'skillshare hub <subcommand> --help' for details.",
		),
	)
}

func printHubIndexHelp() {
	printHelp("skillshare hub index [options]", "Build an index.json file from installed skills. The generated index\ncan be used with 'skillshare search --hub' for private search.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-s, --source <path>", "Source directory to scan (default: auto-detect)"},
			{"-o, --output <path>", "Output file path (default: <source>/skillshare-hub.json)"},
			{"--full", "Include full metadata (flatName, type, version, etc.)"},
			{"--audit", "Run security audit on each skill and include risk scores"},
			{"-p, --project", "Use project mode (.skillshare/)"},
			{"-g, --global", "Use global mode (~/.config/skillshare/)"},
		}},
		helpExamples(
			helpRow{"skillshare hub index", "Build minimal index"},
			helpRow{"skillshare hub index --full", "Build with full metadata"},
			helpRow{"skillshare hub index --audit", "Build with risk scores"},
			helpRow{"skillshare hub index --full --audit", "Full metadata + risk scores"},
			helpRow{"skillshare hub index -o /tmp/index.json", "Custom output path"},
			helpRow{"skillshare hub index -s ~/my-skills", "Custom source directory"},
			helpRow{"skillshare hub index -p", "Project mode"},
		),
	)
}
