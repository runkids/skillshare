package install

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// buildDiscoverySkillSource constructs metadata Source string for a skill
// selected from a discovery result. The string is shared through the project
// config, so ParseSource must split it back into the same repository and
// subdirectory (#859).
func buildDiscoverySkillSource(source *Source, skillPath string) string {
	if source.Type == SourceTypeArchive {
		member := source.Subdir
		if skillPath != "." {
			member = path.Join(member, skillPath)
		}
		return archiveSkillSource(source, member)
	}
	if skillPath == "." {
		return source.Raw
	}
	if source.HasSubdir() {
		return source.Raw + "/" + skillPath
	}
	repo := strings.TrimRight(source.Raw, "/")
	switch {
	// Whole-repo SSH and file:// sources encode subdir using //path.
	case source.Type == SourceTypeGitSSH, strings.HasPrefix(source.CloneURL, "file://"):
		return repo + "//" + skillPath
	// GitLab and Bitbucket paths have no fixed depth: without ".git" or a web
	// URL marker the parser takes every segment as the repository.
	case source.webHost != "" && !strings.HasSuffix(repo, ".git") && !hasWebURLMarker(source.webHost, repo):
		return repo + ".git/" + skillPath
	}
	return repo + "/" + skillPath
}

// hasWebURLMarker reports whether raw is a GitLab (/-/tree/ref) or Bitbucket
// (/src/ref) web URL, which parseGitHTTPS splits at the marker in the path.
func hasWebURLMarker(webHost, raw string) bool {
	m := gitHTTPSPattern.FindStringSubmatch(raw)
	if m == nil {
		return false
	}
	return strings.Contains(m[3], "/-/") || webHost == "bitbucket" && strings.Contains(m[3], "/src/")
}

// isDiscoverySkillSource reports whether entry records skillPath of source.
// Installs from before #859 recorded a single-slash source, which GitLab,
// Bitbucket and file:// hosts also use for a repository of its own, so that
// form counts only when the recorded repository and path agree.
func isDiscoverySkillSource(entry *MetadataEntry, source *Source, skillPath string) bool {
	if entry.Source == buildDiscoverySkillSource(source, skillPath) {
		return true
	}
	return entry.Source == source.Raw+"/"+skillPath &&
		repoURLsMatch(entry.RepoURL, source.CloneURL) && entry.Subdir == filepath.ToSlash(skillPath)
}

// RecordedSource parses the source a skill's entry was installed from.
// Installs from before #859 recorded a skill picked out of a repository as
// "<repo>/<path>", which GitLab, Bitbucket and file:// sources parse as a
// repository of its own. The entry's recorded repository and path then say
// what was installed, and the source is rebuilt in the form that parses back.
func RecordedSource(entry *MetadataEntry, opts ParseOptions) (*Source, error) {
	source, _, err := recordedSource(entry, opts)
	return source, err
}

// RecordedSourceString is the source string that installs what entry
// records: entry.Source, or the rebuilt one when RecordedSource repairs it.
func RecordedSourceString(entry *MetadataEntry, opts ParseOptions) string {
	if source, repaired, err := recordedSource(entry, opts); err == nil && repaired {
		return source.Raw
	}
	return entry.Source
}

func recordedSource(entry *MetadataEntry, opts ParseOptions) (source *Source, repaired bool, err error) {
	source, err = ParseSourceWithOptions(entry.Source, opts)
	if err != nil || entry.Subdir == "" || source.Subdir != "" || repoURLsMatch(source.CloneURL, entry.RepoURL) {
		return source, false, err
	}
	repoRaw, ok := strings.CutSuffix(entry.Source, "/"+entry.Subdir)
	if !ok {
		return source, false, nil
	}
	repo, err := ParseSourceWithOptions(repoRaw, opts)
	if err != nil || repo.HasSubdir() || !repoURLsMatch(repo.CloneURL, entry.RepoURL) {
		return source, false, nil
	}
	rebuilt, err := ParseSourceWithOptions(buildDiscoverySkillSource(repo, entry.Subdir), opts)
	if err != nil || rebuilt.Subdir != entry.Subdir {
		return source, false, nil
	}
	return rebuilt, true, nil
}

