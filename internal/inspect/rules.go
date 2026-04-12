package inspect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxRuleFileSize = 512 * 1024

type ruleLocation struct {
	sourceTool string
	scope      Scope
	path       string
	walk       bool
}

func ScanRules(projectRoot string) ([]RuleItem, []string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve home directory: %w", err)
	}

	root := strings.TrimSpace(projectRoot)
	if root != "" {
		root, err = filepath.Abs(root)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve project root: %w", err)
		}
	}
	overlapHomeProject := root != "" && sameResolvedPath(home, root)

	var locations []ruleLocation
	if !overlapHomeProject {
		locations = append(locations,
			ruleLocation{sourceTool: "claude", scope: ScopeUser, path: filepath.Join(home, ".claude", "CLAUDE.md")},
			ruleLocation{sourceTool: "codex", scope: ScopeUser, path: filepath.Join(home, ".codex", "AGENTS.md")},
			ruleLocation{sourceTool: "gemini", scope: ScopeUser, path: filepath.Join(home, ".gemini", "GEMINI.md")},
			ruleLocation{sourceTool: "claude", scope: ScopeUser, path: filepath.Join(home, ".claude", "rules"), walk: true},
		)
	}

	if root != "" {
		locations = append(locations,
			ruleLocation{sourceTool: "claude", scope: ScopeProject, path: filepath.Join(root, "CLAUDE.md")},
			ruleLocation{sourceTool: "codex", scope: ScopeProject, path: filepath.Join(root, "AGENTS.md")},
			ruleLocation{sourceTool: "gemini", scope: ScopeProject, path: filepath.Join(root, "GEMINI.md")},
			ruleLocation{sourceTool: "pi", scope: ScopeProject, path: filepath.Join(root, ".pi", "SYSTEM.md")},
			ruleLocation{sourceTool: "pi", scope: ScopeProject, path: filepath.Join(root, ".pi", "APPEND_SYSTEM.md")},
			ruleLocation{sourceTool: "claude", scope: ScopeProject, path: filepath.Join(root, ".claude", "CLAUDE.md")},
			ruleLocation{sourceTool: "codex", scope: ScopeProject, path: filepath.Join(root, ".codex", "AGENTS.md")},
			ruleLocation{sourceTool: "gemini", scope: ScopeProject, path: filepath.Join(root, ".gemini", "GEMINI.md")},
			ruleLocation{sourceTool: "claude", scope: ScopeProject, path: filepath.Join(root, ".claude", "rules"), walk: true},
			ruleLocation{sourceTool: "gemini", scope: ScopeProject, path: filepath.Join(root, ".gemini", "rules"), walk: true},
		)
	}

	var (
		items    []RuleItem
		warnings []string
	)

	for _, loc := range locations {
		if loc.walk {
			files := collectRegularFiles(loc.path, &warnings)
			for _, file := range files {
				item, warn, ok := readRuleItem(file, loc.sourceTool, loc.scope)
				if warn != "" {
					warnings = append(warnings, warn)
				}
				if !ok {
					continue
				}
				items = append(items, item)
			}
			continue
		}

		item, warn, ok := readRuleItem(loc.path, loc.sourceTool, loc.scope)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if !ok {
			continue
		}
		items = append(items, item)
	}

	items = dedupeRuleItems(items)

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		if items[i].SourceTool != items[j].SourceTool {
			return items[i].SourceTool < items[j].SourceTool
		}
		return items[i].Name < items[j].Name
	})

	return items, dedupeWarnings(warnings), nil
}

func dedupeRuleItems(items []RuleItem) []RuleItem {
	deduped := make([]RuleItem, 0, len(items))
	byPath := make(map[string]int, len(items))

	for _, item := range items {
		path := item.Path
		if !filepath.IsAbs(path) {
			if absPath, err := filepath.Abs(path); err == nil {
				path = absPath
				item.Path = absPath
			}
		}

		if idx, ok := byPath[path]; ok {
			existing := deduped[idx]
			if existing.Scope == ScopeUser && item.Scope == ScopeProject {
				deduped[idx] = item
			}
			continue
		}

		byPath[path] = len(deduped)
		deduped = append(deduped, item)
	}

	return deduped
}

func readRuleItem(path, sourceTool string, scope Scope) (RuleItem, string, bool) {
	data, warn, ok := readValidatedRegularFile(path, "rule file", maxRuleFileSize)
	if warn != "" {
		return RuleItem{}, warn, false
	}
	if !ok {
		return RuleItem{}, "", false
	}
	if !isLikelyTextRuleContent(data) {
		return RuleItem{}, fmt.Sprintf("%s: skipped non-text rule file", path), false
	}

	item := RuleItem{
		Name:        filepath.Base(path),
		ID:          stableDiscoveryID("rule", sourceTool, string(scope), resolvedComparablePath(path)),
		SourceTool:  sourceTool,
		Scope:       scope,
		Path:        path,
		Exists:      true,
		Collectible: true,
		Content:     string(data),
		Size:        int64(len(data)),
	}

	scopedPaths, warn := parseRuleFrontmatter(path, data)
	item.ScopedPaths = scopedPaths
	item.IsScoped = len(scopedPaths) > 0
	if warn != "" {
		return item, warn, true
	}

	return item, "", true
}

