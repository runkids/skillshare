package install

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/utils"
)

// BatchUpdateProgressPrefix marks per-skill grouped update progress messages
// emitted through InstallOptions.OnProgress.
const BatchUpdateProgressPrefix = "skillshare:batch-update:"

// BatchUpdateResult holds results for a batch of skills from the same repository
type BatchUpdateResult struct {
	RepoURL    string
	CommitHash string
	Results    map[string]*InstallResult // map[skillRelPath]result
	Errors     map[string]error          // map[skillRelPath]error
	// Current lists subdirs skipped because the installed copy already matches
	// the fetched repo state.
	Current map[string]bool
}

// UpdateSkillsFromRepo updates multiple skills that belong to the same git repository.
// It clones the repository once to a temporary directory and then installs/updates
// each requested skill from that local copy.
//
// skillTargets maps repo-internal subdir (meta.Subdir) → local absolute destination path.
// This avoids assuming the local path mirrors the repo structure.
// branch is the branch the skills were installed from (empty = remote default).
func UpdateSkillsFromRepo(repoURL, branch string, skillTargets map[string]string, opts InstallOptions) (*BatchUpdateResult, error) {
	if repoURL == "" {
		return nil, fmt.Errorf("repoURL is required")
	}

	source, err := ParseSource(repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo URL %q: %w", repoURL, err)
	}
	source.Branch = branch

	// 1. Discover skills from the repo (clones once)
	discovery, err := DiscoverFromGitWithProgress(source, opts.OnProgress)
	if err != nil {
		return nil, err
	}
	defer CleanupDiscovery(discovery)

	result := &BatchUpdateResult{
		RepoURL:    repoURL,
		CommitHash: discovery.CommitHash,
		Results:    make(map[string]*InstallResult),
		Errors:     make(map[string]error),
		Current:    make(map[string]bool),
	}
	repoPath := filepath.Join(discovery.RepoPath, "repo")
	if result.CommitHash == "" {
		if hash, hashErr := getGitCommit(repoPath); hashErr == nil {
			result.CommitHash = hash
		}
	}

	// Create a map for quick lookup in discovery
	discoveryMap := make(map[string]SkillInfo)
	for _, s := range discovery.Skills {
		discoveryMap[s.Path] = s
	}
	treeHashCache := make(map[string]string)
	// Read-only snapshot: each skill's own entry is read before its install
	// rewrites the store, so later writes cannot invalidate it.
	var store *MetadataStore
	if opts.SourceDir != "" {
		store, _ = loadMetadataFile(opts.SourceDir)
	}
	totalSkills := len(skillTargets)
	processedSkills := 0
	// Store entries skipped on a tree-hash match whose version lags HEAD; they
	// are refreshed with one load and save after the loop.
	var staleVersionKeys []string

	// 2. Install/Update each requested skill from the discovered repo
	for repoSubdir, destPath := range skillTargets {
		lookupKey := repoSubdir
		if lookupKey == "" {
			lookupKey = "."
		}
		skillInfo, ok := discoveryMap[lookupKey]
		if !ok {
			// Discovery skips target dot-dirs (.claude, .codex, ...), so a skill
			// installed from such a subdir never appears in the map even though it
			// still exists upstream. Resolve the subdir directly before declaring
			// the path deleted, otherwise the skill is misreported as stale.
			skillInfo, ok = lookupSkillSubdir(repoPath, lookupKey)
		}
		if !ok {
			result.Errors[repoSubdir] = fmt.Errorf("skill path %q not found in repository", repoSubdir)
			processedSkills++
			emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
			continue
		}
		// Fast path for update --all on huge inventories:
		// if installed metadata already points at this exact repo commit/tree,
		// skip reinstall/copy entirely.
		// Metadata outlives a deleted install, so require the skill on disk.
		if opts.Update && !opts.DryRun && !opts.Reinstall && skillFileExists(destPath) {
			installed := installedSkillMeta(store, opts.SourceDir, destPath)
			if isSkillCurrentAtRepoState(installed, repoSubdir, result.CommitHash, repoPath, treeHashCache) {
				// Best-effort metadata refresh:
				// when skipping by tree-hash match on a moved HEAD, keep the version
				// aligned to latest commit to avoid stale version reporting.
				if strings.TrimSpace(installed.Version) != strings.TrimSpace(result.CommitHash) {
					if installed.storeKey == "" {
						_ = refreshSkillMetaVersionIfNeeded(destPath, result.CommitHash)
					} else {
						staleVersionKeys = append(staleVersionKeys, installed.storeKey)
					}
				}
				result.Current[repoSubdir] = true
				processedSkills++
				emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
				continue
			}
		}

		// Use a local copy of options to avoid side effects
		skillOpts := opts
		skillOpts.OnProgress = nil // Progress handled by clone stage

		installResult, err := InstallFromDiscovery(discovery, skillInfo, destPath, skillOpts)
		if err != nil {
			result.Errors[repoSubdir] = err
			processedSkills++
			emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
			continue
		}
		result.Results[repoSubdir] = installResult
		processedSkills++
		emitBatchSkillProgress(opts.OnProgress, processedSkills, totalSkills, repoSubdir)
	}

	_ = refreshStoreVersions(opts.SourceDir, staleVersionKeys, result.CommitHash)
	return result, nil
}

