package main

import (
	"fmt"
	"os"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/hub"
	"skillshare/internal/ui"
	appversion "skillshare/internal/version"
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
		if mode == modeAuto && projectConfigExists(cwd) {
			mode = modeProject
		} else if mode == modeAuto {
			mode = modeGlobal
		}
		applyModeLabel(mode)
		return cmdHubAdd(rest, mode, cwd)
	case "list", "ls":
		mode, _, err := parseModeArgs(subargs)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		if mode == modeAuto && projectConfigExists(cwd) {
			mode = modeProject
		} else if mode == modeAuto {
			mode = modeGlobal
		}
		applyModeLabel(mode)
		return cmdHubList(mode, cwd)
	case "remove", "rm":
		mode, rest, err := parseModeArgs(subargs)
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}
		if mode == modeAuto && projectConfigExists(cwd) {
			mode = modeProject
		} else if mode == modeAuto {
			mode = modeGlobal
		}
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
		if mode == modeAuto && projectConfigExists(cwd) {
			mode = modeProject
		} else if mode == modeAuto {
			mode = modeGlobal
		}
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
	if mode == modeAuto && projectConfigExists(cwd) {
		mode = modeProject
	} else if mode == modeAuto {
		mode = modeGlobal
	}

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

	// Show logo
	ui.Logo(appversion.Version)
	ui.StepStart("Building", "hub index")

	spinner := ui.StartTreeSpinner("Scanning source directory...", false)

	idx, err := hub.BuildIndex(sourcePath, full, auditSkills)
	if err != nil {
		spinner.Fail("Failed to build index")
		return err
	}

	if auditSkills {
		spinner.Success(fmt.Sprintf("Found %d skill(s), audit complete", len(idx.Skills)))
	} else {
		spinner.Success(fmt.Sprintf("Found %d skill(s)", len(idx.Skills)))
	}

	// Write to file
	writeSpinner := ui.StartTreeSpinner("Writing index...", true)

	if err := hub.WriteIndex(outputPath, idx); err != nil {
		writeSpinner.Fail("Failed to write index")
		return err
	}

	writeSpinner.Success(fmt.Sprintf("Wrote %s", outputPath))

	// Summary
	fmt.Println()
	switch {
	case full && auditSkills:
		ui.Info("Mode: full + audit (metadata and risk scores included)")
	case full:
		ui.Info("Mode: full (metadata included)")
	case auditSkills:
		ui.Info("Mode: audit (risk scores included)")
	default:
		ui.Info("Mode: minimal (name, description, source only)")
	}
	ui.Info("Skills: %d", len(idx.Skills))
	ui.Info("Output: %s", outputPath)

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
	return cfg.Source, nil
}

func printHubHelp() {
	fmt.Println(`Usage: skillshare hub <subcommand> [options]

Manage skill hubs — saved hub sources for search.

Subcommands:
  add <url>       Save a hub source (--label to set name)
  list            List saved hubs (* marks default)
  remove <label>  Remove a saved hub
  default [label] Show or set the default hub (--reset to clear)
  index           Build an index.json from source skills
  help            Show this help

Run 'skillshare hub <subcommand> --help' for details.`)
}

func printHubIndexHelp() {
	fmt.Println(`Usage: skillshare hub index [options]

Build an index.json file from installed skills. The generated index
can be used with 'skillshare search --hub' for private search.

Options:
  --source, -s <path>   Source directory to scan (default: auto-detect)
  --output, -o <path>   Output file path (default: <source>/skillshare-hub.json)
  --full                Include full metadata (flatName, type, version, etc.)
  --audit               Run security audit on each skill and include risk scores
  --project, -p         Use project mode (.skillshare/)
  --global, -g          Use global mode (~/.config/skillshare/)
  --help, -h            Show this help

Examples:
  skillshare hub index                           Build minimal index
  skillshare hub index --full                    Build with full metadata
  skillshare hub index --audit                   Build with risk scores
  skillshare hub index --full --audit            Full metadata + risk scores
  skillshare hub index -o /tmp/index.json        Custom output path
  skillshare hub index -s ~/my-skills            Custom source directory
  skillshare hub index -p                        Project mode`)
}
