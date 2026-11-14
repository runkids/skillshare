package config

import (
	"maps"
	"os"
	"path/filepath"

	"skillshare/internal/profile"
)

// ProfilePaths identifies machine-local state without creating it.
func ProfilePaths(configPath string) (statePath, lockPath string, err error) {
	id, err := ProjectConfigIdentity(configPath)
	if err != nil {
		return "", "", err
	}
	base := filepath.Join(ProjectStateDir(), "profiles", id)
	return base + ".yaml", base + ".lock", nil
}

func resolveProfile(c profile.Catalog, path string) (profile.Selector, profile.State) {
	statePath, _, err := ProfilePaths(path)
	var state profile.State
	if err == nil {
		state, err = profile.ReadState(statePath)
	}
	return profile.Resolve(c, state, path, err), state
}

func (c *Config) loadProfile(path string) {
	c.profilePath = path
	c.profileSelector, c.profileState = resolveProfile(profile.Parse(&c.Profiles), path)
	for name, target := range c.Targets {
		if target.projectRoot == "" {
			target.profileSelector = c.profileSelector
			c.Targets[name] = target
		}
	}
}

func (c *ProjectConfig) loadProfile(path string) {
	c.profilePath = path
	c.profileSelector, c.profileState = resolveProfile(profile.Parse(&c.Profiles), path)
}

func (c *Config) ActiveProfileError() error {
	_, err := c.profileSelector.Selects("")
	return err
}

func (c *ProjectConfig) ActiveProfileError() error {
	_, err := c.profileSelector.Selects("")
	return err
}

// ProfileState is the state read when this config was loaded.
func (c *Config) ProfileState() profile.State { return c.profileState }

// ProfileState is the state read when this project config was loaded.
func (c *ProjectConfig) ProfileState() profile.State { return c.profileState }

func (c *Config) ProfileSelector() profile.Selector { return c.profileSelector }

func (c *ProjectConfig) ProfileSelector() profile.Selector { return c.profileSelector }

// WithActiveProfile resolves a candidate without changing config, state or targets.
// The target map is copied because attaching the selector updates its values.
func (c *Config) WithActiveProfile(name string) *Config {
	candidate := *c
	candidate.Targets = maps.Clone(c.Targets)
	candidate.profileState.Active = name
	candidate.profileSelector = profile.Resolve(profile.Parse(&c.Profiles), candidate.profileState, c.profilePath, nil)
	for key, target := range candidate.Targets {
		if target.projectRoot == "" {
			target.profileSelector = candidate.profileSelector
			candidate.Targets[key] = target
		}
	}
	return &candidate
}

func (c *ProjectConfig) WithActiveProfile(name string) *ProjectConfig {
	candidate := *c
	candidate.profileState.Active = name
	candidate.profileSelector = profile.Resolve(profile.Parse(&c.Profiles), candidate.profileState, c.profilePath, nil)
	return &candidate
}

// ProfileConfigured probes for the locking boundary without creating files.
// A state read error still requires the lock and a reload, never legacy sync.
func ProfileConfigured(configPath string, profilesPresent bool) (bool, error) {
	if profilesPresent {
		return true, nil
	}
	path, _, err := ProfilePaths(configPath)
	if err != nil {
		return false, err
	}
	_, err = os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
