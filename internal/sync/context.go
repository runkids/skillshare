package sync

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"skillshare/internal/utils"
)

// EstimateTokens approximates how many tokens s costs. ASCII text runs about
// four characters per token; anything wider (CJK, emoji, accented letters)
// is counted as one token per rune, which is what tokenizers charge for it.
// ponytail: two-bucket heuristic. Swap in a real tokenizer if a model's count
// ever needs to be exact rather than comparable.
func EstimateTokens(s string) int {
	var ascii, wide int
	for _, r := range s {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			wide++
		}
	}
	return ascii/4 + wide
}

// skillContext is the context cost of one SKILL.md: the always-loaded layer
// (name + description) and the on-demand layer (body after the frontmatter).
type skillContext struct {
	Name        string
	Description string
	DescChars   int // rune count of name + description
	BodyChars   int // rune count of body after frontmatter
	DescTokens  int
	BodyTokens  int
}

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
	sc, _ := calcContextFromContent(content)
	return sc.DescChars, sc.BodyChars, sc.Description, nil
}

// calcContextFromContent parses frontmatter name+description and body from
// pre-read SKILL.md content, returning rune and token counts for each layer.
// yamlErr is non-nil when the frontmatter YAML between --- delimiters cannot
// be parsed; body metrics are still valid in that case.
func calcContextFromContent(content []byte) (sc skillContext, yamlErr error) {
	s := utils.TrimBOM(string(content))
	if len(s) == 0 {
		return sc, nil
	}

	// Find frontmatter boundaries (between --- delimiters)
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "---") {
		// No frontmatter — entire content is body
		body := strings.TrimSpace(s)
		sc.BodyChars = utf8.RuneCountInString(body)
		sc.BodyTokens = EstimateTokens(body)
		return sc, nil
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
		return sc, nil
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
	alwaysLoaded := fm.Name
	if fm.Description != "" {
		alwaysLoaded = fm.Name + " " + fm.Description
	}

	sc.Name = fm.Name
	sc.Description = fm.Description
	sc.DescChars = utf8.RuneCountInString(alwaysLoaded)
	sc.BodyChars = utf8.RuneCountInString(body)
	sc.DescTokens = EstimateTokens(alwaysLoaded)
	sc.BodyTokens = EstimateTokens(body)
	return sc, yamlErr
}
