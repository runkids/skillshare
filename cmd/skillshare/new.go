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

	// Generate template
	template := generateSkillTemplate(skillName)

	if dryRun {
		ui.Header(ui.WithModeLabel("New Skill (dry-run)"))
		ui.Info("Would create: %s", skillDir)
		ui.Info("Would write: %s", skillFile)
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

// generateSkillTemplate creates the SKILL.md content
// Template follows Anthropic's skill-building guide best practices:
// - Description includes WHAT + WHEN (trigger phrases)
// - Progressive disclosure: frontmatter → body → references/
// - Step-based instructions, examples, and troubleshooting sections
func generateSkillTemplate(name string) string {
	// Convert hyphen-case to Title Case for heading
	title := toTitleCase(name)

	return fmt.Sprintf(`---
name: %s
description: >-
  Describe what this skill does. Use when user asks to
  "trigger phrase 1", "trigger phrase 2", or needs help
  with a specific task.
# ── Optional fields ──────────────────────────────────
# targets: []                         # e.g. [claude, cursor] — omit for all targets
# license: MIT
# allowed-tools: "Bash(python:*) WebFetch"
# metadata:
#   author: Your Name
#   version: 1.0.0
---

# %s

Brief overview of what this skill does and its value.

## When to Use

Use this skill when the user:
- Asks to "specific trigger phrase"
- Mentions specific keywords or file types
- Needs help with a particular task

Do NOT use this skill for:
- Unrelated tasks (clarify scope boundaries)

## Instructions

### Step 1: Gather Context

Explain what to check or collect before starting.

### Step 2: Execute

Describe the core action clearly and specifically.

### Step 3: Validate

Explain how to verify the result is correct.

## Examples

**Example:** Common scenario
User says: "Help me with <%s-related task>"
Actions:
1. First action
2. Second action
Result: Expected outcome

## Troubleshooting

**Error:** Common error message
**Cause:** Why it happens
**Solution:** How to fix it
`, name, title, name)
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

Options:
  --project, -p   Create in project (.skillshare/skills/)
  --global, -g    Create in global (~/.config/skillshare/skills/)
  --dry-run, -n   Preview without creating files
  --help, -h      Show this help

Arguments:
  <name>          Skill name (lowercase, hyphens allowed)

Examples:
  skillshare new my-skill              # Create a new skill
  skillshare new my-skill -p           # Create in project
  skillshare new my-skill --dry-run    # Preview first`)
}
