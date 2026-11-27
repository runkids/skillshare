package install

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/sourcewalk"
)

// FindTrackedCheckout maps a name typed by the user to a tracked checkout
// under sourceRoot. It tries the name as given, then with "_" added to the
// last path segment, so "org/team" finds the checkout "org/_team" that
// `install --track --into org` creates. A name that would leave sourceRoot
// never matches. It returns the slash-form name relative to sourceRoot and the
// checkout path, or empty strings when nothing matches.
func FindTrackedCheckout(sourceRoot, input string) (name, repoPath string) {
	sourceRoot = filepath.Clean(sourceRoot)
	for _, candidate := range trackedRepoCandidates(input) {
		p := filepath.Join(sourceRoot, filepath.FromSlash(candidate))
		rel, err := filepath.Rel(sourceRoot, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if IsTrackedCheckout(p) {
			return candidate, p
		}
	}
	return "", ""
}

// FindTrackedCheckoutUnlessSkill is FindTrackedCheckout, except that the
// shorthand inside a folder (org/team -> org/_team) does not apply when a
// directory already exists at org/team: that is the skill the name refers to.
func FindTrackedCheckoutUnlessSkill(sourceRoot, input string) (name, repoPath string) {
	name, repoPath = FindTrackedCheckout(sourceRoot, input)
	if repoPath == "" {
		return "", ""
	}
	if typed := cleanName(input); name != typed && path.Dir(name) != "." {
		if info, err := os.Stat(filepath.Join(sourceRoot, filepath.FromSlash(typed))); err == nil && info.IsDir() {
			return "", ""
		}
	}
	return name, repoPath
}

// cleanName turns a typed name into a clean slash path ("org/team/" -> "org/team").
func cleanName(input string) string {
	return path.Clean(filepath.ToSlash(input))
}

// trackedRepoCandidates lists the slash-form names a tracked repo may have for
// input: input itself, then input with "_" added to its last path segment.
func trackedRepoCandidates(input string) []string {
	input = cleanName(input)
	if base := path.Base(input); !strings.HasPrefix(base, "_") {
		return []string{input, path.Join(path.Dir(input), "_"+base)}
	}
	return []string{input}
}

// MatchTrackedRepos returns the repos (slash paths, as GetTrackedRepos lists
// them) that input names: by full path, with "_" added to the last segment, or
// by basename, with or without its "_".
func MatchTrackedRepos(repos []string, input string) []string {
	input = cleanName(input)
	candidates := trackedRepoCandidates(input)
	var matches []string
	for _, repo := range repos {
		base := path.Base(repo)
		if slices.Contains(candidates, repo) || base == input || base == "_"+input {
			matches = append(matches, repo)
		}
	}
	return matches
}

// ResolveTrackedRepo is FindTrackedCheckout plus a fallback to the one tracked
// repo whose basename matches the name (with or without its "_"). More than
// one such repo is an error. Returns empty strings and no error when nothing
// matches.
func ResolveTrackedRepo(sourceRoot, input string, walk ...sourcewalk.Options) (name, repoPath string, err error) {
	if name, repoPath = FindTrackedCheckout(sourceRoot, input); repoPath != "" {
		return name, repoPath, nil
	}
	repos, err := GetTrackedRepos(sourceRoot, walk...)
	if err != nil {
		return "", "", fmt.Errorf("failed to list tracked repositories: %w", err)
	}
	matches := MatchTrackedRepos(repos, input)
	if len(matches) > 1 {
		return "", "", fmt.Errorf("multiple tracked repositories match: %s — use the full path", input)
	}
	if len(matches) == 0 {
		return "", "", nil
	}
	match := matches[0]
	return match, filepath.Join(filepath.Clean(sourceRoot), filepath.FromSlash(match)), nil
}

// DeclarationsWithoutCheckout lists declared paths that have no checkout here.
// IsTrackedCheckout follows links: a link to a checkout keeps ordinary removal.
func DeclarationsWithoutCheckout(sourceDir string, store *MetadataStore) []string {
	var names []string
	for _, key := range store.TrackedDeclarations() {
		rel := filepath.ToSlash(KeyToRelPath(key, store.Get(key)))
		if !IsTrackedCheckout(filepath.Join(sourceDir, filepath.FromSlash(rel))) {
			names = append(names, rel)
		}
	}
	return names
}

// FindTrackedDeclaration resolves an exact path before tracked-repo shorthand.
// Only declarations with no checkout are returned; exact distinguishes an
// occupied declared path from shorthand that must yield to an existing skill.
func FindTrackedDeclaration(sourceDir string, store *MetadataStore, input string) (name string, exact bool, err error) {
	names := DeclarationsWithoutCheckout(sourceDir, store)
	for _, rel := range names {
		if rel == cleanName(input) {
			return rel, true, nil
		}
	}
	matches := MatchTrackedRepos(names, input)
	if len(matches) > 1 {
		return "", false, fmt.Errorf("multiple tracked repositories match: %s — use the full path", input)
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	return matches[0], false, nil
}
