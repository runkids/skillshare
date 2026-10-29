package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
)

// GitUpdateCandidate isolates a pull from a checkout already published to tools.
// Close removes only the temporary clone; publication never fetches upstream
// refs, though a partial clone may lazily fetch the blobs it lacks.
// It lives in install because internal/git imports install for authentication.
type GitUpdateCandidate struct {
	Path       string
	BeforeHash string
	repoPath   string
	tempDir    string
	discard    bool
	state      [3]string // status, staged patch, unstaged patch
	env        []string  // for local transports between repoPath and Path
}

// StageGitUpdate clones the current branch and preserves the pull's local Git
// configuration. discard omits local edits for the update command's force mode.
func StageGitUpdate(repoPath string, discard bool) (_ *GitUpdateCandidate, err error) {
	repoPath, err = filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	before, err := getGitFullHash(repoPath)
	if err != nil {
		return nil, fmt.Errorf("cannot determine candidate base commit: %w", err)
	}
	if before == "" {
		return nil, fmt.Errorf("cannot determine candidate base commit: empty commit hash")
	}
	state, err := gitCandidateState(repoPath)
	if err != nil && !discard {
		return nil, err
	}
	temp, err := os.MkdirTemp("", "skillshare-git-update-*")
	if err != nil {
		return nil, err
	}
	// Partial clones lack intermediate blobs; git >= 2.45.1 refuses to
	// lazy-fetch them through a local upload-pack unless re-enabled.
	env := append(AuthEnvForURL(getRemoteURL(repoPath)), "GIT_NO_LAZY_FETCH=0")
	c := &GitUpdateCandidate{Path: filepath.Join(temp, filepath.Base(repoPath)), BeforeHash: before, repoPath: repoPath, tempDir: temp, discard: discard, state: state, env: env}
	defer func() {
		if err != nil {
			c.Close()
		}
	}()
	// Shared objects are immutable; fetched and merged candidate objects stay
	// private until Publish imports the approved revision.
	if err = runGitCommandEnv([]string{"clone", "--quiet", "--shared", "--no-checkout", repoPath, c.Path}, "", c.env); err != nil {
		return nil, err
	}
	configPath, err := gitOutput(repoPath, "rev-parse", "--path-format=absolute", "--git-path", "config")
	if err != nil {
		return nil, err
	}
	if err = copyFile(configPath, filepath.Join(c.Path, ".git", "config")); err != nil {
		return nil, err
	}
	// Sparse state must be copied before checkout, including per-worktree config.
	for _, name := range []string{"config.worktree", "info/sparse-checkout"} {
		path, e := gitOutput(repoPath, "rev-parse", "--git-path", name)
		if e != nil {
			return nil, e
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(repoPath, path)
		}
		if _, e := os.Stat(path); os.IsNotExist(e) {
			continue
		}
		if err = copyFile(path, filepath.Join(c.Path, ".git", name)); err != nil {
			return nil, err
		}
	}
	if err = runGitCommandEnv([]string{"config", "--worktree", "core.worktree", c.Path}, c.Path, nil); err != nil {
		return nil, err
	}
	if err = gitResetHard(c.Path, before); err != nil {
		return nil, err
	}
	// Relative filesystem remotes must still resolve from the original repo.
	remotes, err := gitOutput(repoPath, "remote")
	if err != nil {
		return nil, err
	}
	for _, remote := range strings.Fields(remotes) {
		u, e := gitOutput(repoPath, "remote", "get-url", remote)
		if e != nil {
			return nil, e
		}
		if u != "" && !filepath.IsAbs(u) && !strings.Contains(u, ":") && !strings.HasPrefix(u, "~") {
			if err = runGitCommandEnv([]string{"remote", "set-url", remote, filepath.Join(repoPath, u)}, c.Path, nil); err != nil {
				return nil, err
			}
		}
	}
	if !discard {
		if err = applyGitCandidatePatches(c.Path, state[1:]); err != nil {
			return nil, err
		}
	}
	// Include untracked and ignored files, as the existing audit did. Overlay
	// edited worktree bytes too: git apply can change them via core.autocrlf.
	commands := [][]string{{"ls-files", "--others", "-z"}}
	if !discard {
		commands = append(commands,
			[]string{"diff", "--name-only", "--diff-filter=ACMRT", "-z", "HEAD"},
			[]string{"diff", "--name-only", "--diff-filter=ACMRT", "-z"})
	}
	files := make(map[string]bool)
	for _, args := range commands {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, e := cmd.Output()
		if e != nil {
			return nil, e
		}
		for _, name := range strings.Split(string(out), "\x00") {
			if name != "" {
				files[name] = true
			}
		}
	}
	for name := range files {
		src, dst := filepath.Join(repoPath, name), filepath.Join(c.Path, name)
		if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return nil, err
		}
		info, e := os.Lstat(src)
		if e != nil {
			return nil, e
		}
		// Never write through a candidate symlink when replacing a local edit.
		if err = os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(src)
			if e != nil {
				return nil, e
			}
			err = os.Symlink(target, dst)
		} else {
			err = copyFile(src, dst)
		}
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// prepareCandidateAudit keeps path-sensitive scans equivalent after relocation.
func prepareCandidateAudit(sourceDir, logicalPath, candidatePath string) error {
	candidatePath, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return err
	}
	if err := sourcewalk.Walk(candidatePath, sourcewalk.Options{}, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("cannot resolve candidate symlink %s: %w", path, err)
		}
		rel, err := filepath.Rel(candidatePath, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("candidate symlink %s escapes the isolated checkout", path)
		}
		return nil
	}); err != nil {
		return err
	}
	if sourceDir == "" {
		sourceDir = filepath.Dir(logicalPath)
	}
	rel, err := filepath.Rel(sourceDir, logicalPath)
	if err != nil {
		return err
	}
	store, err := LoadMetadata(sourceDir)
	if err != nil {
		return nil // The ordinary scanner also skips unavailable metadata.
	}
	if entry := store.GetByPath(filepath.ToSlash(rel)); entry != nil {
		snapshot := NewMetadataStore()
		snapshot.Set(filepath.Base(candidatePath), entry)
		return snapshot.Save(filepath.Dir(candidatePath))
	}
	return nil
}