func discoverySourceRoot(discovery *DiscoveryResult) string {
	if discovery.Source != nil && discovery.Source.Type == SourceTypeLocalPath {
		return discovery.RepoPath
	}
	return filepath.Join(discovery.RepoPath, "repo")
}

// descendantSkillPaths returns the set of slash-normalized paths (relative to
// the source root of `skill`) of every other discovered skill that lives
// strictly inside `skill`. The returned map is suitable as the excludes
// argument to copyDirExcluding.
//
// For the root skill (Path="."), every other discovered skill counts as a
// descendant. For a non-root skill, only nested sub-skills count. Returns nil
// when there is nothing to exclude so callers hit the fast path in
// copyDirExcluding.
func descendantSkillPaths(discovery *DiscoveryResult, skill SkillInfo) map[string]bool {
	if discovery == nil || len(discovery.Skills) <= 1 {
		return nil
	}
	current := filepath.ToSlash(skill.Path)
	var prefix string
	if current != "." {
		prefix = current + "/"
	}
	excludes := make(map[string]bool)
	for _, other := range discovery.Skills {
		otherPath := filepath.ToSlash(other.Path)
		if otherPath == current {
			continue
		}
		if current == "." {
			// Root skill: every other skill path is a strict descendant.
			if otherPath != "." {
				excludes[otherPath] = true
			}
			continue
		}
		// Non-root: only paths strictly under current count.
		if rel, ok := strings.CutPrefix(otherPath, prefix); ok {
			// rel is the path relative to `skill` (which is how
			// copyDirExcluding walks `srcPath`).
			excludes[rel] = true
		}
	}
	if len(excludes) == 0 {
		return nil
	}
	return excludes
}

func installAgentRelativePath(agent AgentInfo) string {
	return strings.TrimPrefix(filepath.ToSlash(agent.Path), "agents/")
}

func resolveDiscoveredAgentSourcePath(discovery *DiscoveryResult, agent AgentInfo) string {
	sourceRoot := discoverySourceRoot(discovery)
	if discovery.Source.HasSubdir() {
		return filepath.Join(sourceRoot, discovery.Source.Subdir, agent.Path)
	}
	return filepath.Join(sourceRoot, agent.Path)
}

func buildDiscoveredAgentSource(discovery *DiscoveryResult, agent AgentInfo) *Source {
	return &Source{
		Type:     discovery.Source.Type,
		Raw:      buildDiscoverySkillSource(discovery.Source, agent.Path),
		CloneURL: discovery.Source.CloneURL,
		Subdir:   agent.Path,
		Name:     agent.Name,
		Branch:   discovery.Source.Branch,
		Commit:   discovery.Source.Commit,
	}
}

// ErrAgentSourceLost reports agent metadata whose source an update before the
// #777 fix wrote back without its repository: nothing is left to fetch from.
var ErrAgentSourceLost = errors.New("source was lost by an earlier update; reinstall this agent")

// AgentSourceLost reports whether the entry is in the state ErrAgentSourceLost
// describes. Only that bug wrote an untyped ("unknown") source with no repo URL.
func (e *MetadataEntry) AgentSourceLost() bool {
	return e.RepoURL == "" && strings.HasPrefix(e.Type, SourceTypeUnknown.String())
}

// AgentRepoSource rebuilds the repository-level source an agent was
// discovered from. entry.Source is the agent's own source, which
// buildDiscoverySkillSource formed by appending entry.Subdir (the agent's
// path) to it, so the path is cut off before parsing. Dashboard updates
// before the #777 fix appended the path a second time; every copy is cut.
func AgentRepoSource(entry *MetadataEntry, opts ParseOptions) (*Source, error) {
	if entry.AgentSourceLost() {
		return nil, ErrAgentSourceLost
	}
	raw := entry.Source
	for entry.Subdir != "" {
		root, ok := strings.CutSuffix(raw, "/"+entry.Subdir)
		if !ok {
			break
		}
		// Whole-repo SSH and file:// sources join the path with "//".
		raw = strings.TrimSuffix(root, "/")
	}
	source, err := ParseSourceWithOptions(raw, opts)
	if err != nil {
		return nil, err
	}
	source.ApplyRecordedBranch(entry.Branch)
	return source, nil
}

