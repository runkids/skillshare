package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/mcp"
	"skillshare/internal/ui"
)

func cmdMCP(args []string) error {
	if len(args) == 0 {
		printMCPUsage()
		return nil
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "list", "ls":
		return cmdMCPList(rest)
	case "add":
		return cmdMCPAdd(rest)
	case "remove", "rm":
		return cmdMCPRemove(rest)
	case "status":
		return cmdMCPStatus(rest)
	default:
		printMCPUsage()
		return fmt.Errorf("unknown mcp subcommand: %s", sub)
	}
}

func printMCPUsage() {
	y := "\033[33m"
	c := "\033[36m"
	r := "\033[0m"
	fmt.Println("MCP MANAGEMENT")
	fmt.Printf("  %smcp list%s                             %sList all MCP servers%s\n", y, r, ui.Dim, r)
	fmt.Printf("  %smcp add%s %s<name> --command <cmd>%s     %sAdd an MCP server%s\n", y, r, c, r, ui.Dim, r)
	fmt.Printf("  %smcp remove%s %s<name>%s                  %sRemove an MCP server%s\n", y, r, c, r, ui.Dim, r)
	fmt.Printf("  %smcp status%s                          %sShow sync status for all targets%s\n", y, r, ui.Dim, r)
	fmt.Println()
}

// resolveMCPConfigPath resolves the mcp.yaml path based on -p/-g flags.
func resolveMCPConfigPath(args []string) (string, []string, error) {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return "", nil, err
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

	var mcpConfigPath string
	if mode == modeProject {
		mcpConfigPath = mcp.ProjectMCPConfigPath(cwd)
	} else {
		mcpConfigPath = mcp.MCPConfigPath(config.BaseDir())
	}

	return mcpConfigPath, rest, nil
}


// cmdMCPList implements `mcp list [--json] [-g/-p]`
func cmdMCPList(args []string) error {
	jsonOutput := hasFlag(args, "--json")

	mcpConfigPath, _, err := resolveMCPConfigPath(args)
	if err != nil {
		return err
	}

	cfg, err := mcp.LoadMCPConfig(mcpConfigPath)
	if err != nil {
		if jsonOutput {
			return writeJSONError(err)
		}
		return err
	}

	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(cfg)
	}

	// Launch interactive TUI if we're in a terminal and have servers to show
	noTUI := hasFlag(args, "--no-tui")
	if !noTUI && ui.IsTTY() && len(cfg.Servers) > 0 {
		return runMCPListTUI(mcpConfigPath, cfg, ui.ModeLabel)
	}

	ui.Header(ui.WithModeLabel("MCP Servers"))

	if len(cfg.Servers) == 0 {
		fmt.Printf("%sNo MCP servers configured.%s\n", ui.Dim, ui.Reset)
		fmt.Printf("%sAdd one with: skillshare mcp add <name> --command <cmd>%s\n", ui.Dim, ui.Reset)
		return nil
	}

	names := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		srv := cfg.Servers[name]

		transport := srv.Command
		if srv.IsRemote() {
			transport = srv.URL
		}

		status := ""
		if srv.Disabled {
			status = fmt.Sprintf(" %s(disabled)%s", ui.Dim, ui.Reset)
		}

		fmt.Printf("  %s%s%s%s\n", ui.Bold, name, ui.Reset, status)
		fmt.Printf("    %s%s%s\n", ui.Dim, transport, ui.Reset)

		if len(srv.Targets) > 0 {
			fmt.Printf("    targets: %s%s%s\n", ui.Dim, strings.Join(srv.Targets, ", "), ui.Reset)
		}
	}

	return nil
}

// cmdMCPAdd implements `mcp add <name> --command <cmd> [--args ...] [--url <url>] [--env K=V ...] [--targets t1,t2]`
func cmdMCPAdd(args []string) error {
	// Split mode flags from the rest first
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	// No name argument → launch TUI wizard
	if len(rest) == 0 && ui.IsTTY() {
		cwd, _ := os.Getwd()
		if mode == modeAuto {
			if projectConfigExists(cwd) {
				mode = modeProject
			} else {
				mode = modeGlobal
			}
		}
		applyModeLabel(mode)
		var mcpConfigPath string
		if mode == modeProject {
			mcpConfigPath = mcp.ProjectMCPConfigPath(cwd)
		} else {
			mcpConfigPath = mcp.MCPConfigPath(config.BaseDir())
		}
		return runMCPAddTUI(mcpConfigPath)
	}

	if len(rest) == 0 {
		return fmt.Errorf("usage: skillshare mcp add <name> --command <cmd> [--args arg...] [--url <url>] [--env K=V...] [--targets t1,t2]")
	}

	name := rest[0]
	rest = rest[1:]

	var (
		command    string
		cmdArgs    []string
		url        string
		envPairs   []string
		targetsStr string
	)

	// Manual flag parsing
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--command":
			if i+1 >= len(rest) {
				return fmt.Errorf("--command requires a value")
			}
			i++
			command = rest[i]
		case "--args":
			if i+1 >= len(rest) {
				return fmt.Errorf("--args requires a value")
			}
			i++
			cmdArgs = append(cmdArgs, rest[i])
		case "--url":
			if i+1 >= len(rest) {
				return fmt.Errorf("--url requires a value")
			}
			i++
			url = rest[i]
		case "--env":
			if i+1 >= len(rest) {
				return fmt.Errorf("--env requires a value")
			}
			i++
			envPairs = append(envPairs, rest[i])
		case "--targets":
			if i+1 >= len(rest) {
				return fmt.Errorf("--targets requires a value")
			}
			i++
			targetsStr = rest[i]
		}
	}

	// Resolve config path using already-parsed mode
	cwd, _ := os.Getwd()
	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}
	applyModeLabel(mode)

	var mcpConfigPath string
	if mode == modeProject {
		mcpConfigPath = mcp.ProjectMCPConfigPath(cwd)
	} else {
		mcpConfigPath = mcp.MCPConfigPath(config.BaseDir())
	}

	cfg, err := mcp.LoadMCPConfig(mcpConfigPath)
	if err != nil {
		return err
	}

	if _, exists := cfg.Servers[name]; exists {
		return fmt.Errorf("server %q already exists; remove it first with: skillshare mcp remove %s", name, name)
	}

	// Build MCPServer
	srv := mcp.MCPServer{
		Command: command,
		Args:    cmdArgs,
		URL:     url,
	}

	// Parse env K=V pairs
	if len(envPairs) > 0 {
		srv.Env = make(map[string]string)
		for _, pair := range envPairs {
			k, v, ok := strings.Cut(pair, "=")
			if !ok {
				return fmt.Errorf("invalid --env value %q: expected KEY=VALUE", pair)
			}
			srv.Env[k] = v
		}
	}

	// Parse targets
	if targetsStr != "" {
		for t := range strings.SplitSeq(targetsStr, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				srv.Targets = append(srv.Targets, t)
			}
		}
	}

	cfg.Servers[name] = srv

	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := cfg.Save(mcpConfigPath); err != nil {
		return err
	}

	ui.Success("Added MCP server %q", name)
	fmt.Printf("%sRun 'skillshare sync mcp' to push to targets.%s\n", ui.Dim, ui.Reset)
	return nil
}

