package main

import "skillshare/internal/skill"

// skillPattern is a local alias so existing code (new_tui.go, new.go) compiles
// without modification.
type skillPattern = skill.Pattern

// skillCategory is a local alias for the same reason.
type skillCategory = skill.Category

var skillPatterns = skill.Patterns
var skillCategories = skill.Categories

func findPattern(name string) *skill.Pattern {
	return skill.FindPattern(name)
}

func generatePatternTemplate(name, pattern, category string) string {
	return skill.GenerateContent(name, pattern, category)
}

func generateSkillTemplate(name string) string {
	return skill.GenerateContent(name, "none", "")
}

func toTitleCase(s string) string {
	return skill.ToTitleCase(s)
}