func writeDiscoveredAgentMetadata(discovery *DiscoveryResult, agent AgentInfo, destFile, sourceDir string) error {
	source := buildDiscoveredAgentSource(discovery, agent)
	meta := NewMetaFromSource(source)
	meta.Kind = "agent"
	if discovery.CommitHash != "" {
		meta.Version = discovery.CommitHash
	}
	if hash, hashErr := computeSingleFileHash(destFile); hashErr == nil {
		meta.FileHashes = map[string]string{filepath.Base(destFile): hash}
	}
	return WriteMetaToStore(sourceDir, destFile, meta)
}

func installAgentFromDiscoveryInternal(discovery *DiscoveryResult, agent AgentInfo, destFile string, opts InstallOptions, update bool) (*InstallResult, error) {
	if discovery.Source.Type == SourceTypeArchive {
		return nil, fmt.Errorf("ZIP sources support skills only")
	}
	result := &InstallResult{
		SkillName: agent.Name,
		Source:    buildDiscoverySkillSource(discovery.Source, agent.Path),
		SkillPath: destFile,
	}

	if opts.DryRun {
		result.Action = "would install"
		if update {
			result.Action = "would reinstall from source"
		}
		return result, nil
	}

	src, err := sourcefs.Create(opts.SourceDir, opts.SourceFollow)
	if err != nil {
		return nil, fmt.Errorf("failed to open agents directory: %w", err)
	}
	defer src.Close()
	rel, err := src.Rel(destFile)
	if err != nil {
		return nil, err
	}
	if err := src.CheckNoLink(rel); err != nil {
		return nil, fmt.Errorf("unsafe agent destination: %w", err)
	}
	if _, err := src.Lstat(rel); err == nil {
		if !update && !opts.Force {
			result.Action = "skipped"
			result.Warnings = append(result.Warnings, "agent already exists (use --force to overwrite)")
			return result, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to check agent destination: %w", err)
	}

	data, err := os.ReadFile(resolveDiscoveredAgentSourcePath(discovery, agent))
	if err != nil {
		return nil, fmt.Errorf("failed to read agent %s: %w", agent.FileName, err)
	}

	// Audit an isolated file before publishing, including forced installs.
	// A rejected audit can only clean up this temporary tree, never the
	// existing agent or a file reached through a destination link.
	tempDir, err := os.MkdirTemp("", "skillshare-agent-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	stage, err := sourcefs.Open(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open staged agent directory: %w", err)
	}
	defer stage.Close()
	tempName := filepath.Base(destFile)
	if err := stage.WriteFile(tempName, data, 0o644); err != nil {
		return nil, fmt.Errorf("failed to stage agent %s: %w", agent.FileName, err)
	}
	auditOpts := opts
	auditOpts.AuditAcceptRoot, auditOpts.AuditAcceptPath = opts.auditAcceptTarget(destFile)
	auditOpts.SourceDir = tempDir
	auditOpts.SourceFollow = nil
	if err := auditInstalledAgent(filepath.Join(tempDir, tempName), result, auditOpts); err != nil {
		return nil, err
	}

	if err := src.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create agents directory: %w", err)
	}
	if err := src.WriteFileAtomic(rel, data, 0o644); err != nil {
		return nil, fmt.Errorf("failed to write agent %s: %w", agent.FileName, err)
	}
	if err := writeDiscoveredAgentMetadata(discovery, agent, destFile, opts.SourceDir); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	result.Action = "installed"
	if update {
		result.Action = "updated"
	}
	return result, nil
}

