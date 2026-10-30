package plugin

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// openCodeImportPath accepts both file://C:/... and file:///C:/... spellings.
// A remote URL authority must never be silently treated as a local directory.
func openCodeImportPath(id string) string {
	u, err := url.Parse(id)
	if err != nil || u.Scheme != "file" {
		return id
	}
	path := u.Path
	drive := func(s string) bool {
		return len(s) == 2 && s[1] == ':' && (s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z')
	}
	switch {
	case drive(u.Host):
		path = u.Host + path
	case u.Host != "" && u.Host != "localhost":
		return ""
	case len(path) >= 3 && path[0] == '/' && drive(path[1:3]):
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

// A native file belongs to a package only when that package resolves to this
// entry. Npm identifiers have no local root and keep their inventory identity.
func openCodeImportRoot(id, entry string) (string, bool) {
	path := openCodeImportPath(id)
	if !filepath.IsAbs(path) {
		return "", !strings.HasPrefix(id, "file:")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		if info, err := os.Stat(filepath.Join(parent, "package.json")); err == nil && info.Mode().IsRegular() {
			resolved, err := openCodeEntry(parent, entry)
			if err == nil && filepath.Clean(filepath.Join(parent, resolved)) == filepath.Clean(path) {
				return parent, true
			}
		}
		if filepath.Dir(parent) == parent {
			return "", false
		}
	}
}
