package audit

import (
	"net/netip"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var safeYAMLPositional = regexp.MustCompile(`^(?:yaml\.)?C?SafeLoader$`)
var phpFunctionDeclaration = regexp.MustCompile(`(?i)\bfunction\s+&?\s*$`)
var safeYAMLArgument = regexp.MustCompile(`^Loader\s*=\s*(?:yaml\.)?C?SafeLoader$`)
var safeTorchArgument = regexp.MustCompile(`^weights_only\s*=\s*True$`)
var metadataURLRegex = regexp.MustCompile("(?i)https?://[^\\s\"'`<>]+")

// Only embedded rules opt into these narrow inspections. A custom replacement
// regex keeps ordinary regex/exclude semantics, even when it reuses a rule ID.
func (r rule) matchesSensitiveLine(line string) bool {
	if r.matchKind == "" || r.matchKind == "ssrf-metadata-0" {
		return r.Regex.MatchString(line)
	}
	for _, loc := range r.Regex.FindAllStringSubmatchIndex(line, -1) {
		if r.matchKind == "unsafe-deserialization-php" {
			if !phpFunctionDeclaration.MatchString(line[:loc[0]]) {
				return true
			}
			continue
		}
		if r.matchKind == "ssrf-internal-0" {
			for _, target := range requestTargets(line, loc) {
				if internalRequestURL(target) {
					return true
				}
			}
			continue
		}
		// The regex ends at the opening parenthesis. Inspect top-level arguments
		// of this call only; a safe sibling, string or nested keyword is irrelevant.
		args, complete := callArguments(line, loc[1]-1)
		safe := false
		if complete && (r.matchKind != "unsafe-deserialization-yaml" || strings.HasPrefix(line[loc[0]:loc[1]], "yaml.load")) {
			if r.matchKind == "unsafe-deserialization-yaml" && len(args) > 1 && safeYAMLPositional.MatchString(args[1]) {
				safe = true
			}
			for _, arg := range args {
				if (r.matchKind == "unsafe-deserialization-yaml" && safeYAMLArgument.MatchString(arg)) ||
					(r.matchKind == "unsafe-deserialization-torch" && safeTorchArgument.MatchString(arg)) {
					safe = true
				}
			}
		}
		if !safe {
			return true
		}
	}
	return false
}

// requestTargets keeps shell option values, such as a Referer header, from
// being mistaken for the destination. Other request APIs use their regex capture.
func requestTargets(line string, match []int) []string {
	matched := line[match[0]:match[1]]
	if strings.HasPrefix(matched, "curl") || strings.HasPrefix(matched, "wget") {
		return shellRequestTargets(line[match[0]:])
	}
	var targets []string
	for i := 2; i+1 < len(match); i += 2 {
		if match[i] >= 0 {
			targets = append(targets, line[match[i]:match[i+1]])
		}
	}
	return targets
}

func shellRequestTargets(command string) []string {
	var words []string
	var b strings.Builder
	var quote byte
	escaped, started := false, false
	flush := func() {
		if started {
			words = append(words, b.String())
			b.Reset()
			started = false
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		if escaped {
			b.WriteByte(c)
			started = true
			escaped = false
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				b.WriteByte(c)
			}
			started = true
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			started = true
		case ' ', '\t':
			flush()
		case ';', '|', ')', '\n', '\r':
			flush()
			goto parsed
		default:
			b.WriteByte(c)
			started = true
		}
	}
	flush()
parsed:
	var targets []string
	for i := 1; i < len(words); i++ {
		w := words[i]
		switch w {
		case "-H", "--header", "-d", "--data", "--data-raw", "--data-binary", "--data-urlencode", "-F", "--form", "--form-string", "-o", "--output", "--output-document", "-u", "--user", "-X", "--request", "-x", "--proxy", "-e", "--referer", "-T", "--upload-file", "--resolve":
			i++
			continue
		}
		if w == "-O" && words[0] == "wget" {
			i++
			continue
		}
		if value, ok := strings.CutPrefix(w, "--url="); ok {
			w = value
		} else if strings.HasPrefix(w, "-") {
			continue
		}
		if strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://") {
			targets = append(targets, w)
		}
	}
	return targets
}

