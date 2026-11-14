// Package clidiscovery ranks CLI help metadata without running commands or
// loading configuration. The caller supplies only public, supported commands.
package clidiscovery

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Result is a command and a safe invocation for inspecting its usage.
// Ranking scores are deliberately not part of the output contract.
type Result struct {
	Command string `json:"command"`
	Summary string `json:"summary"`
	Usage   string `json:"usage"`
}

// Search matches task words against command names and summaries. Exact names
// rank above description matches; prefixes of at least three runes and a small
// set of task synonyms also match. Ties use canonical command order.
func Search(entries []Result, query string, limit int) []Result {
	results := make([]Result, 0)
	if limit <= 0 {
		return results
	}
	terms := words(query)
	slices.Sort(terms)
	terms = slices.Compact(terms)
	type match struct {
		result Result
		score  int
	}
	matches := make([]match, 0)
	for _, entry := range entries {
		// The tool name is shared by every entry and must not match every task.
		_, name, _ := strings.Cut(entry.Command, " ")
		names, summary := words(name), words(entry.Summary)
		score := 0
		for _, term := range terms {
			score += 3*wordMatch(names, term) + wordMatch(summary, term)
		}
		if score > 0 {
			matches = append(matches, match{entry, score})
		}
	}
	slices.SortFunc(matches, func(a, b match) int {
		if a.score != b.score {
			return b.score - a.score
		}
		return strings.Compare(a.result.Command, b.result.Command)
	})
	for _, match := range matches[:min(limit, 20, len(matches))] {
		results = append(results, match.result)
	}
	return results
}

func words(text string) []string {
	tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		switch token {
		case "a", "an", "and", "for", "from", "i", "in", "into", "is", "it", "my", "of", "on", "the", "to", "want", "with":
			continue
		}
		// Fold ordinary plurals without changing command names such as status.
		if len(token) > 3 && strings.HasSuffix(token, "s") && !strings.HasSuffix(token, "ss") && !strings.HasSuffix(token, "us") && !strings.HasSuffix(token, "is") {
			token = strings.TrimSuffix(token, "s")
		}
		out = append(out, token)
	}
	return out
}

func wordMatch(words []string, term string) int {
	best := 0
	for _, word := range words {
		if word == term {
			return 2
		}
		if utf8.RuneCountInString(term) >= 3 && strings.HasPrefix(word, term) {
			best = 1
		}
		for _, group := range []string{"remove delete uninstall", "list show ls", "create add new", "find search lookup"} {
			synonyms := strings.Fields(group)
			if slices.Contains(synonyms, term) && slices.Contains(synonyms, word) {
				best = 1
			}
		}
	}
	return best
}