// skillsRootFor returns opts.SourceDir, or derives it when the caller left it
// empty: destPath = sourceDir[/into]/skillName, so strip Into + skillName.
func skillsRootFor(destPath string, opts InstallOptions) string {
	if opts.SourceDir != "" {
		return opts.SourceDir
	}
	dir := filepath.Dir(destPath)
	if opts.Into != "" {
		dir = filepath.Dir(dir)
		for i := strings.Count(opts.Into, "/"); i > 0; i-- {
			dir = filepath.Dir(dir)
		}
	}
	return dir
}

func installImpl(source *Source, destPath string, opts InstallOptions) (*InstallResult, error) {
	opts.ParseOptions.CNBHosts, opts.ParseOptions.GiteaHosts = source.CNBHosts, source.GiteaHosts
	opts.SourceDir = skillsRootFor(destPath, opts)
	if source.Type == SourceTypeArchive {
		return installArchive(source, destPath, opts)
	}

	result := &InstallResult{
		SkillName: source.Name,
		Source:    source.Raw,
	}

	// Check if destination exists
	destInfo, destErr := os.Stat(destPath)
	destExists := destErr == nil

	if destExists {
		if opts.Update {
			return handleUpdate(source, destPath, result, opts)
		}
		if !opts.Force && !opts.DryRun {
			hint := buildForceHint(source.Raw, opts.Into)
			var recorded *MetadataEntry
			if rel, err := filepath.Rel(opts.SourceDir, destPath); err == nil {
				recorded = LoadMetadataOrNew(opts.SourceDir).GetExact(rel)
			}
			if err := checkExistingConflict(destPath, recorded, source.CloneURL, source.Subdir, hint); err != nil {
				return nil, err
			}
			// nil means empty/invalid dir — safe to overwrite, fall through.
		}
		// Force mode (or empty dir): remove existing
		if !opts.DryRun {
			if err := removeInSource(opts.SourceDir, destPath, opts.SourceFollow); err != nil {
				return nil, fmt.Errorf("failed to remove existing skill: %w", err)
			}
		}
	} else if destInfo != nil && !destInfo.IsDir() {
		return nil, fmt.Errorf("destination exists but is not a directory")
	}

	result.SkillPath = destPath

	// Execute installation based on source type
	switch source.Type {
	case SourceTypeLocalPath:
		return installFromLocal(source, destPath, result, opts)
	case SourceTypeGitHub, SourceTypeGitHTTPS, SourceTypeGitSSH:
		return installFromGit(source, destPath, result, opts)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", source.Type)
	}
}

func installFromLocal(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	// Verify source exists
	srcInfo, err := os.Stat(source.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source path does not exist: %s", source.Path)
		}
		return nil, fmt.Errorf("cannot access source path: %w", err)
	}
	if !srcInfo.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", source.Path)
	}

	if opts.DryRun {
		result.Action = "would copy"
		return result, nil
	}

	if opts.skillFileOnly {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination: %w", err)
		}
		if err := copyFile(filepath.Join(source.Path, "SKILL.md"), filepath.Join(destPath, "SKILL.md")); err != nil {
			return nil, fmt.Errorf("failed to copy SKILL.md: %w", err)
		}
	} else if err := copyDir(source.Path, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	// Write metadata with file hashes
	meta := NewMetaFromSource(source)
	meta.Layout = LayoutDirectory
	if opts.skillFileOnly {
		meta.Layout = LayoutSkillFile
	}
	if hashes, hashErr := ComputeFileHashes(destPath, opts.SourceFollow); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	result.Action = "copied"
	return result, nil
}

