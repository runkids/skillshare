package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/tailscale/hujson"
)

func (s *Service) piSettingsPath() (string, error) {
	if s.ProjectRoot != "" {
		return filepath.Join(s.ProjectRoot, ".pi", "settings.json"), nil
	}
	dir := os.Getenv("PI_CODING_AGENT_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".pi", "agent")
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("PI_CODING_AGENT_DIR must be absolute")
	}
	return filepath.Join(dir, "settings.json"), nil
}

// Pi has no machine-readable list command. Read its documented settings without
// loading extensions; leave all writes to pi install/remove.
func (s *Service) piInventory() ([]Installed, string, error) {
	path, err := s.piSettingsPath()
	if err != nil {
		return nil, "", err
	}
	raw, _, entries, err := readPackageConfig(path, "packages")
	if err != nil {
		return nil, "", err
	}
	result := []Installed{}
	for _, entry := range entries {
		var source string
		filtered := json.Unmarshal(entry, &source) != nil
		if filtered {
			var object struct {
				Source string `json:"source"`
			}
			if json.Unmarshal(entry, &object) != nil {
				return nil, "", fmt.Errorf("invalid Pi package entry")
			}
			source = object.Source
		}
		if !strings.HasPrefix(source, "npm:") && !strings.HasPrefix(source, "git:") && !strings.Contains(source, "://") && !strings.HasPrefix(source, "git@") {
			if strings.HasPrefix(source, "~/") {
				home, e := os.UserHomeDir()
				if e != nil {
					return nil, "", e
				}
				source = filepath.Join(home, source[2:])
			} else if !filepath.IsAbs(source) {
				source = filepath.Join(filepath.Dir(path), source)
			}
		}
		if !validTargetID("pi", source) {
			return nil, "", fmt.Errorf("invalid Pi package source")
		}
		result = append(result, Installed{ID: source, Name: logicalName(source), Installed: true, Enabled: true, Filtered: filtered})
	}
	return result, hash(raw), nil
}

func logicalName(id string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(id, "npm:"), "@")
	name = strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if i := strings.Index(name, "@"); i >= 0 {
		name = name[:i]
	}
	if namePattern.MatchString(name) {
		return name
	}
	return ""
}

func (s *Service) openCodeConfigPath() (string, error) {
	if os.Getenv("OPENCODE_CONFIG_CONTENT") != "" || os.Getenv("OPENCODE_CONFIG_DIR") != "" {
		return "", fmt.Errorf("OpenCode inline/config-directory overrides are not supported for plugin sync; use its standard config")
	}
	if override := os.Getenv("OPENCODE_CONFIG"); override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("OPENCODE_CONFIG must be absolute")
		}
		if s.ProjectRoot != "" {
			return "", fmt.Errorf("unset OPENCODE_CONFIG before project plugin sync to avoid editing a shared config")
		}
		return override, nil
	}
	dir := s.ProjectRoot
	if dir == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
		if !filepath.IsAbs(base) {
			return "", fmt.Errorf("XDG_CONFIG_HOME must be absolute")
		}
		dir = filepath.Join(base, "opencode")
	}
	var found string
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		path := filepath.Join(dir, name)
		if _, err := os.Lstat(path); err == nil {
			if found != "" {
				return "", fmt.Errorf("both opencode.json and opencode.jsonc exist; consolidate before plugin sync")
			}
			found = path
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if found != "" {
		return found, nil
	}
	return filepath.Join(dir, "opencode.json"), nil
}

func openCodeKey(version string) (string, error) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if strings.HasPrefix(v, "1.") {
		return "plugin", nil
	}
	if strings.HasPrefix(v, "2.") {
		return "plugins", nil
	}
	return "", fmt.Errorf("unrecognized OpenCode version; plugin config adapter supports versions 1.x and 2.x")
}

