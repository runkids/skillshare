package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

type syncMCPJSONOutput struct {
	Targets  []syncMCPJSONTarget `json:"targets"`
	Duration string              `json:"duration"`
}

type syncMCPJSONTarget struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Path    string   `json:"path"`
	Added   []string `json:"added,omitempty"`
	Updated []string `json:"updated,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func cmdSyncMCP(args []string) error {
	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	dryRun, _, jsonOutput := parseSyncFlags(rest)

	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	isProject := mode == modeProject

	// Resolve configDir and projectRoot
	var configDir string
	var projectRoot string
	var oplogConfigPath string

	if isProject {
		projectRoot = cwd
		configDir = filepath.Join(projectRoot, ".skillshare")
		oplogConfigPath = config.ProjectConfigPath(projectRoot)
	} else {
		configDir = config.BaseDir()
		projectRoot = ""
		oplogConfigPath = config.ConfigPath()
	}

	// Load MCPConfig
	var mcpConfigPath string
	if isProject {
		mcpConfigPath = mcp.ProjectMCPConfigPath(projectRoot)
	} else {
		mcpConfigPath = mcp.MCPConfigPath(configDir)
	}

	mcpCfg, err := mcp.LoadMCPConfig(mcpConfigPath)
	if err != nil {
		if jsonOutput {
			return writeJSONError(err)
		}
		return err
	}

	// Validate
	if err := mcpCfg.Validate(); err != nil {
		if jsonOutput {
			return writeJSONError(err)
		}
		return err
	}

	// Get targets for this mode
	targets := mcp.MCPTargetsForMode(isProject)

	if len(targets) == 0 && !jsonOutput {
		ui.Info("No MCP targets available for this mode.")
		return nil
	}

	// Determine MCP sync mode from global config
	mcpMode := "merge" // default
	if globalCfg, loadErr := config.Load(); loadErr == nil && globalCfg.MCPMode != "" {
		mcpMode = globalCfg.MCPMode
	}

	if !jsonOutput {
		ui.Header(ui.WithModeLabel("Sync MCP"))
		if dryRun {
			ui.Warning("Dry run mode - no changes will be made")
		}
	}

	// resolveTargetPath returns the path where the MCP config should be written
	resolveTargetPath := func(tgt mcp.MCPTargetSpec) string {
		if isProject {
			return tgt.ProjectConfigPath(projectRoot)
		}
		return tgt.GlobalConfigPath()
	}

	var jsonTargets []syncMCPJSONTarget
	var totalErrors int

	switch mcpMode {
	case "merge":
		totalErrors, jsonTargets = syncMCPMerge(mcpCfg, targets, configDir, resolveTargetPath, dryRun, jsonOutput)

	case "copy":
		generatedDir := mcp.GeneratedDir(configDir)
		generatedFiles, genErr := mcp.GenerateAllTargetFiles(mcpCfg, targets, generatedDir)
		if genErr != nil {
			if jsonOutput {
				return writeJSONError(genErr)
			}
			return genErr
		}
		totalErrors, jsonTargets = syncMCPCopy(targets, generatedFiles, resolveTargetPath, dryRun, jsonOutput)

	default: // "symlink"
		generatedDir := mcp.GeneratedDir(configDir)
		generatedFiles, genErr := mcp.GenerateAllTargetFiles(mcpCfg, targets, generatedDir)
		if genErr != nil {
			if jsonOutput {
				return writeJSONError(genErr)
			}
			return genErr
		}
		totalErrors, jsonTargets = syncMCPSymlink(targets, generatedFiles, generatedDir, resolveTargetPath, dryRun, jsonOutput)
	}

	// Oplog
	status := "ok"
	if totalErrors > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("sync-mcp", status, time.Since(start))
	e.Args = map[string]any{
		"targets_count": len(targets),
		"errors":        totalErrors,
		"dry_run":       dryRun,
		"scope":         modeString(mode),
		"mcp_mode":      mcpMode,
	}
	oplog.WriteWithLimit(oplogConfigPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

	if jsonOutput {
		output := syncMCPJSONOutput{
			Targets:  jsonTargets,
			Duration: formatDuration(start),
		}
		return writeJSON(&output)
	}

	if totalErrors > 0 {
		return fmt.Errorf("%d MCP sync error(s)", totalErrors)
	}
	return nil
}

// syncMCPMerge performs merge-mode sync: read target JSON, upsert skillshare servers,
// remove previously-managed servers no longer in source, preserve everything else.
func syncMCPMerge(
	mcpCfg *mcp.MCPConfig,
	targets []mcp.MCPTargetSpec,
	configDir string,
	resolveTargetPath func(mcp.MCPTargetSpec) string,
	dryRun bool,
	jsonOutput bool,
) (totalErrors int, jsonTargets []syncMCPJSONTarget) {
	statePath := mcp.MCPStatePath(configDir)
	state, err := mcp.LoadMCPState(statePath)
	if err != nil {
		// Non-fatal: proceed with empty state
		state = &mcp.MCPState{Targets: map[string]mcp.MCPTargetState{}}
	}

	for _, tgt := range targets {
		targetPath := resolveTargetPath(tgt)
		if targetPath == "" {
			continue
		}

		servers := mcpCfg.ServersForTarget(tgt.Name)
		prev := state.PreviousServers(tgt.Name)

		result, mergeErr := mcp.MergeToTarget(targetPath, servers, prev, tgt, dryRun)
		if mergeErr != nil {
			totalErrors++
		}

		// Record server names for state update
		serverNames := make([]string, 0, len(servers))
		for name := range servers {
			serverNames = append(serverNames, name)
		}
		if !dryRun {
			state.UpdateTarget(tgt.Name, serverNames, targetPath)
		}

		if jsonOutput {
			jt := syncMCPJSONTarget{
				Name:  tgt.Name,
				Path:  targetPath,
			}
			if mergeErr != nil {
				jt.Status = "error"
				jt.Error = mergeErr.Error()
			} else if len(result.Added)+len(result.Updated)+len(result.Removed) == 0 {
				jt.Status = "up-to-date"
			} else {
				jt.Status = "merged"
				jt.Added = result.Added
				jt.Updated = result.Updated
				jt.Removed = result.Removed
			}
			jsonTargets = append(jsonTargets, jt)
		} else {
			if mergeErr != nil {
				ui.Error("%s: %s", tgt.Name, mergeErr.Error())
			} else if len(result.Added)+len(result.Updated)+len(result.Removed) == 0 {
				ui.Success("%s: up to date", tgt.Name)
			} else {
				ui.Success("%s: merged (%d added, %d updated, %d removed)",
					tgt.Name,
					len(result.Added),
					len(result.Updated),
					len(result.Removed),
				)
			}
		}
	}

	if !dryRun {
		if saveErr := state.Save(statePath); saveErr != nil && !jsonOutput {
			ui.Warning("failed to save MCP state: %s", saveErr.Error())
		}
	}

	return totalErrors, jsonTargets
}

// syncMCPCopy performs copy-mode sync: write generated JSON directly to each target path.
func syncMCPCopy(
	targets []mcp.MCPTargetSpec,
	generatedFiles map[string]string,
	resolveTargetPath func(mcp.MCPTargetSpec) string,
	dryRun bool,
	jsonOutput bool,
) (totalErrors int, jsonTargets []syncMCPJSONTarget) {
	for _, tgt := range targets {
		genPath, hasGen := generatedFiles[tgt.Name]
		if !hasGen {
			continue
		}

		targetPath := resolveTargetPath(tgt)
		if targetPath == "" {
			continue
		}

		result := mcp.CopyToTarget(tgt.Name, genPath, targetPath, dryRun)

		if result.Status == "error" {
			totalErrors++
		}

		if jsonOutput {
			jt := syncMCPJSONTarget{
				Name:   tgt.Name,
				Status: result.Status,
				Path:   result.Path,
			}
			if result.Error != "" {
				jt.Error = result.Error
			}
			jsonTargets = append(jsonTargets, jt)
		} else {
			shortPath := shortenPath(targetPath)
			switch result.Status {
			case "copied":
				ui.Success("%s: copied → %s", tgt.Name, shortPath)
			case "updated":
				ui.Success("%s: updated → %s", tgt.Name, shortPath)
			case "ok":
				ui.Success("%s: up to date (%s)", tgt.Name, shortPath)
			case "error":
				ui.Error("%s: %s", tgt.Name, result.Error)
			}
		}
	}

	return totalErrors, jsonTargets
}

// syncMCPSymlink performs symlink-mode sync (original behavior).
func syncMCPSymlink(
	targets []mcp.MCPTargetSpec,
	generatedFiles map[string]string,
	generatedDir string,
	resolveTargetPath func(mcp.MCPTargetSpec) string,
	dryRun bool,
	jsonOutput bool,
) (totalErrors int, jsonTargets []syncMCPJSONTarget) {
	for _, tgt := range targets {
		genPath, hasGen := generatedFiles[tgt.Name]
		if !hasGen {
			continue
		}

		targetPath := resolveTargetPath(tgt)
		if targetPath == "" {
			continue
		}

		result := mcp.SyncTarget(tgt.Name, genPath, targetPath, dryRun)

		if result.Status == "error" {
			totalErrors++
		}

		if jsonOutput {
			jt := syncMCPJSONTarget{
				Name:   tgt.Name,
				Status: result.Status,
				Path:   result.Path,
			}
			if result.Error != "" {
				jt.Error = result.Error
			}
			jsonTargets = append(jsonTargets, jt)
		} else {
			shortPath := shortenPath(targetPath)
			switch result.Status {
			case "linked":
				ui.Success("%s: linked → %s", tgt.Name, shortPath)
			case "updated":
				ui.Success("%s: updated → %s", tgt.Name, shortPath)
			case "ok":
				ui.Success("%s: up to date (%s)", tgt.Name, shortPath)
			case "skipped":
				ui.Warning("%s: skipped (existing file at %s)", tgt.Name, shortPath)
				ui.Info("  back up and remove the file, then re-run 'skillshare sync mcp'")
			case "error":
				ui.Error("%s: %s", tgt.Name, result.Error)
			}
		}
	}

	// Cleanup stale links for targets without generated files
	prunedNames := mcp.CleanupStaleLinks(targets, generatedDir, resolveTargetPath, generatedFiles)
	for _, name := range prunedNames {
		if !jsonOutput {
			ui.Info("%s: pruned (no matching servers)", name)
		} else {
			jsonTargets = append(jsonTargets, syncMCPJSONTarget{
				Name:   name,
				Status: "pruned",
			})
		}
	}

	return totalErrors, jsonTargets
}