func applyGitCandidatePatches(repoPath string, patches []string) error {
	for i, patch := range patches {
		if patch == "" {
			continue
		}
		args := []string{"apply", "--binary"}
		if i == 0 {
			args = append(args, "--index")
		}
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Stdin = repoPath, strings.NewReader(patch)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cannot apply local edits: %w: %s", err, out)
		}
	}
	return nil
}

func gitCandidateState(repoPath string) (state [3]string, err error) {
	for i, args := range [][]string{
		{"status", "--porcelain", "-z", "-uall"},
		{"diff", "--binary", "--cached", "HEAD"},
		{"diff", "--binary"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		out, e := cmd.Output()
		if e != nil {
			return state, e
		}
		state[i] = string(out)
	}
	return state, nil
}

// Publish imports precisely revision from the local candidate, not upstream.
// The active checkout must still match the state used to prepare the candidate.
func (c *GitUpdateCandidate) Publish(revision string) error {
	head, err := getGitFullHash(c.repoPath)
	if err != nil {
		return err
	}
	state, err := gitCandidateState(c.repoPath)
	if err != nil && !c.discard {
		return err
	}
	if head != c.BeforeHash || state != c.state {
		return fmt.Errorf("checkout changed while auditing; update aborted, retry after committing or stashing changes")
	}
	var localEdits string
	if c.discard && (c.state[1] != "" || c.state[2] != "") {
		// Create an immutable snapshot without changing the worktree or stash ref.
		localEdits, err = gitOutput(c.repoPath, "-c", "user.name=skillshare", "-c", "user.email=skillshare@localhost", "stash", "create")
		if err != nil {
			return fmt.Errorf("cannot preserve local edits before force update: %w", err)
		}
	}
	if err := runGitCommandEnv([]string{"fetch", "--quiet", "--no-tags", c.Path, revision, "+refs/remotes/*:refs/remotes/*"}, c.repoPath, c.env); err != nil {
		return err
	}
	restoreCheckout := GitCheckoutRecovery(c.repoPath, revision)
	if c.discard || c.state[0] == "" {
		err = gitResetHard(c.repoPath, revision)
	} else {
		// install --track --update permits compatible local edits.
		err = runGitCommandEnv([]string{"merge", "--ff-only", "--no-edit", revision}, c.repoPath, nil)
	}
	if err != nil {
		_ = runGitCommandEnv([]string{"merge", "--abort"}, c.repoPath, nil)
		if head, _ := getGitFullHash(c.repoPath); head == c.BeforeHash {
			restoreCheckout()
			if localEdits != "" {
				// Include originally deleted paths when restoring the saved worktree.
				baseErr := runGitCommandEnv([]string{"read-tree", "-m", c.BeforeHash}, c.repoPath, nil)
				// Restore every writable path even if another path is read-only.
				restoreErr := runGitCommandEnv([]string{"restore", "--source=" + localEdits, "--worktree", "--", "."}, c.repoPath, nil)
				indexErr := runGitCommandEnv([]string{"read-tree", "-m", localEdits + "^2"}, c.repoPath, nil)
				if baseErr != nil || restoreErr != nil || indexErr != nil {
					err = fmt.Errorf("%w (restoring local edits failed: base: %v; worktree: %v; index: %v)", err, baseErr, restoreErr, indexErr)
				}
			}
		}
	}
	return err
}

func (c *GitUpdateCandidate) Close() {
	_ = os.RemoveAll(c.tempDir)
}
