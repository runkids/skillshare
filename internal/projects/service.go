package projects

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/config"
	"skillshare/internal/projectdir"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

var ErrExists = errors.New("project is already declared")
var ErrFolderNotFound = errors.New("folder not found")
var ErrSave = errors.New("failed to save config")
var ErrDetach = errors.New("failed to detach skills")

// PutOptions controls creation and preparation without persistence.
// Detach also detaches the skills of project targets the edit removes or
// turns off; the dashboard leaves it unset, so its saves never detach.
// Storage picks the layer of a new project (OriginShared or OriginLocal); empty
// uses projects_storage. An edit always stays in the layer that declares it.
type PutOptions struct {
	Create  bool
	DryRun  bool
	Detach  bool
	Storage string
}

// RemoveOptions requests a detach and candidate preview when DryRun is set.
// Removing a project this machine overrides drops only the local copy, so the
// config.yaml declaration applies again, unless Shared removes it as well.
type RemoveOptions struct{ DryRun, Shared bool }
type DetachReport = ssync.DetachReport

// Result retains prepared state and any completed work, including on failure.
type Result struct {
	Item         *Item
	Affected     []string
	Detach       *DetachReport
	Registration string
	Expanded     *config.Config
	Warnings     []string
	// Remaining is the config.yaml declaration that applies after Remove
	// dropped only this machine's copy.
	Remaining *Item
}

// Service owns project operations. Exact declared keys identify entries after
// adapter lookup; unmatched tokens fall back to shared name/root matching.
// Callers install Expanded only on success.
// The loader must return the selected config without writing it.
type Service struct {
	path string
	load func() (*config.Config, error)
	// DetachSkillsTarget permits adapters to retain their detach failure seam.
	DetachSkillsTarget func(string, ssync.DetachOptions) (ssync.DetachReport, error)
}

// NewService binds operations to one config path and its read-only loader.
func NewService(path string, load func() (*config.Config, error)) *Service {
	return &Service{path: path, load: load, DetachSkillsTarget: ssync.DetachSkillsTarget}
}
func resource(rc *config.ResourceTargetConfig) *Resource {
	if rc == nil {
		return nil
	}
	return &Resource{Disabled: !rc.IsEnabled(), Mode: rc.Mode, TargetNaming: rc.TargetNaming, Include: append([]string{}, rc.Include...), Exclude: append([]string{}, rc.Exclude...)}
}
func item(root string, p config.ManagedProject) Item {
	abs := config.ExpandPath(root)
	_, own := projectdir.Find(abs)
	v := Item{Root: root, Path: abs, Name: p.ProjectName(root), CustomName: p.Name, Targets: append([]string{}, p.Targets...), Skills: resource(p.Skills), Agents: resource(p.Agents), Groups: []Group{}, HasOwnConfig: own}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		v.Missing = true
	}
	groups, _ := p.ToolGroups(root)
	for _, g := range groups {
		v.Groups = append(v.Groups, Group{Target: config.ProjectTargetName(v.Name, g.Target()), Tools: g.Tools, SkillsPath: g.SkillsPath, AgentsPath: g.AgentsPath})
	}
	return v
}

