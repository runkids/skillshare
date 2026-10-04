package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// discoverOptions controls the behavior of discoverSourceSkillsInternal.
type discoverOptions struct {
	parseFrontmatter bool // parse SKILL.md frontmatter for targets
	collectIgnored   bool // collect ignored skill paths into IgnoreStats
	collectTracked   bool // collect tracked repo paths (for Lite mode)
	collectContext   bool // compute DescChars/BodyChars during walk (for analyze)
	includeIgnored   bool // include ignored skills in results with Disabled=true
}

var (
	knownTargetNameSet     map[string]bool
	knownTargetNameSetOnce sync.Once
	projectTargetPaths     []projectTargetPath
	projectTargetPathsOnce sync.Once
)

type projectTargetPath struct {
	name string
	path string
}

func getKnownTargetNameSet() map[string]bool {
	knownTargetNameSetOnce.Do(func() {
		targetNames := config.KnownTargetNames()
		knownTargetNameSet = make(map[string]bool, len(targetNames))
		for _, name := range targetNames {
			knownTargetNameSet[name] = true
		}
	})
	return knownTargetNameSet
}

func getProjectTargetPaths() []projectTargetPath {
	projectTargetPathsOnce.Do(func() {
		groupedTargets := config.GroupedProjectTargets()
		projectTargetPaths = make([]projectTargetPath, 0, len(groupedTargets)*2)
		for _, target := range groupedTargets {
			if target.Name == "" || target.Path == "" {
				continue
			}
			p := filepath.ToSlash(target.Path)
			projectTargetPaths = append(projectTargetPaths, projectTargetPath{
				name: target.Name,
				path: p,
			})
			for _, member := range target.Members {
				projectTargetPaths = append(projectTargetPaths, projectTargetPath{
					name: member,
					path: p,
				})
			}
		}
		sort.Slice(projectTargetPaths, func(i, j int) bool {
			if projectTargetPaths[i].path == projectTargetPaths[j].path {
				return projectTargetPaths[i].name < projectTargetPaths[j].name
			}
			return projectTargetPaths[i].path < projectTargetPaths[j].path
		})
	})
	return projectTargetPaths
}

func inferSkillTargetsFromRelPath(relPath string) []string {
	// Prefer explicit project paths from known targets (e.g. ".factory/skills").
	targets := inferTargetsFromProjectPaths(relPath)
	if len(targets) > 0 {
		return targets
	}

	parts := strings.Split(relPath, "/")
	if len(parts) < 3 || parts[1] != "skills" {
		return nil
	}

	// Fallback to top-level host-style paths that match known target names
	// (e.g. "openclaw/skills/*" for openclaw project path "skills").
	host := strings.TrimPrefix(parts[0], ".")
	if host == "" {
		return nil
	}
	if getKnownTargetNameSet()[host] {
		return []string{host}
	}

	return nil
}

func inferTargetsFromProjectPaths(relPath string) []string {
	targets := make([]string, 0, 2)

	for _, target := range getProjectTargetPaths() {
		if matchesHostSkillPath(relPath, target.name, target.path) {
			targets = append(targets, target.name)
		}
	}

	return targets
}

func matchesHostSkillPath(relPath, targetName, targetPath string) bool {
	switch {
	case targetPath == "skills":
		parts := strings.Split(relPath, "/")
		return len(parts) >= 2 && parts[1] == "skills" && strings.TrimPrefix(parts[0], ".") == targetName
	case relPath == targetPath:
		return false
	case strings.HasPrefix(relPath, targetPath+"/"):
		return true
	default:
		return false
	}
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

	err := sourcewalk.Walk(walkRoot, sourcewalk.Options{}, func(path string, info os.FileInfo, err error) error {
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
		if info.IsDir() && !opts.collectIgnored && !opts.includeIgnored {
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
		if info.IsDir() && !opts.collectIgnored && !opts.includeIgnored {
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

			isInRepo := false
			parts := strings.Split(relPath, "/")
			if len(parts) > 0 && utils.IsTrackedRepoDir(parts[0]) {
				isInRepo = true
			}

			// Root-level .skillignore fallback (for files in non-skipped dirs),
			// then the repo-level .skillignore inside tracked repos. With
			// includeIgnored the skill is kept, flagged Disabled, and measured
			// like any other so analyze can price it for symlink-mode targets.
			disabled := rootMatcher.Match(relPath, false) || (isInRepo && isSkillIgnored(parts, walkRoot, ignoreMatchers))
			if disabled {
				if opts.collectIgnored {
					stats.IgnoredSkills = append(stats.IgnoredSkills, relPath)
				}
				if !opts.includeIgnored {
					return nil
				}
			}

			skillFile := filepath.Join(skillDir, "SKILL.md")

			var targets []string
			var ctx skillContext
			var lintIssues []LintIssue

			if opts.collectContext {
				// Single read: parse targets + context from one os.ReadFile
				content, readErr := os.ReadFile(skillFile)
				if readErr == nil {
					targets = utils.ParseFrontmatterListFromBytes(content, "targets")
					var yamlErr error
					ctx, yamlErr = calcContextFromContent(content)
					if yamlErr != nil {
						lintIssues = append(lintIssues, LintIssue{
							Rule:     "malformed-frontmatter",
							Severity: LintError,
							Category: "structure",
							Message:  fmt.Sprintf("frontmatter YAML is malformed: %v", yamlErr),
						})
					}
					lintResults, err := LintSkill(ctx.Name, ctx.Description, ctx.BodyChars)
					if err != nil {
						return fmt.Errorf("lint skill %s: %w", relPath, err)
					}
					lintIssues = append(lintIssues, lintResults...)
				}
			} else if opts.parseFrontmatter {
				targets = utils.ParseFrontmatterList(skillFile, "targets")
			}

			if len(targets) == 0 && (opts.parseFrontmatter || opts.collectContext) {
				targets = inferSkillTargetsFromRelPath(relPath)
			}

			// Use original sourcePath (not walkRoot) so SourcePath preserves
			// the caller's logical path, even if sourcePath is a symlink.
			skills = append(skills, DiscoveredSkill{
				SourcePath:  filepath.Join(sourcePath, relPath),
				RelPath:     relPath,
				FlatName:    utils.PathToFlatName(relPath),
				IsInRepo:    isInRepo,
				Targets:     targets,
				DescChars:   ctx.DescChars,
				BodyChars:   ctx.BodyChars,
				DescTokens:  ctx.DescTokens,
				BodyTokens:  ctx.BodyTokens,
				Description: ctx.Description,
				LintIssues:  lintIssues,
				Disabled:    disabled,
			})
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to walk source directory: %w", err)
	}

	if opts.parseFrontmatter || opts.collectContext {
		applyTargetOverrides(skills, walkRoot)
	}

	return skills, trackedRepos, stats, nil
}

// applyTargetOverrides replaces the frontmatter targets of tracked-repo skills
// with the overrides stored in the source's .metadata.json, so every consumer
// of discovery sees the same effective targets. An empty override means all
// targets.
func applyTargetOverrides(skills []DiscoveredSkill, sourceDir string) {
	var overrides map[string][]string
	loaded := false
	for i := range skills {
		if !skills[i].IsInRepo {
			continue
		}
		if !loaded {
			overrides = install.LoadTargetOverrides(sourceDir)
			loaded = true
		}
		targets, ok := overrides[skills[i].RelPath]
		if !ok {
			continue
		}
		if len(targets) == 0 {
			skills[i].Targets = nil
		} else {
			skills[i].Targets = append([]string(nil), targets...)
		}
	}
}
