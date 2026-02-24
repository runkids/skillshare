package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	"skillshare/internal/validate"
	appversion "skillshare/internal/version"
)

func cmdInstallProject(args []string, root string) (installLogSummary, error) {
	summary := installLogSummary{
		Mode: "project",
	}

	parsed, showHelp, err := parseInstallArgs(args)
	if showHelp {
		printInstallHelp()
		return summary, nil
	}
	if err != nil {
		return summary, err
	}
	summary.DryRun = parsed.opts.DryRun
	summary.Tracked = parsed.opts.Track
	summary.Source = parsed.sourceArg
	summary.Into = parsed.opts.Into
	summary.SkipAudit = parsed.opts.SkipAudit

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return summary, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return summary, err
	}
	if parsed.opts.AuditThreshold == "" {
		parsed.opts.AuditThreshold = runtime.config.Audit.BlockThreshold
	}
	parsed.opts.AuditProjectRoot = root
	summary.AuditThreshold = parsed.opts.AuditThreshold

	if parsed.sourceArg == "" {
		hasSourceFlags := parsed.opts.Name != "" || parsed.opts.Into != "" ||
			parsed.opts.Track || len(parsed.opts.Skills) > 0 ||
			len(parsed.opts.Exclude) > 0 || parsed.opts.All || parsed.opts.Yes || parsed.opts.Update
		if hasSourceFlags {
			return summary, fmt.Errorf("flags --name, --into, --track, --skill, --exclude, --all, --yes, and --update require a source argument")
		}
		summary.Source = "project-config"
		return installFromProjectConfig(runtime, parsed.opts)
	}

	cfg := &config.Config{Source: runtime.sourcePath}
	source, resolvedFromMeta, err := resolveInstallSource(parsed.sourceArg, parsed.opts, cfg)
	if err != nil {
		return summary, err
	}

	if resolvedFromMeta {
		summary, err = handleDirectInstall(source, cfg, parsed.opts)
		summary.Mode = "project"
		if err != nil {
			return summary, err
		}
		if !parsed.opts.DryRun {
			return summary, reconcileProjectRemoteSkills(runtime)
		}
		return summary, nil
	}

	summary, err = dispatchInstall(source, cfg, parsed.opts)
	summary.Mode = "project"
	if err != nil {
		return summary, err
	}

	if parsed.opts.DryRun {
		return summary, nil
	}

	return summary, reconcileProjectRemoteSkills(runtime)
}

func installFromProjectConfig(runtime *projectRuntime, opts install.InstallOptions) (installLogSummary, error) {
	summary := installLogSummary{
		Mode:   "project",
		Source: "project-config",
		DryRun: opts.DryRun,
	}

	if len(runtime.config.Skills) == 0 {
		ui.Info("No remote skills defined in .skillshare/config.yaml")
		return summary, nil
	}

	ui.Logo(appversion.Version)

	total := len(runtime.config.Skills)
	spinner := ui.StartSpinner(fmt.Sprintf("Installing %d skill(s) from config...", total))

	installed := 0

	for _, skill := range runtime.config.Skills {
		groupDir, bareName := skill.EffectiveParts()
		if strings.TrimSpace(bareName) == "" {
			continue
		}

		displayName := skill.FullName()
		destPath := filepath.Join(runtime.sourcePath, filepath.FromSlash(displayName))
		if _, err := os.Stat(destPath); err == nil {
			ui.StepDone(displayName, "skipped (already exists)")
			continue
		}

		source, err := install.ParseSource(skill.Source)
		if err != nil {
			ui.StepFail(displayName, fmt.Sprintf("invalid source: %v", err))
			continue
		}

		source.Name = bareName

		if skill.Tracked {
			trackOpts := opts
			if groupDir != "" {
				trackOpts.Into = groupDir
			}
			trackedResult, err := install.InstallTrackedRepo(source, runtime.sourcePath, trackOpts)
			if err != nil {
				ui.StepFail(displayName, err.Error())
				continue
			}
			if opts.DryRun {
				ui.StepDone(displayName, trackedResult.Action)
				continue
			}
			ui.StepDone(displayName, fmt.Sprintf("installed (tracked, %d skills)", trackedResult.SkillCount))
			if len(trackedResult.Skills) > 0 {
				summary.InstalledSkills = append(summary.InstalledSkills, trackedResult.Skills...)
			} else {
				summary.InstalledSkills = append(summary.InstalledSkills, displayName)
			}
		} else {
			if err := validate.SkillName(bareName); err != nil {
				ui.StepFail(displayName, fmt.Sprintf("invalid name: %v", err))
				continue
			}
			// Ensure group directory exists
			if groupDir != "" {
				if err := os.MkdirAll(filepath.Join(runtime.sourcePath, filepath.FromSlash(groupDir)), 0755); err != nil {
					ui.StepFail(displayName, fmt.Sprintf("failed to create group directory: %v", err))
					continue
				}
			}
			result, err := install.Install(source, destPath, opts)
			if err != nil {
				ui.StepFail(displayName, err.Error())
				continue
			}
			if opts.DryRun {
				ui.StepDone(displayName, result.Action)
				continue
			}
			if err := install.UpdateGitIgnore(filepath.Join(runtime.root, ".skillshare"), filepath.Join("skills", displayName)); err != nil {
				ui.Warning("Failed to update .skillshare/.gitignore: %v", err)
			}
			ui.StepDone(displayName, "installed")
			summary.InstalledSkills = append(summary.InstalledSkills, displayName)
		}

		installed++
	}

	if opts.DryRun {
		spinner.Stop()
		summary.SkillCount = len(summary.InstalledSkills)
		return summary, nil
	}

	spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to create symlinks")
	summary.SkillCount = len(summary.InstalledSkills)

	if installed > 0 {
		if err := reconcileProjectRemoteSkills(runtime); err != nil {
			return summary, err
		}
	}

	return summary, nil
}
