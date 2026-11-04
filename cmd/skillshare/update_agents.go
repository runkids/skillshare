package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/check"
	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/resource"
	"skillshare/internal/ui"
)

// cmdUpdateAgents handles "skillshare update agents [name|--all]".
func cmdUpdateAgents(args []string, cfg *config.Config, start time.Time) error {
	jsonRequested := hasFlag(args, "--json")
	opts, showHelp, parseErr := parseUpdateAgentArgs(args)
	if showHelp {
		printUpdateHelp()
		return nil
	}
	if parseErr != nil {
		if jsonRequested {
			return writeJSONError(parseErr)
		}
		return parseErr
	}
	if opts.threshold == "" {
		opts.threshold = cfg.Audit.BlockThreshold
	}

	jsonUI := newJSONUISuppressor(opts.jsonOutput)
	defer jsonUI.Flush()

	jsonWriteResult := func(items []agentUpdateItem, cmdErr error) error {
		jsonUI.Flush()
		return updateAgentsOutputJSON(items, opts.dryRun, start, cmdErr)
	}
	failJSON := func(err error) error {
		if opts.jsonOutput {
			jsonUI.Flush()
			return writeJSONError(err)
		}
		return err
	}

	agentsDir := cfg.EffectiveAgentsSource()
	if _, err := os.Stat(agentsDir); err != nil {
		if os.IsNotExist(err) {
			if opts.jsonOutput {
				return jsonWriteResult(nil, nil)
			}
			ui.Done(ui.MarkNone, "No agents source directory "+ui.DimText(shortenPath(agentsDir)), 0)
			return nil
		}
		return failJSON(fmt.Errorf("cannot access agents source: %w", err))
	}

	// Discover agents and check status
	results := check.CheckAgents(agentsDir)
	if len(results) == 0 {
		if opts.jsonOutput {
			return jsonWriteResult(nil, nil)
		}
		ui.Done(ui.MarkNone, "No agents found", 0)
		return nil
	}

	// Filter by name if specified
	if len(opts.names) > 0 {
		results = filterAgentCheckResults(results, opts.names)
		if len(results) == 0 {
			return failJSON(fmt.Errorf("no matching agents found: %s", strings.Join(opts.names, ", ")))
		}
	}

	// Filter by group if specified
	if len(opts.groups) > 0 {
		var err error
		results, err = filterAgentResultsByGroups(results, opts.groups, agentsDir)
		if err != nil {
			return failJSON(err)
		}
		if len(results) == 0 {
			return failJSON(fmt.Errorf("no agents found in group(s): %s", strings.Join(opts.groups, ", ")))
		}
	}

	return runAgentUpdates(agentUpdateScope{
		agentsDir:  agentsDir,
		configPath: config.ConfigPath(),
		noun:       "agents",
		parseOpts:  parseOptsFromConfig(cfg),
	}, results, opts, start, jsonWriteResult)
}

// agentRepoKey groups agents by clone URL + branch + repo subdir so agents
// from the same scope share a single git clone.
type agentRepoKey struct {
	cloneURL   string
	branch     string
	repoSubdir string
}

type agentRepoGroup struct {
	source  *install.Source
	members []check.AgentCheckResult
}

