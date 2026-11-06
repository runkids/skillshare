package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"skillshare/internal/utils"
)

// ProjectStateDir keeps registrations outside the shared config directory,
// including Windows, where StateDir otherwise defaults to roaming AppData.
func ProjectStateDir() string { return projectStateDir(runtime.GOOS) }

func projectStateDir(goos string) string {
	if os.Getenv("XDG_STATE_HOME") != "" || goos != "windows" {
		return StateDir()
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		home, _ := os.UserHomeDir()
		local = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(local, "skillshare", "state")
}

// ProjectConfigIdentity is shared by registry and lock namespaces. Config links
// to one destination share state; lexical project-root links remain separate.
func ProjectConfigIdentity(path string) (string, error) {
	return projectConfigIdentity(path, runtime.GOOS)
}

func projectConfigIdentity(path, goos string) (string, error) {
	dest, err := utils.ConfigWritePath(path, false)
	if err != nil {
		return "", err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	// The automatic Windows directory migration is an alias, not a new config.
	if goos == "windows" && os.Getenv("SKILLSHARE_CONFIG") == "" {
		home, _ := os.UserHomeDir()
		legacy := filepath.Join(home, ".config", "skillshare", "config.yaml")
		legacy, err = utils.ConfigWritePath(legacy, false)
		if err != nil {
			return "", err
		}
		legacy, err = filepath.Abs(legacy)
		if err != nil {
			return "", err
		}
		if utils.PathsEqual(dest, legacy) {
			dest, err = utils.ConfigWritePath(ConfigPath(), false)
			if err != nil {
				return "", err
			}
			dest, err = filepath.Abs(dest)
			if err != nil {
				return "", err
			}
		}
	}
	dest = filepath.Clean(dest)
	if goos == "windows" {
		dest = strings.ToLower(dest)
	}
	sum := sha256.Sum256([]byte(dest))
	return hex.EncodeToString(sum[:8]), nil
}

// LocalProjectsPath returns the selected config's machine-local registry file.
// It never creates directories, locks or files.
func LocalProjectsPath(path string) (string, error) {
	id, err := ProjectConfigIdentity(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(ProjectStateDir(), "projects", id+".yaml"), nil
}

type localProjects struct {
	Version  int                       `yaml:"version"`
	Projects map[string]ManagedProject `yaml:"projects,omitempty"`
	Excluded []string                  `yaml:"excluded,omitempty"`
}

type projectOverlay struct {
	shared                        map[string]ManagedProject
	origins                       map[string]string
	state                         localProjects
	path, configPath, destination string
	raw, sharedRaw                []byte
}

func projectRootIdentity(root string) (string, error) {
	abs := expandPath(root)
	if strings.ContainsRune(root, 0) || !filepath.IsAbs(abs) {
		return "", fmt.Errorf("projects: %s must be an absolute path or start with ~", root)
	}
	abs = filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return abs, nil
}

func projectRootIndex(entries map[string]ManagedProject) (map[string]string, error) {
	index := map[string]string{}
	for _, root := range slices.Sorted(maps.Keys(entries)) {
		id, err := projectRootIdentity(root)
		if err != nil {
			return nil, err
		}
		if other, ok := index[id]; ok {
			return nil, fmt.Errorf("projects: %s and %s declare the same root", other, root)
		}
		index[id] = root
	}
	return index, nil
}

func mergeLocalProjects(shared map[string]ManagedProject, state localProjects) (map[string]ManagedProject, map[string]string, error) {
	sharedIndex, err := projectRootIndex(shared)
	if err != nil {
		return nil, nil, err
	}
	localIndex, err := projectRootIndex(state.Projects)
	if err != nil {
		return nil, nil, err
	}
	excluded := map[string]bool{}
	for _, root := range state.Excluded {
		id, err := projectRootIdentity(root)
		if err != nil {
			return nil, nil, err
		}
		excluded[id] = true
	}
	effective := map[string]ManagedProject{}
	origins := map[string]string{}
	for id, root := range sharedIndex {
		if _, local := localIndex[id]; local || excluded[id] {
			continue
		}
		effective[root] = shared[root]
		origins[root] = "shared"
	}
	for root, p := range state.Projects {
		effective[root] = p
		origins[root] = "local"
	}
	return effective, origins, nil
}

// readLocalProjects refuses redirection of a registry or its namespace folder.
func readLocalProjects(path string) ([]byte, error) {
	for _, p := range []string{filepath.Dir(path), path} {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("local project registry must not be a symlink: %s", p)
		}
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func decodeLocalProjects(data []byte) (localProjects, error) {
	state := localProjects{Version: 1, Projects: map[string]ManagedProject{}}
	if data == nil {
		return state, nil
	}
	state.Version = 0
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&state); err != nil {
		return state, fmt.Errorf("invalid local project registry: %w", err)
	}
	if dec.Decode(new(yaml.Node)) != io.EOF {
		return state, fmt.Errorf("local project registry must contain one YAML document")
	}
	if state.Version != 1 {
		return state, fmt.Errorf("unsupported local project registry version %d", state.Version)
	}
	if state.Projects == nil {
		state.Projects = map[string]ManagedProject{}
	}
	return state, nil
}

func (c *Config) loadLocalProjects(path string, sharedRaw []byte) error {
	localPath, err := LocalProjectsPath(path)
	if err != nil {
		return err
	}
	raw, err := readLocalProjects(localPath)
	if err != nil {
		return err
	}
	state, err := decodeLocalProjects(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", localPath, err)
	}
	dest, err := utils.ConfigWritePath(path, false)
	if err != nil {
		return err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return err
	}
	overlay := &projectOverlay{shared: maps.Clone(c.Projects), state: state, path: localPath, configPath: path, destination: dest, raw: raw, sharedRaw: sharedRaw}
	c.Projects, overlay.origins, err = mergeLocalProjects(overlay.shared, state)
	if err != nil {
		return err
	}
	c.projectOverlay = overlay
	return nil
}

// ProjectOrigin identifies the winning declaration and the file that owns it.
func (c *Config) ProjectOrigin(root string) (string, string) {
	if o := c.projectOverlay; o != nil {
		if o.origins[root] == "local" {
			return "local", o.path
		}
		return "shared", o.configPath
	}
	return "shared", ReadOnlyConfigPath()
}

// HasLocalProject includes exclusions, so conversion cannot discard ordinary
// targets while their replacement declaration would remain hidden.
func (c *Config) HasLocalProject(root string) bool {
	o := c.projectOverlay
	if o == nil {
		return false
	}
	id, err := projectRootIdentity(root)
	if err != nil {
		return false
	}
	for key := range o.state.Projects {
		other, _ := projectRootIdentity(key)
		if id == other {
			return true
		}
	}
	for _, key := range o.state.Excluded {
		other, _ := projectRootIdentity(key)
		if id == other {
			return true
		}
	}
	return false
}

// sharedProjectDeclarations projects effective edits back onto shared intent.
// Hidden declarations survive; local overrides and exclusions never leak out.
func (c *Config) sharedProjectDeclarations() map[string]ManagedProject {
	o := c.projectOverlay
	if o == nil {
		return c.Projects
	}
	out := maps.Clone(o.shared)
	if out == nil {
		out = map[string]ManagedProject{}
	}
	for root, origin := range o.origins {
		if origin != "shared" {
			continue
		}
		if p, ok := c.Projects[root]; ok {
			out[root] = p
		} else {
			delete(out, root)
		}
	}
	for root, p := range c.Projects {
		if o.origins[root] == "" {
			out[root] = p
		}
	}
	return out
}

// DecodeReadOnly validates a replacement shared document against this machine's
// local registry without requiring the current shared YAML to be valid.
func DecodeReadOnly(path string, data []byte) (*Config, error) {
	return decodeConfig(path, data, false)
}
