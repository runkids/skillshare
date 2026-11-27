package contextrouter

import (
	"path"
	"regexp"
	"strings"
)

// pathLinkRe mirrors PATH_LINK: a link whose text and destination are plain
// relative paths, with no spaces, escapes, entities or code markers.
var pathLinkRe = regexp.MustCompile(`^\[([A-Za-z0-9._~/+-]+)\]\(([A-Za-z0-9._~/+-]+)\)\z`)

// pyJoin is posixpath.join for two parts: an absolute second part replaces the first.
func pyJoin(base, rel string) string {
	if strings.HasPrefix(rel, "/") {
		return rel
	}
	return path.Join(base, rel)
}

// compactPathLinks mirrors compact_path_links: a link the scanners recognize
// whose text is its own target's path from the router folder is printed as
// that path in backticks. source is the page's cleaned path in the folder.
// ok is false when halt stopped the scan.
func compactPathLinks(body, source string, halt *stopper) (compacted string, ok bool) {
	base := path.Dir(source)
	lines := strings.Split(body, "\n")
	// The scanner expands tabs among a line's leading markers before it reports
	// offsets; offsets[i] turns an offset on line i back into one in body.
	offsets := make([]int, len(lines))
	original, expanded := 0, 0
	for i, line := range lines {
		width := len(expandLead(line)) - len(line)
		offsets[i] = original - expanded - width
		original += len(line) + 1
		expanded += len(line) + width + 1
	}
	_, spans, ok := linkSpans(lines, halt)
	if !ok {
		return "", false
	}
	var out strings.Builder
	last := 0
	for _, s := range spans {
		start, end := s.start+offsets[s.line-1], s.end+offsets[s.line-1]
		if start < last || end > len(body) {
			continue
		}
		match := pathLinkRe.FindStringSubmatch(body[start:end])
		if match == nil {
			continue
		}
		text, dest := match[1], match[2]
		if strings.HasPrefix(text, "/") || path.Clean(text) != path.Clean(pyJoin(base, dest)) {
			continue
		}
		text = path.Clean(text)
		if strings.Contains(text, "`") {
			continue
		}
		out.WriteString(body[last:start])
		if strings.HasSuffix(out.String(), "`") {
			out.WriteString("<!-- -->") // Keep adjacent code delimiters distinct.
		}
		out.WriteString("`" + text + "`")
		if end < len(body) && body[end] == '`' {
			out.WriteString("<!-- -->")
		}
		last = end
	}
	out.WriteString(body[last:])
	return out.String(), true
}