type agentUpdateItem struct {
	Name    string `json:"name"`
	Source  string `json:"source,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// batchUpdateAgents groups agents by repo URL and clones once per group.
// Agents with no RepoURL fall back to per-agent reinstallAgent.
func batchUpdateAgents(agentsDir string, agents []check.AgentCheckResult, opts *updateAgentArgs, projectRoot string, verbose bool, parseOpts install.ParseOptions) ([]agentUpdateItem, int, int) {
	store := install.LoadMetadataOrNew(agentsDir)
	trackedRepos := map[string][]check.AgentCheckResult{}
	// Each group keeps the first member's parsed source: discovery and the
	// metadata written back need its Raw and Type, not only the key.
	groups := map[agentRepoKey]*agentRepoGroup{}
	var noRepo []check.AgentCheckResult
	var items []agentUpdateItem
	updated, failed := 0, 0

	for _, r := range agents {
		if r.RepoPath != "" {
			trackedRepos[r.RepoPath] = append(trackedRepos[r.RepoPath], r)
			continue
		}
		if r.RepoURL == "" {
			noRepo = append(noRepo, r)
			continue
		}
		entry := store.GetByPath(r.Name)
		if entry == nil || entry.Source == "" {
			noRepo = append(noRepo, r)
			continue
		}

		source, parseErr := install.AgentRepoSource(entry, parseOpts)
		if parseErr != nil {
			noRepo = append(noRepo, r)
			continue
		}

		key := agentRepoKey{
			cloneURL:   source.CloneURL,
			branch:     source.Branch,
			repoSubdir: source.Subdir,
		}
		if groups[key] == nil {
			groups[key] = &agentRepoGroup{source: source}
		}
		groups[key].members = append(groups[key].members, r)
	}

	for repoPath, members := range trackedRepos {
		uc := &updateContext{
			sourcePath:  agentsDir,
			projectRoot: projectRoot,
			opts: &updateOptions{
				force:     opts.force,
				skipAudit: opts.skipAudit,
				threshold: opts.threshold,
			},
		}
		ok, _, err := updateTrackedRepoQuick(uc, repoPath)
		if err != nil {
			for _, m := range members {
				items = append(items, agentUpdateItem{
					Name:    m.Name,
					Source:  m.Source,
					Status:  "failed",
					Message: "tracked repo update failed",
					Error:   err.Error(),
				})
				if verbose {
					ui.StepFail(m.Name, err.Error())
				}
				failed++
			}
			continue
		}
		if !ok {
			for _, m := range members {
				items = append(items, agentUpdateItem{
					Name:    m.Name,
					Source:  m.Source,
					Status:  "skipped",
					Message: "up-to-date",
				})
			}
			continue
		}
		for _, m := range members {
			items = append(items, agentUpdateItem{
				Name:    m.Name,
				Source:  m.Source,
				Status:  "updated",
				Message: "tracked repo updated",
			})
			if verbose {
				ui.StepDone(m.Name, "updated")
			}
			updated++
		}
	}

	// Batch: one clone per repo group
	for _, group := range groups {
		source, members := group.source, group.members

		var discovery *install.DiscoveryResult
		var discErr error
		if source.HasSubdir() {
			discovery, discErr = install.DiscoverFromGitSubdir(source)
		} else {
			discovery, discErr = install.DiscoverFromGit(source)
		}
		if discErr != nil {
			for _, m := range members {
				items = append(items, agentUpdateItem{
					Name:    m.Name,
					Source:  m.Source,
					Status:  "failed",
					Message: "discovery failed",
					Error:   "discovery failed: " + discErr.Error(),
				})
				if verbose {
					ui.StepFail(m.Name, "discovery failed: "+discErr.Error())
				}
				failed++
			}
			continue
		}

		// Build agent name → AgentInfo lookup
		agentIndex := map[string]*install.AgentInfo{}
		for i, a := range discovery.Agents {
			agentIndex[a.Name] = &discovery.Agents[i]
		}

		for _, m := range members {
			agentName := filepath.Base(m.Name)
			target := agentIndex[agentName]
			if target == nil {
				if verbose {
					ui.StepFail(m.Name, "not found in repository")
				}
				items = append(items, agentUpdateItem{
					Name:    m.Name,
					Source:  m.Source,
					Status:  "failed",
					Message: "not found in repository",
					Error:   "not found in repository",
				})
				failed++
				continue
			}

			destDir := agentsDir
			if dir := filepath.Dir(m.Name); dir != "." {
				destDir = filepath.Join(agentsDir, dir)
			}

			installOpts := install.InstallOptions{
				Kind:             "agent",
				Force:            opts.force,
				AuditOverride:    opts.force,
				Update:           true,
				SkipAudit:        opts.skipAudit,
				AuditThreshold:   opts.threshold,
				AuditProjectRoot: projectRoot,
				SourceDir:        agentsDir,
			}
			if _, err := install.UpdateAgentFromDiscovery(discovery, *target, destDir, installOpts); err != nil {
				items = append(items, agentUpdateItem{
					Name:    m.Name,
					Source:  m.Source,
					Status:  "failed",
					Message: "update failed",
					Error:   err.Error(),
				})
				if verbose {
					ui.StepFail(m.Name, err.Error())
				}
				failed++
			} else {
				items = append(items, agentUpdateItem{
					Name:    m.Name,
					Source:  m.Source,
					Status:  "updated",
					Message: "updated",
				})
				if verbose {
					ui.StepDone(m.Name, "updated")
				}
				updated++
			}
		}

		install.CleanupDiscovery(discovery)
	}

	// Fallback: agents without RepoURL
	for _, r := range noRepo {
		if err := reinstallAgent(agentsDir, r, store, opts, projectRoot, parseOpts); err != nil {
			items = append(items, agentUpdateItem{
				Name:    r.Name,
				Source:  r.Source,
				Status:  "failed",
				Message: "update failed",
				Error:   err.Error(),
			})
			if verbose {
				ui.StepFail(r.Name, err.Error())
			}
			failed++
		} else {
			items = append(items, agentUpdateItem{
				Name:    r.Name,
				Source:  r.Source,
				Status:  "updated",
				Message: "updated",
			})
			if verbose {
				ui.StepDone(r.Name, "updated")
			}
			updated++
		}
	}

	return items, updated, failed
}

// reinstallAgent re-installs an agent from its recorded source using
// discovery + InstallAgentFromDiscovery (single-file copy), not the
// directory-based skill installer.
// Used as fallback for agents without RepoURL in the batch path.
func reinstallAgent(agentsDir string, r check.AgentCheckResult, store *install.MetadataStore, opts *updateAgentArgs, projectRoot string, parseOpts install.ParseOptions) error {
	entry := store.GetByPath(r.Name)
	if entry == nil || entry.Source == "" {
		return fmt.Errorf("no source metadata for agent %q", r.Name)
	}

	source, parseErr := install.AgentRepoSource(entry, parseOpts)
	if parseErr != nil {
		return fmt.Errorf("invalid source: %w", parseErr)
	}

	// Discover agents — use subdir-scoped discovery for monorepo installs.
	var discovery *install.DiscoveryResult
	var discErr error
	if source.HasSubdir() {
		discovery, discErr = install.DiscoverFromGitSubdir(source)
	} else {
		discovery, discErr = install.DiscoverFromGit(source)
	}
	if discErr != nil {
		return fmt.Errorf("discovery failed: %w", discErr)
	}
	defer install.CleanupDiscovery(discovery)

	// Find the specific agent by name
	agentName := filepath.Base(r.Name)
	var targetAgent *install.AgentInfo
	for i, a := range discovery.Agents {
		if a.Name == agentName {
			targetAgent = &discovery.Agents[i]
			break
		}
	}
	if targetAgent == nil {
		return fmt.Errorf("agent %q not found in repository", agentName)
	}

	// For grouped agents (r.Name contains "/", e.g. "tools/reviewer"),
	// reconstruct the correct destination subdirectory so the file lands
	// at agents/tools/reviewer.md rather than agents/reviewer.md.
	destDir := agentsDir
	if dir := filepath.Dir(r.Name); dir != "." {
		destDir = filepath.Join(agentsDir, dir)
	}

	installOpts := install.InstallOptions{
		Kind:             "agent",
		Force:            opts.force,
		AuditOverride:    opts.force,
		Update:           true,
		SkipAudit:        opts.skipAudit,
		AuditThreshold:   opts.threshold,
		AuditProjectRoot: projectRoot,
		SourceDir:        agentsDir,
	}
	_, installErr := install.UpdateAgentFromDiscovery(discovery, *targetAgent, destDir, installOpts)
	return installErr
}

// updateAgentArgs holds parsed arguments for agent update.
type updateAgentArgs struct {
	names      []string
	groups     []string
	all        bool
	dryRun     bool
	force      bool
	skipAudit  bool
	threshold  string
	jsonOutput bool
}

func parseUpdateAgentArgs(args []string) (*updateAgentArgs, bool, error) {
	opts := &updateAgentArgs{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--all" || arg == "-a":
			opts.all = true
		case arg == "--dry-run" || arg == "-n":
			opts.dryRun = true
		case arg == "--force" || arg == "-f":
			opts.force = true
		case arg == "--skip-audit":
			opts.skipAudit = true
		case arg == "--audit-threshold" || arg == "--threshold" || arg == "-T":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("%s requires a value", arg)
			}
			threshold, err := normalizeInstallAuditThreshold(args[i])
			if err != nil {
				return nil, false, err
			}
			opts.threshold = threshold
		case arg == "--json":
			opts.jsonOutput = true
		case arg == "--group" || arg == "-G":
			i++
			if i >= len(args) {
				return nil, false, fmt.Errorf("--group requires a value")
			}
			opts.groups = append(opts.groups, args[i])
		case arg == "--help" || arg == "-h":
			return nil, true, nil
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			opts.names = append(opts.names, arg)
		}
	}

	if !opts.all && len(opts.names) == 0 && len(opts.groups) == 0 {
		return nil, false, fmt.Errorf("specify agent name(s), --group, or --all")
	}
	if opts.all && (len(opts.names) > 0 || len(opts.groups) > 0) {
		return nil, false, fmt.Errorf("--all cannot be used with agent names or --group")
	}

	return opts, false, nil
}

func collectTrackedAgentResults(results []check.AgentCheckResult) []check.AgentCheckResult {
	tracked := make([]check.AgentCheckResult, 0, len(results))
	for _, r := range results {
		if r.Source != "" {
			tracked = append(tracked, r)
		}
	}
	return tracked
}

func mergeTrackedAgentResults(results, tracked []check.AgentCheckResult) {
	indexByName := make(map[string]int, len(results))
	for i, r := range results {
		indexByName[r.Name] = i
	}
	for _, r := range tracked {
		if idx, ok := indexByName[r.Name]; ok {
			results[idx] = r
		}
	}
}

func filterAgentCheckResults(results []check.AgentCheckResult, names []string) []check.AgentCheckResult {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
		// Also index without .md suffix so "demo/tutor.md" matches "demo/tutor"
		nameSet[strings.TrimSuffix(n, ".md")] = true
	}
	var filtered []check.AgentCheckResult
	for _, r := range results {
		// Match full path (e.g. "demo/code-reviewer") or basename (e.g. "code-reviewer")
		if nameSet[r.Name] || nameSet[filepath.Base(r.Name)] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// validateAgentGroups checks that each group name corresponds to a subdirectory
// under agentsDir. Returns normalized group names (trailing "/" stripped).
func validateAgentGroups(groups []string, agentsDir string) ([]string, error) {
	normalized := make([]string, len(groups))
	for i, group := range groups {
		group = strings.TrimSuffix(group, "/")
		info, err := os.Stat(filepath.Join(agentsDir, group))
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("agent group %q not found in %s", group, agentsDir)
		}
		normalized[i] = group
	}
	return normalized, nil
}

func matchesAnyGroup(name string, groups []string) bool {
	for _, group := range groups {
		if strings.HasPrefix(name, group+"/") {
			return true
		}
	}
	return false
}

// filterAgentResultsByGroups filters agent check results to those in the given groups.
func filterAgentResultsByGroups(results []check.AgentCheckResult, groups []string, agentsDir string) ([]check.AgentCheckResult, error) {
	groups, err := validateAgentGroups(groups, agentsDir)
	if err != nil {
		return nil, err
	}
	var filtered []check.AgentCheckResult
	for _, r := range results {
		if matchesAnyGroup(r.Name, groups) {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

// filterDiscoveredAgentsByGroups filters discovered agents to those in the given groups.
func filterDiscoveredAgentsByGroups(discovered []resource.DiscoveredResource, groups []string, agentsDir string) ([]resource.DiscoveredResource, error) {
	groups, err := validateAgentGroups(groups, agentsDir)
	if err != nil {
		return nil, err
	}
	var filtered []resource.DiscoveredResource
	for _, d := range discovered {
		if matchesAnyGroup(strings.TrimSuffix(d.RelPath, ".md"), groups) {
			filtered = append(filtered, d)
		}
	}
	return filtered, nil
}

func logUpdateAgentOp(cfgPath string, total, updated, failed int, dryRun bool, start time.Time) {
	status := "ok"
	if failed > 0 && updated > 0 {
		status = "partial"
	} else if failed > 0 {
		status = "error"
	}
	e := oplog.NewEntry("update", status, time.Since(start))
	e.Args = map[string]any{
		"resource_kind":  "agent",
		"agents_total":   total,
		"agents_updated": updated,
		"agents_failed":  failed,
		"dry_run":        dryRun,
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func agentUpdateItemsFromCheckResults(results []check.AgentCheckResult) []agentUpdateItem {
	items := make([]agentUpdateItem, 0, len(results))
	for _, r := range results {
		item := agentUpdateItem{
			Name:    r.Name,
			Source:  r.Source,
			Status:  r.Status,
			Message: r.Message,
		}
		if r.Status == "error" {
			item.Error = r.Message
		}
		items = append(items, item)
	}
	return items
}

func mergeAgentUpdateItems(base, updates []agentUpdateItem) []agentUpdateItem {
	if len(updates) == 0 {
		return base
	}

	merged := append([]agentUpdateItem(nil), base...)
	indexByName := make(map[string]int, len(merged))
	for i, item := range merged {
		indexByName[item.Name] = i
	}

	for _, item := range updates {
		if idx, ok := indexByName[item.Name]; ok {
			merged[idx] = item
			continue
		}
		indexByName[item.Name] = len(merged)
		merged = append(merged, item)
	}

	return merged
}

func updateAgentsOutputJSON(items []agentUpdateItem, dryRun bool, start time.Time, err error) error {
	output := struct {
		Agents   []agentUpdateItem `json:"agents"`
		DryRun   bool              `json:"dry_run"`
		Duration string            `json:"duration"`
	}{
		Agents:   items,
		DryRun:   dryRun,
		Duration: formatDuration(start),
	}
	return writeJSONResult(&output, err)
}

// cmdUpdateAgentsProject handles "skillshare update -p agents [name|--all]".
func cmdUpdateAgentsProject(args []string, projectRoot string, start time.Time) error {
	jsonRequested := hasFlag(args, "--json")
	opts, showHelp, parseErr := parseUpdateAgentArgs(args)
	if showHelp {
		printUpdateHelp()
		return nil
	}
	if parseErr != nil {
		if jsonRequested {
			return writeJSONError(parseErr)
		}
		return parseErr
	}
	jsonUI := newJSONUISuppressor(opts.jsonOutput)
	defer jsonUI.Flush()

	jsonWriteResult := func(items []agentUpdateItem, cmdErr error) error {
		jsonUI.Flush()
		return updateAgentsOutputJSON(items, opts.dryRun, start, cmdErr)
	}
	failJSON := func(err error) error {
		if opts.jsonOutput {
			jsonUI.Flush()
			return writeJSONError(err)
		}
		return err
	}

	if err := ensureProjectConfig(projectRoot); err != nil {
		return failJSON(err)
	}

	runtime, err := loadProjectRuntime(projectRoot)
	if err != nil {
		return failJSON(err)
	}
	if opts.threshold == "" {
		opts.threshold = runtime.config.Audit.BlockThreshold
	}

	agentsDir := runtime.agentsSourcePath
	if _, err := os.Stat(agentsDir); err != nil {
		if os.IsNotExist(err) {
			if opts.jsonOutput {
				return jsonWriteResult(nil, nil)
			}
			ui.Done(ui.MarkNone, "No project agents directory "+ui.DimText(shortenPath(agentsDir)), 0)
			return nil
		}
		return failJSON(fmt.Errorf("cannot access project agents: %w", err))
	}

	results := check.CheckAgents(agentsDir)
	if len(results) == 0 {
		if opts.jsonOutput {
			return jsonWriteResult(nil, nil)
		}
		ui.Done(ui.MarkNone, "No project agents found", 0)
		return nil
	}

	if len(opts.names) > 0 {
		results = filterAgentCheckResults(results, opts.names)
		if len(results) == 0 {
			return failJSON(fmt.Errorf("no matching agents found: %s", strings.Join(opts.names, ", ")))
		}
	}

	if len(opts.groups) > 0 {
		var err error
		results, err = filterAgentResultsByGroups(results, opts.groups, agentsDir)
		if err != nil {
			return failJSON(err)
		}
		if len(results) == 0 {
			return failJSON(fmt.Errorf("no agents found in group(s): %s", strings.Join(opts.groups, ", ")))
		}
	}

	return runAgentUpdates(agentUpdateScope{
		agentsDir:   agentsDir,
		projectRoot: projectRoot,
		configPath:  config.ProjectConfigPath(projectRoot),
		noun:        "project agents",
		parseOpts:   parseOptsFromProjectConfig(runtime.config),
	}, results, opts, start, jsonWriteResult)
}

// agentUpdateScope is what differs between global and project agent updates.
type agentUpdateScope struct {
	agentsDir   string
	projectRoot string // "" in global mode
	configPath  string // where the operation log is written
	noun        string // "agents" or "project agents", for messages
	parseOpts   install.ParseOptions
}

// runAgentUpdates checks the filtered agents against their remotes, updates
// those with updates available, and reports the outcome. Agents that could
// not be checked are listed and make the command fail.
func runAgentUpdates(scope agentUpdateScope, results []check.AgentCheckResult, opts *updateAgentArgs, start time.Time, writeJSON func([]agentUpdateItem, error) error) error {
	finish := func(items []agentUpdateItem, err error) error {
		if opts.jsonOutput {
			return writeJSON(items, err)
		}
		return err
	}

	tracked := collectTrackedAgentResults(results)
	if len(tracked) == 0 {
		if err := agentUpdateError(0, reportUncheckedAgents(results, !opts.jsonOutput)); err != nil || opts.jsonOutput {
			return finish(agentUpdateItemsFromCheckResults(results), err)
		}
		ui.Done(ui.MarkNone, "No tracked "+scope.noun+" to update (all are local)", 0)
		return nil
	}

	if opts.jsonOutput {
		check.EnrichAgentResultsWithRemote(tracked, nil)
	} else {
		// Stopped after the check, not per remote: agents with nothing to
		// ask a remote about never call back.
		sp := ui.StartSpinner("Checking " + plural(len(tracked), "agent") + " for updates...")
		check.EnrichAgentResultsWithRemote(tracked, nil)
		sp.Stop()
	}
	mergeTrackedAgentResults(results, tracked)

	var updatable []check.AgentCheckResult
	for _, r := range tracked {
		if r.Status == "update_available" {
			updatable = append(updatable, r)
		}
	}
	items := agentUpdateItemsFromCheckResults(results)
	unchecked := reportUncheckedAgents(results, !opts.jsonOutput)

	if len(updatable) == 0 {
		if err := agentUpdateError(0, unchecked); err != nil || opts.jsonOutput {
			return finish(items, err)
		}
		ui.Done(ui.MarkOK, "All "+scope.noun+" are up to date", time.Since(start))
		return nil
	}

	updated, failed := 0, 0
	if opts.dryRun {
		if !opts.jsonOutput {
			printAgentUpdatesAvailable(updatable)
		}
	} else {
		var updatedItems []agentUpdateItem
		updatedItems, updated, failed = batchUpdateAgents(scope.agentsDir, updatable, opts, scope.projectRoot, !opts.jsonOutput, scope.parseOpts)
		items = mergeAgentUpdateItems(items, updatedItems)
		if !opts.jsonOutput {
			printAgentUpdateDone(updated, failed, start)
		}
	}

	logUpdateAgentOp(scope.configPath, len(updatable), updated, failed, opts.dryRun, start)
	return finish(items, agentUpdateError(failed, unchecked))
}

// reportUncheckedAgents lists the agents whose update status could not be
// determined, so a failed check is not reported as up to date, and returns
// how many there were.
func reportUncheckedAgents(results []check.AgentCheckResult, verbose bool) int {
	n := 0
	for _, r := range results {
		if r.Status != "error" {
			continue
		}
		n++
		if verbose {
			ui.StepFail(r.Name, r.Message)
		}
	}
	return n
}

// agentUpdateError reports agents that failed to update and agents whose
// status could not be checked, or nil when there were none.
func agentUpdateError(failed, unchecked int) error {
	var parts []string
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d agent(s) failed to update", failed))
	}
	if unchecked > 0 {
		parts = append(parts, fmt.Sprintf("%d agent(s) could not be checked", unchecked))
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}

// printAgentUpdatesAvailable lists what a dry run would update.
func printAgentUpdatesAvailable(updatable []check.AgentCheckResult) {
	names := make([]string, len(updatable))
	for i, r := range updatable {
		names[i] = r.Name
	}
	width := ui.RowWidth(names...)
	for _, r := range updatable {
		ui.Row(ui.MarkNone, r.Name, "update available from "+sourceLabel(r.Source), width)
	}
	fmt.Println()
	ui.DryRun()
}

// printAgentUpdateDone closes an agent update with what changed.
func printAgentUpdateDone(updated, failed int, start time.Time) {
	mark, text := ui.MarkOK, "Updated "+plural(updated, "agent")
	if failed > 0 {
		mark, text = ui.MarkWarn, fmt.Sprintf("%s, %d failed", text, failed)
		if updated == 0 {
			mark = ui.MarkFail
		}
	}
	fmt.Println()
	ui.Done(mark, text, time.Since(start))
	if updated > 0 {
		ui.Next("skillshare sync agents", "link the changes into your targets")
	}
}
