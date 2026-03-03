package audit

import (
	"regexp"
	"regexp/syntax"
	"strings"
	"unicode"
)

type literalConstraint struct {
	text string
	fold bool
}

// deriveRulePrefilter extracts a conservative literal prefilter from regex.
// The prefilter must be present in every possible match; otherwise "" is used.
func deriveRulePrefilter(raw string, re *regexp.Regexp) (string, bool) {
	if re == nil {
		return "", false
	}

	ast, err := syntax.Parse(raw, syntax.Perl)
	if err == nil {
		if best := pickBestPrefilter(requiredLiterals(ast.Simplify())); best.text != "" {
			if best.fold {
				return strings.ToLower(best.text), true
			}
			return best.text, false
		}
	}

	// Fallback to Go regexp's literal prefix acceleration when available.
	prefix, _ := re.LiteralPrefix()
	if !isSelectiveLiteral(prefix) {
		return "", false
	}
	if regexMayFoldCase(raw) {
		return strings.ToLower(prefix), true
	}
	return prefix, false
}

func requiredLiterals(re *syntax.Regexp) []literalConstraint {
	if re == nil {
		return nil
	}

	switch re.Op {
	case syntax.OpLiteral:
		if len(re.Rune) == 0 {
			return nil
		}
		return []literalConstraint{{
			text: string(re.Rune),
			fold: re.Flags&syntax.FoldCase != 0,
		}}
	case syntax.OpCapture:
		if len(re.Sub) == 0 {
			return nil
		}
		return requiredLiterals(re.Sub[0])
	case syntax.OpConcat:
		var out []literalConstraint
		for _, sub := range re.Sub {
			out = unionConstraints(out, requiredLiterals(sub))
		}
		return out
	case syntax.OpAlternate:
		if len(re.Sub) == 0 {
			return nil
		}
		out := requiredLiterals(re.Sub[0])
		for _, sub := range re.Sub[1:] {
			out = intersectConstraints(out, requiredLiterals(sub))
			if len(out) == 0 {
				return nil
			}
		}
		return out
	case syntax.OpPlus:
		if len(re.Sub) == 0 {
			return nil
		}
		return requiredLiterals(re.Sub[0])
	case syntax.OpRepeat:
		if re.Min > 0 && len(re.Sub) > 0 {
			return requiredLiterals(re.Sub[0])
		}
		return nil
	case syntax.OpStar, syntax.OpQuest:
		return nil
	default:
		return nil
	}
}

func pickBestPrefilter(candidates []literalConstraint) literalConstraint {
	best := literalConstraint{}
	bestLen := 0
	for _, c := range candidates {
		c = normalizeConstraint(c)
		if !isSelectiveLiteral(c.text) {
			continue
		}
		l := len(c.text)
		if l > bestLen {
			best = c
			bestLen = l
			continue
		}
		if l == bestLen && best.fold && !c.fold {
			// Prefer case-sensitive prefilters when selectivity is identical.
			best = c
		}
	}
	return best
}

func unionConstraints(base, extra []literalConstraint) []literalConstraint {
	out := append([]literalConstraint{}, base...)
	for _, c := range extra {
		out = appendUniqueConstraint(out, c)
	}
	return out
}

func intersectConstraints(a, b []literalConstraint) []literalConstraint {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	var out []literalConstraint
	for _, ca := range a {
		for _, cb := range b {
			if merged, ok := mergeConstraintPair(ca, cb); ok {
				out = appendUniqueConstraint(out, merged)
			}
		}
	}
	return out
}

func mergeConstraintPair(a, b literalConstraint) (literalConstraint, bool) {
	a = normalizeConstraint(a)
	b = normalizeConstraint(b)

	if strings.EqualFold(a.text, b.text) {
		if a.fold || b.fold || a.text != b.text {
			return literalConstraint{text: strings.ToLower(a.text), fold: true}, true
		}
		return literalConstraint{text: a.text, fold: false}, true
	}
	return literalConstraint{}, false
}

func appendUniqueConstraint(out []literalConstraint, c literalConstraint) []literalConstraint {
	c = normalizeConstraint(c)
	for _, existing := range out {
		if equivalentConstraint(existing, c) {
			return out
		}
	}
	return append(out, c)
}

func equivalentConstraint(a, b literalConstraint) bool {
	a = normalizeConstraint(a)
	b = normalizeConstraint(b)
	return a.fold == b.fold && a.text == b.text
}

func normalizeConstraint(c literalConstraint) literalConstraint {
	if c.fold {
		c.text = strings.ToLower(c.text)
	}
	return c
}

func isSelectiveLiteral(s string) bool {
	if len(s) < 3 {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func regexMayFoldCase(raw string) bool {
	return strings.Contains(strings.ToLower(raw), "(?i")
}
