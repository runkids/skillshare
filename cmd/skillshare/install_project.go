package main

import (
	"fmt"
	"time"

	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/ui"
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

	if err := ensureProjectConfig(root); err != nil {
		return summary, err
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

	// A forced reinstall moves a skill that is already pinned.
	if !parsed.opts.DryRun {
		defer trackProjectLock(runtime)()
	}

	cfg := configFromProjectRuntime(runtime)
	source, resolvedFromMeta, err := resolveInstallSource(parsed.sourceArg, parsed.opts, cfg)
	if err == nil && parsed.opts.Branch != "" {
		source.Branch = parsed.opts.Branch
	}
	if err != nil {
		return summary, err
	}
	if err := install.RejectProjectRootLocalInstall(source, root); err != nil {
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

	ctx := &projectInstallContext{runtime: runtime}

	lock, err := install.LoadLock(projectdir.Resolve(runtime.root))
	if err != nil {
		return summary, err
	}
	opts.Lock = lock

	if len(ctx.ConfigSkills()) == 0 {
		ui.Info("No remote skills defined in .skillshare/config.yaml")
		return summary, nil
	}

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

	spinner.Stop()
	fmt.Println()
	ui.Done(ui.MarkOK, "Installed "+plural(result.Installed, "skill"), time.Since(spinner.Started()))
	ui.Next("skillshare sync", "link them into your targets")

	return summary, nil
}
