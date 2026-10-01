package hooks

import (
	"path/filepath"
	"strings"
)

// GitInfo describes config-hook capabilities and the include activation boundary.
// Hooks-directory discovery and tool recognizers are intentionally separate.
type GitInfo struct {
	Version     string         `json:"version"`
	ConfigHooks bool           `json:"configHooks"`
	Parallel    bool           `json:"parallel"`
	HooksFile   string         `json:"hooksFile"`
	Include     GitIncludeInfo `json:"include"`
	HooksPath   GitHooksPath   `json:"hooksPath"`
	Reason      string         `json:"reason,omitempty"`
}

type GitIncludeInfo struct {
	Target      string `json:"target"`
	Resolved    string `json:"resolved"`
	Present     bool   `json:"present"`
	Owned       bool   `json:"owned"`
	Writable    bool   `json:"writable"`
	Lines       string `json:"lines"`
	Conditional bool   `json:"conditional,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type GitHooksPath struct {
	Value  string `json:"value"`
	Origin string `json:"origin"`
}

func (s *Service) gitInventory(root string, state ledger) *GitInfo {
	info := &GitInfo{}
	d, err := s.gitDestination(root)
	if err != nil {
		info.Reason = err.Error()
		return info
	}
	info.HooksFile = d.hooksFile
	resolved, err := canonicalPath(d.includeTarget)
	if err != nil {
		resolved = d.includeTarget
	}
	info.Include = GitIncludeInfo{Target: d.includeTarget, Resolved: resolved, Writable: gitWritable(d.includeTarget), Lines: includeLines(d.includeValue)}
	snapshot, err := s.gitSnapshot(d)
	if err != nil {
		info.Reason = "Git configuration unavailable"
		return info
	}
	info.Version, info.ConfigHooks, info.Parallel = snapshot.version, gitAtLeast(snapshot.version, 54), gitAtLeast(snapshot.version, 55)
	for _, row := range snapshot.includes {
		if sameGitPath(s.resolveGitInclude(row), d.hooksFile) {
			info.Include.Present = true
			info.Include.Conditional = info.Include.Conditional || row.Conditional
		}
	}
	owner, _ := filepath.Abs(s.ConfigPath)
	key := elementKey(owner, d.identity, d.includeTarget, "include.path", "", 0)
	_, info.Include.Owned = state.Records[key]
	if info.Include.Present && (!info.Include.Conditional || s.ProjectRoot != "") {
		scope := []string{}
		if s.ProjectRoot == "" {
			scope = []string{"--global"}
		}
		rows, err := s.gitRead(s.ProjectRoot, scope, `^hook\.`)
		if err == nil {
			active := false
			for _, row := range rows {
				active = active || sameGitPath(row.Origin, d.hooksFile)
			}
			info.Include.Active = &active
		}
	}
	if len(snapshot.hooksPath) > 0 {
		row := snapshot.hooksPath[len(snapshot.hooksPath)-1]
		info.HooksPath = GitHooksPath{Value: row.Value, Origin: "file:" + row.Origin}
	}
	return info
}

func (s *Service) gitUnmanaged(state ledger) []Unmanaged {
	d, err := s.gitDestination("")
	if err != nil {
		return nil
	}
	snapshot, err := s.gitSnapshot(d)
	if err != nil {
		return nil
	}
	names := map[string]map[string]bool{}
	for _, row := range snapshot.hooks {
		key := strings.TrimPrefix(row.Key, "hook.")
		name, field, ok := strings.Cut(key, ".")
		// Friendly names may contain dots; the final key component is the field.
		if i := strings.LastIndex(key, "."); i >= 0 {
			name, field, ok = key[:i], key[i+1:], true
		}
		if !ok || field != "command" && field != "event" {
			continue
		}
		owned := false
		for _, r := range state.Records {
			if r.Target == "git" && sameGitPath(r.Path, row.Origin) && r.Event == "hook."+name {
				owned = true
			}
		}
		if owned {
			continue
		}
		if names[row.Origin] == nil {
			names[row.Origin] = map[string]bool{}
		}
		names[row.Origin][name] = true
	}
	var out []Unmanaged
	for _, origin := range sortedKeys(names) {
		out = append(out, Unmanaged{Target: "git", Path: origin, Names: sortedKeys(names[origin])})
	}
	return out
}
