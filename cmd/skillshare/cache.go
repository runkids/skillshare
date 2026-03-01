package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/cache"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/uidist"
	versioncheck "skillshare/internal/version"

	"golang.org/x/term"
)

func cmdCache(args []string) error {
	if len(args) == 0 {
		// No subcommand: TUI (if TTY) or fallback to list
		if term.IsTerminal(int(os.Stdout.Fd())) {
			return cacheRunTUI()
		}
		return cacheList()
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "list", "ls":
		return cacheList()
	case "clean":
		return cacheClean(subArgs)
	case "--help", "-h", "help":
		printCacheHelp()
		return nil
	default:
		printCacheHelp()
		return fmt.Errorf("unknown subcommand: %s", sub)
	}
}

// cacheList prints a text summary of all discovery + UI cache entries.
func cacheList() error {
	cacheDir := config.CacheDir()
	items := cache.ListDiskCaches(cacheDir)

	// Discovery caches
	ui.Header("Discovery Cache")
	if len(items) == 0 {
		ui.Info("  (none)")
	} else {
		for _, item := range items {
			printDiscoveryCacheItem(item)
		}
	}
	fmt.Println()

	// UI cache
	ui.Header("UI Cache")
	uiVersions := listUIVersions(cacheDir)
	if len(uiVersions) == 0 {
		ui.Info("  (none)")
	} else {
		currentVer := versioncheck.Version
		for _, v := range uiVersions {
			label := ""
			if v.name == "v"+currentVer || v.name == currentVer {
				label = "  (current)"
			}
			ui.Info("  %s  %s%s", v.path, formatBytes(v.size), label)
		}
	}
	fmt.Println()

	// Summary
	var totalDiscoverySize int64
	for _, item := range items {
		totalDiscoverySize += item.Size
	}
	var totalUISize int64
	for _, v := range uiVersions {
		totalUISize += v.size
	}
	ui.Info("Total: %d discovery file(s) (%s) + %d UI version(s) (%s)",
		len(items), formatBytes(totalDiscoverySize),
		len(uiVersions), formatBytes(totalUISize))

	return nil
}

// printDiscoveryCacheItem renders a single discovery cache row.
func printDiscoveryCacheItem(item cache.CacheItem) {
	if item.Error != "" {
		ui.Warning("  %s  %s  corrupt", item.Path, formatBytes(item.Size))
		return
	}
	status := "valid"
	if item.Orphan {
		status = "orphan"
	}
	skillsLabel := fmt.Sprintf("%d skills", item.EntryCount)
	ui.Info("  %s  %s  %s  %s  %s", item.Path, skillsLabel, formatBytes(item.Size), status, item.RootDir)
}

