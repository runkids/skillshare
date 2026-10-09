package install

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"skillshare/internal/sourcefs"
)

// localFileURL builds the file:// URL for a local path. A Windows drive path
// (C:/repo) needs the empty authority (file:///C:/repo), and characters such as
// spaces, '#' and '%' must be escaped because git decodes the URL.
func localFileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// localCloneSource returns the filesystem path a local source clones from: the
// plain path, or the path a file:// URL names, or "" for a remote source.
func localCloneSource(source *Source) string {
	if source.Path != "" {
		return source.Path
	}
	u, err := url.Parse(source.CloneURL)
	if err != nil || u.Scheme != "file" {
		return ""
	}
	p := u.Path
	switch {
	case u.Host != "" && !strings.EqualFold(u.Host, "localhost"):
		p = "//" + u.Host + p // UNC: file://server/share/repo
	case len(p) > 2 && p[0] == '/' && p[2] == ':':
		p = p[1:] // file:///C:/repo
	}
	return filepath.FromSlash(p)
}

// pathWithin reports whether path is dir itself or lies below it, after
// resolving links.
func pathWithin(dir, path string) bool {
	resolve := func(p string) string {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		return p
	}
	rel, err := filepath.Rel(resolve(dir), resolve(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// normalizeTrackSource turns a local path that is a git repository into the
// file:// form that parseFileURL builds, and rejects other non-git sources.
func normalizeTrackSource(source *Source) error {
	if source.Type == SourceTypeLocalPath && IsLocalGitRepo(source.Path) {
		source.Type = SourceTypeGitHTTPS
		source.CloneURL = localFileURL(source.Path)
		return validateCloneURL(source.CloneURL)
	}
	if source.IsGit() {
		return nil
	}
	if source.Type == SourceTypeLocalPath {
		return fmt.Errorf("--track requires a git repository source; %s is not a git repository root (use file:///path for a repository URL)", source.Path)
	}
	return fmt.Errorf("--track requires a git repository source")
}

func installTrackedRepoImpl(source *Source, sourceDir string, opts InstallOptions) (*TrackedRepoResult, error) {
	if err := normalizeTrackSource(source); err != nil {
		return nil, err
	}
	if err := resolveWebRef(source); err != nil {
		return nil, err
	}

	// Determine repo name: opts.Name (--name or the name recorded in config) >
	// TrackName (owner-repo, so two repos both called "skills" don't collide) >
	// source.Name. ParseSource always fills source.Name with the URL basename,
	// so it must not win over TrackName.
	repoName := opts.Name
	if repoName == "" {
		repoName = source.TrackName()
		// Before owner-repo naming the clone was _<basename>; keep using that
		// checkout when it is the same repo, so a repeat install does not clone twice.
		legacy := filepath.Join(sourceDir, opts.Into, "_"+strings.TrimPrefix(source.Name, "_"))
		if source.Name != "" && repoURLsMatch(getRemoteURL(legacy), source.CloneURL) {
			repoName = source.Name
		}
	}
	if repoName == "" {
		repoName = source.Name
	}

	// Prefix with _ to indicate tracked repo (avoid double prefix if user already added _)
	trackedName := repoName
	if !strings.HasPrefix(repoName, "_") {
		trackedName = "_" + repoName
	}
	if err := validateTrackedRepoDirName(trackedName); err != nil {
		return nil, fmt.Errorf("invalid tracked repo name %q: %w", trackedName, err)
	}
	src, err := sourcefs.Create(sourceDir, opts.SourceFollow)
	if err != nil {
		return nil, fmt.Errorf("failed to create source directory: %w", err)
	}
	defer src.Close()
	destRel := trackedName
	if opts.Into != "" {
		if err := src.MkdirAll(opts.Into, 0755); err != nil {
			return nil, fmt.Errorf("failed to create --into directory: %w", err)
		}
		destRel = filepath.Join(opts.Into, trackedName)
	}
	destPath := filepath.Join(sourceDir, destRel)

	result := &TrackedRepoResult{
		RepoName: trackedName,
		RepoPath: destPath,
	}

	// Check if already exists
	replacing := false
	if _, err := os.Stat(destPath); err == nil {
		if opts.Update {
			return updateTrackedRepo(destPath, result, opts)
		}
		if !opts.Force {
			hint := buildForceHint(source.Raw, opts.Into) // includes --into if set
			hint = strings.Replace(hint, " --force", " --track --force", 1)
			// Tracked repos store a git remote; read it for comparison.
			existingURL := getRemoteURL(destPath)
			if existingURL != "" && repoURLsMatch(existingURL, source.CloneURL) {
				return nil, fmt.Errorf("%w: tracked repo '%s'. Use 'skillshare update' or --force to overwrite", ErrSkipSameRepo, trackedName)
			}
			if existingURL != "" {
				return nil, fmt.Errorf("tracked repo '%s' already exists (installed from %s). To overwrite: %s", trackedName, existingURL, hint)
			}
			return nil, fmt.Errorf("tracked repo '%s' already exists. To overwrite: %s", trackedName, hint)
		}
		// A local source that is, or lies inside, the destination would be
		// replaced under itself; a clone cannot carry its uncommitted work.
		if p := localCloneSource(source); p != "" && pathWithin(destPath, p) {
			return nil, fmt.Errorf("source %s is inside the install destination %s; nothing to install", p, destPath)
		}
		// Force mode - remove existing. A link is refused, even in a dry
		// run: removing it would disconnect the repo it points to.
		if err := src.CheckNoLink(destRel); err != nil {
			return nil, err
		}
		replacing = true
	}

	if opts.DryRun {
		result.Action = "would clone"
		return result, nil
	}

	// Clone tracked repos with a download-optimized strategy first, then
	// fallback to the legacy full clone for compatibility.
	// Prefer CLI --branch over source.Branch (from config); both beat empty.
	cloneBranch := opts.Branch
	if cloneBranch == "" {
		cloneBranch = source.Branch
	}
	// Clone beside the destination and move it in only after every check
	// passed, so a failure never costs the existing repo, and cleanup never
	// removes a destination another installer created meanwhile.
	stamp := strconv.FormatInt(time.Now().UnixNano(), 36)
	cloneRel := filepath.Join(filepath.Dir(destRel), ".skillshare-clone-"+stamp)
	clonePath := filepath.Join(sourceDir, cloneRel)
	installed := false
	defer func() {
		if !installed {
			_ = src.RemoveAll(cloneRel)
		}
	}()
	// A tag or commit SHA has no branch for `git pull` to follow. A SHA fails
	// at clone (--branch rejects it); a tag clones but leaves HEAD detached.
	if err := cloneTrackedRepoForSource(source, clonePath, cloneBranch, opts.OnProgress); err != nil {
		if IsCommitSHA(cloneBranch) {
			return nil, errTrackedNeedsBranch(cloneBranch)
		}
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}
	if isDetachedHead(clonePath) {
		return nil, errTrackedNeedsBranch(cloneBranch)
	}
	if source.Commit != "" {
		if err := resetTrackedToCommit(clonePath, source.Commit, source.authEnv()); err != nil {
			return nil, err
		}
	}

	if source.HasSubdir() {
		if err := submoduleError(clonePath, source.Subdir, source.authEnv()); err != nil {
			return nil, err
		}
	}

	// Discover skills in the cloned repo. Include root SKILL.md so the count
	// matches what `skillshare sync` will see: every SKILL.md inside a tracked
	// repo (root and nested alike) becomes an independent skill on sync.
	skills := discoverSkills(clonePath, true)
	result.SkillCount = len(skills)
	for _, skill := range skills {
		if skill.Path == "." {
			skill.Name = trackedName // the root skill is named after its directory, not the staging one
		}
		result.Skills = append(result.Skills, skill.Name)
	}
	result.Warnings = append(result.Warnings, SubmoduleWarnings(clonePath, source.authEnv())...)

	// Also discover agents in the tracked repo
	agents := discoverAgents(clonePath, len(skills) > 0)
	result.AgentCount = len(agents)
	if len(agents) > 0 {
		for _, agent := range agents {
			result.Agents = append(result.Agents, agent.Name)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d agent(s) found in tracked repo", len(agents)))
	}

	if len(skills) == 0 && len(agents) == 0 {
		result.Warnings = append(result.Warnings, "no SKILL.md files or agents found in repository")
	} else if len(skills) == 0 {
		// Only agents found — not a warning, just informational
	}

	// Security audit on the entire tracked repo. Accepted findings are keyed by
	// the final path, not the staging one.
	opts.AuditAcceptRoot, opts.AuditAcceptPath = opts.auditAcceptTarget(destPath)
	if err := auditTrackedRepo(clonePath, result, opts); err != nil {
		return nil, err
	}

	if replacing {
		// Keep the old repo until the new one is in place, as swapStagedIntoSource does.
		backup := filepath.Join(filepath.Dir(destRel), ".skillshare-"+filepath.Base(destRel)+".old."+stamp)
		if err := src.Rename(destRel, backup); err != nil {
			return nil, fmt.Errorf("failed to move the existing repo aside: %w", err)
		}
		if err := src.Rename(cloneRel, destRel); err != nil {
			if restoreErr := src.Rename(backup, destRel); restoreErr != nil {
				return nil, fmt.Errorf("failed to move the new clone into place: %w; the previous repo is left at %s: %v", err, backup, restoreErr)
			}
			return nil, fmt.Errorf("failed to move the new clone into place: %w", err)
		}
		if err := src.RemoveAll(backup); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to remove the previous repo %s: %v", backup, err))
		}
	} else if err := src.Rename(cloneRel, destRel); err != nil {
		return nil, fmt.Errorf("failed to move the new clone into place: %w", err)
	}
	installed = true

	// Auto-add to .gitignore to prevent committing tracked repo contents
	gitignoreEntry := trackedName
	if opts.Into != "" {
		gitignoreEntry = filepath.Join(opts.Into, trackedName)
	}
	if err := UpdateGitIgnore(sourceDir, gitignoreEntry); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to update .gitignore: %v", err))
	}

	result.Action = "cloned"
	return result, nil
}

func errTrackedNeedsBranch(ref string) error {
	return fmt.Errorf("tracked repos must follow a branch; %q is a tag or commit — install without --track to pin it", ref)
}

// updateTrackedRepo performs git pull on an existing tracked repo
func updateTrackedRepo(repoPath string, result *TrackedRepoResult, opts InstallOptions) (*TrackedRepoResult, error) {
	if !IsGitRepo(repoPath) {
		return nil, fmt.Errorf("'%s' is not a git repository", repoPath)
	}

	if opts.DryRun {
		result.Action = "would update (git pull)"
		return result, nil
	}

	if err := checkFollowedCheckoutClean(repoPath, opts); err != nil {
		return nil, err
	}

	// Record hash before pull for rollback on audit failure
	beforeHash, err := getGitFullHash(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to determine rollback commit before update (aborting for safety): %w", err)
	}
	if beforeHash == "" {
		return nil, fmt.Errorf("failed to determine rollback commit before update (aborting for safety): empty commit hash")
	}

	if err := gitPull(repoPath, opts.OnProgress); err != nil {
		return nil, fmt.Errorf("failed to update: %w", err)
	}

	// Post-pull audit: rollback via git reset (not os.RemoveAll) to preserve repo.
	if err := auditTrackedRepoUpdate(repoPath, beforeHash, result, opts); err != nil {
		return nil, err
	}

	// Re-discover skills (include root so count matches `sync` view).
	skills := discoverSkills(repoPath, true)
	result.SkillCount = len(skills)
	for _, skill := range skills {
		result.Skills = append(result.Skills, skill.Name)
	}
	result.Warnings = append(result.Warnings, SubmoduleWarnings(repoPath, nil)...)

	// Also discover agents in the tracked repo
	agents := discoverAgents(repoPath, len(skills) > 0)
	result.AgentCount = len(agents)
	if len(agents) > 0 {
		for _, agent := range agents {
			result.Agents = append(result.Agents, agent.Name)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d agent(s) found in tracked repo", len(agents)))
	}

	result.Action = "updated"
	return result, nil
}

// cloneRepoFull performs a full git clone (quiet mode for cleaner output)
func cloneRepoFull(url, destPath, branch string, onProgress ProgressCallback) error {
	args := []string{"clone"}
	if onProgress != nil {
		args = append(args, "--progress")
	} else {
		args = append(args, "--quiet")
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, destPath)
	return runGitCommandWithProgress(args, "", authEnv(url), onProgress)
}

func cloneTrackedRepoForSource(source *Source, destPath, branch string, onProgress ProgressCallback) error {
	err := cloneTrackedRepoForParsedSource(source, destPath, branch, onProgress)
	if err == nil {
		return nil
	}
	if !shouldRetryNestedGitLabURL(err) {
		return err
	}

	var lastErr error
	for _, fallback := range cloneFallbackSourcesForNestedGitLabURL(source) {
		if cleanupErr := removeAll(destPath); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
			return fmt.Errorf("clone failed (%v), and cleanup before fallback failed: %w", err, cleanupErr)
		}
		if onProgress != nil {
			onProgress("Clone failed; retrying as a nested GitLab repository...")
		}
		fallbackErr := cloneTrackedRepoForParsedSource(&fallback, destPath, branch, onProgress)
		if fallbackErr == nil {
			*source = fallback
			return nil
		}
		lastErr = fallbackErr
	}

	if lastErr != nil {
		return fmt.Errorf("%w (nested GitLab fallback also failed: %v)", err, lastErr)
	}
	return err
}

// cloneTrackedRepo clones a tracked repository with an optimized payload first
// and falls back to full clone when the remote does not support partial/shallow
// capabilities.
//
// When subdir is provided, sparse checkout is attempted first to reduce payload
// while preserving .git for future tracked updates.
func cloneTrackedRepo(url, subdir, destPath, branch string, onProgress ProgressCallback) error {
	return cloneTrackedRepoWithEnv(url, subdir, destPath, branch, authEnv(url), onProgress)
}

func cloneTrackedRepoForParsedSource(source *Source, destPath, branch string, onProgress ProgressCallback) error {
	if source == nil {
		return fmt.Errorf("nil source")
	}
	return cloneTrackedRepoWithEnv(source.CloneURL, source.Subdir, destPath, branch, source.authEnv(), onProgress)
}

func cloneTrackedRepoWithEnv(url, subdir, destPath, branch string, extraEnv []string, onProgress ProgressCallback) error {
	subdir = strings.TrimSpace(subdir)
	if subdir != "" && gitSupportsSparseCheckout() {
		if onProgress != nil {
			onProgress("Preparing sparse checkout...")
		}
		if err := sparseCloneSubdir(url, subdir, destPath, branch, extraEnv, onProgress); err == nil {
			checkedSubdir := strings.TrimLeft(subdir, "/")
			if checkedSubdir == "" {
				return nil
			}
			if info, statErr := os.Stat(filepath.Join(destPath, checkedSubdir)); statErr == nil && info.IsDir() {
				return nil
			}
			if cleanupErr := removeAll(destPath); cleanupErr != nil {
				return fmt.Errorf("sparse checkout produced no subdirectory %q, and cleanup failed: %w", subdir, cleanupErr)
			}
			if onProgress != nil {
				onProgress("Sparse checkout path missing; retrying standard clone...")
			}
		} else if shouldFallbackSparseTrackedClone(err) {
			// sparseCloneSubdir may have already created destPath. Clean it before
			// falling back to a standard clone strategy.
			if cleanupErr := removeAll(destPath); cleanupErr != nil {
				return fmt.Errorf("sparse checkout failed (%v), and cleanup failed: %w", err, cleanupErr)
			}
			if onProgress != nil {
				onProgress("Sparse checkout unavailable; retrying standard clone...")
			}
		} else {
			return err
		}
	}

	args := []string{
		"clone",
		"--filter=blob:none",
		"--depth", "1",
		"--single-branch",
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	if onProgress != nil {
		args = append(args, "--progress")
	} else {
		args = append(args, "--quiet")
	}
	args = append(args, url, destPath)

	err := runGitCommandWithProgress(args, "", extraEnv, onProgress)
	if err == nil {
		return nil
	}
	if !shouldFallbackTrackedClone(err) {
		return err
	}
	if onProgress != nil {
		onProgress("Remote lacks partial clone support; retrying standard clone...")
	}
	return cloneRepoWithEnv(url, destPath, branch, false, extraEnv, onProgress)
}

// isAuthOrAccessError returns true for auth failures and access denials that
// should NOT trigger a fallback clone strategy (retrying won't help).
func isAuthOrAccessError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	if IsAuthError(s) {
		return true
	}
	low := strings.ToLower(s)
	return strings.Contains(low, "permission denied") ||
		strings.Contains(low, "repository not found")
}

func shouldFallbackSparseTrackedClone(err error) bool {
	if err == nil {
		return false
	}
	if isAuthOrAccessError(err) {
		return false
	}

	// For compatibility, fallback to the legacy tracked clone flow for
	// capability, sparse-path, and server-specific sparse checkout errors.
	return true
}

func shouldFallbackTrackedClone(err error) bool {
	if err == nil {
		return false
	}
	if isAuthOrAccessError(err) {
		return false
	}

	low := strings.ToLower(err.Error())
	capabilityHints := []string{
		"does not support",
		"not support",
		"filter",
		"shallow",
		"depth",
		"single-branch",
		"partial clone",
		"dumb http",
	}
	for _, hint := range capabilityHints {
		if strings.Contains(low, hint) {
			return true
		}
	}
	return false
}

// GetUpdatableSkills returns skill names that have metadata with a remote source.
// It walks subdirectories recursively so nested skills are found.