// Prose mentions remain visible without blocking at the default threshold.
// File/fence context is a syntax heuristic, not proof that a request executes.
func (r rule) findingSeverity(line, filename string, codeContext bool) string {
	if !r.usesMetadataContext() {
		return r.Severity
	}
	remaining := []byte(line)
	if r.requestRegex != nil {
		for _, match := range r.requestRegex.FindAllStringSubmatchIndex(line, -1) {
			for _, candidate := range requestTargets(line, match) {
				target := strings.TrimRight(candidate, "`")
				u, err := url.Parse(target)
				if err != nil || u.Hostname() == "" {
					continue
				}
				if r.Regex.MatchString(u.Hostname()) {
					return r.Severity
				}
			}
		}
	}
	maskNonMetadataURLPathMentions(line, remaining, r.Regex)
	if (codeContext || metadataCodeFile(filename)) && r.Regex.Match(remaining) {
		return r.Severity
	}
	return SeverityHigh
}

// Metadata addresses in another host's URL path or query are mentions, not destinations.
func maskNonMetadataURLPathMentions(line string, remaining []byte, metadataRegex *regexp.Regexp) {
	for _, loc := range metadataURLRegex.FindAllStringIndex(line, -1) {
		start, end := loc[0], loc[1]
		target := strings.TrimRight(line[start:end], "`")
		u, err := url.Parse(target)
		if err != nil || u.Hostname() == "" || metadataRegex.MatchString(u.Hostname()) {
			continue
		}
		if !metadataRegex.MatchString(u.Path) && !metadataRegex.MatchString(u.RawQuery) {
			continue
		}
		for i := start; i < end; i++ {
			remaining[i] = ' '
		}
	}
}

func (r rule) usesMetadataContext() bool {
	return r.matchKind == "ssrf-metadata-0" && r.Severity == SeverityCritical && !r.severityOverridden
}

func metadataCodeFile(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".sh", ".bash", ".zsh", ".fish", ".py", ".js", ".ts", ".rb", ".php", ".php3", ".php4", ".php5", ".phtml", ".go", ".rs", ".swift":
		return true
	}
	switch strings.ToLower(filepath.Base(filename)) {
	case "makefile", "dockerfile":
		return true
	}
	return false
}

func metadataCodeFence(language string) bool {
	switch language {
	case "", "python", "javascript", "typescript", "ruby", "shell", "console", "golang", "rust", "makefile", "dockerfile":
		return true
	}
	return metadataCodeFile("code." + language)
}

func internalRequestURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// callArguments handles a single balanced line, quoted strings and nested
// delimiters. It is intentionally not a language parser or provenance analysis.
func callArguments(line string, open int) ([]string, bool) {
	var args []string
	stack := []byte{')'}
	start := open + 1
	var quote byte
	quoteWidth := 1
	escaped := false
	for i := start; i < len(line); i++ {
		ch := line[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				if quoteWidth == 1 {
					quote = 0
				} else if i+2 < len(line) && line[i+1] == quote && line[i+2] == quote {
					quote = 0
					i += 2
				}
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
			quoteWidth = 1
			if ch != '`' && i+2 < len(line) && line[i+1] == ch && line[i+2] == ch {
				quoteWidth = 3
				i += 2
			}
		case '#':
			return nil, false
		case '(':
			stack = append(stack, ')')
		case '[':
			stack = append(stack, ']')
		case '{':
			stack = append(stack, '}')
		case ')', ']', '}':
			if ch != stack[len(stack)-1] {
				return nil, false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return append(args, strings.TrimSpace(line[start:i])), true
			}
		case ',':
			if len(stack) == 1 {
				args = append(args, strings.TrimSpace(line[start:i]))
				start = i + 1
			}
		}
	}
	return nil, false
}
