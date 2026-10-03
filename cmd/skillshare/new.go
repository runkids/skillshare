package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/skill"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
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

	mode = resolveAutoMode(mode, cwd)

	applyModeLabel(mode)

	// Resolve source directory
	var sourceDir string
	if mode == modeProject {
		projectCfg, loadErr := config.LoadProject(cwd)
		if loadErr != nil {
			return fmt.Errorf("failed to load project config: %w", loadErr)
		}
		sourceDir = projectCfg.EffectiveSkillsSource(cwd)
	} else {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w (run 'skillshare init' first)", err)
		}
		sourceDir = cfg.EffectiveSkillsSource()
	}

	// Create skill directory path
	skillDir := filepath.Join(sourceDir, skillName)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("skill '%s' already exists at %s", skillName, skillDir)
	}

	// Determine pattern via wizard (esc cancels)
	selectedPattern := patternFlag
	var selectedCategory string
	createDirs := patternFlag != "" && patternFlag != "none"

	isTTY := runningInInteractiveTTY()

	if selectedPattern == "" && isTTY {
		selectedPattern, selectedCategory, createDirs = runNewWizard()
		if selectedPattern == "" {
			ui.Cancelled("created")
			return nil
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

	var scaffold []string
	if createDirs && pattern != nil {
		scaffold = pattern.ScaffoldDirs
	}

	if dryRun {
		ui.Row(ui.MarkNone, "Would create", utils.FoldHomePath(skillFile), ui.RowWidth("Would create"))
		printScaffoldDirs(scaffold)
		ui.Section("Preview")
		fmt.Println(strings.TrimRight(template, "\n"))
		fmt.Println()
		ui.DryRun()
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

	if len(scaffold) > 0 {
		for _, dir := range scaffold {
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

	ui.Row(ui.MarkOK, "Created", utils.FoldHomePath(skillFile), ui.RowWidth("Created"))
	printScaffoldDirs(scaffold)
	ui.Next("skillshare sync", "link it into your targets once you've edited it")

	return nil
}

// printScaffoldDirs lists the folders a pattern adds next to SKILL.md.
func printScaffoldDirs(dirs []string) {
	if len(dirs) == 0 {
		return
	}
	names := make([]string, len(dirs))
	for i, d := range dirs {
		names[i] = d + "/"
	}
	ui.Note("with " + strings.Join(names, ", "))
}

// isValidSkillName validates skill name format
func isValidSkillName(name string) bool {
	return skill.ValidNameRe.MatchString(name)
}

func printNewHelp() {
	printHelp("skillshare new <name> [options]", "Create a new skill with a SKILL.md template.",
		helpGroup{title: "Options", rows: []helpRow{
			{"-P, --pattern <name>", "Use a design pattern (tool-wrapper, generator, reviewer, inversion, pipeline, none)"},
			{"-p, --project", "Create in project (.skillshare/skills/)"},
			{"-g, --global", "Create in global (~/.config/skillshare/skills/)"},
			{"-n, --dry-run", "Preview without creating files"},
		}},
		helpGroup{title: "Arguments", rows: []helpRow{
			{"<name>", "Skill name (lowercase, hyphens allowed)"},
		}},
		helpExamples(
			helpRow{"skillshare new my-skill", "Create with interactive pattern selection"},
			helpRow{"skillshare new my-skill -P reviewer", "Use reviewer pattern directly"},
			helpRow{"skillshare new my-skill -P none", "Plain template, no pattern"},
			helpRow{"skillshare new my-skill -p", "Create in project"},
			helpRow{"skillshare new my-skill --dry-run", "Preview first"},
		),
	)
}