func readValidatedRegularFile(path, kind string, maxSize int64) ([]byte, string, bool) {
	file, err := openReadOnlyFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", false
		}
		return nil, fmt.Sprintf("%s: %v", path, err), false
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Sprintf("%s: %v", path, err), false
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Sprintf("%s: skipped non-regular %s", path, kind), false
	}
	if stat.Size() > maxSize {
		return nil, fmt.Sprintf("%s: skipped oversized %s (%d bytes)", path, kind, stat.Size()), false
	}

	limited := io.LimitedReader{R: file, N: maxSize + 1}
	data, err := io.ReadAll(&limited)
	if err != nil {
		return nil, fmt.Sprintf("%s: %v", path, err), false
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Sprintf("%s: skipped oversized %s (%d bytes)", path, kind, len(data)), false
	}
	return data, "", true
}

func isLikelyTextRuleContent(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	if bytes.Contains(data, []byte{0x00}) {
		return false
	}
	if hasBinaryMagicPrefix(data) {
		return false
	}

	sample := data
	if len(sample) > 1024 {
		sample = sample[:1024]
	}

	var suspicious int
	for _, b := range sample {
		switch {
		case b == '\n', b == '\r', b == '\t':
		case b >= 0x20 && b != 0x7f:
		default:
			suspicious++
		}
	}

	return suspicious*8 <= len(sample)
}

func hasBinaryMagicPrefix(data []byte) bool {
	signatures := [][]byte{
		[]byte("%PDF-"),
		[]byte("\x7fELF"),
		[]byte("PK\x03\x04"),
		[]byte("\x89PNG"),
		[]byte("GIF87a"),
		[]byte("GIF89a"),
		[]byte("\xff\xd8\xff"),
		[]byte("\x1f\x8b"),
	}
	for _, signature := range signatures {
		if bytes.HasPrefix(data, signature) {
			return true
		}
	}
	return false
}

func parseRuleFrontmatter(path string, data []byte) ([]string, string) {
	raw, ok, hasFrontmatter := extractFrontmatterRaw(string(data))
	if !hasFrontmatter {
		return nil, ""
	}
	if !ok {
		return nil, fmt.Sprintf("%s: invalid frontmatter: missing closing delimiter", path)
	}

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(raw), &fm); err != nil {
		return nil, fmt.Sprintf("%s: invalid frontmatter: %v", path, err)
	}

	val, ok := fm["paths"]
	if !ok || val == nil {
		return nil, ""
	}

	switch v := val.(type) {
	case []any:
		if len(v) == 0 {
			return nil, ""
		}
		paths := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return nil, fmt.Sprintf("%s: invalid paths frontmatter: expected string list", path)
			}
			paths = append(paths, s)
		}
		return paths, ""
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, ""
		}
		return []string{v}, ""
	default:
		return nil, fmt.Sprintf("%s: unsupported paths frontmatter type %T", path, val)
	}
}

func extractFrontmatterRaw(content string) (string, bool, bool) {
	contentLines := strings.Split(content, "\n")
	var frontmatterLines []string
	inFrontmatter := false
	sawOpener := false

	for _, rawLine := range contentLines {
		line := strings.TrimSuffix(rawLine, "\r")
		if !sawOpener {
			line = strings.TrimPrefix(line, "\ufeff")
			if strings.TrimSpace(line) == "" {
				continue
			}
			if line != "---" {
				return "", false, false
			}
			sawOpener = true
			inFrontmatter = true
			continue
		}

		if inFrontmatter && line == "---" {
			return strings.Join(frontmatterLines, "\n"), true, true
		}
		frontmatterLines = append(frontmatterLines, line)
	}

	if sawOpener {
		return "", false, true
	}
	return "", false, false
}

func collectRegularFiles(root string, warnings *[]string) []string {
	info, err := os.Stat(root)
	if err != nil {
		if !os.IsNotExist(err) {
			*warnings = append(*warnings, fmt.Sprintf("%s: %v", root, err))
		}
		return nil
	}
	if !info.IsDir() {
		return nil
	}

	var files []string
	visitedDirs := make(map[string]struct{})
	if canonical, err := filepath.EvalSymlinks(root); err == nil {
		visitedDirs[canonical] = struct{}{}
	}

	var walk func(string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("%s: %v", dir, err))
			return
		}
		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			info, err := os.Stat(path)
			if err != nil {
				*warnings = append(*warnings, fmt.Sprintf("%s: %v", path, err))
				continue
			}
			if info.IsDir() {
				canonical, err := filepath.EvalSymlinks(path)
				if err != nil {
					*warnings = append(*warnings, fmt.Sprintf("%s: %v", path, err))
					continue
				}
				if _, ok := visitedDirs[canonical]; ok {
					continue
				}
				visitedDirs[canonical] = struct{}{}
				walk(path)
				continue
			}
			if !info.Mode().IsRegular() {
				*warnings = append(*warnings, fmt.Sprintf("%s: skipped non-regular rule file", path))
				continue
			}
			files = append(files, path)
		}
	}

	walk(root)
	sort.Strings(files)
	return files
}

func dedupeWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(warnings))
	result := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		result = append(result, warning)
	}
	return result
}

func stableDiscoveryID(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(prefix + "\x1f" + strings.Join(parts, "\x1f")))
	return prefix + "_" + hex.EncodeToString(sum[:8])
}
