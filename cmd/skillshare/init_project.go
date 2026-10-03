package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/projectdir"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

type projectInitOptions struct {
	dryRun     bool
	targets    []string // Non-interactive target list
	discover   bool
	selectArg  string // Non-interactive selection for --discover
	mode       string // default mode for new targets: merge|copy|symlink
	configMode string // "local" to gitignore config.yaml (shared skills repo)
	visible    bool   // create a visible skillshare/ directory instead of .skillshare/
}

type detectedProjectTarget struct {
	name         string
	path         string
	exists       bool
	parentExists bool
	members      []string // non-nil for grouped targets sharing the same path
}

func printProjectInitUsage() {
	fmt.Println("Usage: skillshare init -p [flags]")
	fmt.Println()
	fmt.Println("Initialize project-level skillshare config in the current repository.")
	fmt.Println()
	fmt.Println("FLAGS")
	fmt.Println("  --targets <list>          Comma-separated target names to add")
	fmt.Println("  --discover, -d            Detect and add new project targets to existing config")
	fmt.Println("  --select <list>           Select specific targets to add (requires --discover)")
	fmt.Println("  --mode, -m <mode>         Set sync mode (merge, copy, symlink; default: merge).")
	fmt.Println("                            With --discover, applies only to newly added targets")
	fmt.Println("  --config local            Gitignore config.yaml (each developer manages own targets)")
	fmt.Println("  --visible                 Create a visible skillshare/ directory instead of .skillshare/")
	fmt.Println("  --dry-run, -n             Preview without making changes")
	fmt.Println("  --help, -h                Show this help")
	fmt.Println()
	fmt.Println("EXAMPLES")
	fmt.Println("  skillshare init -p")
	fmt.Println("  skillshare init -p --targets claude,cursor --mode copy")
	fmt.Println("  skillshare init -p --discover --select cursor --mode copy")
	fmt.Println("  skillshare init -p --visible")
}

func parseProjectInitArgs(args []string) (projectInitOptions, bool, error) {
	opts := projectInitOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--visible":
			opts.visible = true
		case arg == "--help" || arg == "-h":
			return opts, true, nil
		case arg == "--discover" || arg == "-d":
			opts.discover = true
		case arg == "--select":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--select requires a value")
			}
			i++
			opts.selectArg = args[i]
		case arg == "--targets":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--targets requires a value")
			}
			i++
			opts.targets = strings.Split(args[i], ",")
		case arg == "--config":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--config requires a value (local)")
			}
			i++
			if args[i] != "local" {
				return opts, false, fmt.Errorf("--config only supports 'local'")
			}
			opts.configMode = args[i]
		case arg == "--mode" || arg == "-m":
			if i+1 >= len(args) {
				return opts, false, fmt.Errorf("--mode requires a value (merge, copy, or symlink)")
			}
			i++
			opts.mode = normalizeSyncMode(args[i])
		case arg == "--git-root" || arg == "--git" || arg == "--no-git" || arg == "--remote":
			return opts, false, fmt.Errorf("%s is a global-only flag; project mode has no git integration. Run it in global mode: skillshare init -g %s", arg, arg)
		case strings.HasPrefix(arg, "-"):
			return opts, false, fmt.Errorf("unknown option: %s", arg)
		}
	}

	if opts.selectArg != "" && !opts.discover {
		return opts, false, fmt.Errorf("--select requires --discover flag")
	}
	if err := validateSyncMode(opts.mode); err != nil {
		return opts, false, err
	}

	return opts, false, nil
}

func cmdInitProject(args []string) error {
	opts, showHelp, err := parseProjectInitArgs(args)
	if showHelp {
		printProjectInitUsage()
		return nil
	}
	if err != nil {
		return err
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine project directory: %w", err)
	}

	return performProjectInit(root, opts)
}