func installFromGit(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	// Check if git is available
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}
	if err := resolveWebRef(source); err != nil {
		return nil, err
	}

	// If subdir is specified, install directly
	if source.HasSubdir() {
		return installFromGitSubdir(source, destPath, result, opts)
	}

	// No subdir specified - this should be handled by DiscoverFromGit first
	// If we get here, treat it as "install entire repo as one skill"
	if opts.DryRun {
		result.Action = "would clone"
		return result, nil
	}

	// Clone the repository
	if err := cloneRepoForSource(source, destPath, source.Branch, true, opts.OnProgress); err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Write metadata with file hashes
	meta := NewMetaFromSource(source)
	if hash, err := getGitCommit(destPath); err == nil {
		meta.Version = hash
	}
	if hashes, hashErr := ComputeFileHashes(destPath, opts.SourceFollow); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)
	result.Warnings = append(result.Warnings, SubmoduleWarnings(destPath, source.authEnv())...)

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	result.Action = "cloned"
	return result, nil
}

// DiscoverFromGit clones a repo and discovers available skills

func installFromDiscoveryImpl(discovery *DiscoveryResult, skill SkillInfo, destPath string, opts InstallOptions) (*InstallResult, error) {
	return installFromDiscoveryInternal(discovery, skill, destPath, opts, true)
}

func discoveredSkillSourceParts(discovery *DiscoveryResult, skill SkillInfo) (string, string) {
	// For subdir discovery, skill.Path is relative to the subdir.
	// For whole-repo discovery, skill.Path is relative to repo root.
	if skill.Path == "." {
		// Root skill of a subdir discovery.
		subdir := discovery.Source.Subdir
		if discovery.Source.Type == SourceTypeArchive && subdir == "" {
			subdir = "."
		}
		return buildDiscoverySkillSource(discovery.Source, skill.Path), subdir
	}
	if discovery.Source.HasSubdir() {
		// Nested skill within subdir discovery.
		return buildDiscoverySkillSource(discovery.Source, skill.Path), discovery.Source.Subdir + "/" + skill.Path
	}
	// Whole-repo discovery.
	return buildDiscoverySkillSource(discovery.Source, skill.Path), skill.Path
}

