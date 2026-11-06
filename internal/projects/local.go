package projects

import (
	"errors"
	"fmt"
	"slices"

	"skillshare/internal/config"
)

// Origins of a project, as Config.ProjectOrigin reports them. They also name
// the layer a new project is saved in.
const (
	OriginShared = "shared"
	OriginLocal  = "local"
)

// ErrSharedDiffers stops Share from replacing different config.yaml settings
// unless the caller confirmed it.
var ErrSharedDiffers = errors.New("config.yaml declares this project with different settings")

// PrepareLayers validates an edit of both project layers. Affected lists the
// roots whose effective settings or origin change.
func PrepareLayers(path string, cfg *config.Config, edit func(*config.ProjectLayers) error) (*Candidate, error) {
	c := &Candidate{}
	if cfg == nil {
		c.Err = fmt.Errorf("cannot prepare projects for a nil config")
		return c, c.Err
	}
	c.Save, c.Err = config.PrepareProjectLayers(path, cfg, edit)
	if c.Err != nil {
		return c, c.Err
	}
	if c.Err = validateProjectFilters(c.Save.Expanded.Projects); c.Err != nil {
		c.Save.Err = c.Err
		return c, c.Err
	}
	after := c.Save.Expanded
	c.Affected = affectedProjects(cfg.Projects, after.Projects)
	for root := range cfg.Projects {
		if _, kept := after.Projects[root]; !kept || slices.Contains(c.Affected, root) {
			continue
		}
		before, _ := cfg.ProjectOrigin(root)
		if now, _ := after.ProjectOrigin(root); now != before {
			c.Affected = append(c.Affected, root)
		}
	}
	slices.Sort(c.Affected)
	return c, nil
}

// commit writes a prepared change unless it is a preview or changes no file.
func commit(c *Candidate, dryRun bool, result *Result, registration string) error {
	if dryRun || !c.Save.Changed() {
		return nil
	}
	if err := c.Save.Write(); err != nil {
		return fmt.Errorf("%w: %w", ErrSave, err)
	}
	result.Registration = registration
	return nil
}

func configItem(cfg *config.Config, root string, p config.ManagedProject) Item {
	v := item(root, p)
	v.Origin, v.OriginPath = cfg.ProjectOrigin(root)
	v.SharedCopy = v.Origin == OriginLocal && cfg.HasSharedProject(root)
	return v
}

// resultItem reports root after a change that may respell its key.
func resultItem(cfg *config.Config, root string) Item {
	if key, ok := cfg.ProjectKey(root); ok {
		root = key
	}
	return configItem(cfg, root, cfg.Projects[root])
}

// Localize copies a config.yaml project to this machine without changing
// config.yaml, detaching destinations or synchronizing files. Repeats are noops.
func (s *Service) Localize(key string, dryRun bool) (Result, error) {
	cfg, err := s.load()
	if err != nil {
		return Result{}, err
	}
	root, err := lookup(cfg, key, false)
	if err != nil {
		return Result{}, err
	}
	c, err := PrepareLayers(s.path, cfg, func(l *config.ProjectLayers) error { l.SetLocal(root, cfg.Projects[root]); return nil })
	if err != nil {
		return Result{}, err
	}
	v := resultItem(c.Save.Expanded, root)
	result := Result{Item: &v, Affected: c.Affected, Expanded: c.Save.Expanded, Registration: "kept"}
	return result, commit(c, dryRun, &result, "updated")
}

// ShareOptions previews a move or confirms replacing different shared settings.
type ShareOptions struct{ DryRun, Replace bool }

// Share moves a project from this machine into config.yaml, the reverse of
// Localize. Its settings stay the same, so nothing is detached or synced.
func (s *Service) Share(key string, opts ShareOptions) (Result, error) {
	cfg, err := s.load()
	if err != nil {
		return Result{}, err
	}
	root, err := lookup(cfg, key, false)
	if err != nil {
		return Result{}, err
	}
	entry := cfg.Projects[root]
	origin, _ := cfg.ProjectOrigin(root)
	c, err := PrepareLayers(s.path, cfg, func(l *config.ProjectLayers) error {
		if origin != OriginLocal {
			return nil
		}
		if shared, ok := l.SharedKey(root); ok && !opts.Replace && !config.SameManagedProject(l.Shared[shared], entry) {
			return fmt.Errorf("%w: %s", ErrSharedDiffers, root)
		}
		l.SetShared(root, entry)
		l.ClearLocal(root)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	v := resultItem(c.Save.Expanded, root)
	result := Result{Item: &v, Affected: c.Affected, Expanded: c.Save.Expanded, Registration: "kept"}
	return result, commit(c, opts.DryRun, &result, "updated")
}
