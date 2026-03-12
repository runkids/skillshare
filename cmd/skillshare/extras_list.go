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
	FileCount    int                `json:"file_count"`
	SourceExists bool               `json:"source_exists"`
	Targets      []extrasTargetInfo `json:"targets"`
}

type extrasTargetInfo struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Status string `json:"status"` // "synced", "drift", "not synced", "no source"
}

// buildExtrasListEntries builds list entries for all configured extras.
func buildExtrasListEntries(extras []config.ExtraConfig, sourceFunc func(name string) string) []extrasListEntry {
	entries := make([]extrasListEntry, 0, len(extras))

	for _, extra := range extras {
		sourceDir := sourceFunc(extra.Name)
		entry := extrasListEntry{
			Name:      extra.Name,
			SourceDir: sourceDir,
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
				Path: t.Path,
				Mode: m,
			}

			if !entry.SourceExists {
				ti.Status = "no source"
			} else if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
				ti.Status = "not synced"
			} else {
				ti.Status = sync.CheckSyncStatus(files, sourceDir, resolvedPath, m)
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

	// Check for --json flag
	jsonOutput := false
	for _, a := range rest {
		if a == "--json" {
			jsonOutput = true
		}
	}

	var extras []config.ExtraConfig
	var sourceFunc func(name string) string

	if mode == modeProject {
		projCfg, err := config.LoadProject(cwd)
		if err != nil {
			return err
		}
		extras = projCfg.Extras
		sourceFunc = func(name string) string {
			return config.ExtrasSourceDirProject(cwd, name)
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		extras = cfg.Extras
		sourceFunc = func(name string) string {
			return config.ExtrasSourceDir(cfg.Source, name)
		}
	}

	if len(extras) == 0 {
		if jsonOutput {
			fmt.Println("[]")
			return nil
		}
		ui.Info("No extras configured.")
		ui.Info("Run 'skillshare extras init <name> --target <path>' to add one.")
		return nil
	}

	entries := buildExtrasListEntries(extras, sourceFunc)

	if jsonOutput {
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	ui.Header(ui.WithModeLabel("Extras"))

	for i, entry := range entries {
		if i > 0 {
			fmt.Println()
		}

		// Name + source on same line
		if !entry.SourceExists {
			fmt.Printf("  %-12s %s\n", entry.Name, ui.Dim+"source not found"+ui.Reset)
		} else {
			fileLabel := fmt.Sprintf("%d files", entry.FileCount)
			if entry.FileCount == 1 {
				fileLabel = "1 file"
			}
			fmt.Printf("  %-12s %s (%s)\n", entry.Name, shortenPath(entry.SourceDir), fileLabel)
		}

		// Targets indented below
		for _, t := range entry.Targets {
			var icon, color, statusText string
			switch t.Status {
			case "synced":
				icon, color = "✓", ui.Green
			case "drift":
				icon, color, statusText = "~", ui.Yellow, "  drift"
			case "not synced":
				icon, color, statusText = "✗", ui.Yellow, "  not synced"
			case "no source":
				icon, color, statusText = "-", ui.Cyan, "  no source"
			}
			fmt.Printf("    %s%s%s %s%s (%s)\n", color, icon, ui.Reset, shortenPath(t.Path), statusText, t.Mode)
		}
	}

	return nil
}