func performProjectInit(root string, opts projectInitOptions) error {
	// An already-initialized or partially-initialized project keeps its own
	// directory; only a brand new project honours --visible.
	projectDir, existing := projectdir.Find(root)
	if !existing {
		projectDir, existing = projectdir.FindPartial(root)
	}
	if !existing {
		projectDir = filepath.Join(root, projectdir.Name(opts.visible))
	}
	dirName := filepath.Base(projectDir)
	configPath := filepath.Join(projectDir, projectdir.ConfigFileName)
	partialInitRepair := false
	if info, err := os.Stat(projectDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("project path exists but is not a directory: %s", projectDir)
		}
		if projectConfigExists(root) {
			if opts.discover {
				return reinitProjectWithDiscover(root, opts)
			}
			return fmt.Errorf("project already initialized: %s\nRun 'skillshare init -p --discover' to add new targets", projectDir)
		}
		partialInitRepair = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check project directory: %w", err)
	}

	interactive := runningInInteractiveTTY()
	detected := detectProjectCLIDirectories(root)
	printProjectBanner(root, detected, interactive)

	sharedRepoFlow := false
	if partialInitRepair {
		gitignored, err := isConfigGitignored(projectDir)
		if err != nil {
			ui.Warning("Could not check for shared repo config: %v", err)
		}
		if gitignored {
			// Shared skills repo: cloned from a teammate who used --config local.
			// Generate empty config so each developer manages their own targets.
			sharedRepoFlow = true
			ui.Warning("Detected shared skills directory (from git clone)")
		} else {
			ui.Warning("Detected partial project initialization; repairing missing config")
		}
	}

	var selected []config.ProjectTargetEntry
	selectedMode := opts.mode

	if sharedRepoFlow {
		// Empty targets — developer will add their own via `target add`
		selected = nil
	} else if len(opts.targets) > 0 {
		// --targets provided, skip interactive prompt
		selected = make([]config.ProjectTargetEntry, 0, len(opts.targets))
		for _, name := range opts.targets {
			name = strings.TrimSpace(name)
			if name != "" {
				selected = append(selected, config.ProjectTargetEntry{Name: name})
			}
		}
	} else if partialInitRepair || !interactive {
		// No one to ask: every detected tool, as Enter would choose.
		selected = projectEntries(detected)
	} else {
		available := detected
		if len(available) == 0 {
			available = listAllProjectTargets()
		}
		var err error
		if selected, err = promptProjectTargets(available); err != nil {
			return initCancelled(err)
		}
	}
	if selectedMode == "" && len(opts.targets) == 0 && !sharedRepoFlow && !partialInitRepair && interactive {
		mode, err := promptSyncMode()
		if err != nil {
			return initCancelled(err)
		}
		selectedMode = mode
	}
	if selectedMode == "" {
		selectedMode = "merge"
	}
	if !interactive {
		ui.Answered("Targets", describeTools(entryNames(selected))+" "+theme.Dim().Render("(--targets)"))
		ui.Answered("Sync", selectedMode+" "+theme.Dim().Render("(--mode)"))
	}
	for i := range selected {
		if m := modeOverrideForTarget(selectedMode, "merge"); m != "" {
			selected[i].EnsureSkills().Mode = m
		}
	}

	if opts.dryRun {
		fmt.Println()
		fmt.Println(theme.Primary().Bold(true).Render("Dry run — nothing was written"))
		ui.Info("Would create %s/skills/", dirName)
		ui.Info("Would write config: %s", configPath)
		return nil
	}

	if err := os.MkdirAll(filepath.Join(projectDir, "skills"), 0755); err != nil {
		return fmt.Errorf("failed to create %s/skills: %w", dirName, err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "agents"), 0755); err != nil {
		return fmt.Errorf("failed to create %s/agents: %w", dirName, err)
	}

	if err := ensureProjectGitignore(projectDir, opts.configMode == "local"); err != nil {
		return err
	}

	cfg := &config.ProjectConfig{
		Targets: selected,
		Ignore: []string{
			".DS_Store",
			".git/",
			"__pycache__/",
		},
		Audit: config.AuditConfig{
			BlockThreshold: "CRITICAL",
		},
	}
	if err := cfg.SaveIn(projectDir); err != nil {
		return err
	}

	if err := createProjectTargetDirs(root, selected); err != nil {
		return err
	}

	fmt.Println()
	cfgLine := dirName + "/" + projectdir.ConfigFileName
	if opts.configMode == "local" {
		cfgLine += " " + theme.Dim().Render("(gitignored: each developer manages own targets)")
	}
	ui.Answered("Config", cfgLine)
	ui.Answered("Skills", dirName+"/skills/")
	if !sharedRepoFlow {
		ui.Answered("Targets", describeTools(entryNames(selected)))
	}

	fmt.Println()
	fmt.Println(theme.Primary().Bold(true).Render("Next"))
	cmd := func(c, note string) {
		fmt.Printf("  %s %s\n", theme.Accent().Render(fmt.Sprintf("%-38s", c)), theme.Dim().Render(note))
	}
	if sharedRepoFlow || opts.configMode == "local" {
		cmd("skillshare target add <name> <path> -p", "add a tool to sync")
		cmd("skillshare sync -p", "sync skills to your tools")
	} else {
		cmd("skillshare install <repo> -p", "add skills to this project")
		cmd("skillshare sync -p", "sync skills to your tools")
	}
	return nil
}

// printProjectBanner shows the logo with the project and the tools found in it.
func printProjectBanner(root string, detected []detectedProjectTarget, animate bool) {
	found := "No AI tools found in this project yet"
	if n := len(detected); n > 0 {
		found = "Found " + plural(n, "AI tool") + " here"
	}
	fmt.Println()
	ui.LogoBanner(version, []string{
		"Project " + filepath.Base(root),
		theme.Dim().Render(found),
	}, animate)
	fmt.Println()
}

