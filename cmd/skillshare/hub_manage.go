package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

func cmdHubAdd(args []string, mode runMode, cwd string) error {
	var hubURL string
	var label string

	i := 0
	for i < len(args) {
		arg := args[i]
		key, val, hasEq := strings.Cut(arg, "=")
		switch {
		case key == "--label" || key == "-l":
			if hasEq {
				label = strings.TrimSpace(val)
			} else if i+1 >= len(args) {
				return fmt.Errorf("--label requires a value")
			} else {
				i++
				label = strings.TrimSpace(args[i])
			}
		case key == "--help" || key == "-h":
			printHubAddHelp()
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			if hubURL != "" {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
			hubURL = strings.TrimSpace(arg)
		}
		i++
	}

	if hubURL == "" {
		return fmt.Errorf("usage: skillshare hub add <url> [--label name]")
	}

	// Derive label from URL if not provided
	if label == "" {
		label = deriveLabelFromURL(hubURL)
	}

	hubCfg, saveFn, err := loadHubConfigForWrite(mode, cwd)
	if err != nil {
		return err
	}

	if err := hubCfg.AddHub(config.HubEntry{Label: label, URL: hubURL}); err != nil {
		return err
	}

	// First add auto-sets default
	if len(hubCfg.Hubs) == 1 {
		hubCfg.Default = label
	}

	if err := saveFn(*hubCfg); err != nil {
		return err
	}

	ui.Done(ui.MarkOK, fmt.Sprintf("Added hub %q → %s", label, hubURL), 0)
	if hubCfg.Default == label {
		ui.Note("Set as the default hub")
	}
	return nil
}

func cmdHubList(mode runMode, cwd string) error {
	hubCfg, err := loadHubConfigForRead(mode, cwd)
	if err != nil {
		return err
	}

	if len(hubCfg.Hubs) == 0 {
		ui.Done(ui.MarkNone, "No saved hubs", 0)
		ui.Next("skillshare hub add <url>", "add one")
		return nil
	}

	labels := make([]string, len(hubCfg.Hubs))
	for i, h := range hubCfg.Hubs {
		labels[i] = h.Label
	}
	width := ui.RowWidth(labels...)
	for _, h := range hubCfg.Hubs {
		var notes []string
		if strings.EqualFold(h.Label, hubCfg.Default) {
			notes = append(notes, "default")
		}
		if h.BuiltIn {
			notes = append(notes, "built-in")
		}
		value := h.URL
		if len(notes) > 0 {
			value += ui.DimText(" · " + strings.Join(notes, ", "))
		}
		ui.Row(ui.MarkNone, h.Label, value, width)
	}
	fmt.Println()
	ui.Done(ui.MarkNone, plural(len(hubCfg.Hubs), "hub"), 0)
	return nil
}

func cmdHubRemove(args []string, mode runMode, cwd string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: skillshare hub remove <label>")
	}
	label := strings.TrimSpace(args[0])

	hubCfg, saveFn, err := loadHubConfigForWrite(mode, cwd)
	if err != nil {
		return err
	}

	if err := hubCfg.RemoveHub(label); err != nil {
		return err
	}

	if err := saveFn(*hubCfg); err != nil {
		return err
	}

	ui.Done(ui.MarkOK, fmt.Sprintf("Removed hub %q", label), 0)
	return nil
}

func cmdHubDefault(args []string, mode runMode, cwd string) error {
	var reset bool
	var label string

	for _, arg := range args {
		switch arg {
		case "--reset":
			reset = true
		case "--help", "-h":
			printHubDefaultHelp()
			return nil
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option: %s", arg)
			}
			label = strings.TrimSpace(arg)
		}
	}

	hubCfg, saveFn, err := loadHubConfigForWrite(mode, cwd)
	if err != nil {
		return err
	}

	// Show current default
	if label == "" && !reset {
		if hubCfg.Default == "" {
			ui.Done(ui.MarkNone, "No default hub set (using the community hub)", 0)
		} else {
			url, ok := hubCfg.ResolveHub(hubCfg.Default)
			if ok {
				ui.Row(ui.MarkNone, hubCfg.Default, url, ui.RowWidth(hubCfg.Default))
			} else {
				ui.Row(ui.MarkWarn, hubCfg.Default, "label not found in hubs", ui.RowWidth(hubCfg.Default))
			}
		}
		return nil
	}

	// Reset default
	if reset {
		hubCfg.Default = ""
		if err := saveFn(*hubCfg); err != nil {
			return err
		}
		ui.Done(ui.MarkOK, "Default hub cleared (using the community hub)", 0)
		return nil
	}

	// Set default
	if !hubCfg.HasHub(label) {
		return fmt.Errorf("hub %q not found; run 'skillshare hub list' to see available hubs", label)
	}
	hubCfg.Default = label
	if err := saveFn(*hubCfg); err != nil {
		return err
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Default hub set to %q", label), 0)
	return nil
}

// loadHubConfigForRead loads the HubConfig from the appropriate config.
func loadHubConfigForRead(mode runMode, cwd string) (*config.HubConfig, error) {
	if mode == modeProject {
		pcfg, err := config.LoadProject(cwd)
		if err != nil {
			return &config.HubConfig{}, nil // graceful fallback
		}
		return &pcfg.Hub, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return &config.HubConfig{}, nil // graceful fallback
	}
	return &cfg.Hub, nil
}

// loadHubConfigForWrite loads the HubConfig and returns a save function.
func loadHubConfigForWrite(mode runMode, cwd string) (*config.HubConfig, func(config.HubConfig) error, error) {
	if mode == modeProject {
		pcfg, err := config.LoadProject(cwd)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load project config: %w", err)
		}
		saveFn := func(h config.HubConfig) error {
			pcfg.Hub = h
			return pcfg.Save(cwd)
		}
		return &pcfg.Hub, saveFn, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	saveFn := func(h config.HubConfig) error {
		cfg.Hub = h
		return cfg.Save()
	}
	return &cfg.Hub, saveFn, nil
}

// deriveLabelFromURL extracts a short label from a URL or path.
func deriveLabelFromURL(rawURL string) string {
	// Try parsing as URL
	if u, err := url.Parse(rawURL); err == nil && u.Host != "" {
		// Use last meaningful path segment
		p := strings.TrimSuffix(u.Path, "/")
		base := filepath.Base(p)
		base = strings.TrimSuffix(base, filepath.Ext(base))
		if base != "" && base != "." {
			return base
		}
		return u.Host
	}
	// File path: use filename without extension
	base := filepath.Base(rawURL)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base != "" && base != "." {
		return base
	}
	return rawURL
}

func printHubAddHelp() {
	fmt.Println(`Usage: skillshare hub add <url> [options]

Add a hub source to your saved hubs list.

Options:
  --label, -l <name>   Label for the hub (default: derived from URL)
  --project, -p        Use project mode
  --global, -g         Use global mode
  --help, -h           Show this help

Examples:
  skillshare hub add https://internal.corp/skills/hub.json
  skillshare hub add https://internal.corp/hub.json --label team
  skillshare hub add ./skillshare-hub.json --label local`)
}

func printHubDefaultHelp() {
	fmt.Println(`Usage: skillshare hub default [label] [options]

Show or set the default hub. When set, 'search --hub' uses this hub
instead of the community hub.

Options:
  --reset              Clear default (revert to community hub)
  --project, -p        Use project mode
  --global, -g         Use global mode
  --help, -h           Show this help

Examples:
  skillshare hub default                Show current default
  skillshare hub default team           Set default to "team"
  skillshare hub default --reset        Clear default`)
}
