package sync

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// CalcSkillContext reads a skill's SKILL.md once and returns rune counts for:
//   - descChars: name + description (always loaded into context for skill matching)
//   - bodyChars: everything after the frontmatter closing --- (loaded on demand)
//
// Returns (0, 0, nil) if SKILL.md does not exist or is empty.
func CalcSkillContext(skillPath string) (descChars, bodyChars int, description string, err error) {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		return 0, 0, "", nil
	}
	_, d, b, desc, _ := calcContextFromContent(content)
	return d, b, desc, nil
}

// calcContextFromContent parses frontmatter name+description and body from
// pre-read SKILL.md content, returning rune counts for each layer.
// yamlErr is non-nil when the frontmatter YAML between --- delimiters cannot
// be parsed; body metrics are still valid in that case.
func calcContextFromContent(content []byte) (name string, descChars, bodyChars int, description string, yamlErr error) {
	s := string(content)
	if len(s) == 0 {
		return "", 0, 0, "", nil
	}

	// Find frontmatter boundaries (between --- delimiters)
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "---") {
		// No frontmatter — entire content is body
		return "", 0, utf8.RuneCountInString(strings.TrimSpace(s)), "", nil
	}

	// Find closing ---
	rest := trimmed[3:] // skip opening ---
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	closingIdx := strings.Index(rest, "\n---")
	if closingIdx < 0 {
		// Malformed frontmatter — no closing ---
		return "", 0, 0, "", nil
	}

	fmRaw := rest[:closingIdx]
	body := strings.TrimSpace(rest[closingIdx+4:]) // skip \n---

	// Parse name and description from frontmatter YAML
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	yamlErr = yaml.Unmarshal([]byte(fmRaw), &fm)

	// Build always-loaded string
	var alwaysLoaded string
	if fm.Description != "" {
		alwaysLoaded = fm.Name + " " + fm.Description
	} else {
		alwaysLoaded = fm.Name
	}

	descChars = utf8.RuneCountInString(alwaysLoaded)
	bodyChars = utf8.RuneCountInString(body)

	return fm.Name, descChars, bodyChars, fm.Description, yamlErr
}
