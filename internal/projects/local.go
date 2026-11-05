package projects

import (
	"fmt"
	"slices"

	"skillshare/internal/config"
)

// PrepareLocalProjects shares project validation and affected-root reporting
// with shared conversions, but persists only the machine-local overlay.
func PrepareLocalProjects(path string, cfg *config.Config, mutate func(map[string]config.ManagedProject) error, forceRoot string) (*Candidate, error) {
	c := &Candidate{}
	if cfg == nil {
		c.Err = fmt.Errorf("cannot prepare projects for a nil config")
		return c, c.Err
	}
	entries := cloneProjects(cfg.Projects)
	if err := mutate(entries); err != nil {
		c.Err = err
		return c, err
	}
	if err := validateProjectFilters(entries); err != nil {
		c.Err = err
		return c, err
	}
	c.Save, c.Err = config.PrepareLocalProjects(path, cfg, entries, forceRoot)
	if c.Err != nil {
		return c, c.Err
	}
	c.Affected = affectedProjects(cfg.Projects, c.Save.Expanded.Projects)
	if forceRoot != "" {
		before, _ := cfg.ProjectOrigin(forceRoot)
		after, _ := c.Save.Expanded.ProjectOrigin(forceRoot)
		if before != after && !slices.Contains(c.Affected, forceRoot) {
			c.Affected = append(c.Affected, forceRoot)
			slices.Sort(c.Affected)
		}
	}
	return c, nil
}

func configItem(cfg *config.Config, root string, p config.ManagedProject) Item {
	v := item(root, p)
	v.Origin, v.OriginPath = cfg.ProjectOrigin(root)
	return v
}

// Localize adopts a shared declaration on this machine without deleting shared
// intent, detaching destinations or synchronizing files. Repeats are noops.
func (s *Service) Localize(key string, dryRun bool) (Result, error) {
	cfg, err := s.load()
	if err != nil {
		return Result{}, err
	}
	root, err := lookup(cfg, key, false)
	if err != nil {
		return Result{}, err
	}
	c, err := PrepareLocalProjects(s.path, cfg, func(map[string]config.ManagedProject) error { return nil }, root)
	if err != nil {
		return Result{}, err
	}
	v := configItem(c.Save.Expanded, root, c.Save.Expanded.Projects[root])
	result := Result{Item: &v, Affected: c.Affected, Expanded: c.Save.Expanded, Registration: "kept"}
	if !dryRun && len(c.Affected) > 0 {
		if err := c.Save.Write(); err != nil {
			return result, fmt.Errorf("%w: %w", ErrSave, err)
		}
		result.Registration = "updated"
	}
	return result, nil
}
