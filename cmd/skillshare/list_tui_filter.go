package main

import (
	"strings"
)

// filterQuery holds the parsed result of a structured filter string.
// All fields are lowercased for case-insensitive matching.
type filterQuery struct {
	TypeTag  string // "tracked", "remote", "local" (empty = any)
	GroupTag string // substring match against group segment
	RepoTag  string // substring match against RepoName
	FreeText string // remaining text after removing known tags
}

// parseFilterQuery tokenizes the raw filter string and extracts known tags.
// Unknown key:value tokens are treated as free text.
func parseFilterQuery(raw string) filterQuery {
	var q filterQuery
	var freeTokens []string

	for _, token := range strings.Fields(raw) {
		lower := strings.ToLower(token)

		if val, ok := cutTag(lower, "t:", "type:"); ok {
			q.TypeTag = normalizeTypeValue(val)
			continue
		}
		if val, ok := cutTag(lower, "g:", "group:"); ok {
			q.GroupTag = val
			continue
		}
		if val, ok := cutTag(lower, "r:", "repo:"); ok {
			q.RepoTag = val
			continue
		}

		freeTokens = append(freeTokens, strings.ToLower(token))
	}

	q.FreeText = strings.Join(freeTokens, " ")
	return q
}

// cutTag checks if token starts with any of the prefixes and returns the value after.
func cutTag(token string, prefixes ...string) (string, bool) {
	for _, p := range prefixes {
		if strings.HasPrefix(token, p) {
			val := token[len(p):]
			if val != "" {
				return val, true
			}
		}
	}
	return "", false
}

// normalizeTypeValue maps aliases to canonical type names.
// Strips optional leading "_" (tracked repos use "_" prefix on disk).
func normalizeTypeValue(val string) string {
	val = strings.TrimPrefix(val, "_")
	switch val {
	case "github":
		return "remote"
	case "track":
		return "tracked"
	default:
		return val
	}
}

// matchSkillItem returns true if the skill item matches all non-empty conditions in the query (AND logic).
func matchSkillItem(item skillItem, q filterQuery) bool {
	e := item.entry

	// Type tag — exact match on skill type category
	if q.TypeTag != "" {
		skillType := skillTypeCategory(e)
		if skillType != q.TypeTag {
			return false
		}
	}

	// Group tag — substring match on group segment
	if q.GroupTag != "" {
		group := strings.ToLower(skillGroup(e))
		if !strings.Contains(group, q.GroupTag) {
			return false
		}
	}

	// Repo tag — substring match on RepoName
	if q.RepoTag != "" {
		repo := strings.ToLower(e.RepoName)
		if !strings.Contains(repo, q.RepoTag) {
			return false
		}
	}

	// Free text — substring match on FilterValue
	if q.FreeText != "" {
		fv := strings.ToLower(item.FilterValue())
		if !strings.Contains(fv, q.FreeText) {
			return false
		}
	}

	return true
}

// skillTypeCategory returns "tracked", "remote", or "local" for a skill entry.
func skillTypeCategory(e skillEntry) string {
	switch {
	case e.RepoName != "":
		return "tracked"
	case e.Source != "":
		return "remote"
	default:
		return "local"
	}
}

// skillGroup extracts the group directory segment from a skill entry's RelPath.
// For tracked repos: the second path segment (after the repo dir prefix).
// For non-tracked: the first path segment.
// Root-level skills (no subdirectory) return "".
func skillGroup(e skillEntry) string {
	parts := strings.Split(e.RelPath, "/")
	if len(parts) <= 1 {
		return ""
	}

	if e.RepoName != "" {
		// Tracked: first segment is repo dir, second is group
		if len(parts) >= 3 {
			return parts[1]
		}
		return ""
	}

	// Non-tracked: first segment is group
	return parts[0]
}
