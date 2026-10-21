package projects

import (
	"fmt"
	"reflect"
	"slices"
	"sort"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

// Candidate is a prepared project edit and its effective affected roots.
type Candidate struct {
	Save     *config.SaveCandidate
	Affected []string
	Err      error
}

// PrepareProjects edits an isolated copy and validates the effective YAML that
// Save.Write will persist. Anchor and merge propagation comes from PrepareSave,
// so Affected includes inherited changes as well as directly edited roots.
func PrepareProjects(path string, cfg *config.Config, mutate func(map[string]config.ManagedProject) error) (*Candidate, error) {
	c := &Candidate{}
	if cfg == nil {
		c.Err = fmt.Errorf("cannot prepare projects for a nil config")
		return c, c.Err
	}
	next := *cfg
	next.Projects = cloneProjects(cfg.Projects)
	if err := mutate(next.Projects); err != nil {
		c.Err = err
		return c, err
	}
	c.Save, c.Err = config.PrepareSave(path, &next)
	if c.Err != nil {
		return c, c.Err
	}
	expanded := c.Save.Expanded
	c.Affected = affectedProjects(cfg.Projects, expanded.Projects)
	c.Err = expanded.ValidateProjects(expanded.Projects)
	if c.Err == nil {
		c.Err = validateProjectFilters(expanded.Projects)
	}
	// Project validation belongs here rather than in config's general serializer,
	// but its error must also prevent callers from writing this Save directly.
	c.Save.Err = c.Err
	return c, c.Err
}

func cloneProjects(projects map[string]config.ManagedProject) map[string]config.ManagedProject {
	cloned := make(map[string]config.ManagedProject, len(projects))
	for root, p := range projects {
		p.Targets = slices.Clone(p.Targets)
		p.Skills = cloneProjectResource(p.Skills)
		p.Agents = cloneProjectResource(p.Agents)
		cloned[root] = p
	}
	return cloned
}

func cloneProjectResource(rc *config.ResourceTargetConfig) *config.ResourceTargetConfig {
	if rc == nil {
		return nil
	}
	cloned := *rc
	cloned.Include = slices.Clone(rc.Include)
	cloned.Exclude = slices.Clone(rc.Exclude)
	if rc.Enabled != nil {
		enabled := *rc.Enabled
		cloned.Enabled = &enabled
	}
	return &cloned
}

func affectedProjects(before, after map[string]config.ManagedProject) []string {
	var affected []string
	for root, old := range before {
		next, exists := after[root]
		if !exists || !sameProject(old, next) {
			affected = append(affected, root)
		}
	}
	for root := range after {
		if _, exists := before[root]; !exists {
			affected = append(affected, root)
		}
	}
	sort.Strings(affected)
	return affected
}

func sameProject(a, b config.ManagedProject) bool {
	return a.Name == b.Name && slices.Equal(a.Targets, b.Targets) &&
		sameProjectResource(a.Skills, b.Skills) && sameProjectResource(a.Agents, b.Agents)
}

func sameProjectResource(a, b *config.ResourceTargetConfig) bool {
	if a == nil || b == nil {
		return a == b
	}
	// Empty lists are omitted during serialization. They mean the same thing as
	// nil, while a nil resource is different from an enabled empty resource.
	x, y := *a, *b
	x.Include, x.Exclude = nil, nil
	y.Include, y.Exclude = nil, nil
	return reflect.DeepEqual(x, y) && slices.Equal(a.Include, b.Include) && slices.Equal(a.Exclude, b.Exclude)
}

func validateProjectFilters(projects map[string]config.ManagedProject) error {
	roots := make([]string, 0, len(projects))
	for root := range projects {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	for _, root := range roots {
		p := projects[root]
		for i, rc := range []*config.ResourceTargetConfig{p.Skills, p.Agents} {
			if rc == nil {
				continue
			}
			if _, err := ssync.FilterSkills(nil, rc.Include, rc.Exclude); err != nil {
				return fmt.Errorf("projects: %s: %s: %w", root, []string{"skills", "agents"}[i], err)
			}
		}
	}
	return nil
}
