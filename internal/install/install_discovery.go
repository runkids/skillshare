package install

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
	"skillshare/internal/validate"
)

// TargetDotDirs is the set of hidden directory names (e.g. ".claude", ".cursor")
// to skip during skill discovery. Set by the CLI entrypoint from config data
// to avoid a circular import between install and config packages.
var TargetDotDirs map[string]bool

func discoverFromGitWithProgressImpl(source *Source, onProgress ProgressCallback) (*DiscoveryResult, error) {
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}
	if err := resolveWebRef(source); err != nil {
		return nil, err
	}

	// Clone to temp directory
	tempDir, err := os.MkdirTemp("", "skillshare-discover-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	repoPath := filepath.Join(tempDir, "repo")
	if err := cloneRepoForSource(source, repoPath, source.Branch, true, onProgress); err != nil {
		cleanupTempRepo(tempDir)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Discover skills (include root to support single-skill-at-root repos)
	skills := discoverSkillsIn(repoPath, repoPath, true)

	// Use the source name rather than the temporary clone directory for the root.
	applySkillInstallNames(skills, repoPath, source.Name)

	// Discover agents (agents/ dir or pure agent repo fallback)
	agents := discoverAgents(repoPath, len(skills) > 0)
	skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
	if err != nil {
		cleanupTempRepo(tempDir)
		return nil, err
	}

	commitHash, _ := getGitCommit(repoPath)

	return &DiscoveryResult{
		RepoPath:   tempDir,
		Skills:     skills,
		Agents:     agents,
		Source:     source,
		CommitHash: commitHash,
		Warnings:   SubmoduleWarnings(repoPath, source.authEnv()),
	}, nil
}

// discoverFromGitImpl is the non-progress variant used by the public facade.
func discoverFromGitImpl(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitWithProgressImpl(source, nil)
}

func discoverLocalImpl(source *Source) (*DiscoveryResult, error) {
	if source == nil {
		return nil, fmt.Errorf("source is required")
	}
	if source.Type != SourceTypeLocalPath {
		return nil, fmt.Errorf("source is not a local path")
	}

	info, err := os.Stat(source.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot access source path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", source.Path)
	}

	skills := discoverSkillsIn(source.Path, source.Path, true)
	applySkillInstallNames(skills, source.Path, source.Name)

	agents := discoverAgents(source.Path, len(skills) > 0)
	skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
	if err != nil {
		return nil, err
	}

	return &DiscoveryResult{
		RepoPath: source.Path,
		Skills:   skills,
		Agents:   agents,
		Source:   source,
	}, nil
}

// resolveSubdir resolves a subdirectory path within a cloned repo.
// It first checks for an exact match. If not found, it scans the repo for
// SKILL.md files and looks for a skill whose name matches filepath.Base(subdir).
// Returns the resolved subdir path (may differ from input) or an error.

func resolveSubdir(repoPath, subdir string) (string, error) {
	// 1. Exact match — fast path
	exact := filepath.Join(repoPath, subdir)
	info, err := os.Stat(exact)
	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("'%s' is not a directory", subdir)
		}
		return subdir, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot access subdirectory: %w", err)
	}

	// 2. Fuzzy match — scan for SKILL.md files whose directory basename matches
	baseName := filepath.Base(subdir)
	skills := discoverSkills(repoPath, false) // exclude root
	var candidates []string
	for _, sk := range skills {
		if filepath.Base(sk.Path) == baseName {
			candidates = append(candidates, sk.Path)
		}
	}

	switch len(candidates) {
	case 0:
		return "", fmt.Errorf("subdirectory '%s' does not exist in repository", subdir)
	case 1:
		return candidates[0], nil
	default:
		return "", fmt.Errorf("subdirectory '%s' is ambiguous — multiple matches found:\n  %s",
			subdir, strings.Join(candidates, "\n  "))
	}
}

