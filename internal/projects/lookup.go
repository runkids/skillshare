package projects

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// ErrNotFound means no declared project matched the supplied name or root.
var ErrNotFound = errors.New("project not found")

// AmbiguousError lists every matching declared key in sorted order.
type AmbiguousError struct{ Candidates []string }

func (e *AmbiguousError) Error() string {
	return "ambiguous project; candidates: " + strings.Join(e.Candidates, ", ")
}

// FindRoot resolves only a root, preserving the matching declared key exactly.
func FindRoot(projects map[string]config.ManagedProject, root, cwd, home string) (string, error) {
	return find(projects, root, cwd, home, false)
}

// Find resolves a CLI token as a name and relative root unless it explicitly
// selects a root. Name/root collisions never depend on map iteration order.
func Find(projects map[string]config.ManagedProject, token, cwd, home string) (string, error) {
	return find(projects, token, cwd, home, !explicitRoot(token))
}

func find(projects map[string]config.ManagedProject, token, cwd, home string, names bool) (string, error) {
	identity, err := Identity(token, cwd, home)
	if err != nil {
		return "", err
	}
	var candidates []string
	for key, project := range projects {
		root, err := Identity(key, cwd, home)
		if err != nil {
			continue
		}
		// Default names use the declared spelling, just like config expansion.
		// Only tilde expansion needs the resolver's explicit home context.
		nameRoot := key
		if utils.HasTildePrefix(key) {
			nameRoot = filepath.Join(home, key[1:])
		}
		if utils.PathsEqual(identity, root) || (names && project.ProjectName(nameRoot) == token) {
			candidates = append(candidates, key)
		}
	}
	sort.Strings(candidates)
	switch len(candidates) {
	case 0:
		return "", ErrNotFound
	case 1:
		return candidates[0], nil
	default:
		return "", &AmbiguousError{Candidates: candidates}
	}
}

func explicitRoot(token string) bool {
	if token == "" {
		return false
	}
	if os.IsPathSeparator(token[0]) || token[0] == '~' {
		return true
	}
	if strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") {
		return true
	}
	if filepath.Separator == '\\' && (strings.HasPrefix(token, `.\`) || strings.HasPrefix(token, `..\`)) {
		return true
	}
	// Drive and UNC spellings select roots even on a non-Windows host; their
	// identity still follows the host's native filepath semantics.
	return len(token) >= 2 && ((token[0] >= 'a' && token[0] <= 'z') || (token[0] >= 'A' && token[0] <= 'Z')) && token[1] == ':' || strings.HasPrefix(token, `\\`)
}

// ValidateName rejects CLI names that lookup would interpret only as roots.
// Existing dashboard registrations keep their existing validation contract.
func ValidateName(name string) error {
	if name != "" && explicitRoot(name) {
		return fmt.Errorf("project name %q looks like a root; choose a name without a root prefix", name)
	}
	return nil
}
