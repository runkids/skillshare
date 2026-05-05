package main

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	appversion "skillshare/internal/version"
)

func cmdInstallProject(args []string, root string) (installLogSummary, error) {
	parsed, showHelp, err := parseInstallArgs(args)
	if showHelp {
		printInstallHelp()
		return installLogSummary{Mode: "project"}, nil
	}
	if err != nil {
		return installLogSummary{Mode: "project"}, err
	}
	applyInstallJSONDefaults(parsed)
	return cmdInstallProjectParsed(parsed, root)
}

func cmdInstallProjectParsed(parsed *installArgs, root string) (installLogSummary, error) {
	summary := installLogSummary{
		Mode: "project",
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
			parsed.opts.Track || parsed.opts.HasSkillFilter() || parsed.opts.HasAgentFilter() ||
			len(parsed.opts.Exclude) > 0 || parsed.opts.Update || parsed.opts.Branch != "" ||
			parsed.opts.Kind != ""
		if hasSourceFlags || ((parsed.opts.All || parsed.opts.Yes) && !parsed.jsonOutput) {
			return summary, fmt.Errorf("flags --name, --into, --track, --skill, --agent, --kind, --exclude, --all, --yes, --branch, and --update require a source argument")
		}
		summary.Source = "project-config"
		return installFromProjectConfig(runtime, parsed.opts)
	}

	cfg := &config.Config{Source: runtime.sourcePath, AgentsSource: runtime.agentsSourcePath, GitLabHosts: runtime.config.GitLabHosts, AzureHosts: runtime.config.AzureHosts}
	source, resolvedFromMeta, err := resolveInstallSource(parsed.sourceArg, parsed.opts, cfg)
	if err == nil && parsed.opts.Branch != "" {
		source.Branch = parsed.opts.Branch
	}
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
			freshStore, loadErr := install.LoadMetadata(runtime.sourcePath)
			if loadErr == nil {
				runtime.skillsStore = freshStore
			}
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

	// Reload metadata store: install may have written new entries via WriteMeta
	// that the pre-install runtime doesn't know about.
	freshStore, loadErr := install.LoadMetadata(runtime.sourcePath)
	if loadErr == nil {
		runtime.skillsStore = freshStore
	}

	return summary, reconcileProjectRemoteSkills(runtime)
}

func installFromProjectConfig(runtime *projectRuntime, opts install.InstallOptions) (installLogSummary, error) {
	summary := installLogSummary{
		Mode:   "project",
		Source: "project-config",
		DryRun: opts.DryRun,
	}

	ctx := &projectInstallContext{runtime: runtime}

	if len(ctx.ConfigSkills()) == 0 {
		ui.Info("No remote skills defined in .skillshare/config.yaml")
		return summary, nil
	}

	ui.Logo(appversion.Version)
	total := len(ctx.ConfigSkills())
	spinner := ui.StartSpinner(fmt.Sprintf("Installing %d skill(s) from config...", total))

	result, err := install.InstallFromConfig(ctx, opts)
	if err != nil {
		spinner.Fail("Install failed")
		summary.InstalledSkills = result.InstalledSkills
		summary.FailedSkills = result.FailedSkills
		summary.SkillCount = len(result.InstalledSkills)
		return summary, err
	}

	summary.InstalledSkills = result.InstalledSkills
	summary.FailedSkills = result.FailedSkills
	summary.SkillCount = len(result.InstalledSkills)

	if opts.DryRun {
		spinner.Stop()
		return summary, nil
	}

	spinner.Success(fmt.Sprintf("Installed %d skill(s)", result.Installed))
	ui.SectionLabel("Next Steps")
	ui.Info("Run 'skillshare sync' to create symlinks")

	return summary, nil
}
