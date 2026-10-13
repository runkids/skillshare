package plugin

import (
	"path"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

// piTri is a selection that may be unknown: Skillshare evaluates Pi's resource
// filters only where it reproduces Pi exactly, and says so everywhere else.
type piTri int

const (
	piUnknown piTri = iota
	piOn
	piOff
)

func (t piTri) String() string {
	switch t {
	case piOn:
		return "loads"
	case piOff:
		return "skipped"
	}
	return "unknown"
}

func triNot(t piTri) piTri {
	switch t {
	case piOn:
		return piOff
	case piOff:
		return piOn
	}
	return piUnknown
}

func triAnd(a, b piTri) piTri {
	if a == piOff || b == piOff {
		return piOff
	}
	if a == piOn && b == piOn {
		return piOn
	}
	return piUnknown
}

// piGlobSupported reports whether a non-exact Pi pattern is in the grammar this
// package reproduces: '/'-separated segments of literals with '*' and '?' only,
// as minimatch 10 matches them without options (dot files need a literal dot).
// Globstars, braces, classes, extglobs, escapes, comments, negation, paths Pi
// rewrites and '.' or '..' segments are left to Pi. The golden test checks this against
// Pi's minimatch.
func piGlobSupported(pattern string) bool {
	if pattern == "" || piRewrittenPattern(pattern, runtime.GOOS) || strings.ContainsAny(pattern, "[]{}()|\\") || strings.Contains(pattern, "**") ||
		strings.HasPrefix(pattern, "#") || strings.HasPrefix(pattern, "!") {
		return false
	}
	for i, seg := range strings.Split(pattern, "/") {
		if seg == "" && i == 0 {
			continue // absolute pattern
		}
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// piGlobMatch matches subject against a supported pattern, segment by segment.
func piGlobMatch(pattern, subject string) piTri {
	if piCountsDiffer(pattern, subject) {
		return piUnknown
	}
	ps, ss := strings.Split(pattern, "/"), strings.Split(subject, "/")
	if len(ps) != len(ss) {
		return piOff
	}
	for i := range ps {
		// Without the dot option, '*' and '?' never match a leading dot.
		if strings.HasPrefix(ss[i], ".") && !strings.HasPrefix(ps[i], ".") {
			return piOff
		}
		if ok, err := path.Match(ps[i], ss[i]); err != nil || !ok {
			return piOff
		}
	}
	return piOn
}

// piCountsDiffer reports whether Go's '?' (one rune) and minimatch's '?' (one
// UTF-16 code unit) can disagree for this pair: an astral character, or a name
// that is not UTF-8 and so is decoded differently.
func piCountsDiffer(pattern, subject string) bool {
	if !strings.Contains(pattern, "?") {
		return false
	}
	if !utf8.ValidString(pattern) || !utf8.ValidString(subject) {
		return true
	}
	return strings.ContainsFunc(pattern+subject, func(r rune) bool { return r > 0xFFFF })
}

// piGlobMatches is Pi's matchesAnyPattern for one non-SKILL.md file: the pattern
// is tried against the path relative to the package, the base name, and the
// absolute path.
func piGlobMatches(pattern, rel, abs string) piTri {
	if !piGlobSupported(pattern) {
		return piUnknown
	}
	result := piOff
	for _, subject := range []string{rel, path.Base(rel), abs} {
		switch piGlobMatch(pattern, subject) {
		case piOn:
			return piOn
		case piUnknown:
			result = piUnknown
		}
	}
	return result
}

// piShellDrive is the Git Bash, MSYS, Cygwin and WSL drive path that Pi's
// normalizeWindowsShellPath turns into a Windows drive path.
var piShellDrive = regexp.MustCompile(`(?i)^/(?:mnt/|cygdrive/)?[a-z](?:/.*)?$`)

// piRewrittenPattern reports a pattern Pi 1.2.0 and later rewrites with
// normalizePath before matching while older Pi compares it as written: a home
// path, a file:// URL, and on Windows a ~\ home path or a shell drive path. Its
// match depends on the Pi version, so Skillshare leaves it unknown.
func piRewrittenPattern(pattern, goos string) bool {
	switch {
	case pattern == "~" || strings.HasPrefix(pattern, "~/") || strings.HasPrefix(pattern, "file://"):
		return true
	case goos != "windows":
		return false
	}
	return strings.HasPrefix(pattern, `~\`) ||
		!strings.HasPrefix(pattern, "//") && !strings.Contains(pattern, `\`) && piShellDrive.MatchString(pattern)
}

// piExactState is piExactMatches as a selection: unknown for a pattern Pi
// rewrites after dropping a leading "./" or ".\".
func piExactState(pattern, rel, abs string) piTri {
	switch {
	case piRewrittenPattern(piDropDot(pattern), runtime.GOOS):
		return piUnknown
	case piExactMatches(pattern, rel, abs):
		return piOn
	}
	return piOff
}

// piDropDot drops the leading "./" or ".\" that Pi's normalizeExactPattern drops
// on every platform.
func piDropDot(pattern string) string {
	if strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, `.\`) {
		return pattern[2:]
	}
	return pattern
}

// piExactPattern is Pi's normalizeExactPattern before 1.2.0: the leading dot
// dropped, then toPosixPath, which turns "\" into "/" only on Windows.
func piExactPattern(pattern, goos string) string {
	pattern = piDropDot(pattern)
	if goos == "windows" {
		pattern = strings.ReplaceAll(pattern, `\`, "/")
	}
	return pattern
}

// piExactMatches is Pi's matchesAnyExactPattern: a plain comparison with the
// normalized pattern.
func piExactMatches(pattern, rel, abs string) bool {
	pattern = piExactPattern(pattern, runtime.GOOS)
	return pattern == rel || pattern == abs
}

// piExactRule reports whether a rule is an exact "+path" or "-path" for the file
// at rel, written relative or as its absolute path, as Pi compares them.
func piExactRule(rule, rel, abs string) bool {
	return (strings.HasPrefix(rule, "+") || strings.HasPrefix(rule, "-")) && piExactMatches(rule[1:], rel, abs)
}

type piEvaluation struct {
	state piTri
	exact string   // the last exact rule that decided the file, or a rewritten rule that may
	globs []string // the include and ! rules that match it, or may
}

// piEvaluate is Pi's applyPatterns for one file of a non-empty rule list:
// includes (or everything), then ! excludes, then +exact, then -exact.
func piEvaluate(rules []string, rel, abs string) piEvaluation {
	var e piEvaluation
	included, hasInclude := piOff, false
	excluded := piOff
	forceOn, forceOff := false, false
	rewrittenOn, rewrittenOff := "", "" // the last rewritten rule that may force the file
	for _, rule := range rules {
		switch {
		case strings.HasPrefix(rule, "+"):
			switch piExactState(rule[1:], rel, abs) {
			case piOn:
				forceOn, e.exact = true, rule
			case piUnknown:
				rewrittenOn = rule
			}
		case strings.HasPrefix(rule, "-"):
			switch piExactState(rule[1:], rel, abs) {
			case piOn:
				forceOff, e.exact = true, rule
			case piUnknown:
				rewrittenOff = rule
			}
		case strings.HasPrefix(rule, "!"):
			if m := piGlobMatches(rule[1:], rel, abs); m != piOff {
				e.globs = append(e.globs, rule)
				if excluded != piOn {
					excluded = m
				}
			}
		default:
			hasInclude = true
			if m := piGlobMatches(rule, rel, abs); m != piOff {
				e.globs = append(e.globs, rule)
				if included != piOn {
					included = m
				}
			}
		}
	}
	if !hasInclude {
		included = piOn
	}
	e.state = triAnd(included, triNot(excluded))
	if forceOn {
		e.state = piOn
	} else if rewrittenOn != "" && e.state != piOn {
		e.state, e.exact = piUnknown, rewrittenOn
	}
	if forceOff {
		e.state = piOff
		// "-" wins over "+", so name it as the deciding rule.
		for _, rule := range rules {
			if strings.HasPrefix(rule, "-") && piExactMatches(rule[1:], rel, abs) {
				e.exact = rule
			}
		}
	} else if rewrittenOff != "" && e.state != piOff {
		e.state, e.exact = piUnknown, rewrittenOff
	}
	return e
}

// piEvaluateDelta is Pi's applyAutoloadDisabledPatterns for one file of a
// project autoload:false entry: the last matching rule decides; no match means
// the file is not named here (piOff with named false).
func piEvaluateDelta(rules []string, rel, abs string) (state piTri, named bool, rule string) {
	for _, r := range rules {
		target, on, exact := r, true, false
		switch {
		case strings.HasPrefix(r, "+"):
			target, exact = r[1:], true
		case strings.HasPrefix(r, "-"):
			target, on, exact = r[1:], false, true
		case strings.HasPrefix(r, "!"):
			target, on = r[1:], false
		}
		var m piTri
		if exact {
			m = piExactState(target, rel, abs)
		} else {
			m = piGlobMatches(target, rel, abs)
		}
		switch m {
		case piOn:
			named, rule, state = true, r, piOff
			if on {
				state = piOn
			}
		case piUnknown:
			// A later rule may still decide; until one does, the result is unknown.
			named, rule, state = true, r, piUnknown
		}
	}
	return state, named, rule
}
