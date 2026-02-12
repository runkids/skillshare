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

	switch args[0] {
	case "index":
		return cmdHubIndex(args[1:])
	case "help", "-h", "--help":
		printHubHelp()
		return nil
	default:
		return fmt.Errorf("unknown hub subcommand: %s\nRun 'skillshare hub help' for usage", args[0])
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

	idx, err := hub.BuildIndex(sourcePath, full)
	if err != nil {
		spinner.Fail("Failed to build index")
		return err
	}

	spinner.Success(fmt.Sprintf("Found %d skill(s)", len(idx.Skills)))

	// Write to file
	writeSpinner := ui.StartTreeSpinner("Writing index...", true)

	if err := hub.WriteIndex(outputPath, idx); err != nil {
		writeSpinner.Fail("Failed to write index")
		return err
	}

	writeSpinner.Success(fmt.Sprintf("Wrote %s", outputPath))

	// Summary
	fmt.Println()
	if full {
		ui.Info("Mode: full (metadata included)")
	} else {
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

Manage private skill hubs.

Subcommands:
  index     Build an index.json from source skills
  help      Show this help

Run 'skillshare hub <subcommand> --help' for details.`)
}

func printHubIndexHelp() {
	fmt.Println(`Usage: skillshare hub index [options]

Build an index.json file from installed skills. The generated index
can be used with 'skillshare search --index-url' for private search.

Options:
  --source, -s <path>   Source directory to scan (default: auto-detect)
  --output, -o <path>   Output file path (default: <source>/skillshare-hub.json)
  --full                Include full metadata (flatName, type, version, etc.)
  --project, -p         Use project mode (.skillshare/)
  --global, -g          Use global mode (~/.config/skillshare/)
  --help, -h            Show this help

Examples:
  skillshare hub index                           Build minimal index
  skillshare hub index --full                    Build with full metadata
  skillshare hub index -o /tmp/index.json        Custom output path
  skillshare hub index -s ~/my-skills            Custom source directory
  skillshare hub index -p                        Project mode`)
}
