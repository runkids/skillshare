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
	Path      string `json:"path"`
	Mode      string `json:"mode"`
	Flatten   bool   `json:"flatten"`
	Extension string `json:"extension,omitempty"` // transform extension name, if any
	Status    string `json:"status"`              // "synced", "drift", "not synced", "no source"
}

// buildExtrasListEntries builds list entries for all configured extras.
// extensionsDir resolves transform extensions (for output_ext-aware status).
func buildExtrasListEntries(extras []config.ExtraConfig, extrasSource, extensionsDir string, sourceFunc func(extra config.ExtraConfig) string) []extrasListEntry {
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
				Path:      t.Path,
				Mode:      m,
				Flatten:   t.Flatten,
				Extension: t.Extension,
			}

			// Transform targets emit generated files via copy semantics; resolve
			// the extension's output_ext so status comparison renames correctly.
			outputExt := ""
			if t.Extension != "" {
				m = "copy"
				if spec, rerr := resolveExtension(t.Extension, extensionsDir); rerr == nil && spec != nil {
					outputExt = spec.OutputExt
				} else if rerr != nil {
					// A misconfigured extension would otherwise be silent: status
					// falls back to comparing the original .md files, reporting
					// false drift. Surface it so the cause is visible.
					fmt.Fprintf(os.Stderr, "warning: extension %q for extra %q could not be resolved (%v); sync status may be inaccurate\n", t.Extension, extra.Name, rerr)
				}
			}

			if !entry.SourceExists {
				ti.Status = "no source"
			} else if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
				ti.Status = "not synced"
			} else {
				ti.Status = sync.CheckSyncStatus(files, sourceDir, resolvedPath, m, t.Flatten, outputExt)
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

	extensionsDir := globalExtensionsDir()
	if mode == modeProject {
		extensionsDir = projectExtensionsDir(cwd)
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
			return config.ExtrasSourceDirProject(projCfg.EffectiveExtrasSource(cwd), extra.Name)
		}
		configPath = config.ProjectConfigPath(cwd)
	} else {
		cfg, err = config.Load()
		if err != nil {
			return err
		}
		extras = cfg.Extras
		// extrasSource holds the user's explicit configuration (legacy
		// extras_source or new sources.extras); used by ResolveExtrasSourceType
		// to distinguish "default" (derived) from "extras_source" (configured).
		extrasSource = cfg.ExtrasSource
		if extrasSource == "" {
			extrasSource = cfg.Sources.Extras
		}
		sourceFunc = func(extra config.ExtraConfig) string {
			return config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource())
		}
		configPath = config.ConfigPath()
	}

	if jsonOutput {
		if len(extras) == 0 {
			fmt.Println("[]")
			return nil
		}
		entries := buildExtrasListEntries(extras, extrasSource, extensionsDir, sourceFunc)
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
				if es == "" {
					es = c.Sources.Extras
				}
			}
			return buildExtrasListEntries(ex, es, extensionsDir, sourceFunc), nil
		}
		return runExtrasListTUI(loadFn, modeLabel, cfg, projCfg, cwd, configPath, sourceFunc)
	}

	if len(extras) == 0 {
		ui.Info("No extras configured.")
		ui.Info("Run 'skillshare extras init <name> --target <path>' to add one.")
		return nil
	}

	// Plain text output
	entries := buildExtrasListEntries(extras, extrasSource, extensionsDir, sourceFunc)
	ui.Header(ui.WithModeLabel("Extras"))

	for i, entry := range entries {
		if i > 0 {
			fmt.Println()
		}
		if !entry.SourceExists {
			fmt.Printf("%s→%s %s  %s\n", ui.Cyan, ui.Reset, entry.Name, ui.Dim+"source not found"+ui.Reset)
		} else {
			fileLabel := fmt.Sprintf("%d files", entry.FileCount)
			if entry.FileCount == 1 {
				fileLabel = "1 file"
			}
			fmt.Printf("%s→%s %s  %s%s · %s%s\n", ui.Cyan, ui.Reset, entry.Name, ui.Dim, shortenPath(entry.SourceDir), fileLabel, ui.Reset)
		}
		for _, t := range entry.Targets {
			var icon, color string
			switch t.Status {
			case "synced":
				icon, color = "✓", ui.Green
			case "drift":
				icon, color = "!", ui.Yellow
			case "not synced":
				icon, color = "✗", ui.Yellow
			case "no source":
				icon, color = "-", ui.Cyan
			}
			var modeLabel string
			if t.Extension != "" {
				modeLabel = "extension: " + t.Extension
			} else {
				modeLabel = t.Mode
				if t.Flatten {
					modeLabel += ", flatten"
				}
			}
			// Status text after mode, dimmed
			statusSuffix := ""
			if t.Status != "synced" {
				statusSuffix = fmt.Sprintf("  %s%s%s", color, t.Status, ui.Reset)
			}
			fmt.Printf("  %s%s%s %s  %s%s%s%s\n", color, icon, ui.Reset, shortenPath(t.Path), ui.Dim, modeLabel, ui.Reset, statusSuffix)
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
