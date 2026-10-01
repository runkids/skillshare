package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/hooks"
	"skillshare/internal/mcp"
	"skillshare/internal/plugin"
	"skillshare/internal/ui"
)

// doctorPluginTimeout bounds the native plugin CLIs a plugin check runs.
const doctorPluginTimeout = 30 * time.Second

// checkNativeResources runs the MCP, hooks and plugin checks against the same config
// and scope as their own commands. cfg is nil in project mode.
func checkNativeResources(cfg *config.Config, projectRoot string, result *doctorResult) {
	mcpService := &mcp.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir(), ConfigDirs: mcp.ConfigDirsFromEnv()}
	hooksService := &hooks.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir(), ConfigDirs: hooks.ConfigDirsFromEnv()}
	pluginService := &plugin.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir()}
	if projectRoot != "" {
		projectConfig := config.ProjectConfigPath(projectRoot)
		mcpService.ConfigPath, mcpService.ProjectRoot = projectConfig, projectRoot
		hooksService.ConfigPath, hooksService.ProjectRoot = projectConfig, projectRoot
		pluginService.ConfigPath, pluginService.ProjectRoot = projectConfig, projectRoot
	} else {
		hooksService.Accounts = hooksAccounts(cfg)
		pluginService.Accounts = pluginAccounts(cfg)
	}
	checkMCPServers(mcpService, result)
	checkHooksEntries(hooksService, result)
	checkPluginPackages(pluginService, result)
}

// addResourceCheck records one check whose status follows its worst finding.
func addResourceCheck(result *doctorResult, name, label, ok string, errors, warnings []string) {
	for range errors {
		result.addError()
	}
	for range warnings {
		result.addWarning()
	}
	details := append(append([]string{}, errors...), warnings...)
	switch {
	case len(errors) > 0:
		result.addCheck(name, checkError, fmt.Sprintf("%s: %d error(s), %d warning(s)", label, len(errors), len(warnings)), details)
	case len(warnings) > 0:
		result.addCheck(name, checkWarning, fmt.Sprintf("%s: %d warning(s)", label, len(warnings)), details)
	default:
		result.addCheck(name, checkPass, ok, nil)
	}
}

func printResourceFindings(errors, warnings []string) {
	for _, e := range errors {
		ui.Error("%s", e)
	}
	for _, w := range warnings {
		ui.Warning("%s", w)
	}
}

// checkMCPServers runs the static part of `mcp check`: no server is started and no
// host is resolved, so doctor stays offline apart from its version check.
func checkMCPServers(service *mcp.Service, result *doctorResult) {
	ui.Header("MCP")
	report, err := service.Check(mcp.CheckOptions{SkipDNS: true, ClientVersion: version})
	if err != nil {
		ui.Error("MCP: %v", err)
		addResourceCheck(result, "mcp", "MCP", "", []string{err.Error()}, nil)
		return
	}
	if len(report.Servers) == 0 {
		ui.Info("MCP: no servers configured")
		result.addInfo("mcp", "No MCP servers configured")
		return
	}
	var errors, warnings []string
	for _, server := range report.Servers {
		name := server.Name
		if server.Project != "" {
			name += " (project " + shortenPath(server.Project) + ")"
		}
		for _, f := range server.Findings {
			line := name + ": " + f.Message
			if f.Target != "" {
				line = name + " → " + f.Target + ": " + f.Message
			}
			switch f.Level {
			case "error":
				errors = append(errors, line)
			case "warning":
				warnings = append(warnings, line)
			}
		}
	}
	ok := fmt.Sprintf("All %d MCP server(s) OK", len(report.Servers))
	if len(errors)+len(warnings) == 0 {
		ui.Success("%s", ok)
	}
	printResourceFindings(errors, warnings)
	if len(errors)+len(warnings) > 0 {
		ui.Info("  Run: skillshare mcp check for details")
	}
	addResourceCheck(result, "mcp", "MCP", ok, errors, warnings)
}

