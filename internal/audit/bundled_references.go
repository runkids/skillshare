package audit

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxReferencePath = 4096
const maxReferenceSteps = 256

// Code examples count as bundle dependencies only with these explicit prefixes.
var bundledLineSuffix = regexp.MustCompile(`:[0-9]+$`)

var bundledCodePrefixes = []string{"./", "../", "scripts/", "references/", "assets/"}

func bundledCodePath(s string) string {
	s = filepath.ToSlash(s)
	if strings.HasPrefix(s, "path:") {
		return strings.TrimPrefix(s, "path:")
	}
	s = bundledLineSuffix.ReplaceAllString(s, "")
	if strings.ContainsAny(s, " \t\r\n*?{}<>$") || len(filepath.Ext(s)) < 2 {
		return ""
	}
	for _, prefix := range bundledCodePrefixes {
		if strings.HasPrefix(s, prefix) {
			return s
		}
	}
	return ""
}

type bundledReference struct {
	markdownLink
	fromRoot bool
	implicit bool
	markdown bool
}

func extractBundledPaths(data []byte) []bundledReference {
	var refs []bundledReference
	fence, paths := "", false
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if marker, ok := detectFenceMarker(line); ok {
			if fence == "" {
				fence, paths = marker, strings.TrimSpace(strings.TrimPrefix(trimmed, marker)) == "skillshare-paths"
			} else if marker == fence {
				fence, paths = "", false
			}
			continue
		}
		add := func(target string, implicit bool) {
			if target != "" {
				portable := filepath.ToSlash(target)
				fromRoot := !strings.HasPrefix(portable, "./") && !strings.HasPrefix(portable, "../")
				refs = append(refs, bundledReference{markdownLink: markdownLink{target: target, line: i + 1, snippet: trimmed}, fromRoot: fromRoot, implicit: implicit})
			}
		}
		if fence != "" {
			if paths {
				add(strings.TrimPrefix(trimmed, "path:"), false)
			}
			continue
		}
		for _, span := range inlineCodeSpans(line) {
			code := strings.Trim(line[span.start:span.end], "`")
			add(bundledCodePath(code), !strings.HasPrefix(code, "path:"))
		}
	}
	return refs
}

func extractMarkdownImageLinks(data []byte) []markdownLink {
	lines := strings.Split(string(data), "\n")
	defs := parseReferenceDefinitions(lines)
	var links []markdownLink
	inCodeFence, fenceMarker := false, ""
	for i, line := range lines {
		if marker, ok := detectFenceMarker(line); ok {
			if !inCodeFence {
				inCodeFence, fenceMarker = true, marker
				continue
			}
			if marker == fenceMarker {
				inCodeFence, fenceMarker = false, ""
				continue
			}
		}
		if inCodeFence {
			continue
		}

		codeSpans := inlineCodeSpans(line)
		for start := 0; start+1 < len(line); start++ {
			if line[start] != '!' || line[start+1] != '[' || isEscapedMarkdownAt(line, start) || indexInSpans(start, codeSpans) {
				continue
			}
			labelEnd := findMatchingBracket(line, start+1)
			if labelEnd == -1 {
				continue
			}
			label := line[start+2 : labelEnd]
			next := labelEnd + 1
			target := ""
			if next < len(line) && line[next] == '(' {
				inside, linkEnd := readParenthesized(line, next)
				if linkEnd != -1 {
					target = parseMarkdownLinkTarget(inside)
				}
			} else if next < len(line) && line[next] == '[' {
				refEnd := findMatchingBracket(line, next)
				if refEnd != -1 {
					ref := strings.TrimSpace(line[next+1 : refEnd])
					if ref == "" {
						ref = label
					}
					target = defs[normalizeReferenceLabel(ref)]
				}
			} else if strings.TrimSpace(line[next:]) == "" && i+1 < len(lines) {
				// Preserve multiline destinations supported by the prior image
				// parser, while keeping image extraction independent of outer links.
				imageLine := line[:start] + line[start+1:]
				if multiline, ok := extractMultilineLink(imageLine, lines[i+1], i+1); ok {
					target = multiline.target
				}
				if target == "" {
					target = defs[normalizeReferenceLabel(label)]
				}
			} else {
				// Shortcut image references use the image text as the definition label.
				target = defs[normalizeReferenceLabel(label)]
			}
			if target != "" {
				links = append(links, markdownLink{
					label:   label,
					target:  target,
					line:    i + 1,
					snippet: strings.TrimSpace(line),
				})
			}
		}
	}
	return links
}

