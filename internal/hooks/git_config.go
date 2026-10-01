package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type gitConfigValue struct {
	Scope, Origin, Key, Value string
	HasValue                  bool
	Conditional               bool    `json:"conditional,omitempty"`
	ExpandedPath              *string `json:"expandedPath,omitempty"`
}
type gitSnapshot struct {
	version                    string
	hooks, includes, hooksPath []gitConfigValue
}

type gitGuard struct {
	service                        *Service
	destination                    gitDestination
	expected                       gitSnapshot
	activationPath, activationHash string
	activationExists               bool
}

func gitHookKey(key string) (name, field string, ok bool) {
	key, ok = strings.CutPrefix(key, "hook.")
	if !ok {
		return "", "", false
	}
	i := strings.LastIndex(key, ".")
	if i < 0 {
		return "", "", false
	}
	return key[:i], key[i+1:], true
}

func (g *gitGuard) check() error {
	now, err := g.service.gitSnapshot(g.destination)
	if err != nil {
		return err
	}
	if now.digest() != g.expected.digest() {
		return fmt.Errorf("Git configuration changed since preview: %w", ErrStaleRevision)
	}
	return g.checkActivation()
}

func (g *gitGuard) completed(f *filePlan) error {
	now, err := g.service.gitSnapshot(g.destination)
	if err != nil {
		return err
	}
	// A global write can also change a project's effective configuration. Accept
	// only this operation's rows; retain every other preview observation.
	allowedHook := func(row gitConfigValue) bool {
		if f.include != nil {
			path := f.gitService.resolveGitInclude(gitConfigValue{Origin: f.path, Value: f.include.Value})
			return sameGitPath(row.Origin, path)
		}
		if !sameGitPath(row.Origin, f.path) {
			return false
		}
		if f.gitSection != nil {
			name, _, ok := gitHookKey(row.Key)
			return ok && name == f.gitSection.Name
		}
		return true
	}
	allowedInclude := func(row gitConfigValue) bool {
		return f.include != nil && sameGitPath(row.Origin, f.path) && row.Key == "include.path" && row.Value == f.include.Value
	}
	strip := func(snapshot gitSnapshot) gitSnapshot {
		snapshot.hooks = slices.DeleteFunc(slices.Clone(snapshot.hooks), allowedHook)
		snapshot.includes = slices.DeleteFunc(slices.Clone(snapshot.includes), allowedInclude)
		if len(snapshot.hooks) == 0 {
			snapshot.hooks = nil
		}
		if len(snapshot.includes) == 0 {
			snapshot.includes = nil
		}
		return snapshot
	}
	if strip(now).digest() != strip(g.expected).digest() {
		return fmt.Errorf("Git configuration changed during sync: %w", ErrStaleRevision)
	}
	g.expected = now
	return nil
}

func parseGitConfig(data []byte) ([]gitConfigValue, error) {
	if len(data) == 0 {
		return []gitConfigValue{}, nil
	}
	parts := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	if len(parts)%3 != 0 {
		return nil, fmt.Errorf("unexpected Git config output")
	}
	out := make([]gitConfigValue, 0, len(parts)/3)
	for i := 0; i < len(parts); i += 3 {
		key, value, has := strings.Cut(parts[i+2], "\n")
		origin, ok := strings.CutPrefix(parts[i+1], "file:")
		if !ok {
			origin = parts[i+1]
		}
		out = append(out, gitConfigValue{Scope: parts[i], Origin: origin, Key: key, Value: value, HasValue: has})
	}
	return out, nil
}

func (s *Service) gitRead(dir string, scope []string, pattern string) ([]gitConfigValue, error) {
	return s.gitReadIncludes(dir, scope, pattern, true)
}

