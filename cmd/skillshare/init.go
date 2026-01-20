package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

const skillshareSkillSource = "github.com/runkids/skillshare/skills/skillshare"
const skillshareSkillURL = "https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/SKILL.md"

func cmdInit(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	sourcePath := "" // Will be determined
	remoteURL := ""
	dryRun := false

	// Non-interactive flags
	copyFrom := ""      // --copy-from: copy from specified name or path
	noCopy := false     // --no-copy: start fresh
	targetsArg := ""    // --targets: comma-separated list
	allTargets := false // --all-targets: add all detected
	noTargets := false  // --no-targets: skip targets
	initGit := false    // --git: initialize git (set by flag)
	noGit := false      // --no-git: skip git
	gitFlagSet := false // track if --git was explicitly set
	discover := false   // --discover: detect and add new agents to existing config
	selectArg := ""     // --select: comma-separated list for --discover mode

	// Parse args
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source", "-s":
			if i+1 >= len(args) {
				return fmt.Errorf("--source requires a path argument")
			}
			sourcePath = args[i+1]
			i++
		case "--remote":
			if i+1 >= len(args) {
				return fmt.Errorf("--remote requires a URL argument")
			}
			remoteURL = args[i+1]
			i++
		case "--dry-run", "-n":
			dryRun = true
		case "--copy-from", "-c":
			if i+1 >= len(args) {
				return fmt.Errorf("--copy-from requires a name or path argument")
			}
			copyFrom = args[i+1]
			i++
		case "--no-copy":
			noCopy = true
		case "--targets", "-t":
			if i+1 >= len(args) {
				return fmt.Errorf("--targets requires a comma-separated list")
			}
			targetsArg = args[i+1]
			i++
		case "--all-targets":
			allTargets = true
		case "--no-targets":
			noTargets = true
		case "--git":
			initGit = true
			gitFlagSet = true
		case "--no-git":
			noGit = true
		case "--discover", "-d":
			discover = true
		case "--select":
			if i+1 >= len(args) {
				return fmt.Errorf("--select requires a comma-separated list")
			}
			selectArg = args[i+1]
			i++
		}
	}

	// Validate mutual exclusions
	if copyFrom != "" && noCopy {
		return fmt.Errorf("--copy-from and --no-copy are mutually exclusive")
	}
	exclusiveCount := 0
	if targetsArg != "" {
		exclusiveCount++
	}
	if allTargets {
		exclusiveCount++
	}
	if noTargets {
		exclusiveCount++
	}
	if exclusiveCount > 1 {
		return fmt.Errorf("--targets, --all-targets, and --no-targets are mutually exclusive")
	}
	if gitFlagSet && noGit {
		return fmt.Errorf("--git and --no-git are mutually exclusive")
	}
	if selectArg != "" && !discover {
		return fmt.Errorf("--select requires --discover flag")
	}

	// --remote implies --git
	if remoteURL != "" && !noGit {
		initGit = true
	}

	// Expand ~ in path
	if utils.HasTildePrefix(sourcePath) {
		sourcePath = filepath.Join(home, sourcePath[1:])
	}

	// Check if already initialized
	if _, err := os.Stat(config.ConfigPath()); err == nil {
		// If --remote provided, just add the remote to existing setup
		if remoteURL != "" {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			setupGitRemote(cfg.Source, remoteURL, dryRun)
			return nil
		}
		// If --discover provided, detect and add new agents
		if discover {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return reinitWithDiscover(cfg, selectArg, dryRun)
		}
		return fmt.Errorf("already initialized. Run 'skillshare init --discover' to add new agents")
	}

	// Detect existing CLI skills directories
	detected := detectCLIDirectories(home)

	// Default source path (same location as config)
	if sourcePath == "" {
		sourcePath = filepath.Join(home, ".config", "skillshare", "skills")
	}

	// Find directories with skills to potentially copy from
	var withSkills []detectedDir
	for _, d := range detected {
		if d.hasSkills {
			withSkills = append(withSkills, d)
		}
	}

	// Determine copy source (non-interactive or prompt)
	copyFromPath, copyFromName := promptCopyFrom(withSkills, copyFrom, noCopy, home)

	if dryRun {
		ui.Warning("Dry run mode - no changes will be made")
	}

	// Create source directory if needed
	if dryRun {
		if _, err := os.Stat(sourcePath); err == nil {
			ui.Info("Source directory exists: %s", sourcePath)
		} else {
			ui.Info("Would create source directory: %s", sourcePath)
		}
	} else if err := os.MkdirAll(sourcePath, 0755); err != nil {
		return fmt.Errorf("failed to create source directory: %w", err)
	}

	// Copy skills from selected directory
	if copyFromPath != "" {
		copySkillsToSource(copyFromPath, sourcePath, dryRun)
	}

	// Build targets list
	targets := buildTargetsList(detected, copyFromPath, copyFromName, targetsArg, allTargets, noTargets)

	// Create config
	cfg := &config.Config{
		Source:  sourcePath,
		Mode:    "merge",
		Targets: targets,
		Ignore: []string{
			"**/.DS_Store",
			"**/.git/**",
		},
	}

	if dryRun {
		summarizeInitConfig(cfg)
	} else if err := cfg.Save(); err != nil {
		return err
	}

	// Initialize git in source directory for safety
	initGitIfNeeded(sourcePath, dryRun, initGit, noGit)

	// Set up git remote for cross-machine sync
	setupGitRemote(sourcePath, remoteURL, dryRun)

	// Create default skillshare skill
	createDefaultSkill(sourcePath, dryRun)

	if dryRun {
		ui.Header("Dry run complete")
		ui.Info("Would write config: %s", config.ConfigPath())
		ui.Info("Run 'skillshare init' to apply these changes")
		return nil
	}

	ui.Header("Initialized successfully")
	ui.Success("Source: %s", sourcePath)
	ui.Success("Config: %s", config.ConfigPath())
	fmt.Println()
	ui.Info("Next steps:")
	fmt.Println("  skillshare sync              # Sync to all targets")
	fmt.Println()
	ui.Info("Pro tip: Let AI manage your skills!")
	fmt.Println("  \"Pull my new skill from Claude and sync to all targets\"")
	fmt.Println("  \"Show me skillshare status\"")

	return nil
}

