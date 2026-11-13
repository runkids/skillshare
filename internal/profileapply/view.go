// Package profileapply composes profile rules with the existing skill executor.
package profileapply

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/profile"
	ssync "skillshare/internal/sync"
)

type Scope struct{ ConfigPath, ProjectRoot string }

func (s Scope) Path() string {
	if s.ProjectRoot != "" {
		return config.ProjectConfigPath(s.ProjectRoot)
	}
	if s.ConfigPath != "" {
		return s.ConfigPath
	}
	return config.ReadOnlyConfigPath()
}

// View is a read-only loaded config. Candidate copies never save shared config.
type View struct {
	Scope                          Scope
	Config                         *config.Config
	Project                        *config.ProjectConfig
	Catalog                        profile.Catalog
	State                          profile.State
	StatePath, LockPath, BackupDir string
}

func Load(scope Scope) (*View, error) {
	v := &View{Scope: scope, BackupDir: backup.BackupDir()}
	var err error
	v.StatePath, v.LockPath, err = config.ProfilePaths(scope.Path())
	if err != nil {
		return nil, err
	}
	if scope.ProjectRoot != "" {
		v.Project, err = config.LoadProjectReadOnly(scope.ProjectRoot)
		if err != nil {
			return nil, err
		}
		v.State = v.Project.ProfileState()
		v.Catalog = profile.Parse(&v.Project.Profiles)
		v.Config = projectView(scope.ProjectRoot, v.Project)
		v.BackupDir = backup.ProjectBackupDir(scope.ProjectRoot)
	} else {
		v.Config, err = config.LoadReadOnlyPath(scope.Path())
		if err != nil {
			return nil, err
		}
		v.State = v.Config.ProfileState()
		v.Catalog = profile.Parse(&v.Config.Profiles)
	}
	return v, nil
}

func projectView(root string, pc *config.ProjectConfig) *config.Config {
	targets, _ := config.ResolveValidProjectTargets(root, pc)
	return &config.Config{Source: pc.EffectiveSkillsSource(root), AgentsSource: pc.EffectiveAgentsSource(root), Targets: targets,
		Mode: "merge", TargetNaming: pc.TargetNaming, Ignore: pc.Ignore, FollowSourceLinks: pc.FollowSourceLinks,
		ContextBudget: pc.ContextBudget, Profiles: pc.Profiles}
}

func (v *View) Selector() profile.Selector {
	if v.Project != nil {
		return v.Project.ProfileSelector()
	}
	return v.Config.ProfileSelector()
}

func (v *View) ActiveProfileError() error {
	_, err := v.Selector().Selects("")
	return err
}

func (v *View) Candidate(name string) *View {
	out := *v
	out.State.Active = name
	if v.Project != nil {
		out.Project = v.Project.WithActiveProfile(name)
		out.Config = projectView(v.Scope.ProjectRoot, out.Project)
	} else {
		out.Config = v.Config.WithActiveProfile(name)
	}
	return &out
}

func (v *View) Definitions() *yaml.Node {
	if v.Project != nil {
		return &v.Project.Profiles
	}
	return &v.Config.Profiles
}

func (v *View) Discover() (ssync.TargetDiscovery, error) {
	source := v.Config.EffectiveSkillsSource()
	// A missing or unreadable root is not an empty selection safe to prune.
	if _, err := os.ReadDir(source); err != nil {
		return ssync.TargetDiscovery{}, fmt.Errorf("read skills source %s: %w", source, err)
	}
	walk := v.Config.SkillsWalk()
	skills, stats, err := ssync.DiscoverSourceSkillsWithStatsAndContext(source, walk)
	incomplete := walk.Follow.Incomplete()
	return ssync.TargetDiscovery{Skills: skills, IgnoreStats: stats, SourceIncomplete: incomplete, Warnings: ssync.SourceLinkWarnings(walk, incomplete)}, err
}

// Info describes desired source selection; observed target counts are separate.
type Info struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Selected int    `json:"selected"`
	Total    int    `json:"total"`
	Pending  bool   `json:"pending"`
	Reason   string `json:"reason,omitempty"`
}

func (v *View) Info(skills []ssync.DiscoveredSkill) *Info {
	configured, err := config.ProfileConfigured(v.Scope.Path(), v.Catalog.Present)
	if !configured && err == nil {
		return nil
	}
	info := &Info{Name: v.State.Active, State: "none", Pending: v.State.Pending}
	if info.Name != "" {
		info.State = "active"
	}
	if err := v.ActiveProfileError(); err != nil {
		info.State, info.Reason = "unavailable", err.Error()
	}
	for _, skill := range skills {
		if skill.Disabled {
			continue
		}
		info.Total++
		if selected, err := v.Selector().Selects(skill.FlatName); err == nil && selected {
			info.Selected++
		}
	}
	return info
}

func (v *View) RestoreCommand(target, snapshot string) string {
	scope := "-g"
	if v.Scope.ProjectRoot != "" {
		scope = "-p"
	}
	return "skillshare restore " + filepath.Base(snapshot) + " --from " + filepath.Base(filepath.Dir(snapshot)) + " " + scope + " --force"
}

// TargetCounts uses the same shared-folder grouping and selection seam as sync.
func (v *View) TargetCounts(skills []ssync.DiscoveredSkill) (map[string]int, error) {
	targets, _ := v.Eligible()
	targets = config.WithSharedSkillsTargets(maps.Clone(targets), v.Config.Mode)
	counts := make(map[string]int, len(targets))
	for name, target := range targets {
		selected, err := ssync.TargetSkills(name, target, v.Config.Mode, v.Config.EffectiveSkillsSource(), skills)
		if err != nil {
			return nil, err
		}
		counts[name] = len(selected)
	}
	return counts, nil
}
