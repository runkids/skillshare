package audit

import (
	"path/filepath"
	"regexp"
	"strings"
)

// A string that opens a statement. f- and t-strings evaluate expressions, so
// their prefixes do not qualify.
var pyStatementString = regexp.MustCompile(`^(?i:rb|br|[rub])?("""|'''|"|')`)

// pythonInertLines reports, per zero-based line of a .py file, whether the
// line holds only a comment or part of a string that is a whole statement,
// such as a docstring. Python never runs either. A string inside brackets,
// after a line continuation, or followed by more code does not qualify.
// Where the tokenizer cannot follow Python, it stops marking lines: a file
// that reads docstrings back or uses bare CR line breaks gets none, and lines
// stay code after a single-quoted string left open at a line end, or an f- or
// t-string replacement field that holds a quote or does not close. This is a
// tokenizer heuristic, not proof that the file is safe.
func pythonInertLines(filename, text string) []bool {
	if !strings.EqualFold(filepath.Ext(filename), ".py") ||
		strings.Contains(text, "__doc__") || strings.Contains(text, "getdoc") ||
		strings.Count(text, "\r") != strings.Count(text, "\r\n") {
		return nil
	}
	lines := strings.Split(text, "\n")
	inert := make([]bool, len(lines))
	var quote string  // delimiter of a string still open at the line start
	fstring := false  // the open string is an f- or t-string
	field := 0        // replacement-field brace depth inside that string
	var pending []int // lines of an open statement string
	stmt := false     // the open or just-closed string is a whole statement
	depth := 0
	continued := false
	for i, line := range lines {
		startsInString := quote != ""
		opener := -1
		if quote == "" && depth == 0 && !continued {
			body := strings.TrimLeft(line, " \t")
			if m := pyStatementString.FindStringSubmatchIndex(body); m != nil {
				opener = len(line) - len(body) + m[2]
			}
		}
		continued = false
		code, comment := false, false
	scan:
		for j := 0; j < len(line); j++ {
			c := line[j]
			if quote != "" {
				switch {
				case c == '\\':
					j++
				case fstring && field > 0 && (c == '"' || c == '\''):
					return inert // a nested string can hide where the field ends
				case fstring && field > 0 && c == '{':
					field++
				case fstring && field > 0 && c == '}':
					field--
				case fstring && (c == '{' || c == '}') && strings.HasPrefix(line[j+1:], string(c)):
					j++ // literal {{ or }}
				case fstring && c == '{':
					field = 1
				case fstring && c == '}':
					return inert
				case strings.HasPrefix(line[j:], quote):
					j += len(quote) - 1
					quote = ""
				}
				continue
			}
			if j < opener {
				continue // indentation and string prefix
			}
			switch c {
			case ' ', '\t', '\r', '\f':
			case '#':
				comment = true
				break scan
			case '"', '\'':
				if j == opener {
					stmt = true
				} else {
					code = true
				}
				prefix := strings.ToLower(line[max(j-2, 0):j])
				fstring = strings.HasSuffix(prefix, "f") || strings.HasSuffix(prefix, "t") ||
					prefix == "fr" || prefix == "tr"
				quote = line[j : j+1]
				if triple := strings.Repeat(quote, 3); strings.HasPrefix(line[j:], triple) {
					quote = triple
				}
				j += len(quote) - 1
			case '\\':
				continued = true
			case '(', '[', '{':
				depth++
				code = true
			case ')', ']', '}':
				depth = max(depth-1, 0)
				code = true
			default:
				code = true
			}
		}
		if len(quote) == 1 {
			return inert // the tokenizer no longer knows where code resumes
		}
		switch {
		case stmt:
			pending = append(pending, i)
			if code || continued {
				pending, stmt = nil, false
			} else if quote == "" {
				for _, p := range pending {
					inert[p] = true
				}
				pending, stmt = nil, false
			}
		case comment && !code && !startsInString:
			inert[i] = true
		}
	}
	return inert
}

// inertSeverity keeps a built-in CRITICAL credential command visible as HIGH
// when it appears only on an inert Python line (1-based), so a security tool
// that names credential paths in its comments or docstrings is not blocked at
// the default threshold. Explicit severity overrides and custom replacements
// keep their configured severity. Lines are tokenized on the first such match.
// The second result is the finding's SeverityContext.
func (r rule) inertSeverity(severity string, inert *[]bool, filename, text string, line int) (string, string) {
	if !lowersInPython(r.pythonInertContext, r.severityOverridden, severity) {
		return severity, ""
	}
	if *inert == nil {
		*inert = pythonInertLines(filename, text)
	}
	if *inert != nil && (*inert)[line-1] {
		return SeverityHigh, SeverityContextPythonInert
	}
	return severity, ""
}

// lowersInPython reports whether a rule's matches on inert Python lines are
// lowered from CRITICAL to HIGH. The scanner and the rule list share it.
func lowersInPython(pythonInertContext, severityOverridden bool, severity string) bool {
	return pythonInertContext && !severityOverridden && severity == SeverityCritical
}