// checkBundledReferences resolves and contains each file's references within
// the skill that holds it, so a nested skill gets the same result inside a
// parent scan as when it is scanned directly.
func checkBundledReferences(root string, files []mdFileInfo, mdLinks map[string][]markdownLink, logicalRoots ...string) []Finding {
	checkers := map[string]bundledPathChecker{}
	var findings []Finding
	for _, f := range files {
		skillDir := filepath.FromSlash(f.skillDir)
		checker, ok := checkers[f.skillDir]
		if !ok {
			var owned []string
			for _, logicalRoot := range logicalRoots {
				if logicalRoot != "" {
					owned = append(owned, filepath.Join(logicalRoot, skillDir))
				}
			}
			checker = newBundledPathChecker(filepath.Join(root, skillDir), owned...)
			checkers[f.skillDir] = checker
		}
		fileDir := filepath.Dir(f.relPath)
		if skillDir != "" {
			if rel, err := filepath.Rel(skillDir, fileDir); err == nil {
				fileDir = rel
			}
		}
		links := mdLinks[f.relPath]
		if mdLinks == nil {
			links = extractMarkdownLinks(f.data)
		}
		// Images use the same target grammar; keep static URL rule behavior unchanged.
		links = append([]markdownLink(nil), links...)
		links = append(links, extractMarkdownImageLinks(f.data)...)
		refs := make([]bundledReference, 0, len(links))
		for _, link := range links {
			refs = append(refs, bundledReference{markdownLink: link, markdown: true})
		}
		refs = append(refs, extractBundledPaths(f.data)...)
		seen := map[string]bool{}
		for _, link := range refs {
			key := fmt.Sprintf("%d|%t|%t|%t|%s", link.line, link.fromRoot, link.implicit, link.markdown, link.target)
			if seen[key] {
				continue
			}
			seen[key] = true
			target := link.target
			// A drive letter is a path, not a URI scheme, on every host.
			drive := isDriveReference(target)
			if !drive && isExternalOrAnchor(target) {
				continue
			}
			raw := stripFragment(target)
			if raw == "" {
				continue
			}
			inspect := func(path string) string {
				if !link.fromRoot && !strings.HasPrefix(filepath.ToSlash(path), "/") && !filepath.IsAbs(path) && !isDriveReference(path) && !strings.HasPrefix(path, "~/") {
					path = filepath.Join(fileDir, ".") + string(filepath.Separator) + path
				}
				return checker.inspect(path)
			}
			status := ""
			if decoded, err := url.PathUnescape(raw); link.markdown && err == nil && decoded != raw {
				status = inspect(decoded)
				if status == "dangling-link" {
					if rawStatus := inspect(raw); rawStatus != "dangling-link" {
						status = rawStatus
					}
				}
			} else {
				status = inspect(raw)
			}
			if status == "" || (link.implicit && status == "reference-escapes-root") {
				continue
			}
			severity, message := SeverityLow, "broken local link"
			if status == "reference-escapes-root" {
				severity, message = SeverityHigh, "reference escapes skill root"
			}
			if status == "reference-check-incomplete" {
				message = "reference could not be checked (unreadable, unsupported, or resolution limit)"
			}
			message = fmt.Sprintf("%s: %q", message, link.target)
			if status == "dangling-link" {
				message += " not found"
			}
			findings = append(findings, Finding{Severity: severity, Pattern: status, RuleID: status, Message: message, File: f.relPath, Line: link.line, Snippet: link.snippet, Analyzer: AnalyzerStructure, Category: CategoryStructure, Confidence: 1})
		}
	}
	return findings
}

func isDriveReference(target string) bool {
	return len(target) > 1 && target[1] == ':' && (target[0] >= 'A' && target[0] <= 'Z' || target[0] >= 'a' && target[0] <= 'z')
}

type bundledPathChecker struct {
	root      string
	spellings []string
}