// cacheClean removes discovery caches and UI caches.
func cacheClean(args []string) error {
	start := time.Now()
	orphanOnly := false
	yes := false

	for _, arg := range args {
		switch arg {
		case "--orphan":
			orphanOnly = true
		case "--yes", "-y":
			yes = true
		case "--help", "-h":
			printCacheHelp()
			return nil
		default:
			return fmt.Errorf("unknown option: %s", arg)
		}
	}

	cacheDir := config.CacheDir()
	items := cache.ListDiskCaches(cacheDir)
	uiVersions := listUIVersions(cacheDir)

	if orphanOnly {
		return cacheCleanOrphans(items, yes, start)
	}

	// Calculate totals
	totalItems := len(items) + len(uiVersions)
	if totalItems == 0 {
		ui.Info("No cache files to clean")
		return nil
	}

	var totalSize int64
	for _, item := range items {
		totalSize += item.Size
	}
	for _, v := range uiVersions {
		totalSize += v.size
	}

	// Confirm
	if !yes && term.IsTerminal(int(os.Stdout.Fd())) {
		ui.Warning("Remove all cache files? (%d items, %s)", totalItems, formatBytes(totalSize))
		fmt.Print("Continue? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	// Remove discovery caches
	gobRemoved, err := cache.ClearAllDiskCaches(cacheDir)
	if err != nil {
		logCacheOp(config.ConfigPath(), "clean", gobRemoved, start, err)
		return err
	}

	// Remove UI cache
	uiErr := uidist.ClearCache()
	if uiErr != nil {
		logCacheOp(config.ConfigPath(), "clean", gobRemoved, start, uiErr)
		return fmt.Errorf("remove UI cache: %w", uiErr)
	}

	totalRemoved := gobRemoved + len(uiVersions)
	ui.Success("Removed %d cache items (%s freed)", totalRemoved, formatBytes(totalSize))
	logCacheOp(config.ConfigPath(), "clean", totalRemoved, start, nil)
	return nil
}

// cacheCleanOrphans removes only orphan discovery gob files.
func cacheCleanOrphans(items []cache.CacheItem, yes bool, start time.Time) error {
	var orphans []cache.CacheItem
	for _, item := range items {
		if item.Orphan {
			orphans = append(orphans, item)
		}
	}

	if len(orphans) == 0 {
		ui.Info("No orphan cache files found")
		return nil
	}

	var totalSize int64
	for _, item := range orphans {
		totalSize += item.Size
	}

	// Confirm
	if !yes && term.IsTerminal(int(os.Stdout.Fd())) {
		ui.Warning("Remove %d orphan cache file(s)? (%s)", len(orphans), formatBytes(totalSize))
		for _, item := range orphans {
			ui.Info("  %s → %s", filepath.Base(item.Path), item.RootDir)
		}
		fmt.Print("Continue? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.ToLower(strings.TrimSpace(input))
		if input != "y" && input != "yes" {
			ui.Info("Cancelled")
			return nil
		}
	}

	removed := 0
	for _, item := range orphans {
		if err := cache.RemoveDiskCache(item.Path); err != nil {
			logCacheOp(config.ConfigPath(), "clean-orphan", removed, start, err)
			return err
		}
		removed++
	}

	ui.Success("Removed %d orphan cache file(s) (%s freed)", removed, formatBytes(totalSize))
	logCacheOp(config.ConfigPath(), "clean-orphan", removed, start, nil)
	return nil
}

// uiVersionEntry represents a cached UI dist version.
type uiVersionEntry struct {
	name string // e.g., "v0.16.6"
	path string // absolute path to version directory
	size int64  // total directory size
}

// listUIVersions returns cached UI dist versions sorted by name.
func listUIVersions(cacheDir string) []uiVersionEntry {
	uiDir := filepath.Join(cacheDir, "ui")
	entries, err := os.ReadDir(uiDir)
	if err != nil {
		return nil
	}

	var versions []uiVersionEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		versionPath := filepath.Join(uiDir, e.Name())
		versions = append(versions, uiVersionEntry{
			name: e.Name(),
			path: versionPath,
			size: dirSizeCache(versionPath),
		})
	}
	return versions
}

// dirSizeCache computes the total size of all files under dir.
func dirSizeCache(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func logCacheOp(cfgPath, action string, count int, start time.Time, cmdErr error) {
	e := oplog.NewEntry("cache", statusFromErr(cmdErr), time.Since(start))
	a := map[string]any{"action": action}
	if count > 0 {
		a["items"] = count
	}
	e.Args = a
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func printCacheHelp() {
	fmt.Println(`Usage: skillshare cache [command] [options]

Manage the discovery and UI cache.

Commands:
  (none)                Interactive TUI (browse + delete)
  list, ls              List all cache entries
  clean                 Remove all cached data (discovery + UI)

Options:
  --orphan              (clean) Only remove orphan discovery caches
  --yes, -y             Skip confirmation prompt
  --help, -h            Show this help

Examples:
  skillshare cache                       # Interactive cache browser
  skillshare cache list                  # List all cache entries
  skillshare cache clean                 # Remove all caches
  skillshare cache clean --orphan        # Remove only orphan caches`)
}
