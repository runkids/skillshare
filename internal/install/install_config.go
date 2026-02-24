package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/ui"
	"skillshare/internal/validate"
)

// InstallFromConfig iterates over the remote skills listed in the config
// (via ctx.ConfigSkills) and installs each one that is not already present.
// It handles both tracked repos and plain skills, delegates per-skill hooks
// to ctx.PostInstallSkill, and calls ctx.Reconcile when at least one skill
// was installed.
//
// The caller is responsible for UI chrome (logo, spinner, next-steps).
func InstallFromConfig(ctx InstallContext, opts InstallOptions) (ConfigInstallResult, error) {
	result := ConfigInstallResult{
		InstalledSkills: make([]string, 0),
		FailedSkills:    make([]string, 0),
	}

	sourcePath := ctx.SourcePath()

	for _, skill := range ctx.ConfigSkills() {
		groupDir, bareName := skill.EffectiveParts()
		if strings.TrimSpace(bareName) == "" {
			continue
		}

		displayName := skill.FullName()
		destPath := filepath.Join(sourcePath, filepath.FromSlash(displayName))

		// Skip skills that already exist on disk.
		if _, err := os.Stat(destPath); err == nil {
			ui.StepDone(displayName, "skipped (already exists)")
			continue
		}

		source, err := ParseSource(skill.Source)
		if err != nil {
			ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			continue
		}
		source.Name = bareName

		if skill.Tracked {
			installed := installTrackedFromConfig(source, sourcePath, displayName, groupDir, opts)
			if installed.failed {
				result.FailedSkills = append(result.FailedSkills, displayName)
				continue
			}
			if opts.DryRun {
				ui.StepDone(displayName, installed.action)
				continue
			}
			result.InstalledSkills = append(result.InstalledSkills, installed.skills...)
		} else {
			ok := installPlainFromConfig(ctx, source, sourcePath, destPath, displayName, bareName, groupDir, opts)
			if !ok {
				result.FailedSkills = append(result.FailedSkills, displayName)
				continue
			}
			if opts.DryRun {
				continue // StepDone already called inside installPlainFromConfig
			}
			result.InstalledSkills = append(result.InstalledSkills, displayName)
		}

		result.Installed++
	}

	// Reconcile config after successful installs.
	if result.Installed > 0 && !opts.DryRun {
		if err := ctx.Reconcile(); err != nil {
			return result, err
		}
	}

	return result, nil
}

// trackedInstallOutcome captures the result of a single tracked-repo install
// within the config loop.
type trackedInstallOutcome struct {
	failed bool
	action string   // only meaningful on dry-run
	skills []string // skills names to record
}

// installTrackedFromConfig installs a single tracked repo from config and
// returns a summary used by the caller loop.
func installTrackedFromConfig(
	source *Source,
	sourcePath string,
	displayName string,
	groupDir string,
	opts InstallOptions,
) trackedInstallOutcome {
	trackOpts := opts
	if groupDir != "" {
		trackOpts.Into = groupDir
	}

	trackedResult, err := InstallTrackedRepo(source, sourcePath, trackOpts)
	if err != nil {
		ui.StepFail(displayName, err.Error())
		return trackedInstallOutcome{failed: true}
	}

	if opts.DryRun {
		return trackedInstallOutcome{action: trackedResult.Action}
	}

	ui.StepDone(displayName, fmt.Sprintf("installed (tracked, %d skills)", trackedResult.SkillCount))

	skills := trackedResult.Skills
	if len(skills) == 0 {
		skills = []string{displayName}
	}
	return trackedInstallOutcome{skills: skills}
}

// installPlainFromConfig installs a single non-tracked skill from config.
// Returns true on success, false on failure (UI messages emitted internally).
func installPlainFromConfig(
	ctx InstallContext,
	source *Source,
	sourcePath string,
	destPath string,
	displayName string,
	bareName string,
	groupDir string,
	opts InstallOptions,
) bool {
	if err := validate.SkillName(bareName); err != nil {
		ui.StepFail(displayName, fmt.Sprintf("invalid name: %v", err))
		return false
	}

	// Ensure group directory exists.
	if groupDir != "" {
		if err := os.MkdirAll(filepath.Join(sourcePath, filepath.FromSlash(groupDir)), 0o755); err != nil {
			ui.StepFail(displayName, fmt.Sprintf("failed to create group directory: %v", err))
			return false
		}
	}

	result, err := Install(source, destPath, opts)
	if err != nil {
		ui.StepFail(displayName, err.Error())
		return false
	}

	if opts.DryRun {
		ui.StepDone(displayName, result.Action)
		return true
	}

	if err := ctx.PostInstallSkill(displayName); err != nil {
		ui.Warning("post-install hook failed for %s: %v", displayName, err)
	}

	ui.StepDone(displayName, "installed")
	return true
}
