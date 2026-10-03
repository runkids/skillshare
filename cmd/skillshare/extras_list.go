package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

type extrasListEntry struct {
	Name         string             `json:"name"`
	File         string             `json:"file,omitempty"` // single-file extra: the synced file in source_dir
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
	As        string `json:"as,omitempty"`        // single-file extra: target filename
	Status    string `json:"status"`              // "synced", "drift", "modified", "not synced", "no source"
}

// buildExtrasListEntries builds list entries for all configured extras.
// extensionsDir resolves transform extensions (for output_ext-aware status).
func buildExtrasListEntries(extras []config.ExtraConfig, extrasSource, extensionsDir string, sourceFunc func(extra config.ExtraConfig) string, projectRoot string) []extrasListEntry {
	entries := make([]extrasListEntry, 0, len(extras))

	for _, extra := range extras {
		sourceDir := sourceFunc(extra)
		entry := extrasListEntry{
			Name:       extra.Name,
			File:       extra.File,
			SourceDir:  sourceDir,
			SourceType: config.ResolveExtrasSourceType(extra, extrasSource),
		}

		files, discoverErr := sync.DiscoverExtraSource(sourceDir, extra.File)
		if discoverErr != nil {
			entry.SourceExists = false
			entry.FileCount = 0
		} else {
			entry.SourceExists = true
			entry.FileCount = len(files)
		}

		for _, t := range extra.Targets {
			m := sync.ExtraTargetMode(t.Mode, extra.File != "")
			resolvedPath := config.ExpandPath(t.Path)
			if projectRoot != "" && !filepath.IsAbs(resolvedPath) {
				resolvedPath = filepath.Join(projectRoot, resolvedPath)
			}
			ti := extrasTargetInfo{
				Path:      t.Path,
				Mode:      m,
				Flatten:   t.Flatten,
				Extension: t.Extension,
				As:        t.As,
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

			if extra.File != "" {
				ti.Status = sync.ExtraFileStatus(sync.NewExtraFile(sourceDir, extra.File, resolvedPath, t.As, m))
			} else if !entry.SourceExists {
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
	mode = resolveAutoMode(mode, cwd)

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
			return config.ResolveExtrasSourceDirProject(extra, projCfg.EffectiveExtrasSource(cwd), cwd)
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

	root := ""
	if mode == modeProject {
		root = cwd
	}

	if jsonOutput {
		if len(extras) == 0 {
			fmt.Println("[]")
			return nil
		}
		entries := buildExtrasListEntries(extras, extrasSource, extensionsDir, sourceFunc, root)
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
			return buildExtrasListEntries(ex, es, extensionsDir, sourceFunc, root), nil
		}
		return runExtrasListTUI(loadFn, modeLabel, cfg, projCfg, cwd, configPath, sourceFunc)
	}

	if len(extras) == 0 {
		ui.Done(ui.MarkNone, "No extras configured", 0)
		ui.Next("skillshare extras init <name> --target <path>", "add one")
		return nil
	}

	// Plain text output
	entries := buildExtrasListEntries(extras, extrasSource, extensionsDir, sourceFunc, root)
	pending := false
	for i, entry := range entries {
		if i > 0 {
			fmt.Println()
		}
		name := theme.Primary().Bold(true).Render(entry.Name)
		if entry.File != "" {
			// Single-file extra: the source is one file, shown in full.
			src := shortenPath(filepath.Join(entry.SourceDir, entry.File))
			if !entry.SourceExists {
				src += " · source not found"
			}
			fmt.Println(name + "  " + ui.DimText(src))
		} else if !entry.SourceExists {
			fmt.Println(name + "  " + ui.DimText("source not found"))
		} else {
			fmt.Println(name + "  " + ui.DimText(shortenPath(entry.SourceDir)+" · "+plural(entry.FileCount, "file")))
		}
		paths := make([]string, len(entry.Targets))
		for j, t := range entry.Targets {
			paths[j] = shortenPath(extrasTargetDisplayPath(entry.File, t))
		}
		width := ui.RowWidth(paths...)
		for j, t := range entry.Targets {
			mark := ui.MarkNone
			switch t.Status {
			case "synced":
				mark = ui.MarkOK
			case "drift", "invalid mode", "not synced":
				mark = ui.MarkWarn
				pending = true
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
			value := ui.DimText(modeLabel)
			if t.Status != "synced" {
				value = t.Status + "  " + value
			}
			ui.Row(mark, paths[j], value, width)
		}
	}

	fmt.Println()
	ui.Done(ui.MarkNone, plural(len(entries), "extra"), 0)
	if pending {
		ui.Next("skillshare sync extras"+projectSuffix(mode), "bring the Targets up to date")
	}
	return nil
}

func printExtrasListHelp() {
	printHelp("skillshare extras list [options]", "List all configured extras and their sync status.",
		helpGroup{title: "Options", rows: []helpRow{
			{"--json", "JSON output"},
			{"--no-tui", "Disable interactive TUI, use plain text output"},
			{"-p, --project", "Use project-mode extras (.skillshare/)"},
			{"-g, --global", "Use global extras (~/.config/skillshare/)"},
		}},
	)
}
