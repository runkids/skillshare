package server

import (
	"path/filepath"
	"strings"
)

// validSourceName reports whether a client-supplied skill or repository name
// stays inside the skills source once joined to it: it must be non-empty,
// relative, and must not start with "..". Nested names such as "group/skill"
// are accepted; the path itself is not checked on disk. Windows separators and
// volume prefixes are rejected on every host, as sourcewalk does for declared
// names, so a name cannot change meaning between platforms.
func validSourceName(name string) bool {
	if strings.TrimSpace(name) == "" || strings.Contains(name, `\`) {
		return false
	}
	if len(name) >= 2 && name[1] == ':' && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || filepath.IsAbs(clean) {
		return false
	}
	return !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
