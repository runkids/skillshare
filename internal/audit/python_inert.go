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
// A file that reads docstrings back gets no inert lines, and lines after a
// single-quoted string left open at a line end (a backslash continuation or a
// multi-line f-string field) stay code. This is a tokenizer heuristic, not
// proof that the file is safe.
func pythonInertLines(filename, text string) []bool {
	if !strings.EqualFold(filepath.Ext(filename), ".py") ||
		strings.Contains(text, "__doc__") || strings.Contains(text, "getdoc") {
		return nil
	}
	lines := strings.Split(text, "\n")
	inert := make([]bool, len(lines))
	var quote string  // delimiter of a string still open at the line start
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
				if c == '\\' {
					j++
				} else if strings.HasPrefix(line[j:], quote) {
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
func (r rule) inertSeverity(severity string, inert *[]bool, filename, text string, line int) string {
	if !r.pythonInertContext || r.severityOverridden || severity != SeverityCritical {
		return severity
	}
	if *inert == nil {
		*inert = pythonInertLines(filename, text)
	}
	if *inert != nil && (*inert)[line-1] {
		return SeverityHigh
	}
	return severity
}
