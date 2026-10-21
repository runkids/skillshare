package projects

import (
	"errors"
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
// Collisions never depend on map iteration order.
func FindRoot(projects map[string]config.ManagedProject, root, cwd, home string) (string, error) {
	identity, err := Identity(root, cwd, home)
	if err != nil {
		return "", err
	}
	var candidates []string
	for key := range projects {
		root, err := Identity(key, cwd, home)
		if err != nil {
			continue
		}
		if utils.PathsEqual(identity, root) {
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
