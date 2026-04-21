package main

import (
	"encoding/json"
	"fmt"
	"os"

	"skillshare/internal/config"
	hookpkg "skillshare/internal/hooks"
	"skillshare/internal/ui"
)

func cmdHooks(args []string) error {
	if len(args) == 0 {
		printHooksHelp()
		return nil
	}
	switch args[0] {
	case "list", "ls":
		return cmdHooksList(args[1:])
	case "sync":
		return cmdHooksSync(args[1:])
	case "import":
		return cmdHooksImport(args[1:])
	case "--help", "-h":
		printHooksHelp()
		return nil
	default:
		return fmt.Errorf("unknown hooks subcommand: %s", args[0])
	}
}

func cmdHooksList(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	jsonOutput := hasFlag(rest, "--json")
	sourceRoot, _, err := hookRoots(mode)
	if err != nil {
		return err
	}
	bundles, err := hookpkg.Discover(sourceRoot)
	if err != nil {
		return err
	}
	if jsonOutput {
		data, _ := json.MarshalIndent(bundles, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	if len(bundles) == 0 {
		ui.Info("No hooks found in %s", shortenPath(sourceRoot))
		return nil
	}
	ui.Header(ui.WithModeLabel("Hooks"))
	for _, bundle := range bundles {
		ui.Info("%s  claude=%d codex=%d", bundle.Name, bundle.Targets["claude"], bundle.Targets["codex"])
	}
	return nil
}

func cmdHooksImport(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	from := ""
	all := false
	ownedOnly := false
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--from":
			if i+1 >= len(rest) {
				return fmt.Errorf("--from requires a value")
			}
			from = rest[i+1]
			i++
		case "--all":
			all = true
		case "--owned-only":
			ownedOnly = true
		}
	}
	if from == "" {
		return fmt.Errorf("usage: skillshare hooks import --from claude|codex [--all|--owned-only]")
	}
	sourceRoot, projectRoot, err := hookRoots(mode)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sourceRoot, 0755); err != nil {
		return err
	}
	bundles, err := hookpkg.Import(sourceRoot, hookpkg.ImportOptions{
		From:      from,
		Project:   projectRoot,
		All:       all,
		OwnedOnly: ownedOnly,
	})
	if err != nil {
		return err
	}
	ui.Success("Imported %d hook bundle(s)", len(bundles))
	return nil
}

func cmdHooksSync(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}
	target := "all"
	jsonOutput := hasFlag(rest, "--json")
	var names []string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--target":
			if i+1 < len(rest) {
				target = rest[i+1]
				i++
			}
		case "--json":
		default:
			names = append(names, rest[i])
		}
	}
	sourceRoot, projectRoot, err := hookRoots(mode)
	if err != nil {
		return err
	}
	var results []hookpkg.SyncResult
	if len(names) > 0 {
		want := map[string]bool{}
		for _, name := range names {
			want[name] = true
		}
		bundles, err := hookpkg.Discover(sourceRoot)
		if err != nil {
			return err
		}
		for _, bundle := range bundles {
			if !want[bundle.Name] {
				continue
			}
			for _, one := range []string{target} {
				if target == "" || target == "all" {
					for _, expanded := range []string{"claude", "codex"} {
						res, err := hookpkg.SyncBundle(bundle, projectRoot, expanded)
						if err != nil {
							return err
						}
						results = append(results, res)
					}
					continue
				}
				res, err := hookpkg.SyncBundle(bundle, projectRoot, one)
				if err != nil {
					return err
				}
				results = append(results, res)
			}
		}
	} else {
		results, err = hookpkg.SyncAll(sourceRoot, projectRoot, target)
		if err != nil {
			return err
		}
	}
	if jsonOutput {
		return writeJSON(map[string]any{"hooks": results})
	}
	ui.Header(ui.WithModeLabel("Syncing hooks"))
	for _, res := range results {
		if res.Root == "" {
			for _, warning := range res.Warnings {
				ui.Info("%s: %s", res.Name, warning)
			}
			continue
		}
		ui.Success("%s -> %s", res.Name, shortenPath(res.Root))
		for _, warning := range res.Warnings {
			ui.Info("  %s", warning)
		}
	}
	return nil
}

func printHooksHelp() {
	fmt.Println(`Usage: skillshare hooks <command> [options]

Commands:
  list [--json]
  import --from claude|codex [--all|--owned-only]
  sync [name...] --target claude|codex|all [--json]`)
}

func hookRoots(mode runMode) (sourceRoot, projectRoot string, err error) {
	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}
	if mode == modeProject {
		return config.HooksSourceDirProject(cwd), cwd, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", "", err
	}
	return cfg.EffectiveHooksSource(), "", nil
}