func installFromDiscoveryInternal(discovery *DiscoveryResult, skill SkillInfo, destPath string, opts InstallOptions, writeMeta bool) (*InstallResult, error) {
	// Same derivation as Install: without it the metadata of an --into install
	// lands in the group folder and the skill never reaches the config.
	opts.SourceDir = skillsRootFor(destPath, opts)
	fullSource, fullSubdir := discoveredSkillSourceParts(discovery, skill)

	result := &InstallResult{
		SkillName: skill.Name,
		Source:    fullSource,
	}

	// Validate ZIP-only options at the shared application boundary too.
	if discovery.Source.Type == SourceTypeArchive {
		if opts.Track || opts.Branch != "" || discovery.Source.Branch != "" || opts.IsAgentMode() || opts.HasAgentFilter() {
			return nil, fmt.Errorf("ZIP sources support skills only, without Git tracking or refs")
		}
		if archiveUnchanged(opts, destPath, fullSource, discovery.Source.ArchiveSHA256) {
			result.SkillPath, result.Action = destPath, "skipped"
			return result, nil
		}
	}
	// Check if destination exists
	stage := false
	if _, err := os.Stat(destPath); err == nil {
		if !opts.Force && !opts.DryRun {
			// Use the original repo URL for force hints, not the per-skill
			// fullSource URL (which isn't a valid install target).
			hint := buildForceHint(discovery.Source.Raw, opts.Into)
			store := LoadMetadataOrNew(opts.SourceDir)
			key := filepath.Join(opts.Into, filepath.Base(destPath))
			meta := store.GetByPath(key)
			knownUpdate := opts.Update && meta != nil && isDiscoverySkillSource(meta, discovery.Source, skill.Path) && meta.Group == opts.Into && !meta.Tracked && meta.EffectiveKind() == "skill"
			if !knownUpdate {
				if err := checkExistingConflict(destPath, store.GetExact(key), discovery.Source.CloneURL, fullSubdir, hint); err != nil {
					return nil, err
				}
			}
			// nil means empty/invalid dir — safe to overwrite, fall through.
		}
		// Update: stage the new content in a temp dir and swap only after the
		// audit passes, so a blocked update keeps the previously installed
		// version on disk (mirrors handleUpdate). Fresh --force installs still
		// overwrite in place.
		stage = opts.Update || (opts.Force && discovery.Source.Type == SourceTypeArchive)
		if !opts.DryRun && !stage {
			if err := removeInSource(opts.SourceDir, destPath, opts.SourceFollow); err != nil {
				return nil, fmt.Errorf("failed to remove existing skill: %w", err)
			}
		}
	}

	result.SkillPath = destPath

	if opts.DryRun {
		result.Action = "would install"
		return result, nil
	}

	workDest := destPath
	if stage {
		tempDir, err := os.MkdirTemp("", "skillshare-update-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)
		workDest = filepath.Join(tempDir, "skill")
	}

	// Determine source path in temp repo
	sourceRoot := discoverySourceRoot(discovery)
	var srcPath string
	if discovery.Source.HasSubdir() {
		// Subdir discovery: paths are relative to the subdir
		if skill.Path == "." {
			srcPath = filepath.Join(sourceRoot, discovery.Source.Subdir)
		} else {
			srcPath = filepath.Join(sourceRoot, discovery.Source.Subdir, skill.Path)
		}
	} else {
		// Whole-repo discovery: paths are relative to repo root
		srcPath = filepath.Join(sourceRoot, skill.Path)
	}

	// In a repo-root orchestrator (root SKILL.md + children), the root skill
	// has no directory boundary — the entire repo root is its "directory",
	// which includes project infrastructure (src/, assets/, CI, etc.) that
	// is not skill content. Copy only the SKILL.md file itself.
	//
	// Do not apply this to scoped subdir discovery: there the selected subdir
	// is the boundary, so sibling files in that subdir are legitimate skill
	// content and must still be copied.
	excludes := descendantSkillPaths(discovery, skill)
	rootIsRepoRoot := skill.Path == "." && discovery.Source != nil && (!discovery.Source.HasSubdir() || discovery.Source.Type == SourceTypeArchive && discovery.Source.Subdir == ".")

	if rootIsRepoRoot && excludes != nil {
		// Repo-root orchestrator: copy only SKILL.md (no directory boundary).
		if err := os.MkdirAll(workDest, 0755); err != nil {
			return nil, fmt.Errorf("failed to create destination: %w", err)
		}
		if err := copyFile(filepath.Join(srcPath, "SKILL.md"), filepath.Join(workDest, "SKILL.md")); err != nil {
			return nil, fmt.Errorf("failed to copy SKILL.md: %w", err)
		}
	} else if err := copyDirExcluding(srcPath, workDest, excludes); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit — on block, cleanup only touches workDest, so a staged
	// update leaves the existing install untouched.
	auditOpts := opts
	if auditOpts.AuditAcceptPath == "" {
		auditOpts.AuditAcceptRoot, auditOpts.AuditAcceptPath = opts.SourceDir, destPath
	}
	if err := auditInstalledSkill(workDest, result, auditOpts); err != nil {
		return nil, err
	}

	if stage {
		if err := swapStagedIntoSource(opts.SourceDir, workDest, destPath, opts.SourceFollow); err != nil {
			return nil, err
		}
	}

	if writeMeta {
		if err := writeDiscoveredSkillMetadata(discovery, skill, destPath, opts.SourceDir, fullSource, fullSubdir, opts.SourceFollow); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
		}
	}

	result.Action = "installed"
	return result, nil
}

