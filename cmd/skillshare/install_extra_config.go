package main

import (
	"fmt"
	"reflect"
	"strings"

	"skillshare/internal/config"
)

type extraConfigUpdate func([]config.ExtraConfig) ([]config.ExtraConfig, error)

type extraConfigCallbacks struct {
	name, remote, branch        string
	load                        func() ([]config.ExtraConfig, error)
	save                        func(extraConfigUpdate) error
	baseline, previous, written *config.ExtraConfig
}

func cloneExtraConfig(e *config.ExtraConfig) *config.ExtraConfig {
	if e == nil {
		return nil
	}
	copy := *e
	if e.Targets != nil {
		copy.Targets = append(make([]config.ExtraTargetConfig, 0, len(e.Targets)), e.Targets...)
	}
	for i := range copy.Targets {
		if e.Targets[i].Include != nil {
			copy.Targets[i].Include = append(make([]string, 0, len(e.Targets[i].Include)), e.Targets[i].Include...)
		}
		if e.Targets[i].Exclude != nil {
			copy.Targets[i].Exclude = append(make([]string, 0, len(e.Targets[i].Exclude)), e.Targets[i].Exclude...)
		}
	}
	return &copy
}

func extraConfigAt(entries []config.ExtraConfig, name string) (int, error) {
	index := -1
	for i := range entries {
		if !strings.EqualFold(entries[i].Name, name) {
			continue
		}
		if entries[i].Name != name || index >= 0 {
			return -1, fmt.Errorf("extra name %q conflicts with an existing entry", name)
		}
		index = i
	}
	return index, nil
}

func newExtraConfigCallbacks(name, remote, branch string, load func() ([]config.ExtraConfig, error), save func(extraConfigUpdate) error) (*extraConfigCallbacks, error) {
	c := &extraConfigCallbacks{name: name, remote: remote, branch: branch, load: load, save: save}
	entries, err := load()
	if err != nil {
		return nil, err
	}
	// Conflicts are checked during recheck or repair, after an interruption is inspected.
	for i := range entries {
		if entries[i].Name == name {
			c.baseline = cloneExtraConfig(&entries[i])
			break
		}
	}
	return c, nil
}

func (c *extraConfigCallbacks) entry(entries []config.ExtraConfig) (int, *config.ExtraConfig, error) {
	i, err := extraConfigAt(entries, c.name)
	if err != nil {
		return -1, nil, err
	}
	if i < 0 {
		return i, nil, nil
	}
	return i, &entries[i], nil
}

func (c *extraConfigCallbacks) Recheck() error {
	entries, err := c.load()
	if err != nil {
		return err
	}
	_, current, err := c.entry(entries)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(c.baseline, current) {
		return fmt.Errorf("extra %q changed; retry", c.name)
	}
	if current != nil && (current.Remote == "" || current.Remote != c.remote) {
		return fmt.Errorf("extra %q has a local or different origin; remove it before reinstalling", c.name)
	}
	return nil
}

func (c *extraConfigCallbacks) merge(remote, branch string, record bool) error {
	var previous, written *config.ExtraConfig
	err := c.save(func(entries []config.ExtraConfig) ([]config.ExtraConfig, error) {
		i, current, err := c.entry(entries)
		if err != nil {
			return nil, err
		}
		if current != nil && (current.Remote == "" || current.Remote != remote) {
			return nil, fmt.Errorf("extra %q has a local or different origin; remove it before reinstalling", c.name)
		}
		previous = cloneExtraConfig(current)
		if i < 0 {
			entries = append(entries, config.ExtraConfig{Name: c.name, Targets: []config.ExtraTargetConfig{}})
			i = len(entries) - 1
		}
		entries[i].Remote, entries[i].Branch = remote, branch
		written = cloneExtraConfig(&entries[i])
		return entries, nil
	})
	if err == nil && record {
		c.previous, c.written = previous, written
	}
	return err
}

func (c *extraConfigCallbacks) WriteConfig() error { return c.merge(c.remote, c.branch, true) }
func (c *extraConfigCallbacks) RepairConfig(remote, branch string) error {
	return c.merge(remote, branch, false)
}

func (c *extraConfigCallbacks) RefreshAfterRepair() error {
	entries, err := c.load()
	if err != nil {
		return err
	}
	_, current, err := c.entry(entries)
	if err != nil {
		return err
	}
	c.baseline = cloneExtraConfig(current)
	return nil
}

func (c *extraConfigCallbacks) RevertConfig() error {
	if c.written == nil {
		return nil
	}
	return c.save(func(entries []config.ExtraConfig) ([]config.ExtraConfig, error) {
		i, current, err := c.entry(entries)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, nil
		} // no write when already removed
		if !reflect.DeepEqual(c.written, current) {
			return nil, fmt.Errorf("extra %q changed by another writer; cannot revert it", c.name)
		}
		if c.previous == nil {
			entries = append(entries[:i], entries[i+1:]...)
		} else {
			entries[i] = *cloneExtraConfig(c.previous)
		}
		return entries, nil
	})
}
