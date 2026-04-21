package main

import (
	"encoding/json"
	"fmt"
	"os"

	"skillshare/internal/config"
	pluginpkg "skillshare/internal/plugins"
	"skillshare/internal/ui"
)

func cmdPlugins(args []string) error {
	if len(args) == 0 {
		printPluginsHelp()
		return nil
	}
	switch args[0] {
	case "list", "ls":
		return cmdPluginsList(args[1:])
	case "import":
		return cmdPluginsImport(args[1:])
	case "sync":
		return cmdPluginsSync(args[1:])
	case "install":
		return cmdPluginsInstall(args[1:])
	case "--help", "-h":
		printPluginsHelp()
		return nil
	default:
		return fmt.Errorf("unknown plugins subcommand: %s", args[0])
	}
}

func cmdPluginsList(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	jsonOutput := hasFlag(rest, "--json")
	sourceRoot, _, err := pluginRoots(mode)
	if err != nil {
		return err
	}
	bundles, err := pluginpkg.Discover(sourceRoot)
	if err != nil {
		return err
	}
	if jsonOutput {
		data, _ := json.MarshalIndent(bundles, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if len(bundles) == 0 {
		ui.Info("No plugins found in %s", shortenPath(sourceRoot))
		return nil
	}
	ui.Header(ui.WithModeLabel("Plugins"))
	for _, bundle := range bundles {
		ui.Info("%s  claude=%t codex=%t", bundle.Name, bundle.HasClaude, bundle.HasCodex)
	}
	return nil
}

func cmdPluginsImport(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	from := ""
	filtered := make([]string, 0, len(rest))
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--from":
			if i+1 >= len(rest) {
				return fmt.Errorf("--from requires a value")
			}
			from = rest[i+1]
			i++
		default:
			filtered = append(filtered, rest[i])
		}
	}
	rest = filtered
	if len(rest) == 0 {
		return fmt.Errorf("usage: skillshare plugins import <plugin-ref-or-path> --from claude|codex")
	}
	sourceRoot, _, err := pluginRoots(mode)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sourceRoot, 0755); err != nil {
		return err
	}
	bundle, err := pluginpkg.Import(sourceRoot, rest[0], pluginpkg.ImportOptions{From: from})
	if err != nil {
		return err
	}
	ui.Success("Imported plugin %s into %s", bundle.Name, shortenPath(sourceRoot))
	return nil
}

func cmdPluginsSync(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	target := "all"
	install := true
	jsonOutput := hasFlag(rest, "--json")
	var names []string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--target":
			if i+1 >= len(rest) {
				return fmt.Errorf("--target requires a value")
			}
			target = rest[i+1]
			i++
		case "--no-install":
			install = false
		case "--json":
		default:
			names = append(names, rest[i])
		}
	}
	sourceRoot, projectRoot, err := pluginRoots(mode)
	if err != nil {
		return err
	}
	if len(names) > 0 {
		want := map[string]bool{}
		for _, name := range names {
			want[name] = true
		}
		bundles, err := pluginpkg.Discover(sourceRoot)
		if err != nil {
			return err
		}
		var results []pluginpkg.SyncResult
		for _, bundle := range bundles {
			if !want[bundle.Name] {
				continue
			}
			for _, one := range []string{target} {
				if target == "" || target == "all" {
					for _, expanded := range []string{"claude", "codex"} {
						res, err := pluginpkg.SyncBundle(bundle, projectRoot, expanded, install)
						if err != nil {
							return err
						}
						results = append(results, res)
					}
					continue
				}
				res, err := pluginpkg.SyncBundle(bundle, projectRoot, one, install)
				if err != nil {
					return err
				}
				results = append(results, res)
			}
		}
		return renderPluginSyncResults(results, jsonOutput)
	}
	results, err := pluginpkg.SyncAll(sourceRoot, projectRoot, target, install)
	if err != nil {
		return err
	}
	return renderPluginSyncResults(results, jsonOutput)
}

func cmdPluginsInstall(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	from := ""
	filtered := make([]string, 0, len(rest))
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--from":
			if i+1 >= len(rest) {
				return fmt.Errorf("--from requires a value")
			}
			from = rest[i+1]
			i++
		default:
			filtered = append(filtered, rest[i])
		}
	}
	rest = filtered
	if len(rest) == 0 {
		return fmt.Errorf("usage: skillshare plugins install <plugin-ref-or-path> --from claude|codex [--target claude|codex|all]")
	}
	sourceRoot, projectRoot, err := pluginRoots(mode)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sourceRoot, 0755); err != nil {
		return err
	}
	bundle, err := pluginpkg.Import(sourceRoot, rest[0], pluginpkg.ImportOptions{From: from})
	if err != nil {
		return err
	}
	target := "all"
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--target" && i+1 < len(rest) {
			target = rest[i+1]
			i++
		}
	}
	_, err = pluginpkg.SyncAll(sourceRoot, projectRoot, target, true)
	if err != nil {
		return err
	}
	ui.Success("Installed plugin %s", bundle.Name)
	return nil
}

func printPluginsHelp() {
	fmt.Println(`Usage: skillshare plugins <command> [options]

Commands:
  list [--json]
  import <plugin-ref-or-path> --from claude|codex
  sync [--target claude|codex|all] [--no-install] [--json]
  install <plugin-ref-or-path> --from claude|codex [--target claude|codex|all]`)
}

func renderPluginSyncResults(results []pluginpkg.SyncResult, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(map[string]any{"plugins": results})
	}
	ui.Header(ui.WithModeLabel("Syncing plugins"))
	for _, res := range results {
		ui.Success("%s -> %s", res.Name, shortenPath(res.Rendered))
		for _, warning := range res.Warnings {
			ui.Info("  %s", warning)
		}
	}
	return nil
}

func pluginRoots(mode runMode) (sourceRoot, projectRoot string, err error) {
	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}
	if mode == modeProject {
		return config.PluginsSourceDirProject(cwd), cwd, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", "", err
	}
	return cfg.EffectivePluginsSource(), "", nil
}
