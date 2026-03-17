package skillignore

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// rule is a parsed .skillignore pattern.
type rule struct {
	segments []string // pattern split by "/"
	negated  bool     // starts with "!"
	anchored bool     // contains "/" (other than trailing) or starts with "/"
	dirOnly  bool     // ends with "/"
}

// normalizePath converts backslashes to forward slashes and strips leading "/".
func normalizePath(p string) string {
	return strings.TrimPrefix(strings.ReplaceAll(p, "\\", "/"), "/")
}

// Matcher holds compiled .skillignore rules.
type Matcher struct {
	rules       []rule
	hasNegation bool
}

// parseRule parses a single .skillignore line into a rule.
// Returns the rule and true if valid, or zero value and false if the line
// should be skipped (blank or comment).
func parseRule(line string) (rule, bool) {
	line = strings.TrimRight(line, " \t")

	// Blank line
	if line == "" {
		return rule{}, false
	}

	// Comment (unless escaped)
	if strings.HasPrefix(line, "#") {
		return rule{}, false
	}

	var r rule

	// Negation
	if strings.HasPrefix(line, "!") {
		r.negated = true
		line = line[1:]
	}

	// Handle leading backslash escapes: \# or \!
	if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
		line = line[1:]
	}

	// Trailing slash → directory-only match
	if strings.HasSuffix(line, "/") {
		r.dirOnly = true
		line = strings.TrimRight(line, "/")
	}

	// Leading slash → anchored, remove it
	if strings.HasPrefix(line, "/") {
		r.anchored = true
		line = strings.TrimPrefix(line, "/")
	}

	// If pattern contains "/" (after stripping leading/trailing), it's anchored
	if !r.anchored && strings.Contains(line, "/") {
		r.anchored = true
	}

	// Split into segments and convert [!x] → [^x] for path.Match compatibility.
	// Gitignore uses [!...] for negated character classes; Go's path.Match uses [^...].
	r.segments = strings.Split(line, "/")
	for i, seg := range r.segments {
		if strings.Contains(seg, "[!") {
			r.segments[i] = strings.ReplaceAll(seg, "[!", "[^")
		}
	}

	if line == "" {
		return rule{}, false
	}

	return r, true
}

// Compile parses pattern lines into a Matcher.
func Compile(lines []string) *Matcher {
	m := &Matcher{}
	for _, line := range lines {
		r, ok := parseRule(line)
		if !ok {
			continue
		}
		if r.negated {
			m.hasNegation = true
		}
		m.rules = append(m.rules, r)
	}
	return m
}

// ReadMatcher reads a .skillignore file from dir and compiles it.
// Returns a non-nil Matcher even if the file doesn't exist (no-op matcher).
func ReadMatcher(dir string) *Matcher {
	data, err := os.ReadFile(filepath.Join(dir, ".skillignore"))
	if err != nil {
		return &Matcher{}
	}
	return Compile(strings.Split(string(data), "\n"))
}

// HasRules reports whether the matcher has any compiled rules.
func (m *Matcher) HasRules() bool {
	return m != nil && len(m.rules) > 0
}

// Match returns true if the given path should be ignored.
// isDir indicates whether the path is a directory.
// It checks parent directories too: if "vendor" is ignored, "vendor/foo/bar"
// is also ignored (unless a later negation rule un-ignores it).
func (m *Matcher) Match(skillPath string, isDir bool) bool {
	if m == nil || len(m.rules) == 0 {
		return false
	}

	skillPath = normalizePath(skillPath)

	// Build list of prefixes to check (parent dirs + full path).
	// For "a/b/c": check "a" (dir), "a/b" (dir), "a/b/c" (isDir).
	parts := strings.Split(skillPath, "/")

	// Evaluate: last matching rule wins. Check from outermost prefix inward.
	ignored := false
	var buf strings.Builder
	buf.Grow(len(skillPath))
	for i, seg := range parts {
		if i > 0 {
			buf.WriteByte('/')
		}
		buf.WriteString(seg)
		prefix := buf.String()

		cIsDir := i < len(parts)-1 || isDir
		for _, r := range m.rules {
			if r.dirOnly && !cIsDir {
				continue
			}
			if matchRule(r, prefix) {
				ignored = !r.negated
			}
		}

		// Early exit: if ignored and no negation rules can un-ignore descendants
		if ignored && !m.hasNegation {
			return true
		}
	}

	return ignored
}

// CanSkipDir returns true if it's safe to skip this entire directory
// during a filesystem walk (e.g., with filepath.SkipDir).
// When negation patterns exist, it returns false if any negation pattern
// could potentially match a descendant inside this directory.
func (m *Matcher) CanSkipDir(dirPath string) bool {
	if m == nil || len(m.rules) == 0 {
		return false
	}

	if !m.Match(dirPath, true) {
		return false
	}

	// If no negation rules, safe to skip.
	if !m.hasNegation {
		return true
	}

	// Check if any negation rule could match inside this directory.
	// Path is already normalized by the Match call above, but normalize
	// for callers that bypass Match in the future.
	dirPath = normalizePath(dirPath)

	for _, r := range m.rules {
		if !r.negated {
			continue
		}
		// Non-anchored negation patterns can match anything inside.
		if !r.anchored {
			return false
		}
		// Anchored negation: check if its prefix overlaps with dirPath.
		negPath := strings.Join(r.segments, "/")
		if strings.HasPrefix(negPath, dirPath+"/") || negPath == dirPath {
			return false
		}
	}

	return true
}

// matchRule checks if a single rule matches a given path.
func matchRule(r rule, p string) bool {
	pathSegs := strings.Split(p, "/")

	if r.anchored {
		// Anchored: match from root
		return matchSegments(r.segments, pathSegs)
	}

	// Non-anchored without "/": match basename at any depth
	if len(r.segments) == 1 {
		basename := pathSegs[len(pathSegs)-1]
		ok, _ := path.Match(r.segments[0], basename)
		return ok
	}

	// Non-anchored with multiple segments: slide pattern across path
	return matchSegments(r.segments, pathSegs)
}

// matchSegments recursively matches pattern segments against path segments.
// Handles ** (zero or more directories), and per-segment globs via path.Match.
func matchSegments(pat, p []string) bool {
	pi, pp := 0, 0
	for pi < len(pat) && pp < len(p) {
		if pat[pi] == "**" {
			// ** at end of pattern matches everything remaining
			if pi == len(pat)-1 {
				return true
			}
			// Try matching ** as zero or more segments
			for skip := pp; skip <= len(p); skip++ {
				if matchSegments(pat[pi+1:], p[skip:]) {
					return true
				}
			}
			return false
		}
		ok, _ := path.Match(pat[pi], p[pp])
		if !ok {
			return false
		}
		pi++
		pp++
	}

	// Handle trailing ** in pattern
	for pi < len(pat) && pat[pi] == "**" {
		pi++
	}

	return pi == len(pat) && pp == len(p)
}

// --- Deprecated API (backward compatibility) ---

// ReadPatterns reads a .skillignore file from dir.
// Deprecated: Use ReadMatcher instead.
func ReadPatterns(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".skillignore"))
	if err != nil {
		return nil
	}
	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, strings.TrimRight(line, "/"))
	}
	return patterns
}

// Match returns true if skillPath matches any pattern using the old matcher.
// Deprecated: Use Matcher.Match instead.
func Match(skillPath string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(skillPath, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if skillPath == p || strings.HasPrefix(skillPath, p+"/") {
			return true
		}
	}
	return false
}
