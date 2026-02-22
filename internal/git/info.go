package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"skillshare/internal/install"
)

// DiffStats holds git diff statistics
type DiffStats struct {
	FilesChanged int
	Insertions   int
	Deletions    int
}

// CommitInfo holds a single commit info
type CommitInfo struct {
	Hash    string
	Message string
}

// UpdateInfo holds info about changes from an update
type UpdateInfo struct {
	Commits    []CommitInfo
	Stats      DiffStats
	UpToDate   bool
	BeforeHash string
	AfterHash  string
}

// ErrNoRemoteBranches indicates the origin remote currently has no branches.
var ErrNoRemoteBranches = errors.New("no remote branches found")

// GetCurrentHash returns the current HEAD hash (short)
func GetCurrentHash(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetCurrentFullHash returns the current HEAD hash (full 40-char).
func GetCurrentFullHash(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ResetHard resets the working tree to the given revision.
func ResetHard(repoPath, rev string) error {
	cmd := exec.Command("git", "reset", "--hard", rev)
	cmd.Dir = repoPath
	return cmd.Run()
}

// Fetch runs git fetch
func Fetch(repoPath string) error {
	return FetchWithEnv(repoPath, nil)
}

// FetchWithEnv runs git fetch with additional environment variables.
func FetchWithEnv(repoPath string, extraEnv []string) error {
	cmd := exec.Command("git", "fetch")
	cmd.Dir = repoPath
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	return cmd.Run()
}

// GetCommitsBetween returns commits between two refs
func GetCommitsBetween(repoPath, from, to string) ([]CommitInfo, error) {
	cmd := exec.Command("git", "-c", "color.ui=false", "log", "--oneline", from+".."+to)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var commits []CommitInfo
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, CommitInfo{
				Hash:    parts[0],
				Message: parts[1],
			})
		}
	}
	return commits, nil
}

// GetDiffStats returns diff statistics between two refs
func GetDiffStats(repoPath, from, to string) (DiffStats, error) {
	cmd := exec.Command("git", "diff", "--shortstat", from+".."+to)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return DiffStats{}, err
	}

	return parseDiffStats(string(out)), nil
}

// parseDiffStats parses git diff --shortstat output
// Example: " 5 files changed, 120 insertions(+), 30 deletions(-)"
func parseDiffStats(output string) DiffStats {
	stats := DiffStats{}
	output = strings.TrimSpace(output)
	if output == "" {
		return stats
	}

	parts := strings.Split(output, ", ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "file") {
			stats.FilesChanged = extractNumber(part)
		} else if strings.Contains(part, "insertion") {
			stats.Insertions = extractNumber(part)
		} else if strings.Contains(part, "deletion") {
			stats.Deletions = extractNumber(part)
		}
	}
	return stats
}

func extractNumber(s string) int {
	var numStr string
	for _, c := range s {
		if c >= '0' && c <= '9' {
			numStr += string(c)
		} else if numStr != "" {
			break
		}
	}
	n, _ := strconv.Atoi(numStr)
	return n
}

// Pull runs git pull and returns update info (quiet mode)
func Pull(repoPath string) (*UpdateInfo, error) {
	return PullWithEnv(repoPath, nil)
}

// PullWithAuth runs git pull with token auth env inferred from origin remote.
func PullWithAuth(repoPath string) (*UpdateInfo, error) {
	return PullWithEnv(repoPath, AuthEnvForRepo(repoPath))
}

// PullWithEnv runs git pull and returns update info (quiet mode) with
// additional environment variables.
func PullWithEnv(repoPath string, extraEnv []string) (*UpdateInfo, error) {
	info := &UpdateInfo{}

	// Get full hash before pull (for reliable rollback)
	beforeHash, err := GetCurrentFullHash(repoPath)
	if err != nil {
		return nil, err
	}
	info.BeforeHash = beforeHash

	// Run git pull (quiet mode)
	cmd := exec.Command("git", "pull", "--quiet")
	cmd.Dir = repoPath
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Get full hash after pull
	afterHash, err := GetCurrentFullHash(repoPath)
	if err != nil {
		return nil, err
	}
	info.AfterHash = afterHash

	// Check if already up to date by comparing hashes
	if beforeHash == afterHash {
		info.UpToDate = true
		return info, nil
	}

	// Get commits between
	commits, _ := GetCommitsBetween(repoPath, beforeHash, afterHash)
	info.Commits = commits

	// Get diff stats
	stats, _ := GetDiffStats(repoPath, beforeHash, afterHash)
	info.Stats = stats

	return info, nil
}

