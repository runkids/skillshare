package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// inspectCommands reads only Skillshare's decoded portable extension, never native slash commands.
func inspectCommands(root string, manifest map[string]json.RawMessage) (map[string]string, error) {
	raw, ok := manifest["extensions"]
	if !ok {
		return nil, nil
	}
	var extensions map[string]json.RawMessage
	if err := json.Unmarshal(raw, &extensions); err != nil {
		return nil, agentError{key: "plugins.error.commandMetadata", message: "Cannot read command metadata: " + err.Error(), args: map[string]string{"detail": err.Error()}, cause: err}
	}
	extension, ok := extensions["cc.runkids.skillshare"]
	if !ok {
		return nil, nil
	}
	var namespace map[string]json.RawMessage
	if err := json.Unmarshal(extension, &namespace); err != nil || namespace == nil {
		return nil, agentError{key: "plugins.error.commandNamespace", message: "Skillshare command extension must be an object"}
	}
	raw, ok = namespace["commands"]
	if !ok {
		return nil, nil
	}
	var commands map[string]string
	if err := json.Unmarshal(raw, &commands); err != nil || commands == nil {
		return nil, agentError{key: "plugins.error.commandMap", message: "Skillshare commands must be a name-to-file map"}
	}
	if err := validateCommands(root, commands); err != nil {
		return nil, err
	}
	return commands, nil
}

func validateCommands(root string, commands map[string]string) error {
	for name, rel := range commands {
		if !namePattern.MatchString(name) {
			return agentError{key: "plugins.error.commandName", message: "Invalid command name: " + name, args: map[string]string{"name": name}}
		}
		if rel == "" || strings.HasPrefix(rel, "/") || strings.ContainsAny(rel, `\:`) {
			return agentError{key: "plugins.error.commandPath", message: "Invalid command path for " + name, args: map[string]string{"name": name, "path": rel}}
		}
		for _, part := range strings.Split(rel, "/") {
			if part == ".." || part == ".git" {
				return agentError{key: "plugins.error.commandPath", message: "Invalid command path for " + name, args: map[string]string{"name": name, "path": rel}}
			}
		}
		path, err := resolveSourcePath(root, filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return agentError{key: "plugins.error.commandFile", message: "Cannot use executable for command " + name + ": " + err.Error(), args: map[string]string{"name": name, "detail": err.Error()}, cause: err}
		}
		info, err := os.Stat(path)
		if err != nil {
			return agentError{key: "plugins.error.commandFile", message: "Cannot use executable for command " + name + ": " + err.Error(), args: map[string]string{"name": name, "detail": err.Error()}, cause: err}
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return agentError{key: "plugins.error.commandExecutable", message: "Command " + name + " must be a regular executable file", args: map[string]string{"name": name}}
		}
	}
	return nil
}