// checkHooksEntries reports what `hooks sync` would still change or refuse.
func checkHooksEntries(service *hooks.Service, result *doctorResult) {
	ui.Header("Hooks")
	inv, err := service.List()
	if err != nil {
		ui.Error("Hooks: %v", err)
		addResourceCheck(result, "hooks", "Hooks", "", []string{err.Error()}, nil)
		return
	}
	if len(inv.Source.Entries) == 0 && len(inv.Source.Projects) == 0 {
		ui.Info("Hooks: none configured")
		result.addInfo("hooks", "No hooks configured")
		return
	}
	var errors, warnings []string
	if inv.PreviewError != "" {
		errors = append(errors, "sync preview failed: "+inv.PreviewError)
	} else if inv.Plan != nil {
		for _, c := range inv.Plan.Changes {
			line := c.Name + " → " + c.Target
			switch c.Action {
			case "unchanged":
			case "conflict":
				warnings = append(warnings, line+": conflict: "+c.Message)
			default:
				warnings = append(warnings, line+": not synced ("+c.Action+")")
			}
		}
		warnings = append(warnings, inv.Plan.Warnings...)
	}
	ok := fmt.Sprintf("All %d hook(s) in sync", len(inv.Source.Entries))
	if len(errors)+len(warnings) == 0 {
		ui.Success("%s", ok)
	}
	printResourceFindings(errors, warnings)
	if len(warnings) > 0 {
		ui.Info("  Run: skillshare sync hooks")
	}
	addResourceCheck(result, "hooks", "Hooks", ok, errors, warnings)
}

// checkPluginPackages previews `plugin sync` without fetching any source. It only asks
// each bound Agent's native CLI what is installed, so nothing runs when no package exists.
func checkPluginPackages(service *plugin.Service, result *doctorResult) {
	ui.Header("Plugins")
	inv, err := service.Packages()
	if err != nil {
		ui.Error("Plugins: %v", err)
		addResourceCheck(result, "plugins", "Plugins", "", []string{err.Error()}, nil)
		return
	}
	if len(inv.Packages) == 0 {
		ui.Info("Plugins: none configured")
		result.addInfo("plugins", "No plugins configured")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorPluginTimeout)
	defer cancel()
	sp := ui.StartSpinner("Checking plugins...")
	plan, err := service.Preview(ctx, plugin.Request{Action: "sync"})
	sp.Stop()
	if err != nil {
		ui.Error("Plugins: %v", err)
		addResourceCheck(result, "plugins", "Plugins", "", []string{err.Error()}, nil)
		return
	}
	var warnings []string
	for _, c := range plan.Changes {
		line := c.Name + " → " + c.Target
		switch c.Action {
		case "noop", "skip":
		case "blocked":
			warnings = append(warnings, line+": blocked: "+c.Message)
		default:
			warnings = append(warnings, line+": not synced ("+c.Action+")")
		}
	}
	ok := fmt.Sprintf("All %d plugin package(s) in sync", len(inv.Packages))
	if len(warnings) == 0 {
		ui.Success("%s", ok)
	}
	printResourceFindings(nil, warnings)
	if len(warnings) > 0 {
		ui.Info("  Run: skillshare sync plugins --dry-run")
	}
	addResourceCheck(result, "plugins", "Plugins", ok, nil, warnings)
}

// extraTargetFindings checks one resolved extra target beyond reachability: links
// that no longer resolve, and files that differ from the source.
func extraTargetFindings(extra config.ExtraConfig, target config.ExtraTargetConfig, sourceDir, targetPath string) (errors, warnings []string) {
	label := extra.Name + " → " + target.Path
	if extra.File == "" {
		for _, name := range findBrokenSymlinks(targetPath) {
			errors = append(errors, label+": broken symlink "+name)
		}
	}
	// diff does not model flattened or transformed targets, so it cannot judge them.
	if target.Flatten || target.Extension != "" {
		return errors, warnings
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return errors, warnings
	}
	resolved := target
	resolved.Path = targetPath
	for _, d := range collectExtrasDiff([]config.ExtraConfig{{Name: extra.Name, File: extra.File, Targets: []config.ExtraTargetConfig{resolved}}}, func(config.ExtraConfig) string { return sourceDir }) {
		if !d.synced && d.errMsg == "" && len(d.items) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s: %d file(s) out of sync (%s %s)", label, len(d.items), d.items[0].file, d.items[0].reason))
		}
	}
	return errors, warnings
}

// resolveDoctorExtraTarget makes an extra target path absolute, as sync does.
func resolveDoctorExtraTarget(path string, isProject bool, projectRoot string) string {
	p := config.ExpandPath(path)
	if isProject && !filepath.IsAbs(p) {
		p = filepath.Join(projectRoot, p)
	}
	return p
}
