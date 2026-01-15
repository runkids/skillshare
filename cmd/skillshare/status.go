package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

func cmdStatus(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	printSourceStatus(cfg)
	printTargetsStatus(cfg)
	checkSkillVersion(cfg)

	return nil
}

func printSourceStatus(cfg *config.Config) {
	ui.Header("Source")
	info, err := os.Stat(cfg.Source)
	if err != nil {
		ui.Error("%s (not found)", cfg.Source)
		return
	}

	entries, _ := os.ReadDir(cfg.Source)
	skillCount := 0
	for _, e := range entries {
		if e.IsDir() && !utils.IsHidden(e.Name()) {
			skillCount++
		}
	}
	ui.Success("%s (%d skills, %s)", cfg.Source, skillCount, info.ModTime().Format("2006-01-02 15:04"))
}

func printTargetsStatus(cfg *config.Config) {
	ui.Header("Targets")
	for name, target := range cfg.Targets {
		mode := getTargetMode(target.Mode, cfg.Mode)
		statusStr, detail := getTargetStatusDetail(target, cfg.Source, mode)
		ui.Status(name, statusStr, detail)
	}
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

func getTargetStatusDetail(target config.TargetConfig, source, mode string) (string, string) {
	if mode == "merge" {
		return getMergeStatusDetail(target, source, mode)
	}
	return getSymlinkStatusDetail(target, source, mode)
}

func getMergeStatusDetail(target config.TargetConfig, source, mode string) (string, string) {
	status, linkedCount, localCount := sync.CheckStatusMerge(target.Path, source)

	switch status {
	case sync.StatusMerged:
		return "merged", fmt.Sprintf("[%s] %s (%d shared, %d local)", mode, target.Path, linkedCount, localCount)
	case sync.StatusLinked:
		// Configured as merge but actually using symlink - needs resync
		return "linked", fmt.Sprintf("[%s->needs sync] %s", mode, target.Path)
	default:
		return status.String(), fmt.Sprintf("[%s] %s (%d local)", mode, target.Path, localCount)
	}
}

func getSymlinkStatusDetail(target config.TargetConfig, source, mode string) (string, string) {
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

	return status.String(), detail
}

func checkSkillVersion(cfg *config.Config) {
	skillFile := filepath.Join(cfg.Source, "skillshare", "SKILL.md")
	skillVersion := readSkillVersion(skillFile)

	if skillVersion == "" {
		// No skill or no version - suggest update
		ui.Header("Skill Update")
		ui.Warning("skillshare skill not found or missing version")
		ui.Info("Run: skillshare update")
		return
	}

	// Compare versions (simple string comparison works for semver)
	if version != "dev" && skillVersion != version {
		ui.Header("Skill Update")
		ui.Warning("skillshare skill is outdated (skill: %s, CLI: %s)", skillVersion, version)
		ui.Info("Run: skillshare update && skillshare sync")
	}
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
