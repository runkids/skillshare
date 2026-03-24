package main

import (
	"encoding/json"
	"fmt"
	"os"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

type extrasListEntry struct {
	Name         string             `json:"name"`
	SourceDir    string             `json:"source_dir"`
	SourceType   string             `json:"source_type"`
	FileCount    int                `json:"file_count"`
	SourceExists bool               `json:"source_exists"`
	Targets      []extrasTargetInfo `json:"targets"`
}

type extrasTargetInfo struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Flatten bool   `json:"flatten"`
	Status  string `json:"status"` // "synced", "drift", "not synced", "no source"
}

// buildExtrasListEntries builds list entries for all configured extras.
func buildExtrasListEntries(extras []config.ExtraConfig, extrasSource string, sourceFunc func(extra config.ExtraConfig) string) []extrasListEntry {
	entries := make([]extrasListEntry, 0, len(extras))

	for _, extra := range extras {
		sourceDir := sourceFunc(extra)
		entry := extrasListEntry{
			Name:       extra.Name,
			SourceDir:  sourceDir,
			SourceType: config.ResolveExtrasSourceType(extra, extrasSource),
		}

		files, discoverErr := sync.DiscoverExtraFiles(sourceDir)
		if discoverErr != nil {
			entry.SourceExists = false
			entry.FileCount = 0
		} else {
			entry.SourceExists = true
			entry.FileCount = len(files)
		}

		for _, t := range extra.Targets {
			m := sync.EffectiveMode(t.Mode)
			resolvedPath := config.ExpandPath(t.Path)
			ti := extrasTargetInfo{
				Path:    t.Path,
				Mode:    m,
				Flatten: t.Flatten,
			}

			if !entry.SourceExists {
				ti.Status = "no source"
			} else if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
				ti.Status = "not synced"
			} else {
				ti.Status = sync.CheckSyncStatus(files, sourceDir, resolvedPath, m, t.Flatten)
			}

			entry.Targets = append(entry.Targets, ti)
		}

		entries = append(entries, entry)
	}

	return entries
}

func cmdExtrasList(args []string) error {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	jsonOutput := false
	noTUI := false
	for _, a := range rest {
		switch a {
		case "--json":
			jsonOutput = true
		case "--no-tui":
			noTUI = true
		case "--help", "-h":
			printExtrasListHelp()
			return nil
		}
	}

	var extras []config.ExtraConfig
	var sourceFunc func(extra config.ExtraConfig) string
	var cfg *config.Config
	var projCfg *config.ProjectConfig
	var configPath string
	var extrasSource string

	if mode == modeProject {
		projCfg, err = config.LoadProject(cwd)
		if err != nil {
			return err
		}
		extras = projCfg.Extras
		sourceFunc = func(extra config.ExtraConfig) string {
			return config.ExtrasSourceDirProject(cwd, extra.Name)
		}
		configPath = config.ProjectConfigPath(cwd)
	} else {
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		extras = cfg.Extras
		extrasSource = cfg.ExtrasSource
		sourceFunc = func(extra config.ExtraConfig) string {
			return config.ResolveExtrasSourceDir(extra, cfg.ExtrasSource, cfg.Source)
		}
		configPath = config.ConfigPath()
	}

	if jsonOutput {
		if len(extras) == 0 {
			fmt.Println("[]")
			return nil
		}
		entries := buildExtrasListEntries(extras, extrasSource, sourceFunc)
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// TUI dispatch
	if shouldLaunchTUI(noTUI, cfg) && len(extras) > 0 {
		modeLabel := "global"
		if mode == modeProject {
			modeLabel = "project"
		}
		loadFn := func() ([]extrasListEntry, error) {
			var ex []config.ExtraConfig
			var es string
			if mode == modeProject {
				p, loadErr := config.LoadProject(cwd)
				if loadErr != nil {
					return nil, loadErr
				}
				ex = p.Extras
			} else {
				c, loadErr := config.Load()
				if loadErr != nil {
					return nil, loadErr
				}
				ex = c.Extras
				es = c.ExtrasSource
			}
			return buildExtrasListEntries(ex, es, sourceFunc), nil
		}
		return runExtrasListTUI(loadFn, modeLabel, cfg, projCfg, cwd, configPath, sourceFunc)
	}

	if len(extras) == 0 {
		ui.Info("No extras configured.")
		ui.Info("Run 'skillshare extras init <name> --target <path>' to add one.")
		return nil
	}

	// Plain text output
	entries := buildExtrasListEntries(extras, extrasSource, sourceFunc)
	ui.Header(ui.WithModeLabel("Extras"))

	for i, entry := range entries {
		if i > 0 {
			fmt.Println()
		}
		if !entry.SourceExists {
			fmt.Printf("  %-12s %s\n", entry.Name, ui.Dim+"source not found"+ui.Reset)
		} else {
			fileLabel := fmt.Sprintf("%d files", entry.FileCount)
			if entry.FileCount == 1 {
				fileLabel = "1 file"
			}
			fmt.Printf("  %-12s %s (%s)\n", entry.Name, shortenPath(entry.SourceDir), fileLabel)
		}
		for _, t := range entry.Targets {
			var icon, color, statusText string
			switch t.Status {
			case "synced":
				icon, color = "✓", ui.Green
			case "drift":
				icon, color, statusText = "△", ui.Yellow, "  drift"
			case "not synced":
				icon, color, statusText = "✗", ui.Yellow, "  not synced"
			case "no source":
				icon, color, statusText = "-", ui.Cyan, "  no source"
			}
			modeLabel := t.Mode
			if t.Flatten {
				modeLabel += ", flatten"
			}
			fmt.Printf("    %s%s%s %s%s (%s)\n", color, icon, ui.Reset, shortenPath(t.Path), statusText, modeLabel)
		}
	}

	return nil
}

func printExtrasListHelp() {
	fmt.Println(`Usage: skillshare extras list [options]

List all configured extras and their sync status.

Options:
  --json               JSON output
  --no-tui             Disable interactive TUI, use plain text output
  --project, -p        Use project-mode extras (.skillshare/)
  --global, -g         Use global extras (~/.config/skillshare/)
  --help, -h           Show this help`)
}