// List returns the dashboard project DTOs, ordered by name and declared key.
func (s *Service) List() ([]Item, error) {
	cfg, err := s.load()
	if err != nil {
		return nil, err
	}
	items := []Item{}
	for root, p := range cfg.Projects {
		items = append(items, configItem(cfg, root, p))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Root < items[j].Root
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}
func lookup(cfg *config.Config, key string, rootOnly bool) (string, error) {
	// Exact declared keys are internal IDs already resolved by an adapter.
	if !rootOnly {
		if _, exists := cfg.Projects[key]; exists {
			return key, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if rootOnly {
		return FindRoot(cfg.Projects, key, cwd, home)
	}
	return Find(cfg.Projects, key, cwd, home)
}

// Show returns one registered project.
func (s *Service) Show(key string) (Item, error) {
	cfg, err := s.load()
	if err != nil {
		return Item{}, err
	}
	root, err := lookup(cfg, key, false)
	if err != nil {
		return Item{}, err
	}
	return configItem(cfg, root, cfg.Projects[root]), nil
}

// Put replaces a full project value and validates its effective YAML candidate.
// Like the dashboard on main, it detaches skills only when opts.Detach is set.
// An unchanged value and a dry run never write the config.
func (s *Service) Put(root string, entry config.ManagedProject, opts PutOptions) (Result, error) {
	cfg, err := s.load()
	if err != nil {
		return Result{}, err
	}
	// An adapter may already have selected one declaration by its unique name.
	// Preserve that exact key; HTTP resolves roots before calling Put.
	found := root
	if _, exists := cfg.Projects[root]; opts.Create || !exists {
		found, err = lookup(cfg, root, true)
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Result{}, err
	}
	if err == nil {
		root = found
		if opts.Create {
			return Result{}, fmt.Errorf("%w: %s", ErrExists, root)
		}
	} else if !opts.Create {
		return Result{}, fmt.Errorf("%w: %s", ErrNotFound, root)
	}
	if opts.Create {
		if info, err := os.Stat(config.ExpandPath(root)); err != nil || !info.IsDir() {
			return Result{}, fmt.Errorf("%w: %s", ErrFolderNotFound, root)
		}
	}
	storage := opts.Storage
	if !opts.Create {
		storage, _ = cfg.ProjectOrigin(root)
	} else if storage == "" {
		storage = cfg.NewProjectOrigin()
	}
	if storage != OriginShared && storage != OriginLocal {
		return Result{}, fmt.Errorf("invalid project storage %q (valid: %s, %s)", storage, OriginShared, OriginLocal)
	}
	c, err := PrepareLayers(s.path, cfg, func(l *config.ProjectLayers) error {
		if storage == OriginLocal {
			l.SetLocal(root, entry)
		} else {
			// A re-added project is no longer hidden on this machine.
			l.SetShared(root, entry)
			l.ClearLocal(root)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	old := configItem(cfg, root, cfg.Projects[root])
	result := Result{Item: &old, Affected: c.Affected, Expanded: cfg, Registration: "kept"}
	if err := c.Save.Check(); err != nil {
		return result, err
	}
	if opts.Detach {
		report, err := s.detachDeparting(cfg, c.Save.Expanded, c.Affected, opts.DryRun)
		result.Detach = &report
		if err != nil {
			return result, err
		}
	}
	if err := commit(c, opts.DryRun, &result, "updated"); err != nil {
		return result, err
	}
	v := resultItem(c.Save.Expanded, root)
	result.Item = &v
	result.Expanded = c.Save.Expanded
	return result, nil
}

// detachDeparting compares paths, rather than target names, so renamed and
// shared destinations stay owned. Effective candidates include alias changes.
func (s *Service) detachDeparting(before, after *config.Config, affected []string, dryRun bool) (DetachReport, error) {
	report := DetachReport{}
	survivors := enabledSkillTargets(after)
	seen := []string{}
	departing := map[string]string{}
	for _, root := range affected {
		project := before.Projects[root]
		groups, err := project.ToolGroups(root)
		if err != nil {
			return report, fmt.Errorf("%w: %w", ErrDetach, err)
		}
		// Declared groups retain their paths even when both resources are off.
		for _, group := range groups {
			departing[config.ProjectTargetName(project.ProjectName(root), group.Target())] = group.SkillsPath
		}
	}
	for _, name := range slices.Sorted(maps.Keys(departing)) {
		path := filepath.Clean(config.ExpandPath(departing[name]))
		if slices.ContainsFunc(seen, func(p string) bool { return utils.PathsEqual(p, path) }) {
			continue
		}
		seen = append(seen, path)
		// A synthetic probe lets the existing ownership helper compare all survivors,
		// including a target with the same name after an edit.
		probe := maps.Clone(survivors)
		probe["@departing"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: path}}
		if config.SkillsPathKeptBy(probe, "@departing", nil) != "" {
			report.Retained = append(report.Retained, path)
			continue
		}
		detached, err := s.DetachSkillsTarget(path, ssync.DetachOptions{Source: before.EffectiveSkillsSource(), DryRun: dryRun})
		report.Removed = append(report.Removed, detached.Removed...)
		report.CopiedBack = append(report.CopiedBack, detached.CopiedBack...)
		report.Retained = append(report.Retained, detached.Retained...)
		if err != nil {
			return report, fmt.Errorf("%w: %s: %w", ErrDetach, name, err)
		}
	}
	return report, nil
}

func enabledSkillTargets(cfg *config.Config) map[string]config.TargetConfig {
	targets := map[string]config.TargetConfig{}
	for name, target := range cfg.Targets {
		if !target.SkillsConfig().IsEnabled() {
			continue
		}
		if target.ProjectRoot() != "" {
			enabled := false
			for root, p := range cfg.Projects {
				if utils.PathsEqual(filepath.Clean(config.ExpandPath(root)), filepath.Clean(target.ProjectRoot())) && strings.HasPrefix(name, p.ProjectName(root)+config.ProjectTargetSep) {
					enabled = p.Skills != nil && p.Skills.IsEnabled()
					break
				}
			}
			if !enabled {
				continue
			}
		}
		targets[name] = target
	}
	return targets
}

// Remove validates before detaching skills, then persists the registration change.
// Agents stay in place; a failure preserves the completed detach report.
func (s *Service) Remove(key string, opts RemoveOptions) (Result, error) {
	cfg, err := s.load()
	if err != nil {
		return Result{}, err
	}
	root, err := lookup(cfg, key, false)
	if err != nil {
		return Result{}, err
	}
	v := configItem(cfg, root, cfg.Projects[root])
	result := Result{Item: &v, Registration: "kept"}
	keepShared := v.SharedCopy && !opts.Shared
	c, err := PrepareLayers(s.path, cfg, func(l *config.ProjectLayers) error {
		if v.Origin == OriginLocal {
			l.ClearLocal(root)
		}
		if !keepShared {
			l.DeleteShared(root)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	if err := c.Save.Check(); err != nil {
		return result, err
	}
	report, err := s.detachDeparting(cfg, c.Save.Expanded, c.Affected, opts.DryRun)
	result.Detach = &report
	if err != nil {
		return result, err
	}
	if !opts.DryRun {
		if err := c.Save.Write(); err != nil {
			return result, fmt.Errorf("%w: %w", ErrSave, err)
		}
		result.Registration = "removed"
	}
	result.Affected = c.Affected
	result.Expanded = c.Save.Expanded
	if keepShared {
		remaining := resultItem(c.Save.Expanded, root)
		result.Remaining = &remaining
	} else {
		result.Warnings = leftoverWarnings(cfg, root)
	}
	return result, nil
}
func field(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return field(node.Content[0], key)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
func leftoverWarnings(cfg *config.Config, root string) []string {
	normalizedRoot := filepath.Clean(config.ExpandPath(root))
	sections := map[string]*yaml.Node{"hooks.projects": field(&cfg.Hooks, "projects")}
	if cfg.MCP != nil {
		sections["mcp.projects"] = cfg.MCP.Projects
	}
	var warnings []string
	for _, name := range []string{"mcp.projects", "hooks.projects"} {
		node := sections[name]
		if node == nil {
			continue
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if utils.PathsEqual(filepath.Clean(config.ExpandPath(key)), normalizedRoot) {
				warnings = append(warnings, fmt.Sprintf("%s still contains root %s", name, key))
			}
		}
	}
	return warnings
}