// cmdMCPRemove implements `mcp remove <name> [-g/-p]`
func cmdMCPRemove(args []string) error {
	mcpConfigPath, rest, err := resolveMCPConfigPath(args)
	if err != nil {
		return err
	}

	if len(rest) == 0 {
		return fmt.Errorf("usage: skillshare mcp remove <name>")
	}

	name := rest[0]

	cfg, err := mcp.LoadMCPConfig(mcpConfigPath)
	if err != nil {
		return err
	}

	if _, exists := cfg.Servers[name]; !exists {
		return fmt.Errorf("server %q not found", name)
	}

	delete(cfg.Servers, name)

	if err := cfg.Save(mcpConfigPath); err != nil {
		return err
	}

	ui.Success("Removed MCP server %q", name)
	fmt.Printf("%sRun 'skillshare sync mcp' to clean up symlinks.%s\n", ui.Dim, ui.Reset)
	return nil
}

// mcpStatusEntry is the JSON representation of a target's sync status.
type mcpStatusEntry struct {
	Target    string `json:"target"`
	Status    string `json:"status"`
	TargetPath string `json:"target_path,omitempty"`
}

// cmdMCPStatus implements `mcp status [--json] [-g/-p]`
func cmdMCPStatus(args []string) error {
	jsonOutput := hasFlag(args, "--json")

	mode, _, err := parseModeArgs(args)
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
	isProject := mode == modeProject

	var configDir string
	if isProject {
		configDir = filepath.Join(cwd, ".skillshare")
	} else {
		configDir = config.BaseDir()
	}

	generatedDir := mcp.GeneratedDir(configDir)
	targets := mcp.MCPTargetsForMode(isProject)

	var entries []mcpStatusEntry

	for _, tgt := range targets {
		var targetPath string
		if isProject {
			targetPath = tgt.ProjectConfigPath(cwd)
		} else {
			targetPath = tgt.GlobalConfigPath()
		}

		generatedFile := filepath.Join(generatedDir, tgt.Name+".json")
		status := computeMCPStatus(generatedFile, targetPath, generatedDir)

		entries = append(entries, mcpStatusEntry{
			Target:    tgt.Name,
			Status:    status,
			TargetPath: targetPath,
		})
	}

	if jsonOutput {
		return writeJSON(&entries)
	}

	ui.Header(ui.WithModeLabel("MCP Sync Status"))

	if len(entries) == 0 {
		fmt.Printf("%sNo MCP targets available for this mode.%s\n", ui.Dim, ui.Reset)
		return nil
	}

	for _, e := range entries {
		shortPath := ""
		if e.TargetPath != "" {
			shortPath = shortenPath(e.TargetPath)
		}

		switch e.Status {
		case "linked":
			ui.Success("%s: linked → %s", e.Target, shortPath)
		case "not_linked":
			ui.Warning("%s: not linked (%s)", e.Target, shortPath)
		case "conflict":
			ui.Warning("%s: file exists, not managed by skillshare (%s)", e.Target, shortPath)
		case "no_file":
			ui.Info("%s: no generated file, run 'skillshare sync mcp' first", e.Target)
		}
	}

	return nil
}

// computeMCPStatus determines the sync status for one target:
//   - "no_file"    — generated JSON file doesn't exist
//   - "not_linked" — target path doesn't exist (not yet linked)
//   - "linked"     — target is a symlink pointing into generatedDir
//   - "conflict"   — target exists but is not a managed symlink
func computeMCPStatus(generatedFile, targetPath, generatedDir string) string {
	// Check if we have a generated file for this target
	if _, err := os.Stat(generatedFile); err != nil {
		return "no_file"
	}

	if targetPath == "" {
		return "no_file"
	}

	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "not_linked"
		}
		return "conflict"
	}

	// Must be a symlink
	if info.Mode()&os.ModeSymlink == 0 {
		return "conflict"
	}

	// Check it points into our generatedDir
	link, err := os.Readlink(targetPath)
	if err != nil {
		return "conflict"
	}

	if strings.HasPrefix(link, generatedDir) {
		return "linked"
	}

	return "conflict"
}