// promptSyncMode asks how skills reach the tools; Enter keeps merge.
func promptSyncMode() (string, error) {
	mode, err := ui.Select("How should skills reach your tools?", []ui.Option{
		{Label: "merge     link each skill  " + theme.Dim().Render("(default)"), Value: "merge"},
		{Label: "copy      real files  " + theme.Dim().Render("· for tools that can't follow links"), Value: "copy"},
		{Label: "symlink   link the whole folder", Value: "symlink"},
	}, "merge")
	if err != nil {
		return "", err
	}
	ui.Answered("Sync", mode+" "+theme.Dim().Render("("+syncModeNotes[mode]+")"))
	return mode, nil
}

func projectEntries(targets []detectedProjectTarget) []config.ProjectTargetEntry {
	entries := make([]config.ProjectTargetEntry, 0, len(targets))
	for _, t := range targets {
		entries = append(entries, config.ProjectTargetEntry{Name: t.name})
	}
	return entries
}

func entryNames(entries []config.ProjectTargetEntry) []string {
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names
}

// detectProjectCLIDirectories finds the tools set up in the project. It
// only reads the disk and prints nothing.
func detectProjectCLIDirectories(root string) []detectedProjectTarget {
	grouped := config.GroupedProjectTargets()

	var detected []detectedProjectTarget
	for _, g := range grouped {
		relPath := filepath.FromSlash(g.Path)
		fullPath := filepath.Join(root, relPath)

		entry := detectedProjectTarget{
			name:    g.Name,
			path:    relPath,
			members: g.Members,
		}

		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			entry.exists = true
			detected = append(detected, entry)
			continue
		}

		parentRel := filepath.Dir(relPath)
		if parentRel != "." {
			parentPath := filepath.Join(root, parentRel)
			if _, err := os.Stat(parentPath); err == nil {
				entry.parentExists = true
				detected = append(detected, entry)
			}
		}
	}

	return detected
}

func listAllProjectTargets() []detectedProjectTarget {
	grouped := config.GroupedProjectTargets()

	var available []detectedProjectTarget
	for _, g := range grouped {
		available = append(available, detectedProjectTarget{
			name:    g.Name,
			path:    filepath.FromSlash(g.Path),
			members: g.Members,
		})
	}
	return available
}

// promptProjectTargets asks which tools to sync; detected ones start checked.
func promptProjectTargets(available []detectedProjectTarget) ([]config.ProjectTargetEntry, error) {
	options := make([]ui.Option, len(available))
	var checked []string
	for i, t := range available {
		note := filepath.ToSlash(t.path)
		if len(t.members) > 0 {
			note += " · " + strings.Join(t.members, ", ")
		}
		if !t.exists && t.parentExists {
			note += " · not set up yet"
		}
		options[i] = ui.Option{Label: fmt.Sprintf("%-14s %s", t.name, theme.Dim().Render(note)), Value: t.name}
		if t.exists || t.parentExists {
			checked = append(checked, t.name)
		}
	}
	names, err := ui.MultiSelect("Sync skills to which tools?", options, checked)
	if err != nil {
		return nil, err
	}
	ui.Answered("Targets", describeTools(names))
	selected := make([]config.ProjectTargetEntry, 0, len(names))
	for _, name := range names {
		selected = append(selected, config.ProjectTargetEntry{Name: name})
	}
	return selected, nil
}

