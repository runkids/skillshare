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
	var skillName string
	var dryRun bool

	// Parse arguments
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--dry-run" || arg == "-n":
			dryRun = true
		case arg == "--help" || arg == "-h":
			printNewHelp()
			return nil
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

	// Load config to get source directory
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'skillshare init' first)", err)
	}

	// Create skill directory path
	skillDir := filepath.Join(cfg.Source, skillName)
	skillFile := filepath.Join(skillDir, "SKILL.md")

	// Check if skill already exists
	if _, err := os.Stat(skillDir); err == nil {
		return fmt.Errorf("skill '%s' already exists at %s", skillName, skillDir)
	}

	// Generate template
	template := generateSkillTemplate(skillName)

	if dryRun {
		ui.Header("New Skill (dry-run)")
		fmt.Println(strings.Repeat("-", 45))
		ui.Info("Would create: %s", skillDir)
		ui.Info("Would write: %s", skillFile)
		fmt.Println()
		ui.Info("Template preview:")
		fmt.Println(strings.Repeat("-", 45))
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

	ui.Header("New Skill Created")
	fmt.Println(strings.Repeat("-", 45))
	ui.Success("Created: %s", skillFile)
	fmt.Println()
	ui.Info("Next steps:")
	fmt.Printf("  1. Edit %s\n", skillFile)
	fmt.Println("  2. Run 'skillshare sync' to deploy")

	return nil
}

// isValidSkillName validates skill name format
func isValidSkillName(name string) bool {
	// Allow lowercase letters, numbers, hyphens, and underscores
	// Must start with a letter or underscore
	matched, _ := regexp.MatchString(`^[a-z_][a-z0-9_-]*$`, name)
	return matched
}

// generateSkillTemplate creates the SKILL.md content
func generateSkillTemplate(name string) string {
	// Convert hyphen-case to Title Case for heading
	title := toTitleCase(name)

	return fmt.Sprintf(`---
name: %s
description: Brief description of what this skill does
---

# %s

Instructions for the agent when this skill is activated.

## When to Use

Describe when this skill should be used.

## Instructions

1. First step
2. Second step
3. Additional steps as needed
`, name, title)
}

// toTitleCase converts kebab-case to Title Case
func toTitleCase(s string) string {
	words := strings.Split(s, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func printNewHelp() {
	fmt.Println(`Usage: skillshare new <name> [options]

Create a new skill with a SKILL.md template.

The skill will be created in your source directory:
  ~/.config/skillshare/skills/<name>/SKILL.md

Arguments:
  <name>          Skill name (lowercase, hyphens allowed)

Options:
  --dry-run, -n   Preview without creating files
  --help, -h      Show this help

Examples:
  skillshare new my-skill              # Create a new skill
  skillshare new my-skill --dry-run    # Preview first
  skillshare new code-review           # Another example`)
}