func (s *Service) gitReadIncludes(dir string, scope []string, pattern string, includes bool) ([]gitConfigValue, error) {
	args := append([]string{"config"}, scope...)
	flag := "--no-includes"
	if includes {
		flag = "--includes"
	}
	args = append(args, flag, "--null", "--show-origin", "--show-scope", "--get-regexp", pattern)
	out, err := s.gitRunner().Run(dir, nil, args...)
	if gitNotFoundValue(err) {
		return []gitConfigValue{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := parseGitConfig(out)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if !filepath.IsAbs(rows[i].Origin) {
			// Git reports ordinary checkout origins relative to its command directory.
			// Retain lexical symlinks: relative includes use the origin's directory.
			rows[i].Origin, err = filepath.Abs(filepath.Join(dir, rows[i].Origin))
			if err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func (s *Service) gitSnapshot(d gitDestination) (gitSnapshot, error) {
	var snapshot gitSnapshot
	out, err := s.gitRunner().Run("", nil, "version")
	if err != nil {
		return snapshot, err
	}
	snapshot.version = strings.TrimSpace(strings.TrimPrefix(string(out), "git version "))
	scope := []string{}
	if s.ProjectRoot == "" {
		scope = []string{"--global"}
	}
	snapshot.hooks, err = s.gitRead(s.ProjectRoot, scope, `^hook\.`)
	if err != nil {
		return snapshot, err
	}
	if s.ProjectRoot == "" {
		system, e := s.gitRead("", []string{"--system"}, `^hook\.`)
		if e != nil {
			return snapshot, e
		}
		snapshot.hooks = append(system, snapshot.hooks...)
	}
	filtered := snapshot.hooks[:0]
	for _, row := range snapshot.hooks {
		if !sameGitPath(row.Origin, d.hooksFile) {
			filtered = append(filtered, row)
		}
	}
	snapshot.hooks = filtered
	snapshot.includes, err = s.gitIncludeValues(s.ProjectRoot, scope, true)
	if err != nil {
		return snapshot, err
	}
	if s.ProjectRoot != "" {
		filtered := snapshot.includes[:0]
		for _, row := range snapshot.includes {
			if row.Scope == "local" || row.Scope == "worktree" {
				filtered = append(filtered, row)
			}
		}
		snapshot.includes = filtered
	}
	snapshot.includes, err = s.gitIncludeGraph(d, snapshot.includes)
	if err != nil {
		return snapshot, err
	}
	snapshot.hooksPath, err = s.gitRead(s.ProjectRoot, scope, `^core\.hookspath$`)
	if err == nil && s.ProjectRoot == "" {
		system, e := s.gitRead("", []string{"--system"}, `^core\.hookspath$`)
		if e != nil {
			return snapshot, e
		}
		snapshot.hooksPath = append(system, snapshot.hooksPath...)
	}
	return snapshot, err
}

func (g gitSnapshot) digest() string {
	b, _ := json.Marshal(struct {
		Version                    string
		Hooks, Includes, HooksPath []gitConfigValue
	}{g.version, g.hooks, g.includes, g.hooksPath})
	return digest(b)
}

func gitAtLeast(version string, minor int) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	major, e1 := strconv.Atoi(parts[0])
	m, e2 := strconv.Atoi(parts[1])
	return e1 == nil && e2 == nil && (major > 2 || major == 2 && m >= minor)
}

func sameGitPath(a, b string) bool {
	x, e1 := canonicalPath(a)
	y, e2 := canonicalPath(b)
	return e1 == nil && e2 == nil && x == y
}

func (s *Service) gitIncludePath(row gitConfigValue) string {
	value := row.Value
	if row.ExpandedPath != nil {
		value = *row.ExpandedPath
	}
	if strings.HasPrefix(value, "~/") {
		home, _ := s.home()
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(filepath.Dir(row.Origin), value)
	}
	return filepath.Clean(value)
}

func (s *Service) resolveGitInclude(row gitConfigValue) string {
	value := s.gitIncludePath(row)
	resolved, err := canonicalPath(value)
	if err != nil {
		return filepath.Clean(value)
	}
	return resolved
}

func gitWritable(path string) bool {
	i, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return checkPath(filepath.Dir(path), path) == nil
	}
	return err == nil && i.Mode().IsRegular() && checkPath(filepath.Dir(path), path) == nil
}

func includeLines(value string) string { return "[include]\n\tpath = " + gitQuote(value) + "\n" }

func (s *Service) gitFileIncludes(path string) ([]gitConfigValue, error) {
	return s.gitIncludeValues("", []string{"--file", path}, false)
}

func (s *Service) gitIncludeValues(dir string, scope []string, includes bool) ([]gitConfigValue, error) {
	const pattern = `^include(if\..*)?\.path$`
	rows, err := s.gitReadIncludes(dir, scope, pattern, includes)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	// Let Git expand ~user and installation-prefix paths. Retain raw values for
	// exact include edits and revisions, and compare the paired observations so
	// an intervening config edit cannot join unrelated rows.
	pathScope := append(slices.Clone(scope), "--path")
	expanded, err := s.gitReadIncludes(dir, pathScope, pattern, includes)
	if err != nil {
		return nil, err
	}
	reread, err := s.gitReadIncludes(dir, scope, pattern, includes)
	if err != nil {
		return nil, err
	}
	if gitValuesDigest(rows) != gitValuesDigest(reread) {
		return nil, fmt.Errorf("Git include declarations changed while expanding paths: %w", ErrStaleRevision)
	}
	if len(rows) != len(expanded) {
		return nil, fmt.Errorf("Git include declarations changed while reading: %w", ErrStaleRevision)
	}
	for i := range rows {
		if rows[i].Key != expanded[i].Key || rows[i].Origin != expanded[i].Origin || rows[i].Scope != expanded[i].Scope {
			return nil, fmt.Errorf("Git include declarations changed while reading: %w", ErrStaleRevision)
		}
		value := expanded[i].Value
		rows[i].ExpandedPath = &value
	}
	return rows, nil
}

// Follow include declarations structurally, including inactive includeIf
// branches. Native effective reads cannot tell us those branches already point
// at our output. Preserve their conditions instead of adding a global include.
func (s *Service) gitIncludeGraph(d gitDestination, effective []gitConfigValue) ([]gitConfigValue, error) {
	roots := []gitConfigValue{{Origin: d.includeTarget, Scope: "global"}}
	if s.ProjectRoot != "" {
		roots[0].Scope = "local"
	}
	for _, row := range effective {
		isChild := false
		for _, parent := range effective {
			isChild = isChild || sameGitPath(s.resolveGitInclude(parent), row.Origin)
		}
		if !isChild && !sameGitPath(row.Origin, d.includeTarget) {
			roots = append(roots, gitConfigValue{Origin: row.Origin, Scope: row.Scope})
		}
	}
	var out []gitConfigValue
	var walk func(string, string, bool, int) error
	walk = func(path, scope string, conditional bool, depth int) error {
		if depth > 10 {
			return fmt.Errorf("Git include nesting exceeds ten levels")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
		rows, err := s.gitFileIncludes(path)
		if err != nil {
			return err
		}
		for _, row := range rows {
			row.Scope = scope
			row.Conditional = conditional || row.Key != "include.path"
			out = append(out, row)
			if err := walk(s.gitIncludePath(row), scope, row.Conditional, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	seen := map[string]bool{}
	for _, root := range roots {
		if seen[root.Origin] {
			continue
		}
		seen[root.Origin] = true
		if err := walk(root.Origin, root.Scope, false, 0); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func gitValuesDigest(rows []gitConfigValue) string { b, _ := json.Marshal(rows); return digest(b) }
