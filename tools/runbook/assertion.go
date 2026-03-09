package main

import "strings"

// negationPrefixes lists recognized negation prefixes, longest first
// to ensure greedy matching.
var negationPrefixes = []string{
	"Should NOT ",
	"should not ",
	"Must NOT ",
	"must not ",
	"Does not ",
	"does not ",
	"NOT ",
	"Not ",
	"not ",
	"No ",
	"no ",
}

// MatchAssertions checks each expected pattern against the command output.
// Patterns prefixed with a negation keyword succeed when the inner text
// is NOT found in output. All matching is case-insensitive.
func MatchAssertions(output string, expected []string) []AssertionResult {
	results := make([]AssertionResult, 0, len(expected))
	lower := strings.ToLower(output)

	for _, pat := range expected {
		r := AssertionResult{Pattern: pat}

		inner := pat
		for _, prefix := range negationPrefixes {
			if strings.HasPrefix(pat, prefix) {
				r.Negated = true
				inner = pat[len(prefix):]
				break
			}
		}

		found := strings.Contains(lower, strings.ToLower(inner))
		if r.Negated {
			r.Matched = !found
		} else {
			r.Matched = found
		}

		results = append(results, r)
	}

	return results
}

// AllPassed returns true when every assertion matched successfully.
func AllPassed(results []AssertionResult) bool {
	for _, r := range results {
		if !r.Matched {
			return false
		}
	}
	return true
}
