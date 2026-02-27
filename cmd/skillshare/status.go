package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

func cmdStatus(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	if mode == modeProject {
		if len(rest) > 0 {
			return fmt.Errorf("unexpected arguments: %v", rest)
		}
		return cmdStatusProject(cwd)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	sp := ui.StartSpinner("Discovering skills...")
	discovered, discoverErr := sync.DiscoverSourceSkills(cfg.Source)
	if discoverErr != nil {
		discovered = nil
	}
	trackedRepos := extractTrackedRepos(discovered)
	sp.Stop()

	printSourceStatus(cfg, len(discovered))
	printTrackedReposStatus(cfg, discovered, trackedRepos)
	if err := printTargetsStatus(cfg, discovered); err != nil {
		return err
	}
	checkSkillVersion(cfg)

	return nil
}

func printSourceStatus(cfg *config.Config, skillCount int) {
	ui.Header("Source")
	info, err := os.Stat(cfg.Source)
	if err != nil {
		ui.Error("%s (not found)", cfg.Source)
		return
	}

	ui.Success("%s (%d skills, %s)", cfg.Source, skillCount, info.ModTime().Format("2006-01-02 15:04"))
}

func printTrackedReposStatus(cfg *config.Config, discovered []sync.DiscoveredSkill, trackedRepos []string) {
	if len(trackedRepos) == 0 {
		return
	}

	ui.Header("Tracked Repositories")
	for _, repoName := range trackedRepos {
		repoPath := filepath.Join(cfg.Source, repoName)

		skillCount := 0
		for _, d := range discovered {
			if d.IsInRepo && strings.HasPrefix(d.RelPath, repoName+"/") {
				skillCount++
			}
		}

		statusStr := "up-to-date"
		statusIcon := "✓"
		if isDirty, _ := checkRepoDirty(repoPath); isDirty {
			statusStr = "has uncommitted changes"
			statusIcon = "!"
		}

		ui.Status(repoName, statusIcon, fmt.Sprintf("%d skills, %s", skillCount, statusStr))
	}
}

// extractTrackedRepos derives tracked repo names from discovered skills.
// Note: repos with zero skills will not appear (acceptable trade-off).
func extractTrackedRepos(discovered []sync.DiscoveredSkill) []string {
	seen := make(map[string]bool)
	var repos []string
	for _, d := range discovered {
		if !d.IsInRepo {
			continue
		}
		// First path segment is the repo name (e.g. "_team" from "_team/frontend/ui")
		idx := strings.Index(d.RelPath, "/")
		if idx <= 0 {
			continue
		}
		repo := d.RelPath[:idx]
		if !seen[repo] {
			seen[repo] = true
			repos = append(repos, repo)
		}
	}
	sort.Strings(repos)
	return repos
}

// checkRepoDirty checks if a git repository has uncommitted changes
func checkRepoDirty(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// targetStatusResult bundles status detail with synced count to avoid
// duplicate CheckStatusMerge/Copy calls.
type targetStatusResult struct {
	statusStr   string
	detail      string
	syncedCount int // -1 for symlink mode (no drift check)
}

func printTargetsStatus(cfg *config.Config, discovered []sync.DiscoveredSkill) error {
	ui.Header("Targets")
	driftTotal := 0
	for name, target := range cfg.Targets {
		mode := getTargetMode(target.Mode, cfg.Mode)
		res := getTargetStatusDetail(target, cfg.Source, mode)
		ui.Status(name, res.statusStr, res.detail)

		if mode == "merge" || mode == "copy" {
			filtered, err := sync.FilterSkills(discovered, target.Include, target.Exclude)
			if err != nil {
				return fmt.Errorf("target %s has invalid include/exclude config: %w", name, err)
			}
			filtered = sync.FilterSkillsByTarget(filtered, name)
			expectedCount := len(filtered)

			if res.syncedCount < expectedCount {
				drift := expectedCount - res.syncedCount
				if drift > driftTotal {
					driftTotal = drift
				}
			}
		} else if len(target.Include) > 0 || len(target.Exclude) > 0 {
			ui.Warning("%s: include/exclude ignored in symlink mode", name)
		}
	}
	if driftTotal > 0 {
		ui.Warning("%d skill(s) not synced — run 'skillshare sync'", driftTotal)
	}
	return nil
}

func getTargetMode(targetMode, globalMode string) string {
	if targetMode != "" {
		return targetMode
	}
	if globalMode != "" {
		return globalMode
	}
	return "merge"
}

func getTargetStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	switch mode {
	case "merge":
		return getMergeStatusDetail(target, source, mode)
	case "copy":
		return getCopyStatusDetail(target, mode)
	default:
		return getSymlinkStatusDetail(target, source, mode)
	}
}

func getMergeStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	status, linkedCount, localCount := sync.CheckStatusMerge(target.Path, source)

	switch status {
	case sync.StatusMerged:
		return targetStatusResult{"merged", fmt.Sprintf("[%s] %s (%d shared, %d local)", mode, target.Path, linkedCount, localCount), linkedCount}
	case sync.StatusLinked:
		return targetStatusResult{"linked", fmt.Sprintf("[%s->needs sync] %s", mode, target.Path), linkedCount}
	default:
		return targetStatusResult{status.String(), fmt.Sprintf("[%s] %s (%d local)", mode, target.Path, localCount), 0}
	}
}