// constrainDiscoveryToTarget narrows discovery to the SKILL.md or agent file
// the source names, if it names one. The agent must be one discovery found, so
// an update that rediscovers the file's directory finds it again.
func constrainDiscoveryToTarget(source *Source, skills []SkillInfo, agents []AgentInfo) ([]SkillInfo, []AgentInfo, error) {
	if source != nil && source.AgentFile != "" {
		for _, agent := range agents {
			if agent.Path == source.AgentFile {
				return nil, []AgentInfo{agent}, nil
			}
		}
		return nil, nil, fmt.Errorf("'%s' is not an agent file in the repository", path.Join(source.Subdir, source.AgentFile))
	}
	if source == nil || !source.TargetsExplicitSkill() {
		return skills, agents, nil
	}

	for _, skill := range skills {
		if skill.Path == "." {
			return []SkillInfo{skill}, nil, nil
		}
	}

	return nil, nil, fmt.Errorf("explicit skill target does not resolve to a root SKILL.md: %s", source.Raw)
}

// DefaultSkillName gives generic container directories a meaningful name,
// while preserving established directory names and rejecting unsafe frontmatter.
func DefaultSkillName(dir, fallback string) string {
	if !validate.IsLikelySkillsPath(fallback) {
		return fallback
	}
	name, err := utils.ParseSkillName(dir)
	if err != nil {
		return fallback
	}
	name = strings.ToLower(strings.Join(strings.Fields(name), "-"))
	if validate.SkillName(name) != nil {
		return fallback
	}
	return name
}

// ExistingSkillInstallName preserves a recorded destination for source-based updates.
func ExistingSkillInstallName(sourceDir string, source *Source, skill SkillInfo, opts InstallOptions) (string, error) {
	if !opts.Update || opts.Name != "" {
		return skill.Name, nil
	}
	fullSource := buildDiscoverySkillSource(source, skill.Path)
	store := LoadMetadataOrNew(sourceDir)
	var existing string
	for _, key := range store.List() {
		entry := store.Get(key)
		if entry.Tracked || entry.EffectiveKind() != "skill" || !isDiscoverySkillSource(entry, source, skill.Path) {
			continue
		}
		rel := KeyToRelPath(key, entry)
		if filepath.Dir(rel) != filepath.Clean(opts.Into) {
			continue
		}
		if _, err := os.Stat(filepath.Join(sourceDir, rel, "SKILL.md")); err != nil {
			continue
		}
		if existing != "" {
			return "", fmt.Errorf("multiple installed skills match source %s; use --name to choose the update destination", fullSource)
		}
		existing = filepath.Base(rel)
	}
	if existing != "" {
		return existing, nil
	}
	destPath := filepath.Join(sourceDir, opts.Into, skill.Name)
	if !opts.Force {
		if _, err := os.Stat(destPath); err == nil && !dirIsEmpty(destPath) {
			return "", fmt.Errorf("already exists at %s; use --force to overwrite", destPath)
		}
	}
	return skill.Name, nil
}

// discoverSkills finds directories containing SKILL.md
// If includeRoot is true, root-level SKILL.md is also included (with Path=".")
func discoverSkills(repoPath string, includeRoot bool) []SkillInfo {
	return discoverSkillsWithin(repoPath, repoPath, includeRoot)
}

// discoverSkillsWithin is discoverSkills for dir inside a clone at root: it
// also finds skills behind links to folders that stay inside root.
func discoverSkillsWithin(root, dir string, includeRoot bool) []SkillInfo {
	skills := discoverSkillsIn(dir, root, includeRoot)
	applySkillInstallNames(skills, dir, filepath.Base(dir))
	return skills
}

func applySkillInstallNames(skills []SkillInfo, repoPath, rootName string) {
	directoryNames := make(map[string]int)
	installNames := make(map[string]int)
	fallbacks := make([]string, len(skills))
	for i := range skills {
		name := filepath.Base(skills[i].Path)
		if skills[i].Path == "." {
			name = rootName
		}
		fallbacks[i] = name
		directoryNames[strings.ToLower(name)]++
		skills[i].Name = DefaultSkillName(filepath.Join(repoPath, skills[i].Path), name)
		installNames[strings.ToLower(skills[i].Name)]++
	}
	for i := range skills {
		name := skills[i].Name
		// A new frontmatter name must not overwrite another discovered skill.
		if name != fallbacks[i] && (directoryNames[strings.ToLower(name)] > 0 || installNames[strings.ToLower(name)] > 1) {
			skills[i].Name = fallbacks[i]
		}
	}
}

