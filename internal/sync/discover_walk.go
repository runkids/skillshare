package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/skillignore"
	"skillshare/internal/utils"
)

// discoverOptions controls the behavior of discoverSourceSkillsInternal.
type discoverOptions struct {
	parseFrontmatter bool // parse SKILL.md frontmatter for targets
	collectIgnored   bool // collect ignored skill paths into IgnoreStats
	collectTracked   bool // collect tracked repo paths (for Lite mode)
	collectContext   bool // compute DescChars/BodyChars during walk (for analyze)
}

// discoverSourceSkillsInternal is the shared walk implementation used by all
// public discovery functions. It returns:
//   - discovered skills
//   - tracked repo paths (only when opts.collectTracked is true; nil otherwise)
//   - ignore stats (only when opts.collectIgnored is true; nil otherwise)
//   - error
func discoverSourceSkillsInternal(sourcePath string, opts discoverOptions) ([]DiscoveredSkill, []string, *skillignore.IgnoreStats, error) {
	var skills []DiscoveredSkill
	var trackedRepos []string
	ignoreMatchers := make(map[string]*skillignore.Matcher) // tracked repo abs path → .skillignore matcher

	walkRoot := utils.ResolveSymlink(sourcePath)
	rootMatcher := skillignore.ReadMatcher(walkRoot)

	// Stats collection (only allocated when needed)
	var stats *skillignore.IgnoreStats
	if opts.collectIgnored {
		stats = &skillignore.IgnoreStats{}
		// Record root .skillignore if it exists
		rootIgnorePath := filepath.Join(walkRoot, ".skillignore")
		if _, err := os.Stat(rootIgnorePath); err == nil {
			stats.RootFile = rootIgnorePath
		}
		// Record root .skillignore.local if it exists
		if rootMatcher.HasLocal {
			stats.RootLocalFile = filepath.Join(walkRoot, ".skillignore.local")
		}
		if pats := rootMatcher.Patterns(); len(pats) > 0 {
			stats.Patterns = append(stats.Patterns, pats...)
		}
	}

	err := filepath.Walk(walkRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Skip directories matching root-level .skillignore.
		// When collectIgnored is true, disable CanSkipDir so the walk
		// descends into ignored directories and the file-level Match
		// check can record each ignored SKILL.md path.
		if info.IsDir() && !opts.collectIgnored {
			relPath, relErr := filepath.Rel(walkRoot, path)
			if relErr == nil && relPath != "." {
				relPath = strings.ReplaceAll(relPath, "\\", "/")
				if rootMatcher.CanSkipDir(relPath) {
					return filepath.SkipDir
				}
			}
		}

		// Collect tracked repos: _-prefixed directories that are git repos
		if info.IsDir() && info.Name() != "." && utils.IsTrackedRepoDir(info.Name()) {
			gitDir := filepath.Join(path, ".git")
			if _, statErr := os.Stat(gitDir); statErr == nil {
				if opts.collectTracked {
					relPath, relErr := filepath.Rel(walkRoot, path)
					if relErr == nil && relPath != "." {
						trackedRepos = append(trackedRepos, relPath)
					}
				}
				m := skillignore.ReadMatcher(path)
				if m.HasRules() {
					ignoreMatchers[path] = m
					// Record repo-level .skillignore in stats
					if opts.collectIgnored {
						repoIgnorePath := filepath.Join(path, ".skillignore")
						stats.RepoFiles = append(stats.RepoFiles, repoIgnorePath)
						if m.HasLocal {
							stats.RepoLocalFiles = append(stats.RepoLocalFiles, filepath.Join(path, ".skillignore.local"))
						}
						if pats := m.Patterns(); len(pats) > 0 {
							stats.Patterns = append(stats.Patterns, pats...)
						}
					}
				}
			}
		}

		// Skip directories matching repo-level .skillignore inside tracked repos.
		// Same CanSkipDir bypass as above when collectIgnored is true.
		if info.IsDir() && !opts.collectIgnored {
			relPath, relErr := filepath.Rel(walkRoot, path)
			if relErr == nil && relPath != "." {
				relPath = strings.ReplaceAll(relPath, "\\", "/")
				parts := strings.Split(relPath, "/")
				if len(parts) > 1 && utils.IsTrackedRepoDir(parts[0]) {
					repoAbsPath := filepath.Join(walkRoot, parts[0])
					if m, ok := ignoreMatchers[repoAbsPath]; ok {
						repoRelPath := strings.Join(parts[1:], "/")
						if m.CanSkipDir(repoRelPath) {
							return filepath.SkipDir
						}
					}
				}
			}
		}

		// Look for SKILL.md files
		if !info.IsDir() && info.Name() == "SKILL.md" {
			skillDir := filepath.Dir(path)
			relPath, err := filepath.Rel(walkRoot, skillDir)
			if err != nil {
				return nil
			}

			if relPath == "." {
				return nil
			}

			relPath = strings.ReplaceAll(relPath, "\\", "/")

			// Root-level .skillignore fallback (for files in non-skipped dirs)
			if rootMatcher.Match(relPath, false) {
				if opts.collectIgnored {
					stats.IgnoredSkills = append(stats.IgnoredSkills, relPath)
				}
				return nil
			}

			isInRepo := false
			parts := strings.Split(relPath, "/")
			if len(parts) > 0 && utils.IsTrackedRepoDir(parts[0]) {
				isInRepo = true
			}

			if isInRepo && isSkillIgnored(parts, walkRoot, ignoreMatchers) {
				if opts.collectIgnored {
					stats.IgnoredSkills = append(stats.IgnoredSkills, relPath)
				}
				return nil
			}

			skillFile := filepath.Join(skillDir, "SKILL.md")

			var targets []string
			var descChars, bodyChars int
			var description string
			var lintIssues []LintIssue

			if opts.collectContext {
				// Single read: parse targets + context from one os.ReadFile
				content, readErr := os.ReadFile(skillFile)
				if readErr == nil {
					targets = utils.ParseFrontmatterListFromBytes(content, "targets")
					descChars, bodyChars, description = calcContextFromContent(content)
					fmName := parseFrontmatterName(content)
					lintIssues = LintSkill(fmName, description, bodyChars)
				}
			} else if opts.parseFrontmatter {
				targets = utils.ParseFrontmatterList(skillFile, "targets")
			}

			// Use original sourcePath (not walkRoot) so SourcePath preserves
			// the caller's logical path, even if sourcePath is a symlink.
			skills = append(skills, DiscoveredSkill{
				SourcePath:  filepath.Join(sourcePath, relPath),
				RelPath:     relPath,
				FlatName:    utils.PathToFlatName(relPath),
				IsInRepo:    isInRepo,
				Targets:     targets,
				DescChars:   descChars,
				BodyChars:   bodyChars,
				Description: description,
				LintIssues:  lintIssues,
			})
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to walk source directory: %w", err)
	}

	return skills, trackedRepos, stats, nil
}