func readPackageConfig(path, key string) ([]byte, *hujson.Value, []json.RawMessage, error) {
	if err := noSymlink(path); err != nil {
		return nil, nil, nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, nil, err
	}
	parse := raw
	if len(parse) == 0 {
		parse = []byte("{}")
	}
	value, err := hujson.Parse(parse)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid native config: %w", err)
	}
	object, ok := value.Value.(*hujson.Object)
	if !ok {
		return nil, nil, nil, fmt.Errorf("native config must be an object")
	}
	seen := map[string]bool{}
	for _, member := range object.Members {
		var name string
		_ = json.Unmarshal(member.Name.Pack(), &name)
		if seen[name] {
			return nil, nil, nil, fmt.Errorf("duplicate native config key %s", name)
		}
		seen[name] = true
	}
	clone := value.Clone()
	clone.Standardize()
	var body map[string]json.RawMessage
	if err = json.Unmarshal(clone.Pack(), &body); err != nil {
		return nil, nil, nil, err
	}
	entries := []json.RawMessage{}
	if field, ok := body[key]; ok {
		if err = json.Unmarshal(field, &entries); err != nil || entries == nil {
			return nil, nil, nil, fmt.Errorf("native %s must be an array", key)
		}
	}
	return raw, &value, entries, nil
}

func (s *Service) openCodeInventory(version string) ([]Installed, string, error) {
	key, err := openCodeKey(version)
	if err != nil {
		return nil, "", err
	}
	path, err := s.openCodeConfigPath()
	if err != nil {
		return nil, "", err
	}
	raw, _, entries, err := readPackageConfig(path, key)
	if err != nil {
		return nil, "", err
	}
	result := []Installed{}
	for _, entry := range entries {
		var id string
		filtered := json.Unmarshal(entry, &id) != nil
		if filtered {
			var object struct {
				Package string `json:"package"`
			}
			if json.Unmarshal(entry, &object) != nil || object.Package == "" {
				return nil, "", fmt.Errorf("unsupported OpenCode plugin entry")
			}
			id = object.Package
		}
		if key == "plugins" && (strings.HasPrefix(id, "-") || id == "*" || strings.HasSuffix(id, ".*")) {
			continue
		}
		if !validTargetID("opencode", id) {
			return nil, "", fmt.Errorf("unsupported OpenCode plugin control entry; manage it in OpenCode")
		}
		result = append(result, Installed{ID: id, Name: logicalName(id), Installed: true, Enabled: true, Filtered: filtered})
	}
	return result, hash(raw), nil
}

func (s *Service) applyOpenCode(ctx context.Context, c Change, b Binding) error {

	version, err := s.run(ctx, "opencode", "--version")
	if err != nil {
		return err
	}
	key, err := openCodeKey(string(version))
	if err != nil {
		return err
	}
	if c.Action == "update" && b.Source == "" {
		if key != "plugins" || s.ProjectRoot != "" {
			return fmt.Errorf("imported package updates require OpenCode v2 global scope")
		}
		_, err := s.run(ctx, "opencode", "plugin", "update", b.ID)
		return err
	}
	path, err := s.openCodeConfigPath()
	if err != nil {
		return err
	}
	if err = noSymlink(path); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	lock := flock.New(path + ".skillshare-plugin.lock")
	if err = lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	raw, value, entries, err := readPackageConfig(path, key)
	if err != nil {
		return err
	}
	remove := c.Action == "remove" || c.Action == "uninstall"
	next := []json.RawMessage{}
	found := false
	for _, entry := range entries {
		var id string
		if json.Unmarshal(entry, &id) == nil && id == b.ID {
			found = true
			if remove {
				continue
			}
		}
		next = append(next, entry)
	}
	if !remove && !found {
		data, _ := json.Marshal(b.ID)
		next = append(next, data)
	}
	if found && !remove || !found && remove {
		return nil
	}
	data, _ := json.Marshal(next)
	patch, _ := json.Marshal([]map[string]any{{"op": "add", "path": "/" + key, "value": json.RawMessage(data)}})
	if err = value.Patch(patch); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, raw) {
		return fmt.Errorf("OpenCode configuration changed; preview again")
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return atomicNativeWrite(path, value.Pack(), mode)
}