// discoverSkillsByDirectory finds skills without following links, as sync
// sees a tracked repository.
func discoverSkillsByDirectory(repoPath string, includeRoot bool) []SkillInfo {
	return discoverSkillsIn(repoPath, "", includeRoot)
}

// discoverSkillsIn finds directories under repoPath that contain SKILL.md.
// With linkRoot set, it also descends into links to folders that stay inside
// linkRoot, listing their skills under the link's path. A skill reachable both
// directly and through a link is listed once, under its direct path.
func discoverSkillsIn(repoPath, linkRoot string, includeRoot bool) []SkillInfo {
	var found []discoveredSkill
	canonicalRepo := sourcewalk.Canonical(repoPath)

	visit := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip .git and known target dotdirs (.claude, .cursor, .skillshare, etc.)
		// to avoid counting target-synced copies as source skills. The walk root
		// is exempt: it was requested explicitly (e.g. `install repo/.claude`),
		// so filtering it would discover nothing at all.
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || (path != repoPath && TargetDotDirs[name]) {
				return filepath.SkipDir
			}
		}

		// Check if this is a SKILL.md file
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, _ := filepath.Rel(repoPath, skillDir)
			if relPath == "." && !includeRoot {
				return nil
			}
			fm := utils.ParseFrontmatterFields(path, []string{"description", "license"})
			skill := discoveredSkill{SkillInfo: SkillInfo{
				Name:        filepath.Base(skillDir),
				Path:        strings.ReplaceAll(relPath, "\\", "/"),
				License:     fm["license"],
				Description: fm["description"],
			}}
			if relPath == "." {
				skill.Name = filepath.Base(repoPath)
			}
			if linkRoot != "" {
				skill.folder = sourcewalk.Canonical(skillDir)
				skill.direct = skill.folder == filepath.Join(canonicalRepo, relPath)
			}
			found = append(found, skill)
		}

		return nil
	}
	walk := func(dir string) {
		if linkRoot == "" {
			filepath.Walk(dir, visit)
			return
		}
		walkFollowingLinksWithin(linkRoot, dir, visit)
	}
	walk(repoPath)
	if len(found) == 0 {
		if dir, ok := sourcewalk.AgentsSkillsDir(repoPath, TargetDotDirs); ok {
			walk(dir)
		}
	}

	// Apply .skillignore filtering before dropping link duplicates, so an
	// ignored direct path leaves the skill listed under its link.
	// Skill paths are directories (they contain SKILL.md), so pass isDir=true.
	matcher := skillignore.ReadMatcher(repoPath)
	if matcher.HasRules() {
		filtered := found[:0]
		for _, s := range found {
			if !matcher.Match(s.Path, true) {
				filtered = append(filtered, s)
			}
		}
		found = filtered
	}
	if linkRoot != "" {
		found = dropLinkedDuplicates(found)
	}

	var skills []SkillInfo
	for _, s := range found {
		skills = append(skills, s.SkillInfo)
	}
	return skills
}

// discoveredSkill is a skill found while following links.
type discoveredSkill struct {
	SkillInfo
	folder string // the skill folder after links
	direct bool   // reached without a link
}

// dropLinkedDuplicates keeps one entry per skill folder, preferring the path
// that reaches it without a link, so a convenience link does not list a skill
// twice or move it from the path it was installed from.
func dropLinkedDuplicates(found []discoveredSkill) []discoveredSkill {
	index := make(map[string]int, len(found))
	var out []discoveredSkill
	for _, s := range found {
		if j, ok := index[s.folder]; ok {
			if !out[j].direct && s.direct {
				out[j] = s
			}
			continue
		}
		index[s.folder] = len(out)
		out = append(out, s)
	}
	return out
}

