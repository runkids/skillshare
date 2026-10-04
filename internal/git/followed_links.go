package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// FollowedLink describes a declared link physically reachable by git staging.
type FollowedLink struct {
	Path       string // Relative to the canonical staging directory.
	IgnoreFile string
	IgnoreLine string
}

// FollowedLinksStaged checks physical reachability without resolving the final
// link component. Git never traverses a link on its way to another entry.
func FollowedLinksStaged(stagingDir string, follow *sourcewalk.FollowSet) ([]FollowedLink, error) {
	return declaredLinkLocations(stagingDir, follow, false)
}

func declaredLinkLocations(stagingDir string, follow *sourcewalk.FollowSet, includeMissing bool) ([]FollowedLink, error) {
	if follow == nil || len(follow.ParsedEntries()) == 0 {
		return nil, nil
	}
	stagingRoot, err := sourcewalk.Canonicalize(stagingDir)
	if err != nil {
		return nil, fmt.Errorf("resolve staging directory: %w", err)
	}
	parent := follow.SourceRoot()
	if parent == "" {
		return nil, fmt.Errorf("cannot resolve declared links' source parent")
	}
	var links []FollowedLink
	for _, name := range follow.ParsedEntries() {
		path := filepath.Join(parent, name)
		if !utils.PathHasPrefix(path, strings.TrimRight(stagingRoot, string(filepath.Separator))+string(filepath.Separator)) {
			continue
		}
		rel, err := filepath.Rel(stagingRoot, path)
		if err != nil {
			return nil, err
		}
		// Inspect the existing path to its parent, excluding the declared link.
		crossed, err := linkComponent(stagingRoot, filepath.Dir(rel))
		if err != nil {
			return nil, err
		}
		if crossed != "" {
			continue
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) && !includeMissing {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if info != nil && !utils.IsLinkMode(path, info.Mode()) {
			continue
		}
		links = append(links, FollowedLink{Path: filepath.ToSlash(rel), IgnoreFile: filepath.Join(parent, ".gitignore"), IgnoreLine: install.FollowedIgnoreLine(name)})
	}
	return links, nil
}

// linkComponent returns the first existing link component, including the final
// component. A missing suffix has no link to replace; other errors fail closed.
func linkComponent(root, rel string) (string, error) {
	current := root
	if rel == "." {
		return "", nil
	}
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if utils.IsLinkMode(current, info.Mode()) {
			return current, nil
		}
	}
	return "", nil
}

// FollowedLinksError blocks staging until the user fixes all listed entries.
type FollowedLinksError struct{ Problems []string }

func (e *FollowedLinksError) Error() string {
	return "followed links must be untracked and ignored before staging:\n" + strings.Join(e.Problems, "\n")
}

func CheckFollowedLinks(stagingDir string, follow *sourcewalk.FollowSet) error {
	links, err := FollowedLinksStaged(stagingDir, follow)
	if err != nil {
		return err
	}
	var problems []string
	for _, link := range links {
		indexed, err := IsPathIndexed(stagingDir, link.Path)
		if err != nil {
			return err
		}
		if indexed {
			problems = append(problems, fmt.Sprintf("%s is indexed; run %s and add %q to %s", link.Path, UntrackCommand(link.Path), link.IgnoreLine, link.IgnoreFile))
			continue
		}
		ignored, err := IsPathIgnored(stagingDir, link.Path)
		if err != nil {
			return err
		}
		if !ignored {
			problems = append(problems, fmt.Sprintf("%s is not-ignored; add %q to %s", link.Path, link.IgnoreLine, link.IgnoreFile))
		}
	}
	if len(problems) > 0 {
		return &FollowedLinksError{Problems: problems}
	}
	return nil
}

// IsPathIndexed uses literal pathspecs so accepted names never expand as globs.
func IsPathIndexed(dir, path string) (bool, error) {
	cmd := exec.Command("git", "--literal-pathspecs", "ls-files", "--error-unmatch", "--", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check indexed path %s: %w", path, err)
}

func IsPathIgnored(dir, path string) (bool, error) {
	cmd := exec.Command("git", "check-ignore", "-q", "--no-index", "--", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check ignored path %s: %w", path, err)
}

// UntrackCommand quotes literal names for the platform's interactive shell.
func UntrackCommand(path string) string {
	if runtime.GOOS == "windows" {
		return "git rm --cached -- '" + strings.ReplaceAll(path, "'", "''") + "'"
	}
	return "git rm --cached -- '" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}