type detectedDir struct {
	name       string
	path       string
	skillCount int
	hasSkills  bool
	exists     bool // true if skills dir exists, false if only parent exists
}

func detectCLIDirectories(home string) []detectedDir {
	ui.Header("Detecting CLI skills directories")
	defaultTargets := config.DefaultTargets()
	var detected []detectedDir

	for name, target := range defaultTargets {
		if info, err := os.Stat(target.Path); err == nil && info.IsDir() {
			// Skills directory exists - count skills
			entries, _ := os.ReadDir(target.Path)
			skillCount := 0
			for _, e := range entries {
				if e.IsDir() && !utils.IsHidden(e.Name()) {
					skillCount++
				}
			}
			detected = append(detected, detectedDir{
				name:       name,
				path:       target.Path,
				skillCount: skillCount,
				hasSkills:  skillCount > 0,
				exists:     true,
			})
			if skillCount > 0 {
				ui.Success("Found: %s (%d skills)", name, skillCount)
			} else {
				ui.Info("Found: %s (empty)", name)
			}
		} else {
			// Skills directory doesn't exist - check if parent exists (CLI installed)
			parent := filepath.Dir(target.Path)
			if _, err := os.Stat(parent); err == nil {
				detected = append(detected, detectedDir{
					name:       name,
					path:       target.Path,
					skillCount: 0,
					hasSkills:  false,
					exists:     false,
				})
				ui.Info("Found: %s (not initialized)", name)
			}
		}
	}

	return detected
}