func writeDiscoveredSkillMetadata(discovery *DiscoveryResult, skill SkillInfo, destPath, sourceDir, fullSource, fullSubdir string, follow ...*sourcewalk.Follow) error {
	source := &Source{
		Type:          discovery.Source.Type,
		Raw:           fullSource,
		CloneURL:      discovery.Source.CloneURL,
		Subdir:        fullSubdir,
		Name:          skill.Name,
		Branch:        discovery.Source.Branch,
		Commit:        discovery.Source.Commit,
		ArchiveSHA256: discovery.Source.ArchiveSHA256,
	}
	meta := NewMetaFromSource(source)
	meta.Layout = discoveredLocalLayout(discovery, skill)
	sourceRoot := discoverySourceRoot(discovery)
	if discovery.CommitHash != "" {
		meta.Version = discovery.CommitHash
	} else if discovery.Source.Type != SourceTypeArchive {
		if hash, err := getGitCommit(sourceRoot); err == nil {
			meta.Version = hash
		}
	}
	if fullSubdir != "" && discovery.Source.Type != SourceTypeArchive {
		meta.TreeHash = getSubdirTreeHash(sourceRoot, fullSubdir)
	}
	if hashes, hashErr := ComputeFileHashes(destPath, follow...); hashErr == nil {
		meta.FileHashes = hashes
	}
	return WriteMetaToStore(sourceDir, destPath, meta)
}

func installFromGitSubdir(source *Source, destPath string, result *InstallResult, opts InstallOptions) (*InstallResult, error) {
	if opts.DryRun {
		result.Action = "would clone and extract"
		return result, nil
	}

	// Clone to temp directory
	tempDir, err := os.MkdirTemp("", "skillshare-install-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer cleanupTempRepo(tempDir)

	tempRepoPath := filepath.Join(tempDir, "repo")
	var subdirPath string
	var resolved string
	var commitHash string

	// The Contents API paths below download the default branch and ignore any
	// branch, tag or commit, so they only run when none was asked for.
	hasRef := source.ref() != ""

	// Fast path 1: sparse checkout (preferred for speed if git is modern)
	// Works for GitHub and non-GitHub hosts.
	if gitSupportsSparseCheckout() {
		resolved = source.Subdir
		if err := sparseCloneSubdir(source.CloneURL, resolved, tempRepoPath, source.ref(), source.authEnv(), opts.OnProgress); err == nil {
			if err := submoduleError(tempRepoPath, resolved, source.authEnv()); err != nil {
				return nil, err
			}
			subdirPath = filepath.Join(tempRepoPath, resolved)
			if info, statErr := os.Stat(subdirPath); statErr != nil || !info.IsDir() {
				subdirPath = ""
				result.Warnings = append(result.Warnings, "sparse checkout install fallback: subdirectory missing after checkout")
				cleanupTempRepo(tempRepoPath)
			} else {
				source.recordCommit(tempRepoPath)
				if hash, hashErr := getGitCommit(tempRepoPath); hashErr == nil {
					commitHash = hash
				}
			}
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sparse checkout install fallback: %v", err))
			cleanupTempRepo(tempRepoPath)
			subdirPath = ""
		}
	}

	// Fast path 2: GitHub/GHE Contents API
	// Fallback for when sparse checkout is unavailable or fails.
	if subdirPath == "" && !hasRef && isGitHubAPISource(source) {
		owner, repo := source.GitHubOwner(), source.GitHubRepo()
		resolved = source.Subdir
		subdirPath = filepath.Join(tempRepoPath, resolved)
		hash, dlErr := downloadGitHubDir(owner, repo, source.Subdir, subdirPath, source, opts.OnProgress)
		if dlErr == nil {
			commitHash = hash
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("GitHub API install fallback: %v", dlErr))
			subdirPath = ""
			cleanupTempRepo(tempRepoPath)
		}
	}

	// Fast path 2b: CNB Contents API
	if subdirPath == "" && !hasRef && isCNBAPISource(source) {
		repo := cnbRepoPath(source.CloneURL)
		resolved = source.Subdir
		subdirPath = filepath.Join(tempRepoPath, resolved)
		hash, dlErr := downloadCNBDir(repo, source.Subdir, subdirPath, source, opts.OnProgress)
		if dlErr == nil {
			commitHash = hash
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("CNB API install fallback: %v", dlErr))
			subdirPath = ""
			_ = os.RemoveAll(tempRepoPath)
		}
	}

	// Fast path 2b: Gitea Contents API
	if subdirPath == "" && !hasRef && isGiteaAPISource(source) {
		owner, repo := giteaOwnerRepo(source.CloneURL)
		resolved = source.Subdir
		subdirPath = filepath.Join(tempRepoPath, resolved)
		hash, dlErr := downloadGiteaDir(owner, repo, source.Subdir, subdirPath, source, opts.OnProgress)
		if dlErr == nil {
			commitHash = hash
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Gitea API install fallback: %v", dlErr))
			subdirPath = ""
			_ = os.RemoveAll(tempRepoPath)
		}
	}

	// Fallback: full clone + fuzzy subdir resolution
	if subdirPath == "" {
		cleanupTempRepo(tempRepoPath)
		if opts.OnProgress != nil {
			opts.OnProgress("Cloning repository...")
		}
		if err := cloneRepoForSource(source, tempRepoPath, source.Branch, true, opts.OnProgress); err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
		if err := submoduleError(tempRepoPath, source.Subdir, source.authEnv()); err != nil {
			return nil, err
		}

		var err error
		resolved, err = resolveSubdir(tempRepoPath, source.Subdir)
		if err != nil {
			return nil, err
		}
		if resolved != source.Subdir {
			source.Subdir = resolved
			source.Name = filepath.Base(resolved)
			result.SkillName = source.Name
		}
		subdirPath = filepath.Join(tempRepoPath, resolved)
		if hash, hashErr := getGitCommit(tempRepoPath); hashErr == nil {
			commitHash = hash
		}
	}

	// Copy subdirectory to destination
	if err := copyDir(subdirPath, destPath); err != nil {
		return nil, fmt.Errorf("failed to copy skill: %w", err)
	}

	// Security audit
	if err := auditInstalledSkill(destPath, result, opts); err != nil {
		return nil, err
	}

	// Write metadata with file hashes
	meta := NewMetaFromSource(source)
	if commitHash != "" {
		meta.Version = commitHash
	}
	if resolved != "" {
		meta.TreeHash = getSubdirTreeHash(tempRepoPath, resolved)
	}
	if hashes, hashErr := ComputeFileHashes(destPath, opts.SourceFollow); hashErr == nil {
		meta.FileHashes = hashes
	}
	if err := WriteMetaToStore(opts.SourceDir, destPath, meta); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to write metadata: %v", err))
	}

	// Check for SKILL.md
	checkSkillFile(destPath, result)

	result.Action = "cloned and extracted"
	return result, nil
}

