package codexskills

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"skillshare/internal/sync"
)

type Group struct {
	Name        string   `json:"name"`
	Keep        string   `json:"keep"`
	Disable     []string `json:"disable"`
	fingerprint string
}

type Skipped struct {
	Name   string   `json:"name"`
	Paths  []string `json:"paths"`
	Reason string   `json:"reason"`
}

type Plan struct {
	Groups   []Group   `json:"groups"`
	Skipped  []Skipped `json:"skipped"`
	Applied  []string  `json:"applied"`
	Backup   string    `json:"backup,omitempty"`
	Verified bool      `json:"verified"`
	Error    string    `json:"error,omitempty"`
}

// BuildPlan leaves conflicting names alone, including differences in supporting
// files and invocation metadata. Source entries win over identical projections.
func BuildPlan(skills []Skill, source string) (Plan, error) {
	p := Plan{Groups: []Group{}, Skipped: []Skipped{}, Applied: []string{}}
	var err error
	if source != "" {
		source, err = filepath.EvalSymlinks(source)
		if err != nil {
			return p, err
		}
	}
	byName := map[string][]Skill{}
	seen := map[string]bool{}
	for _, s := range skills {
		if !s.Enabled {
			continue
		}
		path, err := filepath.EvalSymlinks(s.Path)
		if err != nil {
			return p, fmt.Errorf("resolve %s: %w", s.Path, err)
		}
		if seen[path] {
			continue // One canonical file cannot be disabled per alias.
		}
		seen[path] = true
		s.Path = path
		byName[s.Name] = append(byName[s.Name], s)
	}
	var names []string
	for name, entries := range byName {
		if len(entries) > 1 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		entries := byName[name]
		sort.Slice(entries, func(i, j int) bool {
			a, b := within(source, entries[i].Path), within(source, entries[j].Path)
			if a != b {
				return a
			}
			depthA, depthB := strings.Count(entries[i].Path, string(filepath.Separator)), strings.Count(entries[j].Path, string(filepath.Separator))
			if depthA != depthB {
				return depthA < depthB
			}
			return entries[i].Path < entries[j].Path
		})
		var paths []string
		var first, reason string
		for _, s := range entries {
			paths = append(paths, s.Path)
			if s.Scope != "user" || s.PluginID != "" {
				reason = "includes a repository, system, or plugin skill; review manually"
				continue
			}
			sum, err := fingerprint(filepath.Dir(s.Path))
			if err != nil {
				reason = err.Error()
				continue
			}
			if first == "" {
				first = sum
			} else if first != sum {
				reason = "skill folders differ (instructions, supporting files, metadata, or executable bits)"
			}
		}
		if reason != "" {
			p.Skipped = append(p.Skipped, Skipped{Name: name, Paths: paths, Reason: reason})
			continue
		}
		p.Groups = append(p.Groups, Group{Name: name, Keep: paths[0], Disable: paths[1:], fingerprint: first})
	}
	return p, nil
}

func within(root, path string) bool {
	if root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func fingerprint(dir string) (string, error) {
	// Reuse sync's content checksum, and also account for executable permissions.
	// Internal symlinks and parent-relative dependencies need a human review.
	modes := sha256.New()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			fmt.Fprintf(modes, "dir:%s\x00", rel)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("folder contains a symlink or special file; review manually")
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(modes, "%s\x00%o\x00", rel, info.Mode().Perm()&0111)
		if strings.EqualFold(filepath.Ext(path), ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), "../") || strings.Contains(string(content), `..\`) {
				return fmt.Errorf("folder has parent-relative Markdown references; review manually")
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sum, err := sync.DirChecksum(dir)
	return fmt.Sprintf("%s:%x", sum, modes.Sum(nil)), err
}

type Catalog interface {
	List(cwd string) ([]Skill, error)
	Disable(path string) error
}

// Apply verifies the preview against fresh state before writing. Each native
// config edit preserves unrelated settings; failures report acknowledged writes
// and the backup instead of blindly restoring over possible concurrent edits.
func Apply(c Catalog, cwd, source, configPath string, preview Plan) (Plan, error) {
	before, err := c.List(cwd)
	if err != nil {
		return preview, err
	}
	fresh, err := BuildPlan(before, source)
	if err != nil {
		return preview, err
	}
	if !reflect.DeepEqual(fresh.Groups, preview.Groups) || !reflect.DeepEqual(fresh.Skipped, preview.Skipped) {
		return preview, fmt.Errorf("skill catalog changed after preview; run again to review the new plan")
	}
	if len(preview.Groups) == 0 {
		preview.Verified = true
		return preview, nil
	}
	preview.Backup, err = backupConfig(configPath)
	if err != nil {
		return preview, err
	}
	disabled := map[string]bool{}
	for _, group := range preview.Groups {
		for _, path := range group.Disable {
			if err := c.Disable(path); err != nil {
				return preview, fmt.Errorf("write outcome for %s is unverified: %w; inspect Codex settings and backup %s", path, err, preview.Backup)
			}
			preview.Applied = append(preview.Applied, path)
			disabled[path] = true
		}
	}
	after, err := c.List(cwd)
	if err != nil {
		return preview, fmt.Errorf("settings written but catalog verification failed: %w", err)
	}
	states := map[string]bool{}
	for _, s := range after {
		path, err := filepath.EvalSymlinks(s.Path)
		if err != nil {
			return preview, err
		}
		states[path] = s.Enabled
	}
	for _, s := range before {
		path, err := filepath.EvalSymlinks(s.Path)
		if err != nil {
			return preview, err
		}
		want := s.Enabled && !disabled[path]
		if enabled, ok := states[path]; !ok || enabled != want {
			return preview, fmt.Errorf("settings written but unexpected enabled state for %s; inspect Codex settings and backup %s", s.Path, preview.Backup)
		}
	}
	preview.Verified = true
	return preview, nil
}

func backupConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "config.toml.skillshare-dedup-backup-*")
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return f.Name(), writeErr
	}
	return f.Name(), closeErr
}