func newBundledPathChecker(root string, logicalRoots ...string) bundledPathChecker {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return bundledPathChecker{}
	}
	// Resolve only the declared root and its ancestors, never a referenced
	// target. Cache equivalent root spellings once per scan.
	physicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return bundledPathChecker{}
	}
	spellings := []string{absoluteRoot, physicalRoot}
	for _, logicalRoot := range logicalRoots {
		if logicalRoot == "" {
			continue
		}
		logicalAbsolute, err := filepath.Abs(logicalRoot)
		if err != nil {
			continue
		}
		logicalPhysical, err := filepath.EvalSymlinks(logicalAbsolute)
		if err != nil {
			continue
		}
		if relative, ok := bundledAbsoluteRelative(physicalRoot, logicalPhysical); ok && relative == "." {
			spellings = append(spellings, logicalAbsolute)
		}
	}
	for parent, steps := filepath.Dir(absoluteRoot), 0; steps < maxReferenceSteps; steps++ {
		physicalParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return bundledPathChecker{}
		}
		// Preserve a symlinked parent's spelling with the physical root's
		// suffix (e.g. /var/.../bundle instead of /private/var/.../bundle).
		if suffix, ok := bundledAbsoluteRelative(physicalParent, physicalRoot); ok {
			spellings = append(spellings, filepath.Join(parent, suffix))
		}
		if suffix, ok := bundledAbsoluteRelative(parent, absoluteRoot); ok {
			spellings = append(spellings, filepath.Join(physicalParent, suffix))
		}
		next := filepath.Dir(parent)
		if next == parent {
			return bundledPathChecker{root: absoluteRoot, spellings: spellings}
		}
		parent = next
	}
	return bundledPathChecker{}
}

func (c bundledPathChecker) absoluteRelative(target string) (string, bool) {
	for _, root := range c.spellings {
		if relative, ok := bundledAbsoluteRelative(root, target); ok {
			return relative, true
		}
	}
	return "", false
}

func inspectBundledPath(root, target string) string {
	return newBundledPathChecker(root).inspect(target)
}

// inspect never opens a referenced file. Root's containment checks also
// protect component operations against concurrent symlink replacement.
func (c bundledPathChecker) inspect(target string) string {
	target = filepath.ToSlash(target)
	if (strings.HasPrefix(target, "/") || isDriveReference(target)) && !filepath.IsAbs(target) || strings.HasPrefix(target, "~/") {
		return "reference-escapes-root"
	}
	if len(target) > maxReferencePath || strings.ContainsAny(target, "\x00\\") {
		return "reference-check-incomplete"
	}
	if c.root == "" {
		return "reference-check-incomplete"
	}
	if filepath.IsAbs(target) {
		relative, ok := c.absoluteRelative(target)
		if !ok {
			return "reference-escapes-root"
		}
		target = relative
	}
	dir, err := os.OpenRoot(c.root)
	if err != nil {
		return "reference-check-incomplete"
	}
	defer dir.Close()
	parts := strings.Split(filepath.ToSlash(target), "/")
	var resolved []string
	links := 0
	for step := 0; len(parts) > 0; step++ {
		if step >= maxReferenceSteps {
			return "reference-check-incomplete"
		}
		part := parts[0]
		parts = parts[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(resolved) == 0 {
				return "reference-escapes-root"
			}
			resolved = resolved[:len(resolved)-1]
			continue
		}
		name := filepath.Join(append(append([]string(nil), resolved...), part)...)
		info, err := dir.Lstat(name)
		if os.IsNotExist(err) {
			return "dangling-link"
		}
		if err != nil {
			return "reference-check-incomplete"
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			links++
			if links > 40 {
				return "reference-check-incomplete"
			}
			dest, err := dir.Readlink(name)
			if err != nil {
				return "reference-check-incomplete"
			}
			dest = filepath.ToSlash(dest)
			if (strings.HasPrefix(dest, "/") || isDriveReference(dest)) && !filepath.IsAbs(dest) {
				return "reference-escapes-root"
			}
			if filepath.IsAbs(dest) {
				relative, ok := c.absoluteRelative(dest)
				if !ok {
					return "reference-escapes-root"
				}
				resolved, dest = nil, relative
			}
			if len(dest) > maxReferencePath || strings.ContainsAny(dest, "\x00\\") {
				return "reference-check-incomplete"
			}
			parts = append(strings.Split(filepath.ToSlash(dest), "/"), parts...)
			continue
		}
		if len(parts) > 0 && !info.IsDir() {
			return "dangling-link"
		}
		resolved = append(resolved, part)
	}
	return ""
}

// Strip only the declared root prefix. filepath.Rel would clean away symlink/..
// pairs before the component walker can inspect them.
func bundledAbsoluteRelative(root, target string) (string, bool) {
	root, target = filepath.ToSlash(root), filepath.ToSlash(target)
	equal := func(a, b string) bool {
		if filepath.Separator == '\\' {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	if equal(root, target) {
		return ".", true
	}
	prefix := strings.TrimRight(root, "/") + "/"
	if len(target) < len(prefix) || !equal(target[:len(prefix)], prefix) {
		return "", false
	}
	return target[len(prefix):], true
}