func promptCopyFrom(withSkills []detectedDir, copyFromArg string, noCopy bool, home string) (copyFrom, copyFromName string) {
	// Non-interactive: --no-copy
	if noCopy {
		ui.Info("Starting with empty source (--no-copy)")
		return "", ""
	}

	// Non-interactive: --copy-from
	if copyFromArg != "" {
		// First, try to match by name (e.g., "claude", "cursor")
		for _, d := range withSkills {
			if strings.EqualFold(d.name, copyFromArg) {
				ui.Success("Will copy skills from %s (matched by name)", d.name)
				return d.path, d.name
			}
		}

		// Treat as path
		path := copyFromArg
		if utils.HasTildePrefix(path) {
			path = filepath.Join(home, path[1:])
		}

		// Verify path exists
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			ui.Success("Will copy skills from %s", path)
			return path, ""
		}

		ui.Warning("Copy source not found: %s", copyFromArg)
		return "", ""
	}

	// Interactive mode
	if len(withSkills) == 0 {
		return "", ""
	}

	ui.Header("Initialize from existing skills?")
	fmt.Println("  Copy skills from an existing directory to the shared source?")
	fmt.Println()

	for i, d := range withSkills {
		fmt.Printf("  [%d] Copy from %s (%d skills)\n", i+1, d.name, d.skillCount)
	}
	fmt.Printf("  [%d] Start fresh (empty source)\n", len(withSkills)+1)
	fmt.Println()

	fmt.Print("  Enter choice [1]: ")
	var input string
	fmt.Scanln(&input)

	choice := 1
	if input != "" {
		fmt.Sscanf(input, "%d", &choice)
	}

	if choice >= 1 && choice <= len(withSkills) {
		copyFrom = withSkills[choice-1].path
		copyFromName = withSkills[choice-1].name
		ui.Success("Will copy skills from %s", copyFromName)
	} else {
		ui.Info("Starting with empty source")
	}

	return copyFrom, copyFromName
}

func copySkillsToSource(copyFrom, sourcePath string, dryRun bool) {
	entries, err := os.ReadDir(copyFrom)
	if err != nil {
		ui.Warning("Failed to read %s: %v", copyFrom, err)
		return
	}

	if dryRun {
		copyCount := 0
		for _, entry := range entries {
			if entry.IsDir() && !utils.IsHidden(entry.Name()) {
				copyCount++
			}
		}
		ui.Info("Would copy %d skills to %s", copyCount, sourcePath)
		return
	}

	ui.Info("Copying skills to %s...", sourcePath)
	copied := 0
	for _, entry := range entries {
		if !entry.IsDir() || utils.IsHidden(entry.Name()) {
			continue
		}
		srcPath := filepath.Join(copyFrom, entry.Name())
		dstPath := filepath.Join(sourcePath, entry.Name())

		// Skip if already exists
		if _, err := os.Stat(dstPath); err == nil {
			continue
		}

		// Copy directory
		if err := copyDir(srcPath, dstPath); err != nil {
			ui.Warning("Failed to copy %s: %v", entry.Name(), err)
			continue
		}
		copied++
	}
	ui.Success("Copied %d skills to source", copied)
}

func buildTargetsList(detected []detectedDir, copyFrom, copyFromName, targetsArg string, allTargets, noTargets bool) map[string]config.TargetConfig {
	defaultTargets := config.DefaultTargets()
	targets := make(map[string]config.TargetConfig)

	// Non-interactive: --no-targets
	if noTargets {
		ui.Info("Skipping targets (--no-targets)")
		return targets
	}

	// Non-interactive: --all-targets
	if allTargets {
		for _, d := range detected {
			targets[d.name] = defaultTargets[d.name]
		}
		if len(targets) > 0 {
			ui.Success("Added all %d detected targets (--all-targets)", len(targets))
		} else {
			ui.Warning("No CLI skills directories detected")
		}
		return targets
	}

	// Non-interactive: --targets (comma-separated list)
	if targetsArg != "" {
		names := strings.Split(targetsArg, ",")
		added := 0
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			// Check if it's a known target name
			if target, ok := defaultTargets[name]; ok {
				targets[name] = target
				added++
			} else {
				ui.Warning("Unknown target: %s (skipped)", name)
			}
		}
		if added > 0 {
			ui.Success("Added %d targets from --targets", added)
		}
		return targets
	}

	// Interactive mode: Build multi-select items from detected directories
	if len(detected) == 0 {
		ui.Warning("No CLI skills directories detected.")
		return targets
	}

	// Create options for survey.MultiSelect
	options := make([]string, len(detected))
	var defaultIndices []int
	for i, d := range detected {
		status := ""
		if d.exists {
			if d.skillCount > 0 {
				status = fmt.Sprintf("(%d skills)", d.skillCount)
			} else {
				status = "(empty)"
			}
		} else {
			status = "(not initialized)"
		}
		options[i] = fmt.Sprintf("%-12s %s %s", d.name, d.path, status)

		// Pre-select if this is the copyFrom target
		if d.name == copyFromName {
			defaultIndices = append(defaultIndices, i)
		}
	}

	// Show multi-select UI using survey
	var selectedIndices []int
	prompt := &survey.MultiSelect{
		Message:  "Select targets to sync:",
		Options:  options,
		Default:  defaultIndices,
		PageSize: 15,
		Help:     "Use arrow keys to navigate, space to select, enter to confirm",
	}

	if err := survey.AskOne(prompt, &selectedIndices); err != nil {
		return targets // User cancelled
	}

	// Add selected targets
	for _, idx := range selectedIndices {
		name := detected[idx].name
		targets[name] = defaultTargets[name]
	}

	if len(targets) > 0 {
		ui.Success("Added %d target(s)", len(targets))
	} else {
		ui.Info("No targets selected")
	}

	return targets
}