// walkFollowingLinksWithin walks dir like filepath.Walk, and also walks the
// folders that links under it point to when they stay inside root and out of
// its .git folder. A link inside a skill folder is part of that skill and is
// not followed. A followed folder is reported under the link's path. Each link
// is followed once, and a link back to a folder the walk is inside is not
// followed, so cycles stop and the walk stays linear in the number of links.
// Two links to one folder are both walked; the caller drops duplicates after
// .skillignore.
func walkFollowingLinksWithin(root, dir string, fn filepath.WalkFunc) {
	canonicalRoot := sourcewalk.Canonical(root)
	gitDir := filepath.Join(canonicalRoot, ".git")
	inside := func(path string) bool {
		return pathSameOrDescendant(path, canonicalRoot) && !pathSameOrDescendant(path, gitDir)
	}
	inWalk := map[string]bool{}
	followedLinks := map[string]bool{}
	var walk func(logical, real string)
	walk = func(logical, real string) {
		inWalk[real] = true
		defer delete(inWalk, real)
		filepath.Walk(real, func(path string, info os.FileInfo, err error) error {
			logicalPath := logical + path[len(real):]
			if err == nil && path != real && utils.IsLinkMode(path, info.Mode()) {
				// Canonical, not EvalSymlinks: on Windows EvalSymlinks leaves a
				// junction unresolved.
				target := sourcewalk.Canonical(path)
				if !inWalk[target] && !followedLinks[path] && inside(target) && !skillAbove(filepath.Dir(path), real) {
					if targetInfo, statErr := os.Stat(target); statErr == nil && targetInfo.IsDir() {
						if r := fn(logicalPath, sourcewalk.NamedInfo(targetInfo, info.Name()), nil); r != nil {
							if r == filepath.SkipDir {
								return nil
							}
							return r
						}
						followedLinks[path] = true
						walk(logicalPath, target)
						return nil
					}
				}
			}
			if path == real && logical != real && err == nil {
				// The walk root reached through a link: report it under the
				// link's name too.
				info = sourcewalk.NamedInfo(info, filepath.Base(logical))
			}
			return fn(logicalPath, info, err)
		})
	}
	if real := sourcewalk.Canonical(dir); inside(real) {
		walk(dir, real)
	}
}

// skillAbove reports whether dir, or a folder above it up to top, holds
// SKILL.md.
func skillAbove(dir, top string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			return true
		}
		if dir == top || filepath.Dir(dir) == dir {
			return false
		}
		dir = filepath.Dir(dir)
	}
}

// discoverAgents finds .md files in an agents/ convention directory.
// Also detects "pure agent repos" — repos with no SKILL.md and no agents/ dir
// but with .md files at root (per D5 rule 4).
func discoverAgents(repoPath string, hasSkills bool) []AgentInfo {
	var agents []AgentInfo

	// Rule 2: Check agents/ convention directory
	agentsDir := filepath.Join(repoPath, "agents")
	if info, err := os.Stat(agentsDir); err == nil && info.IsDir() {
		agents = append(agents, scanAgentDir(repoPath, agentsDir)...)
		return agents
	}

	// Rule 4: Pure agent repo fallback — no skills, no agents/ dir, root has .md files
	if !hasSkills {
		agents = append(agents, scanAgentDir(repoPath, repoPath)...)
	}

	return agents
}

// scanAgentDir scans a directory for .md agent files, excluding conventional files.
func scanAgentDir(repoRoot, dir string) []AgentInfo {
	var agents []AgentInfo

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			// Same walk-root exemption as discoverSkills: an explicitly
			// requested dot-dir must still be scanned.
			if name == ".git" || (path != dir && TargetDotDirs[name]) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		// Skip conventional excludes
		if conventionalAgentExcludes[info.Name()] {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return nil
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		name := strings.TrimSuffix(info.Name(), ".md")

		agents = append(agents, AgentInfo{
			Name:     name,
			Path:     relPath,
			FileName: info.Name(),
		})

		return nil
	})

	return agents
}

// scopeToAgentFile points a source whose path ends in a .md file other than
// SKILL.md (repo/agents/demo.md) at the file's directory and remembers the
// file. Every discovery path then reads a directory, and the agent is recorded
// as `install repo/agents -a demo` records it (#1013).
func scopeToAgentFile(source *Source) {
	file := path.Base(source.Subdir)
	if !strings.HasSuffix(strings.ToLower(file), ".md") || strings.EqualFold(file, "SKILL.md") {
		return
	}
	source.AgentFile = file
	source.Subdir = path.Dir(source.Subdir)
	if source.Subdir == "." {
		source.Subdir = ""
	}
}

