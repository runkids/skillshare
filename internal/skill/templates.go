package skill

import (
	"fmt"
	"regexp"
	"strings"
)

// Pattern defines a structural design pattern for skills.
type Pattern struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	ScaffoldDirs []string `json:"scaffoldDirs,omitempty"`
}

// Category defines a skill domain category.
type Category struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// Patterns lists all available skill design patterns.
var Patterns = []Pattern{
	{Name: "tool-wrapper", Description: "Teach agent how to use a library/API", ScaffoldDirs: []string{"references"}},
	{Name: "generator", Description: "Produce structured output from a template", ScaffoldDirs: []string{"assets", "references"}},
	{Name: "reviewer", Description: "Score/audit against a checklist", ScaffoldDirs: []string{"references"}},
	{Name: "inversion", Description: "Agent interviews user before acting", ScaffoldDirs: []string{"assets"}},
	{Name: "pipeline", Description: "Multi-step workflow with checkpoints", ScaffoldDirs: []string{"references", "assets", "scripts"}},
	{Name: "none", Description: "Blank template"},
}

// Categories lists all available skill domain categories.
var Categories = []Category{
	{Key: "library", Label: "Library & API Reference"},
	{Key: "verification", Label: "Product Verification"},
	{Key: "data", Label: "Data Fetching & Analysis"},
	{Key: "automation", Label: "Business Process & Team Automation"},
	{Key: "scaffold", Label: "Code Scaffolding & Templates"},
	{Key: "quality", Label: "Code Quality & Review"},
	{Key: "cicd", Label: "CI/CD & Deployment"},
	{Key: "runbook", Label: "Runbooks & Incident Response"},
	{Key: "infra", Label: "Infrastructure Operations"},
}

// ValidNameRe matches valid skill names: lowercase letters, numbers, hyphens,
// underscores; must start with a letter or underscore.
var ValidNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`)

// FindPattern returns the pattern with the given name, or nil if not found.
func FindPattern(name string) *Pattern {
	for i := range Patterns {
		if Patterns[i].Name == name {
			return &Patterns[i]
		}
	}
	return nil
}

// GenerateContent produces SKILL.md content for the given skill name, pattern,
// and category. When pattern is "none" or empty, a plain template is generated
// (no pattern/category fields in frontmatter).
func GenerateContent(name, pattern, category string) string {
	if pattern == "none" || pattern == "" {
		return generatePlainTemplate(name)
	}
	return generatePatternTemplate(name, pattern, category)
}

// ToTitleCase converts kebab-case to Title Case.
func ToTitleCase(s string) string {
	words := strings.Split(s, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// generatePatternTemplate creates SKILL.md with pattern and optional category
// fields in the frontmatter.
func generatePatternTemplate(name, pattern, category string) string {
	title := ToTitleCase(name)

	var fm strings.Builder
	fm.WriteString("---\n")
	fmt.Fprintf(&fm, "name: %s\n", name)
	fm.WriteString("description: >-\n")
	fm.WriteString("  Describe what this skill does. Use when ...\n")
	fmt.Fprintf(&fm, "pattern: %s\n", pattern)
	if category != "" {
		fmt.Fprintf(&fm, "category: %s\n", category)
	}
	fm.WriteString("---\n\n")

	body := patternBody(pattern, title)
	return fm.String() + body
}

// patternBody returns the markdown body for a given pattern.
func patternBody(pattern, title string) string {
	switch pattern {
	case "tool-wrapper":
		return "# " + title + "\n\n" +
			"## Core Conventions\n\n" +
			"Load and follow the rules in " + "`references/conventions.md`" + " before writing any code.\n\n" +
			"## When Reviewing Code\n\n" +
			"- Check that all API calls follow the conventions\n" +
			"- Verify error handling matches the library's patterns\n" +
			"- Ensure imports and initialization are correct\n\n" +
			"## When Writing Code\n\n" +
			"- Follow the conventions from " + "`references/conventions.md`" + "\n" +
			"- Use idiomatic patterns for this library/API\n" +
			"- Include error handling for common failure modes\n"

	case "generator":
		return "# " + title + "\n\n" +
			"## Steps\n\n" +
			"### Step 1: Load Style Guide\n\n" +
			"Read " + "`references/style-guide.md`" + " for formatting and naming rules.\n\n" +
			"### Step 2: Load Template\n\n" +
			"Read " + "`assets/template.md`" + " as the base structure.\n\n" +
			"### Step 3: Gather Input\n\n" +
			"Ask the user what they need generated. Collect all required variables.\n\n" +
			"### Step 4: Generate\n\n" +
			"Fill in the template following the style guide. Ensure all placeholders are replaced.\n\n" +
			"### Step 5: Deliver\n\n" +
			"Present the generated output. Ask if adjustments are needed.\n"

	case "reviewer":
		return "# " + title + "\n\n" +
			"## Steps\n\n" +
			"### Step 1: Load Checklist\n\n" +
			"Read " + "`references/review-checklist.md`" + " for the complete list of review criteria.\n\n" +
			"### Step 2: Understand\n\n" +
			"Read the code/document under review. Identify its purpose and scope.\n\n" +
			"### Step 3: Apply Rules\n\n" +
			"Evaluate each checklist item. Classify findings by severity:\n" +
			"- **Critical**: Must fix before proceeding\n" +
			"- **Warning**: Should fix, may cause issues later\n" +
			"- **Info**: Suggestion for improvement\n\n" +
			"### Step 4: Report\n\n" +
			"Produce a review report with:\n" +
			"1. Summary (pass/fail + one-line verdict)\n" +
			"2. Findings (severity, location, description)\n" +
			"3. Score (percentage of checklist items passed)\n" +
			"4. Top 3 recommended fixes\n"

	case "inversion":
		return "# " + title + "\n\n" +
			"**DO NOT start building until all phases are complete.**\n\n" +
			"## Phase 1: Discovery\n\n" +
			"Ask the user these questions before proceeding:\n" +
			"- What is the goal?\n" +
			"- Who is the audience?\n" +
			"- What does success look like?\n\n" +
			"## Phase 2: Constraints\n\n" +
			"Ask the user about constraints:\n" +
			"- What are the technical limitations?\n" +
			"- What is the timeline?\n" +
			"- Are there existing patterns to follow?\n\n" +
			"## Phase 3: Synthesis\n\n" +
			"Based on the answers, load " + "`assets/template.md`" + " and produce a plan.\n" +
			"Present the plan for approval before executing.\n"

	case "pipeline":
		return "# " + title + "\n\n" +
			"## Steps\n\n" +
			"### Step 1: Prepare\n\n" +
			"Gather inputs and validate prerequisites for [" + title + "].\n\n" +
			"### Step 2: Gate Check\n\n" +
			"Present the plan to the user.\n\n" +
			"**Do NOT proceed until user confirms.**\n\n" +
			"### Step 3: Execute\n\n" +
			"Run the [" + title + "] pipeline. After each stage, verify output before continuing.\n\n" +
			"### Step 4: Quality Check\n\n" +
			"Review results against " + "`references/quality-checklist.md`" + ".\n" +
			"Report pass/fail status for each criterion.\n"

	default:
		return "# " + title + "\n\n" +
			"Describe your skill here.\n"
	}
}

// generatePlainTemplate creates a full SKILL.md without pattern/category fields.
func generatePlainTemplate(name string) string {
	title := ToTitleCase(name)

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
