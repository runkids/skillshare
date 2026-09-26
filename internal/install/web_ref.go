package install

import (
	"context"
	"fmt"
	"path"
	"strings"
)

// webRef is the "{ref}/{path}" tail of a GitHub, GitLab or Bitbucket web URL,
// such as "v1.2.0/skills/foo" from github.com/o/r/tree/v1.2.0/skills/foo.
type webRef struct {
	tail string
	blob bool
}

// split returns the path left after ref, or ok=false when ref is not a
// leading run of whole segments of the tail.
func (w webRef) split(ref string) (subdir string, explicit, ok bool) {
	var rest string
	switch {
	case w.tail == ref:
	case strings.HasPrefix(w.tail, ref+"/"):
		rest = w.tail[len(ref)+1:]
	default:
		return "", false, false
	}
	subdir, explicit = trimSkillFileSuffix(rest, w.blob)
	return subdir, explicit, true
}

// applyWebRef takes the tail's first segment as the ref and returns the subdir
// after it. The ref (branch, tag or commit SHA) becomes the clone ref, so a
// pasted or hub-listed URL installs the version it names. A branch name that
// contains "/" is corrected later by resolveWebRef.
func (s *Source) applyWebRef(w webRef) string {
	s.webRef = w
	if w.tail == "" {
		return ""
	}
	s.Branch, _, _ = strings.Cut(w.tail, "/")
	subdir, explicit, _ := w.split(s.Branch)
	s.ExplicitSkill = explicit
	return subdir
}

// resolveWebRef settles where the ref ends in a web URL's "{ref}/{path}" when
// the ref may contain "/", as in tree/feature/x/skills/foo. A Branch that
// already covers more of the tail (from --branch or a saved config) only moves
// the subdir. Otherwise the remote's branches and tags decide. A URL ref that
// matches none of them fails instead of installing something else, unless an
// unrelated --branch was given, which then only borrows the URL's subdir.
func resolveWebRef(s *Source) error {
	w := s.webRef
	if !w.ambiguous() {
		return nil
	}
	first, _, _ := strings.Cut(w.tail, "/")
	if s.Branch != "" && s.Branch != first && w.covers(s.Branch) {
		return s.setWebRefSubdir(s.Branch)
	}
	if IsCommitSHA(first) {
		return nil
	}

	refs, err := listRemoteRefs(s)
	if err != nil || refs[first] {
		return nil // on error, let the clone report the real problem
	}
	override := s.Branch != "" && s.Branch != first
	segments := strings.Split(w.tail, "/")
	for i := len(segments); i > 1; i-- {
		if ref := strings.Join(segments[:i], "/"); refs[ref] {
			if !override {
				s.Branch = ref
			}
			return s.setWebRefSubdir(ref)
		}
	}
	if override {
		return nil
	}
	return fmt.Errorf("ref %q from the URL was not found on the remote; pass --branch to choose one (needed when a branch name contains \"/\")", first)
}

// HasAmbiguousWebRef reports whether the source's URL ref might contain "/",
// so its real ref and subdir are only known after resolveWebRef. Callers that
// group sources by ref before installing should install these on their own.
func (s *Source) HasAmbiguousWebRef() bool {
	return s.webRef.ambiguous()
}

func (w webRef) ambiguous() bool {
	return strings.Contains(w.tail, "/")
}

func (w webRef) covers(ref string) bool {
	return w.tail == ref || strings.HasPrefix(w.tail, ref+"/")
}

// setWebRefSubdir moves Subdir, and a Name derived from it, to the path after
// ref. A repo-root copy (Subdir cleared for a whole-repo clone) keeps its root.
func (s *Source) setWebRefSubdir(ref string) error {
	if s.Subdir == "" {
		return nil
	}
	subdir, explicit, _ := s.webRef.split(ref)
	if subdir != "" {
		if err := validateRepoSubdir(subdir); err != nil {
			return err
		}
	}
	if s.Name == path.Base(s.Subdir) {
		if subdir != "" {
			s.Name = path.Base(subdir)
		} else {
			s.Name = strings.TrimSuffix(path.Base(s.CloneURL), ".git")
		}
	}
	s.Subdir = subdir
	s.ExplicitSkill = explicit
	return nil
}

// listRemoteRefs returns the remote's branch and tag names.
func listRemoteRefs(s *Source) (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, "ls-remote", "--heads", "--tags", s.CloneURL)
	cmd.Env = append(cmd.Env, s.authEnv()...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	refs := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		_, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		name = strings.TrimSuffix(name, "^{}")
		if b, ok := strings.CutPrefix(name, "refs/heads/"); ok {
			refs[b] = true
		} else if t, ok := strings.CutPrefix(name, "refs/tags/"); ok {
			refs[t] = true
		}
	}
	return refs, nil
}