func summarizeInitConfig(cfg *config.Config) {
	ui.Header("Planned configuration")
	ui.Info("Source: %s", cfg.Source)

	if len(cfg.Targets) == 0 {
		ui.Info("Targets: none")
		return
	}

	ui.Info("Targets: %d", len(cfg.Targets))
	for name, target := range cfg.Targets {
		mode := target.Mode
		if mode == "" {
			mode = cfg.Mode
		}
		if mode == "" {
			mode = "merge"
		}
		fmt.Printf("  %-12s %s (%s)\n", name, target.Path, mode)
	}
}

func initGitIfNeeded(sourcePath string, dryRun, initGit, noGit bool) {
	// Non-interactive: --no-git
	if noGit {
		ui.Info("Skipped git initialization (--no-git)")
		ui.Warning("Without git, deleted skills cannot be recovered!")
		return
	}

	gitDir := filepath.Join(sourcePath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		ui.Info("Git already initialized in source directory")
		return
	}

	// Non-interactive: --git flag was set, proceed without prompting
	if initGit {
		if dryRun {
			ui.Info("Dry run - would initialize git in %s (--git)", sourcePath)
			return
		}
		doGitInit(sourcePath)
		return
	}

	// Interactive mode
	ui.Header("Git version control")
	fmt.Println("  Git helps protect your skills from accidental deletion.")
	fmt.Println()
	fmt.Print("  Initialize git in source directory? [Y/n]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	if input != "" && input != "y" && input != "yes" {
		if dryRun {
			ui.Info("Dry run - skipped git initialization")
			return
		}
		ui.Info("Skipped git initialization")
		ui.Warning("Without git, deleted skills cannot be recovered!")
		return
	}

	if dryRun {
		ui.Info("Dry run - would initialize git in %s", sourcePath)
		return
	}

	doGitInit(sourcePath)
}

func doGitInit(sourcePath string) {
	// Run git init
	cmd := exec.Command("git", "init")
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		ui.Warning("Failed to initialize git: %v", err)
		return
	}

	// Create .gitignore
	gitignore := filepath.Join(sourcePath, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		os.WriteFile(gitignore, []byte(".DS_Store\n"), 0644)
	}

	// Initial commit if there are files
	entries, _ := os.ReadDir(sourcePath)
	hasFiles := false
	for _, e := range entries {
		if e.Name() != ".git" && e.Name() != ".gitignore" {
			hasFiles = true
			break
		}
	}

	if hasFiles {
		addCmd := exec.Command("git", "add", ".")
		addCmd.Dir = sourcePath
		addCmd.Run()

		commitCmd := exec.Command("git", "commit", "-m", "Initial skills")
		commitCmd.Dir = sourcePath
		commitCmd.Run()
		ui.Success("Git initialized with initial commit")
	} else {
		ui.Success("Git initialized (empty repository)")
	}
}