func createProjectTargetDirs(root string, targets []config.ProjectTargetEntry) error {
	if len(targets) == 0 {
		return nil
	}

	created := map[string]bool{}

	for _, target := range targets {
		name := target.Name
		if !target.SkillsConfig().IsEnabled() {
			continue
		}
		path := target.SkillsConfig().Path
		if path == "" {
			if known, ok := config.LookupProjectTarget(name); ok {
				path = known.Path
			} else {
				continue
			}
		}

		absPath := path
		if !filepath.IsAbs(path) {
			absPath = filepath.Join(root, filepath.FromSlash(path))
		}

		if created[absPath] {
			continue
		}
		created[absPath] = true

		if err := os.MkdirAll(absPath, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	return nil
}

// reinitProjectWithDiscover detects new targets and adds them to existing project config.
// Uses path-based dedup so that e.g. an existing "amp" entry (which maps to .agents/skills)
// correctly prevents the "agents" group from appearing as "new".
func reinitProjectWithDiscover(root string, opts projectInitOptions) error {
	cfg, err := config.LoadProject(root)
	if err != nil {
		return err
	}

	// Build set of existing paths for accurate dedup.
	existingPaths := make(map[string]bool)
	for _, t := range cfg.Targets {
		name := strings.TrimSpace(t.Name)
		if sc := t.SkillsConfig(); sc.Path != "" {
			existingPaths[filepath.FromSlash(sc.Path)] = true
		} else if known, ok := config.LookupProjectTarget(name); ok {
			existingPaths[filepath.FromSlash(known.Path)] = true
		}
	}

	// Detect AI CLI directories (already grouped by shared paths)
	detected := detectProjectCLIDirectories(root)
	noneFound := len(detected) == 0
	if noneFound {
		detected = listAllProjectTargets()
	}

	// Filter out targets whose path is already covered
	var newTargets []detectedProjectTarget
	for _, d := range detected {
		if !existingPaths[filepath.FromSlash(d.path)] {
			newTargets = append(newTargets, d)
		}
	}

	if len(newTargets) == 0 {
		fmt.Println("No new AI tools found")
		return nil
	}

	discoverFound(len(newTargets))

	var selected []config.ProjectTargetEntry
	asked := false

	// Non-interactive: --select (accepts canonical or member names)
	if opts.selectArg != "" {
		names := strings.Split(opts.selectArg, ",")
		seen := make(map[string]bool)
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			target := findGroupedTarget(newTargets, name)
			if target == nil {
				if known, ok := config.LookupProjectTarget(name); ok && existingPaths[filepath.FromSlash(known.Path)] {
					ui.Info("%s is already set up (skipped)", name)
				} else {
					ui.Warning("%s was not found in this project (skipped)", name)
				}
				continue
			}
			if !seen[target.name] {
				seen[target.name] = true
				entry := config.ProjectTargetEntry{Name: target.name}
				if m := modeOverrideForTarget(opts.mode, "merge"); m != "" {
					entry.Skills = &config.ResourceTargetConfig{Mode: m}
				}
				selected = append(selected, entry)
			}
		}
	} else if runningInInteractiveTTY() {
		asked = true
		var err error
		if selected, err = promptProjectTargets(newTargets); err != nil {
			return initCancelled(err)
		}
	} else if !noneFound {
		// No one to ask: add every new tool found, as Enter would choose.
		selected = projectEntries(newTargets)
	}

	if len(selected) == 0 {
		fmt.Println("No AI tools added")
		return nil
	}
	if opts.mode != "" {
		for i := range selected {
			if m := modeOverrideForTarget(opts.mode, "merge"); m != "" {
				selected[i].EnsureSkills().Mode = m
			}
		}
	}

	if opts.dryRun {
		printDiscoverResult("", entryNames(selected), asked, true, "")
		return nil
	}

	// Add to config and save
	cfg.Targets = append(cfg.Targets, selected...)
	if err := cfg.Save(root); err != nil {
		return err
	}

	// Create target directories
	if err := createProjectTargetDirs(root, selected); err != nil {
		return err
	}

	cfgPath := projectdir.ConfigPath(root)
	printDiscoverResult(filepath.Base(filepath.Dir(cfgPath))+"/"+projectdir.ConfigFileName, entryNames(selected), asked, false, "skillshare sync -p")

	return nil
}

// findGroupedTarget finds a target by canonical name or as a member of a group.
func findGroupedTarget(targets []detectedProjectTarget, name string) *detectedProjectTarget {
	for i, d := range targets {
		if d.name == name {
			return &targets[i]
		}
		for _, member := range d.members {
			if member == name {
				return &targets[i]
			}
		}
	}
	return nil
}

// isConfigGitignored checks if config.yaml is gitignored in the project
// directory's .gitignore, indicating this is a shared skills repo where each
// developer manages their own config.
func isConfigGitignored(projectDir string) (bool, error) {
	path := filepath.Join(projectDir, ".gitignore")
	return install.GitignoreContains(path, "config.yaml")
}

func ensureProjectGitignore(gitignoreDir string, gitignoreConfig bool) error {
	dirName := filepath.Base(gitignoreDir)
	gitignorePath := filepath.Join(gitignoreDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		if err := os.WriteFile(gitignorePath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create %s/.gitignore: %w", dirName, err)
		}
	}

	// Always ignore operational directories to avoid committing noise.
	if err := install.UpdateGitIgnoreBatch(gitignoreDir, []string{"logs", "trash", "backups"}); err != nil {
		return fmt.Errorf("failed to update %s/.gitignore: %w", dirName, err)
	}

	// --config local: gitignore config.yaml so each developer manages own targets.
	if gitignoreConfig {
		if err := install.UpdateGitIgnoreFiles(gitignoreDir, []string{"config.yaml"}); err != nil {
			return fmt.Errorf("failed to gitignore config.yaml: %w", err)
		}
	}

	return nil
}
