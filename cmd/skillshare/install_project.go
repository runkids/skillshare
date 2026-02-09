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

	return result, false, nil
}

func cmdInstallProject(args []string, root string) error {
	parsed, showHelp, err := parseProjectInstallArgs(args)
	if showHelp {
		printInstallHelp()
		return nil
	}
	if err != nil {
		return err
	}

	if !projectConfigExists(root) {
		if err := performProjectInit(root, projectInitOptions{}); err != nil {
			return err
		}
	}

	runtime, err := loadProjectRuntime(root)
	if err != nil {
		return err
	}

	if parsed.sourceArg == "" {
		if parsed.opts.Name != "" {
			return fmt.Errorf("--name requires a source; it cannot be used with 'skillshare install -p' (no source)")
		}
		return installFromProjectConfig(runtime, parsed.opts)
	}

	cfg := &config.Config{Source: runtime.sourcePath}
	source, resolvedFromMeta, err := resolveInstallSource(parsed.sourceArg, parsed.opts, cfg)
	if err != nil {
		return err
	}

	if resolvedFromMeta {
		if err := handleDirectInstall(source, cfg, parsed.opts); err != nil {
			return err
		}
		if !parsed.opts.DryRun {
			return reconcileProjectRemoteSkills(runtime)
		}
		return nil
	}

	if err := dispatchInstall(source, cfg, parsed.opts); err != nil {
		return err
	}

	if parsed.opts.DryRun {
		return nil
	}

	return reconcileProjectRemoteSkills(runtime)
}

func installFromProjectConfig(runtime *projectRuntime, opts install.InstallOptions) error {
	if len(runtime.config.Skills) == 0 {
		ui.Info("No remote skills defined in .skillshare/config.yaml")
		return nil
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
		}

		installed++
	}

	if opts.DryRun {
		spinner.Stop()
		return nil
	}

	spinner.Success(fmt.Sprintf("Installed %d skill(s)", installed))
	fmt.Println()
	ui.Info("Run 'skillshare sync' to create symlinks")

	if installed > 0 {
		if err := reconcileProjectRemoteSkills(runtime); err != nil {
			return err
		}
	}

	return nil
}