func setupGitRemote(sourcePath, remoteURL string, dryRun bool) {
	// Check if git is initialized
	gitDir := filepath.Join(sourcePath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if remoteURL != "" {
			ui.Warning("Git not initialized in source directory")
			ui.Info("Run: cd %s && git init", sourcePath)
		}
		return
	}

	// Check if remote already exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = sourcePath
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		existingRemote := strings.TrimSpace(string(output))
		if existingRemote == remoteURL {
			ui.Info("Git remote already configured: %s", existingRemote)
		} else {
			ui.Warning("Git remote already exists: %s", existingRemote)
			ui.Info("To change: git remote set-url origin %s", remoteURL)
		}
		return
	}

	// If --remote flag provided, use it directly
	if remoteURL != "" {
		if dryRun {
			ui.Info("Would add git remote: %s", remoteURL)
			return
		}
		addRemote(sourcePath, remoteURL)
		return
	}

	// Prompt user
	ui.Header("Cross-machine sync")
	fmt.Println("  Set up a git remote to sync skills across machines.")
	fmt.Println()
	fmt.Print("  Set up git remote? [y/N]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	if input != "y" && input != "yes" {
		ui.Info("Skipped remote setup")
		ui.Info("Add later: git remote add origin <url>")
		return
	}

	fmt.Print("  Remote URL (e.g., git@github.com:user/skills.git): ")
	fmt.Scanln(&remoteURL)
	remoteURL = strings.TrimSpace(remoteURL)

	if remoteURL == "" {
		ui.Info("No URL provided, skipped remote setup")
		return
	}

	if dryRun {
		ui.Info("Would add git remote: %s", remoteURL)
		return
	}

	addRemote(sourcePath, remoteURL)
}

func addRemote(sourcePath, remoteURL string) {
	cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = sourcePath
	if err := cmd.Run(); err != nil {
		ui.Warning("Failed to add remote: %v", err)
		return
	}

	ui.Success("Git remote configured: %s", remoteURL)
	ui.Info("Push your skills: skillshare push")
}

const fallbackSkillContent = `---
name: skillshare
description: Manage and sync skills across AI CLI tools
---

# Skillshare CLI

Run ` + "`skillshare update`" + ` to download the full skill with AI integration guide.

## Quick Commands

- ` + "`skillshare status`" + ` - Show sync state
- ` + "`skillshare sync`" + ` - Sync to all targets
- ` + "`skillshare pull <target>`" + ` - Pull from target
- ` + "`skillshare update`" + ` - Update this skill
`

func createDefaultSkill(sourcePath string, dryRun bool) {
	skillshareSkillDir := filepath.Join(sourcePath, "skillshare")
	skillshareSkillFile := filepath.Join(skillshareSkillDir, "SKILL.md")

	if _, err := os.Stat(skillshareSkillFile); err == nil {
		return
	}

	if dryRun {
		ui.Info("Would download default skill: skillshare")
		return
	}

	// Try to install from GitHub using install package
	source, err := install.ParseSource(skillshareSkillSource)
	if err == nil {
		source.Name = "skillshare"
		_, err = install.Install(source, skillshareSkillDir, install.InstallOptions{
			Force:  true,
			DryRun: false,
		})
	}

	if err != nil {
		// Fallback to minimal version
		if err := os.MkdirAll(skillshareSkillDir, 0755); err != nil {
			ui.Warning("Failed to create skillshare skill directory: %v", err)
			return
		}
		if err := os.WriteFile(skillshareSkillFile, []byte(fallbackSkillContent), 0644); err != nil {
			ui.Warning("Failed to create skillshare skill: %v", err)
			return
		}
		ui.Success("Created default skill: skillshare")
		ui.Info("Run 'skillshare upgrade --skill' to get the full version")
		return
	}
	ui.Success("Downloaded default skill: skillshare")
}

// agentInfo holds information about a detected agent for discover mode
type agentInfo struct {
	name        string
	path        string
	description string
}