func checkSkillFile(skillPath string, result *InstallResult) {
	skillFile := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, "no SKILL.md found in skill directory")
	}
}

// InstallAgentFromDiscovery installs a single agent (.md file) from a discovery result.
// Unlike skill install (directory copy), agent install copies a single file.
func InstallAgentFromDiscovery(discovery *DiscoveryResult, agent AgentInfo, destDir string, opts InstallOptions) (*InstallResult, error) {
	// Derive the source root from destDir + Into, not the agent's nested
	// relative path, so callers without SourceDir retain stable metadata keys.
	opts.SourceDir = skillsRootFor(filepath.Join(destDir, filepath.Base(agent.Path)), opts)
	destFile := filepath.Join(destDir, filepath.FromSlash(installAgentRelativePath(agent)))
	return installAgentFromDiscoveryInternal(discovery, agent, destFile, opts, false)
}

func UpdateAgentFromDiscovery(discovery *DiscoveryResult, agent AgentInfo, destDir string, opts InstallOptions) (*InstallResult, error) {
	opts.SourceDir = skillsRootFor(filepath.Join(destDir, filepath.Base(agent.Path)), opts)
	destFile := filepath.Join(destDir, filepath.FromSlash(installAgentRelativePath(agent)))
	return installAgentFromDiscoveryInternal(discovery, agent, destFile, opts, true)
}

// computeSingleFileHash computes the sha256 hash for a single file.
func computeSingleFileHash(filePath string) (string, error) {
	return utils.FileHashFormatted(filePath)
}

// auditInstalledSkill scans the installed skill for security threats.
// It blocks installation when findings are at or above configured threshold
// unless force is enabled.
