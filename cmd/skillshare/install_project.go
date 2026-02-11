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

type projectInstallArgs struct {
	sourceArg string
	opts      install.InstallOptions
}

func parseProjectInstallArgs(args []string) (*projectInstallArgs, bool, error) {
	result := &projectInstallArgs{}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--name":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--name requires a value")
			}
			i++
			result.opts.Name = args[i]
		case arg == "--force" || arg == "-f":
			result.opts.Force = true
		case arg == "--update" || arg == "-u":
			result.opts.Update = true
		case arg == "--dry-run" || arg == "-n":
			result.opts.DryRun = true
		case arg == "--track" || arg == "-t":
			result.opts.Track = true
		case arg == "--skill" || arg == "-s":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--skill requires a value")
			}
			i++
			result.opts.Skills = strings.Split(args[i], ",")
		case arg == "--into":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--into requires a value")
			}
			i++
			result.opts.Into = args[i]
		case arg == "--all":
			result.opts.All = true
		case arg == "--yes" || arg == "-y":
			result.opts.Yes = true
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			if result.sourceArg != "" {
				return nil, false, fmt.Errorf("unexpected argument: %s", arg)
			}
			result.sourceArg = arg
		}
		i++
	}

	// Clean --skill input
	if len(result.opts.Skills) > 0 {
		cleaned := make([]string, 0, len(result.opts.Skills))
		for _, s := range result.opts.Skills {
			s = strings.TrimSpace(s)
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) == 0 {
			return nil, false, fmt.Errorf("--skill requires at least one skill name")
		}
		result.opts.Skills = cleaned
	}

	// Validate mutual exclusion
	if result.opts.HasSkillFilter() && result.opts.All {
		return nil, false, fmt.Errorf("--skill and --all cannot be used together")
	}
	if result.opts.HasSkillFilter() && result.opts.Yes {
		return nil, false, fmt.Errorf("--skill and --yes cannot be used together")
	}
	if result.opts.HasSkillFilter() && result.opts.Track {
		return nil, false, fmt.Errorf("--skill cannot be used with --track")
	}
	if result.opts.ShouldInstallAll() && result.opts.Track {
		return nil, false, fmt.Errorf("--all/--yes cannot be used with --track")
	}

	if result.opts.Into != "" {
		if err := validate.IntoPath(result.opts.Into); err != nil {
			return nil, false, err
		}
	}

	return result, false, nil
}

func cmdInstallProject(args []string, root string) (installLogSummary, error) {
	summary := installLogSummary{
		Mode: "project",
	}

	parsed, showHelp, err := parseProjectInstallArgs(args)
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

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return summary, err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return summary, err
	}

	if parsed.sourceArg == "" {
		if parsed.opts.Name != "" {
			return summary, fmt.Errorf("--name requires a source; it cannot be used with 'skillshare install -p' (no source)")
		}
		if parsed.opts.Into != "" {
			return summary, fmt.Errorf("--into requires a source; it cannot be used with 'skillshare install -p' (no source)")
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
		skillName := strings.TrimSpace(skill.Name)
		if skillName == "" {
			continue
		}

		destPath := filepath.Join(runtime.sourcePath, skillName)
		if _, err := os.Stat(destPath); err == nil {
			ui.StepDone(skillName, "skipped (already exists)")
			continue
		}

		source, err := install.ParseSource(skill.Source)
		if err != nil {
			ui.StepFail(skillName, fmt.Sprintf("invalid source: %v", err))
			continue
		}

		source.Name = skillName

		if skill.Tracked {
			trackedResult, err := install.InstallTrackedRepo(source, runtime.sourcePath, opts)
			if err != nil {
				ui.StepFail(skillName, err.Error())
				continue
			}
			if opts.DryRun {
				ui.StepDone(skillName, trackedResult.Action)
				continue
			}
			ui.StepDone(skillName, fmt.Sprintf("installed (tracked, %d skills)", trackedResult.SkillCount))
			if len(trackedResult.Skills) > 0 {
				summary.InstalledSkills = append(summary.InstalledSkills, trackedResult.Skills...)
			} else {
				summary.InstalledSkills = append(summary.InstalledSkills, skillName)
			}
		} else {
			if err := validate.SkillName(skillName); err != nil {
				ui.StepFail(skillName, fmt.Sprintf("invalid name: %v", err))
				continue
			}
			result, err := install.Install(source, destPath, opts)
			if err != nil {
				ui.StepFail(skillName, err.Error())
				continue
			}
			if opts.DryRun {
				ui.StepDone(skillName, result.Action)
				continue
			}
			if err := install.UpdateGitIgnore(filepath.Join(runtime.root, ".skillshare"), filepath.Join("skills", skillName)); err != nil {
				ui.Warning("Failed to update .skillshare/.gitignore: %v", err)
			}
			ui.StepDone(skillName, "installed")
			summary.InstalledSkills = append(summary.InstalledSkills, skillName)
		}

		installed++
	}

	if opts.DryRun {
		spinner.Stop()
		summary.SkillCount = len(summary.InstalledSkills)
		return summary, nil
	}

	spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	fmt.Println()
	ui.Info("Run 'skillshare sync' to create symlinks")
	summary.SkillCount = len(summary.InstalledSkills)

	if installed > 0 {
		if err := reconcileProjectRemoteSkills(runtime); err != nil {
			return summary, err
		}
	}

	return summary, nil
}