func getCopyStatusDetail(target config.TargetConfig, mode string) targetStatusResult {
	status, managedCount, localCount := sync.CheckStatusCopy(target.Path)

	switch status {
	case sync.StatusCopied:
		return targetStatusResult{"copied", fmt.Sprintf("[%s] %s (%d managed, %d local)", mode, target.Path, managedCount, localCount), managedCount}
	case sync.StatusLinked:
		return targetStatusResult{"linked", fmt.Sprintf("[%s->needs sync] %s", mode, target.Path), managedCount}
	default:
		return targetStatusResult{status.String(), fmt.Sprintf("[%s] %s (%d local)", mode, target.Path, localCount), 0}
	}
}

func getSymlinkStatusDetail(target config.TargetConfig, source, mode string) targetStatusResult {
	status := sync.CheckStatus(target.Path, source)
	detail := fmt.Sprintf("[%s] %s", mode, target.Path)

	switch status {
	case sync.StatusConflict:
		link, _ := os.Readlink(target.Path)
		detail = fmt.Sprintf("[%s] %s -> %s", mode, target.Path, link)
	case sync.StatusMerged:
		// Configured as symlink but actually using merge - needs resync
		detail = fmt.Sprintf("[%s->needs sync] %s", mode, target.Path)
	}

	return targetStatusResult{status.String(), detail, -1}
}

func checkSkillVersion(cfg *config.Config) {
	ui.Header("Version")

	// CLI version
	ui.Success("CLI: %s", version)

	// Skill version
	skillFile := filepath.Join(cfg.Source, "skillshare", "SKILL.md")
	localVersion := readSkillVersion(skillFile)

	if localVersion == "" {
		ui.Warning("Skill: not found or missing version")
		ui.Info("  Run: skillshare upgrade --skill")
		return
	}

	// Fetch remote version (with short timeout)
	remoteVersion := fetchRemoteSkillVersion()
	if remoteVersion == "" {
		// Network error - just show local version
		ui.Info("Skill: %s", localVersion)
		return
	}

	// Compare local vs remote
	if localVersion != remoteVersion {
		ui.Warning("Skill: %s (update available: %s)", localVersion, remoteVersion)
		ui.Info("  Run: skillshare upgrade --skill && skillshare sync")
	} else {
		ui.Success("Skill: %s (up to date)", localVersion)
	}
}

func fetchRemoteSkillVersion() string {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(skillshareSkillURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	scanner := bufio.NewScanner(resp.Body)
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}

		if inFrontmatter && strings.HasPrefix(line, "version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}

func readSkillVersion(skillFile string) string {
	file, err := os.Open(skillFile)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			// End of frontmatter
			break
		}

		if inFrontmatter && strings.HasPrefix(line, "version:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return ""
}
