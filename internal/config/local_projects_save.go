package config

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// PrepareLocalProjects snapshots a registration-only edit. entries is the full
// desired effective map; forceRoot explicitly adopts an unchanged shared entry.
// Its Expanded view uses these exact candidate bytes, not the old disk overlay.
func PrepareLocalProjects(path string, cfg *Config, entries map[string]ManagedProject, forceRoot string) (*SaveCandidate, error) {
	c := &SaveCandidate{private: true}
	if cfg == nil {
		c.Err = fmt.Errorf("cannot prepare projects for a nil config")
		return c, c.Err
	}
	next := *cfg
	if next.projectOverlay == nil {
		raw, err := os.ReadFile(path)
		if err != nil {
			c.Err = err
			return c, err
		}
		if err := next.loadLocalProjects(path, raw); err != nil {
			c.Err = err
			return c, err
		}
	}
	snapshot := *next.projectOverlay
	o := &snapshot
	dest, err := utils.ConfigWritePath(path, false)
	if err == nil {
		dest, err = filepath.Abs(dest)
	}
	selected, identityErr := LocalProjectsPath(path)
	if err != nil || identityErr != nil || !utils.PathsEqual(dest, o.destination) || !utils.PathsEqual(selected, o.path) {
		c.Err = fmt.Errorf("project snapshot belongs to a different config or state location; reload it")
		return c, c.Err
	}
	o.configPath = path
	c.path = o.path
	c.check = o.check
	if err := c.Check(); err != nil {
		c.Err = err
		return c, err
	}
	if _, err := projectRootIndex(entries); err != nil {
		c.Err = err
		return c, err
	}
	// Serialize/decode isolates nested resources and list backing arrays.
	raw, err := yaml.Marshal(o.state)
	if err != nil {
		c.Err = err
		return c, err
	}
	state, err := decodeLocalProjects(raw)
	if err != nil {
		c.Err = err
		return c, err
	}
	erase := func(root string) {
		id, _ := projectRootIdentity(root)
		for key := range state.Projects {
			other, _ := projectRootIdentity(key)
			if id == other {
				delete(state.Projects, key)
			}
		}
		state.Excluded = slices.DeleteFunc(state.Excluded, func(key string) bool { other, _ := projectRootIdentity(key); return id == other })
	}
	sharedIndex, _ := projectRootIndex(o.shared)
	for root := range cfg.Projects {
		if _, kept := entries[root]; kept {
			continue
		}
		erase(root)
		id, _ := projectRootIdentity(root)
		if _, shared := sharedIndex[id]; shared {
			state.Excluded = append(state.Excluded, root)
		}
	}
	for root, p := range entries {
		if old, ok := cfg.Projects[root]; ok && SameManagedProject(old, p) && root != forceRoot {
			continue
		}
		erase(root)
		state.Projects[root] = p
	}
	slices.Sort(state.Excluded)
	c.Bytes, c.Err = yaml.Marshal(state)
	if c.Err != nil {
		return c, c.Err
	}
	// Normalize the effective view exactly as a subsequent read will see it.
	state, c.Err = decodeLocalProjects(c.Bytes)
	if c.Err != nil {
		return c, c.Err
	}
	overlay := *o
	overlay.state = state
	overlay.raw = c.Bytes
	next.Projects, overlay.origins, c.Err = mergeLocalProjects(o.shared, state)
	if c.Err != nil {
		return c, c.Err
	}
	next.projectOverlay = &overlay
	next.Targets = cfg.OwnTargets()
	if c.Err = next.ValidateProjects(next.Projects); c.Err != nil {
		return c, c.Err
	}
	if c.Err = next.expandProjects(); c.Err != nil {
		return c, c.Err
	}
	next.Targets = WithSharedSkillsTargets(next.Targets, next.Mode)
	c.Expanded = &next
	return c, nil
}

func (o *projectOverlay) check() error {
	dest, err := utils.ConfigWritePath(o.configPath, false)
	if err != nil {
		return err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	if !utils.PathsEqual(dest, o.destination) {
		return fmt.Errorf("project config destination changed; retry the command")
	}
	localPath, err := LocalProjectsPath(o.configPath)
	if err != nil {
		return err
	}
	if !utils.PathsEqual(localPath, o.path) {
		return fmt.Errorf("local project registry location changed; retry the command")
	}
	raw, err := os.ReadFile(o.configPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, o.sharedRaw) {
		return fmt.Errorf("shared project configuration changed; retry the command")
	}
	raw, err = readLocalProjects(o.path)
	if err != nil {
		return err
	}
	if (raw == nil) != (o.raw == nil) || !bytes.Equal(raw, o.raw) {
		return fmt.Errorf("local project registry changed; retry the command")
	}
	return nil
}

// SharedProjectRoots includes locally hidden declarations as well as active ones.
func (c *Config) SharedProjectRoots() []string {
	return slices.Sorted(maps.Keys(c.sharedProjectDeclarations()))
}

// SameManagedProject compares declarations while treating omitted and empty lists equally.
func SameManagedProject(a, b ManagedProject) bool {
	return a.Name == b.Name && slices.Equal(a.Targets, b.Targets) &&
		sameManagedProjectResource(a.Skills, b.Skills) && sameManagedProjectResource(a.Agents, b.Agents)
}

func sameManagedProjectResource(a, b *ResourceTargetConfig) bool {
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
