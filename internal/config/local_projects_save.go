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

// ProjectLayers holds the declarations behind Config.Projects. Shared is the
// projects section of config.yaml, hidden entries included; Local and Excluded
// are this machine's registry. Its methods match roots by identity, so ~/app
// and its expanded spelling are the same project.
type ProjectLayers struct {
	Shared   map[string]ManagedProject
	Local    map[string]ManagedProject
	Excluded []string
}

func layerKey(entries map[string]ManagedProject, root string) (string, bool) {
	id, err := projectRootIdentity(root)
	if err != nil {
		return "", false
	}
	for key := range entries {
		if other, _ := projectRootIdentity(key); other == id {
			return key, true
		}
	}
	return "", false
}

// SharedKey returns the key config.yaml declares root under.
func (l ProjectLayers) SharedKey(root string) (string, bool) { return layerKey(l.Shared, root) }

// SetShared keeps the spelling config.yaml already uses for root.
func (l *ProjectLayers) SetShared(root string, p ManagedProject) {
	if key, ok := l.SharedKey(root); ok {
		root = key
	}
	l.Shared[root] = p
}

// DeleteShared removes root from config.yaml.
func (l *ProjectLayers) DeleteShared(root string) {
	if key, ok := l.SharedKey(root); ok {
		delete(l.Shared, key)
	}
}

// ClearLocal drops this machine's declaration and exclusion of root, so the
// config.yaml declaration, if any, applies again.
func (l *ProjectLayers) ClearLocal(root string) {
	id, _ := projectRootIdentity(root)
	for key := range l.Local {
		if other, _ := projectRootIdentity(key); other == id {
			delete(l.Local, key)
		}
	}
	l.Excluded = slices.DeleteFunc(l.Excluded, func(key string) bool { other, _ := projectRootIdentity(key); return other == id })
}

// SetLocal declares root on this machine only, replacing any exclusion.
func (l *ProjectLayers) SetLocal(root string, p ManagedProject) {
	l.ClearLocal(root)
	l.Local[root] = p
}

// PrepareProjectLayers snapshots an edit of both project layers without writing.
// Write saves config.yaml before the local registry, so an interrupted write
// keeps every project: one moved to config.yaml stays local until it is there.
// The candidate's Expanded view uses the exact bytes that Write persists.
func PrepareProjectLayers(path string, cfg *Config, edit func(*ProjectLayers) error) (*SaveCandidate, error) {
	c := &SaveCandidate{}
	if cfg == nil {
		c.Err = fmt.Errorf("cannot prepare projects for a nil config")
		return c, c.Err
	}
	o, err := projectSnapshot(path, cfg)
	if err != nil {
		c.Err = err
		return c, err
	}
	// Both layers start from the bytes the snapshot verified on disk, so an
	// entry changed in place through cfg.Projects cannot hide an edit.
	state, err := decodeLocalProjects(o.raw)
	var oldLocal []byte
	if err == nil {
		oldLocal, err = yaml.Marshal(state)
	}
	if err != nil {
		c.Err = err
		return c, err
	}
	sharedBase, err := projectsSection(o.sharedRaw)
	if err != nil {
		c.Err = err
		return c, err
	}
	sharedCopy, err := projectsSection(o.sharedRaw)
	if err != nil {
		c.Err = err
		return c, err
	}
	layers := ProjectLayers{Shared: sharedCopy, Local: state.Projects, Excluded: state.Excluded}
	if err := edit(&layers); err != nil {
		c.Err = err
		return c, err
	}
	for _, entries := range []map[string]ManagedProject{layers.Shared, layers.Local} {
		if _, err := projectRootIndex(entries); err != nil {
			c.Err = err
			return c, err
		}
	}
	state = localProjects{Version: 1, Projects: layers.Local, Excluded: slices.Sorted(slices.Values(layers.Excluded))}
	localBytes, err := yaml.Marshal(state)
	if err == nil {
		// Normalize the effective view exactly as a subsequent read will see it.
		state, err = decodeLocalProjects(localBytes)
	}
	if err != nil {
		c.Err = err
		return c, err
	}
	overlay := *o
	overlay.state = state
	localChanged := !bytes.Equal(localBytes, oldLocal)
	if localChanged {
		overlay.raw = localBytes
	}
	var sharedBytes []byte
	sharedChanged := !sameDeclarations(sharedBase, layers.Shared)
	if sharedChanged {
		next := *cfg
		next.projectOverlay = nil
		next.Projects = layers.Shared
		if sharedBytes, err = serializeConfig(path, &next); err != nil {
			c.Err = err
			return c, err
		}
		if overlay.shared, err = projectsSection(sharedBytes); err != nil {
			c.Err = fmt.Errorf("failed to parse prepared config: %w", err)
			return c, c.Err
		}
		overlay.sharedRaw = sharedBytes
	}
	next := *cfg
	next.Projects, overlay.origins, err = mergeLocalProjects(overlay.shared, state)
	if err != nil {
		c.Err = err
		return c, err
	}
	next.projectOverlay = &overlay
	next.Targets = cfg.OwnTargets()
	if err := next.ValidateProjects(next.Projects); err != nil {
		c.Err = err
		return c, err
	}
	if err := next.expandProjects(); err != nil {
		c.Err = err
		return c, err
	}
	next.Targets = WithSharedSkillsTargets(next.Targets, next.Mode)

	local := &SaveCandidate{Bytes: localBytes, Expanded: &next, path: o.path, private: true, check: o.check, noop: !localChanged && !sharedChanged}
	if !sharedChanged {
		return local, nil
	}
	c = &SaveCandidate{Bytes: sharedBytes, Expanded: &next, path: path, check: o.check}
	if localChanged {
		// After config.yaml is replaced, the registry write expects the new bytes.
		after := *o
		after.sharedRaw = sharedBytes
		local.check = after.check
		c.then = local
	}
	return c, nil
}

// projectSnapshot returns a private copy of cfg's overlay, loading it when cfg
// was built in memory, and refuses one that is stale or belongs elsewhere.
func projectSnapshot(path string, cfg *Config) (*projectOverlay, error) {
	next := *cfg
	if next.projectOverlay == nil {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := next.loadLocalProjects(path, raw); err != nil {
			return nil, err
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
		return nil, fmt.Errorf("project snapshot belongs to a different config or state location; reload it")
	}
	o.configPath = path
	if err := o.check(); err != nil {
		return nil, err
	}
	return o, nil
}

// projectsSection decodes the projects of a config.yaml document as Load does.
func projectsSection(raw []byte) (map[string]ManagedProject, error) {
	var doc struct {
		Projects map[string]ManagedProject `yaml:"projects"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if doc.Projects == nil {
		doc.Projects = map[string]ManagedProject{}
	}
	return doc.Projects, nil
}

func sameDeclarations(a, b map[string]ManagedProject) bool {
	if len(a) != len(b) {
		return false
	}
	for root, p := range a {
		other, ok := b[root]
		if !ok || !SameManagedProject(p, other) {
			return false
		}
	}
	return true
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
