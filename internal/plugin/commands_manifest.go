package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var commandName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// inspectCommands reads only Skillshare's portable extension, never native slash commands.
func inspectCommands(root string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "plugin.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Schema     string                     `json:"$schema"`
		Extensions map[string]json.RawMessage `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	if manifest.Schema != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" {
		return nil, nil
	}
	extension, ok := manifest.Extensions["cc.runkids.skillshare"]
	if !ok {
		return nil, nil
	}
	var namespace map[string]json.RawMessage
	if err := json.Unmarshal(extension, &namespace); err != nil || namespace == nil {
		return nil, fmt.Errorf("Skillshare command extension must be an object")
	}
	raw, ok = namespace["commands"]
	if !ok {
		return nil, nil
	}
	var commands map[string]string
	if err := json.Unmarshal(raw, &commands); err != nil || commands == nil {
		return nil, fmt.Errorf("Skillshare commands must be a name-to-file map")
	}
	if err := validateCommands(root, commands); err != nil {
		return nil, err
	}
	return commands, nil
}

func validateCommands(root string, commands map[string]string) error {
	for name, rel := range commands {
		if !commandName.MatchString(name) {
			return fmt.Errorf("invalid command name %q", name)
		}
		if rel == "" || strings.HasPrefix(rel, "/") || strings.ContainsAny(rel, `\:`) {
			return fmt.Errorf("invalid command path for %s", name)
		}
		for _, part := range strings.Split(rel, "/") {
			if part == ".." || part == ".git" {
				return fmt.Errorf("invalid command path for %s", name)
			}
		}
		path, err := resolveSourcePath(root, filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("command %s: %w", name, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("command %s: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("command %s must be a regular executable file", name)
		}
	}
	return nil
}
