package search

import (
	"math"
	"strings"
)

// Scoring weights for multi-signal relevance ranking
const (
	weightName        = 0.45
	weightDescription = 0.25
	weightStars       = 0.30
)

// scoreResult computes a composite relevance score for a search result.
// When query is empty (browse mode), scoring is stars-only to preserve
// the existing behavior of showing popular skills first.
func scoreResult(r SearchResult, query string) float64 {
	if query == "" {
		return normalizeStars(r.Stars)
	}

	name := nameMatchScore(r.Name, query)
	desc := descriptionMatchScore(r.Description, query)
	stars := normalizeStars(r.Stars)

	return name*weightName + desc*weightDescription + stars*weightStars
}

// nameMatchScore scores how well a skill name matches the query.
//
//	exact match      → 1.0
//	name contains q  → 0.7
//	word boundary    → 0.6
//	no match         → 0.0
func nameMatchScore(name, query string) float64 {
	nl := strings.ToLower(name)
	ql := strings.ToLower(query)

	if nl == ql {
		return 1.0
	}
	if strings.Contains(nl, ql) {
		return 0.7
	}

	// Word boundary: query matches a hyphen/underscore-separated segment
	for _, seg := range strings.FieldsFunc(nl, func(r rune) bool {
		return r == '-' || r == '_' || r == '/'
	}) {
		if seg == ql {
			return 0.6
		}
	}

	return 0.0
}

// descriptionMatchScore scores how many query words appear in the description.
// Returns the ratio of matched words to total query words.
func descriptionMatchScore(desc, query string) float64 {
	if desc == "" || query == "" {
		return 0.0
	}

	dl := strings.ToLower(desc)
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return 0.0
	}

	matched := 0
	for _, w := range words {
		if strings.Contains(dl, w) {
			matched++
		}
	}

	return float64(matched) / float64(len(words))
}

// normalizeStars maps star count to [0, 1] using log10 scale.
// 0 → 0, 10 → 0.2, 100 → 0.4, 1000 → 0.6, 10000 → 0.8, 100000+ → 1.0
func normalizeStars(stars int) float64 {
	if stars <= 0 {
		return 0.0
	}
	v := math.Log10(float64(stars)) / 5.0 // log10(100000) = 5
	if v > 1.0 {
		return 1.0
	}
	return v
}
