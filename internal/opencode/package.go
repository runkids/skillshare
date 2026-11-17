// Package opencode describes native loader inputs without loading plugin code.
package opencode

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// ConfigKey recognizes the installed major rather than guessing a future API.
func ConfigKey(version string) (string, error) {
	fields := strings.Fields(version)
	if len(fields) > 0 {
		v := strings.TrimPrefix(fields[len(fields)-1], "v")
		if strings.HasPrefix(v, "1.") {
			return "plugin", nil
		}
		if strings.HasPrefix(v, "2.") {
			return "plugins", nil
		}
	}
	return "", fmt.Errorf("unrecognized OpenCode version; supports versions 1.x and 2.x")
}

// FileURL keeps the native file URL spelling used by existing receipts.
func FileURL(p string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(p)}).String()
}

// Package validates the server entry selected by OpenCode v2. It leaves exports,
// dependencies intact. Conditional server exports fail closed; read never executes code.
func Package(read func(string) ([]byte, error)) error {
	var m struct {
		Name    string `json:"name"`
		Exports any    `json:"exports"`
	}
	data, err := read("package.json")
	if err != nil {
		return fmt.Errorf("OpenCode package.json: %w", err)
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("OpenCode package.json: %w", err)
	}
	if m.Name != "" && m.Exports != nil {
		exports := m.Exports
		if obj, ok := exports.(map[string]any); ok {
			if server, has := obj["./server"]; has {
				exports = server
			} else if root, has := obj["."]; has {
				exports = root
			}
		}
		entry, ok := exports.(string)
		if !ok {
			return fmt.Errorf("OpenCode server exports must be a string (. or ./server); conditional and fallback exports cannot be verified offline")
		}
		if !strings.HasPrefix(entry, "./") || strings.ContainsAny(entry, "\\:") || !filepath.IsLocal(filepath.FromSlash(strings.TrimPrefix(entry, "./"))) {
			return fmt.Errorf("OpenCode export %q must stay inside the package", entry)
		}
		if _, err := read(path.Clean(strings.TrimPrefix(entry, "./"))); err != nil {
			return fmt.Errorf("OpenCode export %q is missing: %w", entry, err)
		}
		return nil
	}
	for _, base := range []string{"server", "index"} {
		for _, ext := range []string{".ts", ".tsx", ".js"} {
			if _, err := read(base + ext); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("OpenCode v2 package requires name and exports, or a root server/index .ts, .tsx or .js file; --entry cannot override the directory loader")
}
