package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

func cmdNew(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	var skillName string
	var dryRun bool
	var patternFlag string

	// Parse arguments
	i := 0
	for i < len(rest) {
		arg := rest[i]
		switch {
		case arg == "--dry-run" || arg == "-n":
			dryRun = true
		case arg == "--help" || arg == "-h":
			printNewHelp()
			return nil
		case arg == "--pattern" || arg == "-P":
			i++
			if i >= len(rest) {
				return fmt.Errorf("--pattern requires a value")
			}
			patternFlag = rest[i]
			if findPattern(patternFlag) == nil {
				validNames := make([]string, len(skillPatterns))
				for j, p := range skillPatterns {
					validNames[j] = p.Name
				}
				return fmt.Errorf("unknown pattern: %s (valid: %s)", patternFlag, strings.Join(validNames, ", "))
			}
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if skillName != "" {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
			skillName = arg
		}
		i++
	}

	if skillName == "" {
		printNewHelp()
		return fmt.Errorf("skill name is required")
	}

	// Validate skill name
	if !isValidSkillName(skillName) {
		return fmt.Errorf("invalid skill name: use lowercase letters, numbers, and hyphens only")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	// Resolve source directory
	var sourceDir string
	if mode == modeProject {
		sourceDir = filepath.Join(cwd, ".skillshare", "skills")
	} else {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w (run 'skillshare init' first)", err)
		}
		sourceDir = cfg.Source
	}

	// Create skill directory path
	skillDir := filepath.Join(sourceDir, skillName)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("skill '%s' already exists at %s", skillName, skillDir)
	}

	// Determine pattern via wizard (Esc = back to previous step)
	selectedPattern := patternFlag
	var selectedCategory string
	createDirs := patternFlag != "" && patternFlag != "none"

	isTTY := runningInInteractiveTTY()

	if selectedPattern == "" && isTTY {
		selectedPattern, selectedCategory, createDirs = runNewWizard()
		if selectedPattern == "" {
			return nil // cancelled at pattern step
		}
	} else if selectedPattern != "" && selectedPattern != "none" && isTTY {
		// -P given but still ask category interactively
		c, err := promptCategory()
		if err != nil {
			return fmt.Errorf("category selection: %w", err)
		}
		selectedCategory = c
	}

	if selectedPattern == "" {
		selectedPattern = "none"
	}

	pattern := findPattern(selectedPattern)

	template := generatePatternTemplate(skillName, selectedPattern, selectedCategory)

	if dryRun {
		ui.Header(ui.WithModeLabel("New Skill (dry-run)"))
		ui.Info("Would create: %s", skillDir)
		ui.Info("Would write: %s", skillFile)
		if createDirs && pattern != nil && len(pattern.ScaffoldDirs) > 0 {
			for _, dir := range pattern.ScaffoldDirs {
				ui.Info("Would create: %s/", filepath.Join(skillDir, dir))
			}
		}
		fmt.Println()
		ui.Info("Template preview:")
		fmt.Println(template)
		return nil
	}

	// Create directory
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write SKILL.md
	if err := os.WriteFile(skillFile, []byte(template), 0644); err != nil {
		// Clean up directory on failure
		os.RemoveAll(skillDir)
		return fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	if createDirs && pattern != nil {
		for _, dir := range pattern.ScaffoldDirs {
			dirPath := filepath.Join(skillDir, dir)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", dir, err)
			}
			gitkeep := filepath.Join(dirPath, ".gitkeep")
			if err := os.WriteFile(gitkeep, []byte{}, 0644); err != nil {
				return fmt.Errorf("failed to create %s/.gitkeep: %w", dir, err)
			}
		}
	}

	ui.Header(ui.WithModeLabel("New Skill Created"))
	ui.Success("Created: %s", skillFile)
	fmt.Println()
	ui.Info("Next steps:")
	fmt.Printf("  1. Edit %s\n", skillFile)
	if mode == modeProject {
		fmt.Println("  2. Run 'skillshare sync' to deploy")
	} else {
		fmt.Println("  2. Run 'skillshare sync' to deploy")
	}

	return nil
}

// isValidSkillName validates skill name format
func isValidSkillName(name string) bool {
	// Allow lowercase letters, numbers, hyphens, and underscores
	// Must start with a letter or underscore
	matched, _ := regexp.MatchString(`^[a-z_][a-z0-9_-]*$`, name)
	return matched
}

func printNewHelp() {
	fmt.Println(`Usage: skillshare new <name> [options]

Create a new skill with a SKILL.md template.

Options:
  --pattern, -P <name>  Use a design pattern (tool-wrapper, generator, reviewer, inversion, pipeline, none)
  --project, -p         Create in project (.skillshare/skills/)
  --global, -g          Create in global (~/.config/skillshare/skills/)
  --dry-run, -n         Preview without creating files
  --help, -h            Show this help

Arguments:
  <name>          Skill name (lowercase, hyphens allowed)

Examples:
  skillshare new my-skill                  # Create with interactive pattern selection
  skillshare new my-skill -P reviewer      # Use reviewer pattern directly
  skillshare new my-skill -P none          # Plain template, no pattern
  skillshare new my-skill -p               # Create in project
  skillshare new my-skill --dry-run        # Preview first`)
}
