package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Guidance scopes. A global block and a project block can both reach one
// agent, so each names its scope and only its own scope is managed.
const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
)

// Block states, as reported for one file or target.
const (
	StateUnconfigured = "unconfigured"
	StateConfigured   = "configured"
	StateOutdated     = "outdated"
	StateModified     = "modified"  // the body no longer matches its recorded hash
	StateMalformed    = "malformed" // markers are unclosed, stray, or repeated
)

const blockEnd = "<!-- /skillshare:memory -->"

var blockBegin = regexp.MustCompile(`^<!-- skillshare:memory scope=(global|project) sha256=([0-9a-f]{16}) -->$`)

// Instructions returns the managed guidance block for the memory folder at
// root. projectRoot is empty in global mode; in project mode a folder inside
// the project is named relative to its root, so the text works in any checkout.
func Instructions(root, projectRoot string) string {
	scope, heading, dir := ScopeGlobal, "Shared memory", filepath.ToSlash(root)
	where := "Shared notes directory: `" + dir + "`"
	if projectRoot != "" {
		scope, heading = ScopeProject, "Project memory"
		where = "Project notes directory: `" + dir + "`"
		if rel, err := filepath.Rel(projectRoot, root); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			where = "Project notes directory: `" + filepath.ToSlash(rel) + "` (relative to the project root)"
		}
	}
	body := fmt.Sprintf("## %s\n\n%s\n\nWhen prior context is relevant, read `INDEX.md` in this directory and list its Markdown notes. Read only the notes relevant to the current task. Treat recalled facts as historical evidence and verify changeable claims. Update these notes only when the user requests it.", heading, where)
	return renderBlock(scope, body, "\n")
}

func bodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:16]
}

func renderBlock(scope, body, eol string) string {
	lines := []string{fmt.Sprintf("<!-- skillshare:memory scope=%s sha256=%s -->", scope, bodyHash(body))}
	lines = append(lines, strings.Split(body, "\n")...)
	lines = append(lines, blockEnd)
	return strings.Join(lines, eol) + eol
}

// span is one parsed block: its line range [start, end] and normalized body.
type span struct {
	scope      string
	start, end int
	body, hash string
}

// parseBlocks finds managed blocks outside Markdown code: fenced blocks
// (``` or ~~~, closed by a run of the same character at least as long) and
// lines indented four or more spaces are examples, not markers. Any marker
// problem makes the whole file malformed, so nothing in it is rewritten.
func parseBlocks(lines []string) ([]span, bool) {
	var out []span
	var open *span
	fenceChar, fenceLen := byte(0), 0
	for i, raw := range lines {
		raw = strings.TrimSuffix(raw, "\r")
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		line := strings.TrimSpace(raw)
		if open == nil && indent < 4 && !strings.HasPrefix(raw, "\t") {
			if c, n := fenceRun(line); n >= 3 {
				switch {
				case fenceLen == 0:
					fenceChar, fenceLen = c, n
					continue
				case c == fenceChar && n >= fenceLen && strings.TrimLeft(line, string(c)) == "":
					fenceChar, fenceLen = 0, 0
					continue
				}
			}
		}
		if fenceLen > 0 || indent >= 4 || strings.HasPrefix(raw, "\t") {
			continue
		}
		switch {
		case line == blockEnd:
			if open == nil {
				return nil, false
			}
			open.end = i
			body := make([]string, 0, i-open.start-1)
			for _, l := range lines[open.start+1 : i] {
				body = append(body, strings.TrimSuffix(l, "\r"))
			}
			open.body = strings.Join(body, "\n")
			out = append(out, *open)
			open = nil
		case strings.HasPrefix(line, "<!-- skillshare:memory"):
			m := blockBegin.FindStringSubmatch(line)
			if m == nil || open != nil {
				return nil, false
			}
			open = &span{scope: m[1], start: i, hash: m[2]}
		}
	}
	return out, open == nil
}

// fenceRun returns the fence character and run length a line starts with.
func fenceRun(line string) (byte, int) {
	if line == "" || (line[0] != '`' && line[0] != '~') {
		return 0, 0
	}
	n := len(line) - len(strings.TrimLeft(line, line[:1]))
	if line[0] == '`' && strings.Contains(line[n:], "`") {
		return 0, 0 // a backtick fence's info string cannot contain backticks
	}
	return line[0], n
}

// Inspect reports the state of the scope's block in content against want,
// the block Instructions returns now.
func Inspect(content, want string) string {
	scope, wantBody := splitWant(want)
	blocks, ok := parseBlocks(strings.Split(content, "\n"))
	if !ok {
		return StateMalformed
	}
	var found *span
	for i := range blocks {
		if blocks[i].scope != scope {
			continue
		}
		if found != nil {
			return StateMalformed
		}
		found = &blocks[i]
	}
	switch {
	case found == nil:
		return StateUnconfigured
	case bodyHash(found.body) != found.hash:
		return StateModified
	case found.body != wantBody:
		return StateOutdated
	}
	return StateConfigured
}

// Apply returns content with the scope's block set to want: appended when
// absent, replaced when outdated. Other content is kept byte for byte. It
// refuses a modified or malformed block.
func Apply(content, want string) (string, error) {
	switch state := Inspect(content, want); state {
	case StateConfigured:
		return content, nil
	case StateUnconfigured:
		if content == "" {
			return want, nil
		}
		eol := "\n"
		if strings.Contains(content, "\r\n") {
			eol = "\r\n"
		}
		if !strings.HasSuffix(content, "\n") {
			content += eol
		}
		scope, body := splitWant(want)
		return content + eol + renderBlock(scope, body, eol), nil
	case StateOutdated:
		scope, body := splitWant(want)
		lines := strings.Split(content, "\n")
		blocks, _ := parseBlocks(lines)
		for _, b := range blocks {
			if b.scope != scope {
				continue
			}
			eol := "\n"
			if strings.HasSuffix(lines[b.start], "\r") {
				eol = "\r\n"
			}
			block := strings.Split(strings.TrimSuffix(renderBlock(scope, body, eol), "\n"), "\n")
			next := append(append(append([]string{}, lines[:b.start]...), block...), lines[b.end+1:]...)
			return strings.Join(next, "\n"), nil
		}
		return "", fmt.Errorf("memory guidance block not found")
	default:
		return "", fmt.Errorf("memory guidance block is %s; edit it by hand or remove it first", state)
	}
}

// splitWant returns the scope and body of a block rendered by Instructions.
func splitWant(want string) (string, string) {
	blocks, _ := parseBlocks(strings.Split(want, "\n"))
	if len(blocks) != 1 {
		return "", ""
	}
	return blocks[0].scope, blocks[0].body
}