// FileChange describes a single file change between two git revisions.
type FileChange struct {
	Status       string // A=added, M=modified, D=deleted, R=renamed
	Path         string
	OldPath      string // non-empty for renames
	LinesAdded   int
	LinesDeleted int
}

// GetChangedFiles returns file-level changes between two revisions.
func GetChangedFiles(repoPath, from, to string) ([]FileChange, error) {
	// Get name-status for type of change
	statusCmd := exec.Command("git", "diff", "--name-status", from+".."+to)
	statusCmd.Dir = repoPath
	statusOut, err := statusCmd.Output()
	if err != nil {
		return nil, err
	}

	// Get numstat for line counts
	numCmd := exec.Command("git", "diff", "--numstat", from+".."+to)
	numCmd.Dir = repoPath
	numOut, err := numCmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse name-status
	type statusEntry struct {
		status  string
		path    string
		oldPath string
	}
	var entries []statusEntry
	for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		s := parts[0]
		e := statusEntry{path: parts[len(parts)-1]}
		if strings.HasPrefix(s, "R") {
			e.status = "R"
			if len(parts) >= 3 {
				e.oldPath = parts[1]
				e.path = parts[2]
			}
		} else {
			e.status = s[:1] // A, M, D, etc.
		}
		entries = append(entries, e)
	}

	// Parse numstat into a map keyed by path
	numMap := map[string][2]int{} // path -> [added, deleted]
	for _, line := range strings.Split(strings.TrimSpace(string(numOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		numMap[parts[len(parts)-1]] = [2]int{added, deleted}
	}

	var changes []FileChange
	for _, e := range entries {
		fc := FileChange{
			Status:  e.status,
			Path:    e.path,
			OldPath: e.oldPath,
		}
		if nums, ok := numMap[e.path]; ok {
			fc.LinesAdded = nums[0]
			fc.LinesDeleted = nums[1]
		}
		changes = append(changes, fc)
	}

	return changes, nil
}

// IsDirty checks if repo has uncommitted changes
func IsDirty(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// GetDirtyFiles returns list of modified files
func GetDirtyFiles(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-c", "color.status=false", "status", "--short")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// Restore discards all local changes
func Restore(repoPath string) error {
	cmd := exec.Command("git", "restore", ".")
	cmd.Dir = repoPath
	return cmd.Run()
}

// IsRepo checks if the directory is a git repository
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// HasRemote checks if the repo has at least one remote configured
func HasRemote(dir string) bool {
	cmd := exec.Command("git", "remote")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// StageAll stages all changes (git add -A)
func StageAll(dir string) error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	return cmd.Run()
}

// Commit creates a commit with the given message
func Commit(dir, msg string) error {
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PushRemote pushes to the default remote
func PushRemote(dir string) error {
	return PushRemoteWithEnv(dir, nil)
}

// PushRemoteWithAuth pushes to the default remote with token auth env inferred
// from origin remote.
func PushRemoteWithAuth(dir string) error {
	return PushRemoteWithEnv(dir, AuthEnvForRepo(dir))
}

// PushRemoteWithEnv pushes to the default remote with additional environment
// variables.
func PushRemoteWithEnv(dir string, extraEnv []string) error {
	cmd := exec.Command("git", "push")
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// GetStatus returns git status --porcelain output
func GetStatus(dir string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// HasUpstream checks if the current branch has upstream tracking configured.
func HasUpstream(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetRemoteDefaultBranch returns the default branch name for origin.
// Resolution order:
//  1. refs/remotes/origin/HEAD
//  2. common branch names (main, master)
//  3. first discovered refs/remotes/origin/* branch
func GetRemoteDefaultBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	cmd.Dir = repoPath
	if out, err := cmd.Output(); err == nil {
		branch := strings.TrimSpace(string(out))
		branch = strings.TrimPrefix(branch, "origin/")
		if branch != "" {
			return branch, nil
		}
	}

	for _, branch := range []string{"main", "master"} {
		checkCmd := exec.Command("git", "rev-parse", "--verify", "refs/remotes/origin/"+branch)
		checkCmd.Dir = repoPath
		if err := checkCmd.Run(); err == nil {
			return branch, nil
		}
	}

	listCmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	listCmd.Dir = repoPath
	out, err := listCmd.Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" || ref == "origin/HEAD" {
			continue
		}
		ref = strings.TrimPrefix(ref, "origin/")
		if ref != "" {
			return ref, nil
		}
	}

	return "", ErrNoRemoteBranches
}

// HasRemoteSkillDirs reports whether origin/<remoteBranch> has at least one top-level directory.
func HasRemoteSkillDirs(repoPath, remoteBranch string) (bool, error) {
	lsCmd := exec.Command("git", "ls-tree", "-d", "--name-only", "origin/"+remoteBranch)
	lsCmd.Dir = repoPath
	lsOut, err := lsCmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(lsOut)) != "", nil
}

// HasLocalSkillDirs reports whether the repo root has at least one directory besides .git.
func HasLocalSkillDirs(repoPath string) (bool, error) {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.IsDir() && e.Name() != ".git" {
			return true, nil
		}
	}
	return false, nil
}

// GetBehindCount fetches from origin and returns how many commits local is behind
func GetBehindCount(repoPath string) (int, error) {
	return GetBehindCountWithEnv(repoPath, nil)
}

// GetBehindCountWithAuth fetches from origin and returns how many commits local
// is behind, injecting HTTPS token auth based on origin remote when available.
func GetBehindCountWithAuth(repoPath string) (int, error) {
	return GetBehindCountWithEnv(repoPath, AuthEnvForRepo(repoPath))
}

// GetBehindCountWithEnv is like GetBehindCount but with additional env vars.
func GetBehindCountWithEnv(repoPath string, extraEnv []string) (int, error) {
	if err := FetchWithEnv(repoPath, extraEnv); err != nil {
		return 0, err
	}
	branch, err := GetCurrentBranch(repoPath)
	if err != nil {
		return 0, err
	}
	cmd := exec.Command("git", "rev-list", "--count", "HEAD..origin/"+branch)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n, nil
}

// GetRemoteHeadHash returns the HEAD hash of a remote repo without cloning
func GetRemoteHeadHash(repoURL string) (string, error) {
	return GetRemoteHeadHashWithEnv(repoURL, nil)
}

// GetRemoteHeadHashWithAuth returns remote HEAD hash with HTTPS token auth
// injection when token env vars are available.
func GetRemoteHeadHashWithAuth(repoURL string) (string, error) {
	return GetRemoteHeadHashWithEnv(repoURL, install.AuthEnvForURL(repoURL))
}

// GetRemoteHeadHashWithEnv is like GetRemoteHeadHash but with additional env vars.
func GetRemoteHeadHashWithEnv(repoURL string, extraEnv []string) (string, error) {
	cmd := exec.Command("git", "ls-remote", repoURL, "HEAD")
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Format: "a1b2c3d4e5f6...\tHEAD\n"
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return "", fmt.Errorf("no HEAD ref found")
	}
	hash := parts[0]
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return hash, nil
}

// ForcePull fetches and resets to origin (handles force push)
func ForcePull(repoPath string) (*UpdateInfo, error) {
	return ForcePullWithEnv(repoPath, nil)
}

// ForcePullWithAuth runs force-pull flow with token auth env inferred from origin remote.
func ForcePullWithAuth(repoPath string) (*UpdateInfo, error) {
	return ForcePullWithEnv(repoPath, AuthEnvForRepo(repoPath))
}

// ForcePullWithEnv fetches and resets to origin with additional env vars.
func ForcePullWithEnv(repoPath string, extraEnv []string) (*UpdateInfo, error) {
	info := &UpdateInfo{}

	// Get full hash before (for reliable rollback)
	beforeHash, err := GetCurrentFullHash(repoPath)
	if err != nil {
		return nil, err
	}
	info.BeforeHash = beforeHash

	// Get current branch
	branch, err := GetCurrentBranch(repoPath)
	if err != nil {
		return nil, err
	}

	// Fetch
	if err := FetchWithEnv(repoPath, extraEnv); err != nil {
		return nil, err
	}

	// Reset to origin/branch
	cmd := exec.Command("git", "reset", "--hard", "origin/"+branch)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Get full hash after
	afterHash, err := GetCurrentFullHash(repoPath)
	if err != nil {
		return nil, err
	}
	info.AfterHash = afterHash

	if beforeHash == afterHash {
		info.UpToDate = true
		return info, nil
	}

	// Get commits between (may fail if history diverged, that's ok)
	commits, _ := GetCommitsBetween(repoPath, beforeHash, afterHash)
	info.Commits = commits

	// Get diff stats
	stats, _ := GetDiffStats(repoPath, beforeHash, afterHash)
	info.Stats = stats

	return info, nil
}

// AuthEnvForRepo returns HTTPS token auth env vars for the repo's origin remote.
func AuthEnvForRepo(repoPath string) []string {
	return install.AuthEnvForURL(getRemoteURL(repoPath))
}

func getRemoteURL(repoPath string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