// lookupSkillSubdir resolves a skill subdir that discovery filtered out by
// checking the cloned repo directly for its SKILL.md. Mirrors the direct-stat
// fast path used by single-skill updates (resolveSubdir).
func lookupSkillSubdir(repoPath, subdir string) (SkillInfo, bool) {
	if repoPath == "" || subdir == "" || subdir == "." {
		return SkillInfo{}, false
	}
	cleaned := path.Clean(strings.ReplaceAll(subdir, "\\", "/"))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return SkillInfo{}, false
	}
	skillFile := filepath.Join(repoPath, filepath.FromSlash(cleaned), "SKILL.md")
	info, err := os.Stat(skillFile)
	if err != nil || info.IsDir() {
		return SkillInfo{}, false
	}
	fm := utils.ParseFrontmatterFields(skillFile, []string{"description", "license"})
	return SkillInfo{
		Name:        DefaultSkillName(filepath.Dir(skillFile), path.Base(cleaned)),
		Path:        cleaned,
		License:     fm["license"],
		Description: fm["description"],
	}, true
}

// skillFileExists reports whether dir holds an installed SKILL.md.
func skillFileExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !info.IsDir()
}

// installedMeta is the recorded state of an installed skill, read from the
// centralized store or, for installs that predate it, the legacy sidecar.
type installedMeta struct {
	Version  string
	TreeHash string
	storeKey string // store entry key; empty when read from a sidecar
}

// installedSkillMeta looks up destPath's metadata. store may be nil.
func installedSkillMeta(store *MetadataStore, sourceDir, destPath string) *installedMeta {
	if store != nil && sourceDir != "" {
		if rel, err := filepath.Rel(sourceDir, destPath); err == nil {
			rel = filepath.ToSlash(rel)
			if entry := store.GetByPath(rel); entry != nil {
				return &installedMeta{Version: entry.Version, TreeHash: entry.TreeHash, storeKey: storeKeyFor(store, rel, entry)}
			}
		}
	}
	meta, err := ReadMeta(destPath)
	if err != nil || meta == nil {
		return nil
	}
	return &installedMeta{Version: meta.Version, TreeHash: meta.TreeHash}
}

// storeKeyFor returns the key GetByPath(rel) resolved entry under.
func storeKeyFor(store *MetadataStore, rel string, entry *MetadataEntry) string {
	if store.Entries[rel] == entry {
		return rel
	}
	return path.Base(rel)
}

// isSkillCurrentAtRepoState returns true when installed metadata already
// matches the latest fetched repo state for this skill path.
func isSkillCurrentAtRepoState(meta *installedMeta, repoSubdir, commitHash, repoPath string, treeHashCache map[string]string) bool {
	if commitHash == "" || repoPath == "" || meta == nil {
		return false
	}

	subdir := strings.TrimSpace(repoSubdir)
	metaVersion := strings.TrimSpace(meta.Version)
	commitHash = strings.TrimSpace(commitHash)

	// Fast path: exact repo commit match means installed content is already current.
	// This avoids per-skill git subtree lookups on very large batches.
	if metaVersion != "" && metaVersion == commitHash {
		return true
	}

	if subdir == "" || subdir == "." {
		// Root installs don't have a stable tree hash field; require commit match.
		return false
	}
	if strings.TrimSpace(meta.TreeHash) == "" {
		return false
	}

	treeHash, ok := treeHashCache[subdir]
	if !ok {
		treeHash = getSubdirTreeHash(repoPath, subdir)
		treeHashCache[subdir] = treeHash
	}
	return treeHash != "" && meta.TreeHash == treeHash
}

func emitBatchSkillProgress(onProgress ProgressCallback, done, total int, repoSubdir string) {
	if onProgress == nil || total <= 0 {
		return
	}
	label := strings.TrimSpace(repoSubdir)
	if label == "" {
		label = "."
	}
	onProgress(fmt.Sprintf("%s%d/%d %s", BatchUpdateProgressPrefix, done, total, label))
}

// refreshSkillMetaVersionIfNeeded records commitHash as the version in the
// legacy sidecar of a skill skipped as current.
func refreshSkillMetaVersionIfNeeded(destPath, commitHash string) error {
	commitHash = strings.TrimSpace(commitHash)
	if commitHash == "" {
		return nil
	}
	meta, err := ReadMeta(destPath)
	if err != nil || meta == nil {
		return err
	}
	if strings.TrimSpace(meta.Version) == commitHash {
		return nil
	}
	meta.Version = commitHash
	return WriteMeta(destPath, meta)
}

// refreshStoreVersions records commitHash as the version of the store entries
// in keys, skills skipped on a tree-hash match after HEAD moved. The full
// commit stays: their files did not move, and a project lock pin follows that
// field.
func refreshStoreVersions(sourceDir string, keys []string, commitHash string) error {
	commitHash = strings.TrimSpace(commitHash)
	if sourceDir == "" || commitHash == "" || len(keys) == 0 {
		return nil
	}
	store, err := loadMetadataFile(sourceDir)
	if err != nil {
		return err
	}
	changed := false
	for _, key := range keys {
		if entry := store.Get(key); entry != nil && strings.TrimSpace(entry.Version) != commitHash {
			entry.Version = commitHash
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return store.Save(sourceDir)
}