// reinitWithDiscover detects new agents and allows user to add them to existing config
func reinitWithDiscover(existingCfg *config.Config, selectArg string, dryRun bool) error {
	ui.Header("Discovering new agents")

	// Get all default targets
	defaultTargets := config.DefaultTargets()

	// Find agents not already in config
	var newAgents []agentInfo
	for name, target := range defaultTargets {
		// Skip if already in config
		if _, exists := existingCfg.Targets[name]; exists {
			continue
		}

		// Check if the agent's parent directory exists (CLI is installed)
		parent := filepath.Dir(target.Path)
		if _, err := os.Stat(parent); err != nil {
			continue // CLI not installed, skip
		}

		// Check skills directory status
		status := "(not initialized)"
		if info, err := os.Stat(target.Path); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(target.Path)
			skillCount := 0
			for _, e := range entries {
				if e.IsDir() && !utils.IsHidden(e.Name()) {
					skillCount++
				}
			}
			if skillCount > 0 {
				status = fmt.Sprintf("(%d skills)", skillCount)
			} else {
				status = "(empty)"
			}
		}

		newAgents = append(newAgents, agentInfo{
			name:        name,
			path:        target.Path,
			description: status,
		})
	}

	if len(newAgents) == 0 {
		ui.Info("No new agents detected")
		return nil
	}

	ui.Success("Found %d new agent(s)", len(newAgents))

	// Non-interactive mode with --select
	if selectArg != "" {
		return addSelectedAgentsByName(existingCfg, newAgents, selectArg, dryRun)
	}

	// Interactive mode: use survey.MultiSelect
	options := make([]string, len(newAgents))
	for i, agent := range newAgents {
		options[i] = fmt.Sprintf("%-12s %s %s", agent.name, agent.path, agent.description)
	}

	var selectedIndices []int
	prompt := &survey.MultiSelect{
		Message:  "Select agents to add:",
		Options:  options,
		PageSize: 15,
		Help:     "Use arrow keys to navigate, space to select, enter to confirm",
	}

	if err := survey.AskOne(prompt, &selectedIndices); err != nil {
		return nil // User cancelled (Ctrl+C)
	}

	if len(selectedIndices) == 0 {
		ui.Info("No agents selected")
		return nil
	}

	// Map indices back to agent names
	var selectedNames []string
	for _, idx := range selectedIndices {
		selectedNames = append(selectedNames, newAgents[idx].name)
	}

	// Add selected agents to config
	for _, name := range selectedNames {
		if target, ok := defaultTargets[name]; ok {
			existingCfg.Targets[name] = target
		}
	}

	if dryRun {
		ui.Warning("Dry run - would add %d agent(s) to config", len(selectedNames))
		for _, name := range selectedNames {
			fmt.Printf("  + %s\n", name)
		}
		return nil
	}

	if err := existingCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Added %d agent(s) to config", len(selectedNames))
	for _, name := range selectedNames {
		fmt.Printf("  + %s\n", name)
	}
	ui.Info("Run 'skillshare sync' to sync skills to new targets")

	return nil
}

// addSelectedAgentsByName adds agents specified by --select flag (non-interactive)
func addSelectedAgentsByName(existingCfg *config.Config, newAgents []agentInfo, selectArg string, dryRun bool) error {
	defaultTargets := config.DefaultTargets()

	// Build a map of available new agents for quick lookup
	availableAgents := make(map[string]bool)
	for _, agent := range newAgents {
		availableAgents[agent.name] = true
	}

	// Parse comma-separated selection
	names := strings.Split(selectArg, ",")
	var addedNames []string

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Check if it's in the available new agents
		if !availableAgents[name] {
			// Check if it's already in config
			if _, exists := existingCfg.Targets[name]; exists {
				ui.Info("Agent already in config: %s (skipped)", name)
			} else if _, ok := defaultTargets[name]; !ok {
				ui.Warning("Unknown agent: %s (skipped)", name)
			} else {
				ui.Warning("Agent not detected: %s (skipped)", name)
			}
			continue
		}

		// Add to config
		if target, ok := defaultTargets[name]; ok {
			existingCfg.Targets[name] = target
			addedNames = append(addedNames, name)
		}
	}

	if len(addedNames) == 0 {
		ui.Info("No new agents added")
		return nil
	}

	if dryRun {
		ui.Warning("Dry run - would add %d agent(s) to config", len(addedNames))
		for _, name := range addedNames {
			fmt.Printf("  + %s\n", name)
		}
		return nil
	}

	if err := existingCfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	ui.Success("Added %d agent(s) to config", len(addedNames))
	for _, name := range addedNames {
		fmt.Printf("  + %s\n", name)
	}
	ui.Info("Run 'skillshare sync' to sync skills to new targets")

	return nil
}