var conventionalAgentExcludes = map[string]bool{
	"README.md":    true,
	"CHANGELOG.md": true,
	"LICENSE.md":   true,
	"HISTORY.md":   true,
	"SECURITY.md":  true,
	"SKILL.md":     true,
}

// DiscoverFromGitSubdir clones a repo and discovers skills within a subdirectory
// Unlike DiscoverFromGit, this includes root-level SKILL.md of the subdir

func discoverFromGitSubdirWithProgressImpl(source *Source, onProgress ProgressCallback) (*DiscoveryResult, error) {
	if !isGitInstalled() {
		return nil, fmt.Errorf("git is not installed or not in PATH")
	}

	if !source.HasSubdir() {
		return nil, fmt.Errorf("source has no subdirectory specified")
	}
	if err := resolveWebRef(source); err != nil {
		return nil, err
	}
	scopeToAgentFile(source)
	if !source.HasSubdir() {
		// The whole path was the ref, e.g. tree/feature/x, or named a root agent file.
		return discoverFromGitWithProgressImpl(source, onProgress)
	}

	// Prepare temporary repo directory
	tempDir, err := os.MkdirTemp("", "skillshare-discover-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	repoPath := filepath.Join(tempDir, "repo")
	var warnings []string
	var commitHash string
	var subdirPath string

	// The Contents API paths below ignore any branch, tag or commit.
	hasRef := source.ref() != ""

	// Fast path 1: sparse checkout (preferred for speed if git is modern)
	// Works for GitHub and non-GitHub hosts.
	if gitSupportsSparseCheckout() {
		err := sparseCloneSubdir(source.CloneURL, source.Subdir, repoPath, source.ref(), source.authEnv(), onProgress)
		if err == nil {
			err = sparseLinksFetched(repoPath, source.Subdir, source.authEnv())
		}
		if err == nil {
			if err := submoduleError(repoPath, source.Subdir, source.authEnv()); err != nil {
				cleanupTempRepo(tempDir)
				return nil, err
			}
			subdirPath = filepath.Join(repoPath, source.Subdir)
			if info, statErr := os.Stat(subdirPath); statErr == nil && info.IsDir() {
				source.recordCommit(repoPath)
				if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
					commitHash = hash
				}
				skills := discoverSkillsWithin(repoPath, subdirPath, true)
				agents := discoverAgents(subdirPath, len(skills) > 0)
				skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
				if err != nil {
					cleanupTempRepo(tempDir)
					return nil, err
				}
				return &DiscoveryResult{
					RepoPath:   tempDir,
					Skills:     skills,
					Agents:     agents,
					Source:     source,
					CommitHash: commitHash,
					Warnings:   warnings,
				}, nil
			}
			warnings = append(warnings, "sparse checkout discovery fallback: subdirectory missing after checkout")
			cleanupTempRepo(repoPath)
			subdirPath = ""
		} else {
			warnings = append(warnings, fmt.Sprintf("sparse checkout discovery fallback: %v", err))
			cleanupTempRepo(repoPath)
			subdirPath = ""
		}
	}

	// Fast path 2: GitHub/GHE Contents API
	// Fallback for when sparse checkout is unavailable or fails.
	if subdirPath == "" && !hasRef && isGitHubAPISource(source) {
		owner, repo := source.GitHubOwner(), source.GitHubRepo()
		subdirPath = filepath.Join(repoPath, source.Subdir)
		hash, dlErr := downloadGitHubDir(owner, repo, source.Subdir, subdirPath, source, onProgress)
		if dlErr == nil {
			commitHash = hash
			skills := discoverSkills(subdirPath, true)
			agents := discoverAgents(subdirPath, len(skills) > 0)
			skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
			if err != nil {
				cleanupTempRepo(tempDir)
				return nil, err
			}
			return &DiscoveryResult{
				RepoPath:   tempDir,
				Skills:     skills,
				Agents:     agents,
				Source:     source,
				CommitHash: commitHash,
			}, nil
		}
		warnings = append(warnings, fmt.Sprintf("GitHub API discovery fallback: %v", dlErr))
		cleanupTempRepo(repoPath)
		subdirPath = ""
	}

	// Fast path 2b: CNB Contents API
	if subdirPath == "" && !hasRef && isCNBAPISource(source) {
		repo := cnbRepoPath(source.CloneURL)
		subdirPath = filepath.Join(repoPath, source.Subdir)
		hash, dlErr := downloadCNBDir(repo, source.Subdir, subdirPath, source, onProgress)
		if dlErr == nil {
			commitHash = hash
			skills := discoverSkills(subdirPath, true)
			agents := discoverAgents(subdirPath, len(skills) > 0)
			skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
			if err != nil {
				_ = os.RemoveAll(tempDir)
				return nil, err
			}
			return &DiscoveryResult{
				RepoPath:   tempDir,
				Skills:     skills,
				Agents:     agents,
				Source:     source,
				CommitHash: commitHash,
				Warnings:   warnings,
			}, nil
		}
		warnings = append(warnings, fmt.Sprintf("CNB API discovery fallback: %v", dlErr))
		_ = os.RemoveAll(repoPath)
		subdirPath = ""
	}

	// Fast path 2b: Gitea Contents API
	if subdirPath == "" && !hasRef && isGiteaAPISource(source) {
		owner, repo := giteaOwnerRepo(source.CloneURL)
		subdirPath = filepath.Join(repoPath, source.Subdir)
		hash, dlErr := downloadGiteaDir(owner, repo, source.Subdir, subdirPath, source, onProgress)
		if dlErr == nil {
			commitHash = hash
			skills := discoverSkills(subdirPath, true)
			agents := discoverAgents(subdirPath, len(skills) > 0)
			skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
			if err != nil {
				_ = os.RemoveAll(tempDir)
				return nil, err
			}
			return &DiscoveryResult{
				RepoPath:   tempDir,
				Skills:     skills,
				Agents:     agents,
				Source:     source,
				CommitHash: commitHash,
				Warnings:   warnings,
			}, nil
		}
		warnings = append(warnings, fmt.Sprintf("Gitea API discovery fallback: %v", dlErr))
		_ = os.RemoveAll(repoPath)
		subdirPath = ""
	}

	// Fallback: full clone + fuzzy subdir resolution
	cleanupTempRepo(repoPath)
	if onProgress != nil {
		onProgress("Cloning repository...")
	}
	if err := cloneRepoForSource(source, repoPath, source.Branch, true, onProgress); err != nil {
		cleanupTempRepo(tempDir)
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}
	if err := submoduleError(repoPath, source.Subdir, source.authEnv()); err != nil {
		cleanupTempRepo(tempDir)
		return nil, err
	}

	resolved, err := resolveSubdir(repoPath, source.Subdir)
	if err != nil {
		cleanupTempRepo(tempDir)
		return nil, err
	}
	if resolved != source.Subdir {
		source.Subdir = resolved
		source.Name = filepath.Base(resolved)
	}
	subdirPath = filepath.Join(repoPath, resolved)
	if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
		commitHash = hash
	}

	skills := discoverSkillsWithin(repoPath, subdirPath, true)
	agents := discoverAgents(subdirPath, len(skills) > 0)
	skills, agents, err = constrainDiscoveryToTarget(source, skills, agents)
	if err != nil {
		cleanupTempRepo(tempDir)
		return nil, err
	}
	return &DiscoveryResult{
		RepoPath:   tempDir,
		Skills:     skills,
		Agents:     agents,
		Source:     source,
		CommitHash: commitHash,
		Warnings:   warnings,
	}, nil
}

// discoverFromGitSubdirImpl is the non-progress variant used by the public facade.
func discoverFromGitSubdirImpl(source *Source) (*DiscoveryResult, error) {
	return discoverFromGitSubdirWithProgressImpl(source, nil)
}

// CleanupDiscovery removes the temporary directory created for git-based
// discovery (clone under RepoPath). It only runs when Source is known to be
// a git remote; local paths, missing source metadata, and other types are
// skipped so RepoPath is never treated as disposable unless it came from a
// git clone.

func cleanupDiscoveryImpl(result *DiscoveryResult) {
	if result == nil || result.RepoPath == "" || result.Source == nil {
		return
	}
	if !result.Source.IsGit() {
		return
	}
	cleanupTempRepo(result.RepoPath)
}

// InstallFromDiscovery installs a skill from a discovered repository
