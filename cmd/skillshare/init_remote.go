package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	gitops "skillshare/internal/git"
	ssync "skillshare/internal/sync"
)

const remoteInspectTimeout = 60 * time.Second

// remoteRepo is what init learned about a remote before writing anything.
type remoteRepo struct {
	url       string
	reachable bool
	reason    string   // why it could not be reached
	names     []string // top-level skill folder names
	skills    int      // skills found (nested folders count individually)
	agents    int
	extras    int
	// rootScope means the repo versions the whole skillshare folder
	// (skills/, agents/, extras/), as git_root "root" pushes it.
	rootScope bool
	// subdir is the folder holding the skills in a skills-only repo.
	subdir string
}

func (r *remoteRepo) hasSkills() bool { return r != nil && r.reachable && r.skills > 0 }

// inspectRemote shallow-clones the repo into a temporary folder to see what
// it holds. Nothing outside the temporary folder is touched.
func inspectRemote(url string) *remoteRepo {
	r := &remoteRepo{url: url}
	if strings.HasPrefix(url, "-") {
		r.reason = "invalid URL"
		return r
	}
	tmp, err := os.MkdirTemp("", "skillshare-remote-*")
	if err != nil {
		r.reason = err.Error()
		return r
	}
	defer os.RemoveAll(tmp)

	ctx, cancel := context.WithTimeout(context.Background(), remoteInspectTimeout)
	defer cancel()
	repo := filepath.Join(tmp, "repo")
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--quiet", "--", url, repo)
	cmd.Env = remoteFetchEnv(url)
	if out, err := cmd.CombinedOutput(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			r.reason = "timed out"
		} else {
			r.reason = remoteFailureReason(string(out))
		}
		return r
	}
	r.reachable = true

	skillsDir := repo
	switch {
	case gitignoreHasLine(filepath.Join(repo, ".gitignore"), "config.yaml") && isDir(filepath.Join(repo, "skills")):
		r.rootScope = true
		skillsDir = filepath.Join(repo, "skills")
		r.agents = countEntries(filepath.Join(repo, "agents"))
		r.extras = countEntries(filepath.Join(repo, "extras"))
	case isDir(filepath.Join(repo, "skills")) && !hasSkillDirsBesides(repo, "skills"):
		r.subdir = "skills"
		skillsDir = filepath.Join(repo, "skills")
	}
	entries, _ := os.ReadDir(skillsDir)
	for _, e := range entries {
		if isSkillEntry(skillsDir, e) {
			r.names = append(r.names, e.Name())
		}
	}
	follow := skillFollowSet(skillsDir, nil, repo)
	if discovered, _, err := ssync.DiscoverSourceSkillsWithOptions(skillsDir, ssync.DiscoveryOptions{Follow: follow}); err == nil {
		r.skills = len(discovered)
	} else if follow != nil && follow.Err() != nil {
		r.reachable = false
		r.reason = "failed to discover skills: " + err.Error()
	}
	return r
}

// remoteFailureReason turns git's error output into a short reason.
func remoteFailureReason(out string) string {
	switch {
	case strings.Contains(out, "Could not read from remote"), strings.Contains(out, "Permission denied"):
		return "no access (check your SSH key or token)"
	case strings.Contains(out, "not found"), strings.Contains(out, "does not exist"):
		return "repo not found"
	case strings.Contains(out, "Could not resolve host"):
		return "no network"
	}
	if line, _, _ := strings.Cut(strings.TrimSpace(out), "\n"); line != "" {
		return line
	}
	return "unreachable"
}

// pullRemote fetches origin and moves the repo at gitRoot onto the remote's
// default branch. The caller must make sure gitRoot holds no work of its own.
func pullRemote(gitRoot, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), remoteFetchTimeout*4)
	defer cancel()
	fetch := exec.CommandContext(ctx, "git", "fetch", "origin")
	fetch.Dir = gitRoot
	fetch.Env = remoteFetchEnv(url)
	if out, err := fetch.CombinedOutput(); err != nil {
		return fmt.Errorf("fetch failed: %s", remoteFailureReason(string(out)))
	}
	branch, err := gitops.GetRemoteDefaultBranch(gitRoot)
	if err != nil {
		return err
	}
	return resetToRemoteBranch(gitRoot, branch)
}

// resetToRemoteBranch checks out origin/<branch> and tracks it so push and
// pull work without naming the remote.
func resetToRemoteBranch(gitRoot, branch string) error {
	reset := exec.Command("git", "reset", "--hard", "origin/"+branch)
	reset.Dir = gitRoot
	if out, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %s", strings.TrimSpace(string(out)))
	}
	local := "main"
	current := exec.Command("git", "branch", "--show-current")
	current.Dir = gitRoot
	if out, err := current.Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "" {
			local = b
		}
	}
	track := exec.Command("git", "branch", "--set-upstream-to=origin/"+branch, local)
	track.Dir = gitRoot
	track.Run() // best-effort
	return nil
}

func gitignoreHasLine(path, line string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}

// hasSkillDirsBesides reports whether dir holds a skill folder other than
// the one named skip.
func hasSkillDirsBesides(dir, skip string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != skip && isSkillEntry(dir, e) {
			return true
		}
	}
	return false
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func countEntries(dir string) int {
	entries, _ := os.ReadDir(dir)
	n := 0
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			n++
		}
	}
	return n
}
