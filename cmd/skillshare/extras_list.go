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

	entries := make([]extrasListEntry, 0, len(extras))

	for _, extra := range extras {
		sourceDir := sourceFunc(extra.Name)
		entry := extrasListEntry{
			Name:      extra.Name,
			SourceDir: sourceDir,
		}

		// Check source
		files, discoverErr := sync.DiscoverExtraFiles(sourceDir)
		if discoverErr != nil {
			entry.SourceExists = false
			entry.FileCount = 0
		} else {
			entry.SourceExists = true
			entry.FileCount = len(files)
		}

		// Check each target
		for _, t := range extra.Targets {
			m := sync.EffectiveMode(t.Mode)
			ti := extrasTargetInfo{
				Path: t.Path,
				Mode: m,
			}

			if !entry.SourceExists {
				ti.Status = "no source"
			} else if _, err := os.Stat(t.Path); os.IsNotExist(err) {
				ti.Status = "not synced"
			} else {
				ti.Status = sync.CheckSyncStatus(files, sourceDir, t.Path, m)
			}

			entry.Targets = append(entry.Targets, ti)
		}

		entries = append(entries, entry)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	for _, entry := range entries {
		ui.Header(capitalize(entry.Name))
		if !entry.SourceExists {
			ui.Warning("  Source: not found (%s)", shortenPath(entry.SourceDir))
		} else {
			ui.Success("  Source: %s (%d files)", shortenPath(entry.SourceDir), entry.FileCount)
		}

		for _, t := range entry.Targets {
			statusIcon := "✓"
			printFn := ui.Success
			switch t.Status {
			case "drift":
				statusIcon = "~"
				printFn = ui.Warning
			case "not synced":
				statusIcon = "✗"
				printFn = ui.Warning
			case "no source":
				statusIcon = "-"
				printFn = ui.Info
			}
			printFn("  %s %s  %s (%s)", statusIcon, shortenPath(t.Path), t.Status, t.Mode)
		}
		fmt.Println()
	}

	return nil
}

