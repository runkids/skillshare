package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/inspect"
	"skillshare/internal/resources/adapters"
	managedhooks "skillshare/internal/resources/hooks"
	managed "skillshare/internal/resources/managed"
	managedrules "skillshare/internal/resources/rules"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

type managedSyncResult struct {
	resource string
	updated  []string
	skipped  []string
	pruned   []string
}

func toManagedTargets(entries []syncTargetEntry) []managed.TargetSyncSpec {
	specs := make([]managed.TargetSyncSpec, 0, len(entries))
	for _, entry := range entries {
		specs = append(specs, managed.TargetSyncSpec{
			Name:   entry.name,
			Target: entry.target,
		})
	}
	return specs
}

func syncManagedResourcesForEntries(entries, allEntries []syncTargetEntry, results []syncTargetResult, resources resourceSelection, projectRoot string, dryRun bool) ([]syncTargetResult, int) {
	if len(results) == 0 {
		results = make([]syncTargetResult, len(entries))
	}

	indexByName := make(map[string]int, len(entries))
	for i, entry := range entries {
		indexByName[entry.name] = i
	}

	rows := managed.Sync(managed.SyncRequest{
		ProjectRoot: projectRoot,
		DryRun:      dryRun,
		Resources: managed.ResourceSet{
			Rules: resources.rules,
			Hooks: resources.hooks,
		},
		Targets:    toManagedTargets(entries),
		AllTargets: toManagedTargets(allEntries),
	})

	linesByTarget := make(map[string][]string, len(entries))
	errorsByTarget := make(map[string][]string, len(entries))
	for _, row := range rows {
		idx, ok := indexByName[row.Target]
		if !ok {
			continue
		}

		result := results[idx]
		if result.name == "" {
			entry := entries[idx]
			result = syncTargetResult{
				name:    entry.name,
				mode:    entry.mode,
				include: entry.target.Include,
				exclude: entry.target.Exclude,
			}
		}

		if row.Err != nil {
			errorsByTarget[row.Target] = append(errorsByTarget[row.Target], row.Err.Error())
			results[idx] = result
			continue
		}

		result.stats.updated += len(row.Updated)
		result.stats.pruned += len(row.Pruned)
		if line := managedSyncLine(managedSyncResult{
			resource: row.Resource,
			updated:  row.Updated,
			skipped:  row.Skipped,
			pruned:   row.Pruned,
		}); line != "" {
			linesByTarget[row.Target] = append(linesByTarget[row.Target], line)
		}
		results[idx] = result
	}

	failed := 0
	for i, entry := range entries {
		result := results[i]
		if result.name == "" {
			result = syncTargetResult{
				name:    entry.name,
				mode:    entry.mode,
				include: entry.target.Include,
				exclude: entry.target.Exclude,
			}
		}
		hadPriorError := result.errMsg != ""
		priorError := result.errMsg

		lines := linesByTarget[entry.name]
		errorsByResource := errorsByTarget[entry.name]

		if len(errorsByResource) > 0 {
			managedError := strings.Join(errorsByResource, "; ")
			if hadPriorError {
				result.errMsg = priorError + "; " + managedError
			} else {
				result.errMsg = managedError
				failed++
			}
		}

		if resources.onlyManaged() && result.errMsg == "" {
			if len(lines) == 0 {
				result.message = "no managed resource changes"
			} else {
				result.message = strings.Join(lines, "; ")
			}
		} else {
			result.infos = append(result.infos, lines...)
		}
		results[i] = result
	}

	return results, failed
}

func managedSyncLine(result managedSyncResult) string {
	if result.resource == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if len(result.updated) > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", len(result.updated)))
	}
	if len(result.skipped) > 0 {
		parts = append(parts, fmt.Sprintf("%d unchanged", len(result.skipped)))
	}
	if len(result.pruned) > 0 {
		parts = append(parts, fmt.Sprintf("%d pruned", len(result.pruned)))
	}
	if len(parts) == 0 {
		return result.resource + ": no changes"
	}
	return result.resource + ": " + strings.Join(parts, ", ")
}

func executeManagedCollect(projectRoot string, resources resourceSelection, dryRun, force bool) error {
	result, err := collectManagedResources(projectRoot, resources, dryRun, force)
	return renderManagedCollectResult(projectRoot, resources, dryRun, result, err)
}

func renderManagedCollectResult(projectRoot string, resources resourceSelection, dryRun bool, result *sync.PullResult, collectErr error) error {
	label := "Collecting resources"
	if resources.rules && !resources.hooks {
		label = "Collecting rules"
	} else if resources.hooks && !resources.rules {
		label = "Collecting hooks"
	}
	ui.Header(ui.WithModeLabel(label))

	if result == nil {
		result = &sync.PullResult{Failed: make(map[string]error)}
	}

	if dryRun {
		if len(result.Pulled) == 0 && len(result.Skipped) == 0 {
			if collectErr == nil {
				ui.Info("Dry run - no collectible managed resources found")
			}
			return collectErr
		}
		for _, name := range result.Pulled {
			ui.ListItem("info", name, "would collect")
		}
		for _, name := range result.Skipped {
			ui.ListItem("info", name, "would skip")
		}
		ui.Info("Dry run - no changes made")
		return collectErr
	}

	for _, name := range result.Pulled {
		ui.StepDone(name, "collected into managed store")
	}
	for _, name := range result.Skipped {
		ui.StepSkip(name, "already exists in managed store, use --force to overwrite")
	}
	if len(result.Pulled) > 0 {
		showCollectNextSteps("skills", projectRoot)
	}
	return collectErr
}

func collectManagedResources(projectRoot string, resources resourceSelection, dryRun, force bool) (*sync.PullResult, error) {
	result := &sync.PullResult{Failed: make(map[string]error)}
	var errs []error

	if resources.rules {
		ruleResult, err := collectManagedRules(projectRoot, dryRun, force)
		if err != nil {
			result.Failed["rules"] = err
			errs = append(errs, err)
		} else {
			result = mergePullResults(result, ruleResult)
		}
	}
	if resources.hooks {
		hookResult, err := collectManagedHooks(projectRoot, dryRun, force)
		if err != nil {
			result.Failed["hooks"] = err
			errs = append(errs, err)
		} else {
			result = mergePullResults(result, hookResult)
		}
	}

	sort.Strings(result.Pulled)
	sort.Strings(result.Skipped)
	return result, combineCollectErrors(errs...)
}

func collectManagedRules(projectRoot string, dryRun, force bool) (*sync.PullResult, error) {
	items, _, err := inspect.ScanRules(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("scan rules: %w", err)
	}

	collectible := make([]inspect.RuleItem, 0, len(items))
	for _, item := range items {
		if item.Collectible {
			collectible = append(collectible, item)
		}
	}
	if len(collectible) == 0 {
		return &sync.PullResult{Failed: make(map[string]error)}, nil
	}

	if dryRun {
		preview, err := managed.PreviewCollectRules(projectRoot, collectible, force)
		if err != nil {
			return nil, err
		}
		return &sync.PullResult{
			Pulled:  append([]string{}, preview.Pulled...),
			Skipped: append([]string{}, preview.Skipped...),
			Failed:  make(map[string]error),
		}, nil
	}

	strategy := managedrules.StrategySkip
	if force {
		strategy = managedrules.StrategyOverwrite
	}
	collected, err := managed.CollectRules(projectRoot, collectible, strategy)
	if err != nil {
		return nil, err
	}
	return &sync.PullResult{
		Pulled:  append(append([]string{}, collected.Created...), collected.Overwritten...),
		Skipped: append([]string{}, collected.Skipped...),
		Failed:  make(map[string]error),
	}, nil
}

func collectManagedHooks(projectRoot string, dryRun, force bool) (*sync.PullResult, error) {
	items, _, err := inspect.ScanHooks(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("scan hooks: %w", err)
	}

	collectible := make([]inspect.HookItem, 0, len(items))
	for _, item := range items {
		if item.Collectible {
			collectible = append(collectible, item)
		}
	}
	if len(collectible) == 0 {
		return &sync.PullResult{Failed: make(map[string]error)}, nil
	}

	if dryRun {
		preview, err := managed.PreviewCollectHooks(projectRoot, collectible, force)
		if err != nil {
			return nil, err
		}
		return &sync.PullResult{
			Pulled:  append([]string{}, preview.Pulled...),
			Skipped: append([]string{}, preview.Skipped...),
			Failed:  make(map[string]error),
		}, nil
	}

	strategy := managedhooks.StrategySkip
	if force {
		strategy = managedhooks.StrategyOverwrite
	}
	collected, err := managed.CollectHooks(projectRoot, collectible, strategy)
	if err != nil {
		return nil, err
	}
	return &sync.PullResult{
		Pulled:  append(append([]string{}, collected.Created...), collected.Overwritten...),
		Skipped: append([]string{}, collected.Skipped...),
		Failed:  make(map[string]error),
	}, nil
}

func resolveManagedRuleTarget(name string, target config.TargetConfig, projectRoot string) (string, string, bool) {
	sc := target.SkillsConfig()
	compileTarget, ok := resolveManagedRuleTool(name, sc.Path)
	if !ok {
		return "", "", false
	}
	if strings.TrimSpace(projectRoot) != "" {
		return compileTarget, projectRoot, true
	}
	return compileTarget, managedRuleGlobalRoot(sc.Path), true
}

func createSyncBackup(entry syncTargetEntry, resources resourceSelection) (string, error) {
	if resources.includesManaged() {
		plan, errs := syncBackupPlanForTarget(entry, resources)
		for _, err := range errs {
			ui.Warning("Backup planning for %s: %v", entry.name, err)
		}
		return backup.CreateSnapshot(entry.name, plan.paths, backup.SnapshotOptions{
			RestoreBaseMode:    plan.restoreBaseMode,
			TargetRelativePath: plan.targetRelativePath,
		})
	}
	return backup.Create(entry.name, entry.target.SkillsConfig().Path)
}

func syncBackupPathsForTarget(entry syncTargetEntry, resources resourceSelection) ([]backup.SnapshotPath, []error) {
	plan, errs := syncBackupPlanForTarget(entry, resources)
	return plan.paths, errs
}

type syncBackupPlan struct {
	paths              []backup.SnapshotPath
	restoreBaseMode    backup.SnapshotRestoreBaseMode
	targetRelativePath string
}

type syncBackupSource struct {
	path              string
	followTopSymlinks bool
}

func syncBackupPlanForTarget(entry syncTargetEntry, resources resourceSelection) (syncBackupPlan, []error) {
	skillsTargetPath := entry.target.SkillsConfig().Path
	plan := syncBackupPlan{
		paths: make([]backup.SnapshotPath, 0, 4),
	}
	errs := make([]error, 0, 2)
	baseSources := make([]string, 0, 4)
	if cleanedTarget := filepath.Clean(strings.TrimSpace(skillsTargetPath)); cleanedTarget != "" && cleanedTarget != "." {
		baseSources = append(baseSources, cleanedTarget)
	}

	snapshotSources := make([]syncBackupSource, 0, 4)

	if resources.skills {
		if info, err := os.Lstat(skillsTargetPath); err == nil && info.Mode()&os.ModeSymlink == 0 {
			snapshotSources = append(snapshotSources, syncBackupSource{
				path:              skillsTargetPath,
				followTopSymlinks: true,
			})
		}
	}

	if resources.rules {
		rulePaths, err := managedRuleBackupSourcePaths(entry.name, entry.target, "")
		if err != nil {
			errs = append(errs, fmt.Errorf("rules: %w", err))
		} else {
			baseSources = append(baseSources, rulePaths...)
			for _, path := range rulePaths {
				snapshotSources = append(snapshotSources, syncBackupSource{path: path})
			}
		}
	}

	if resources.hooks {
		hookPaths, err := managedHookBackupSourcePaths(entry.name, entry.target, "")
		if err != nil {
			errs = append(errs, fmt.Errorf("hooks: %w", err))
		} else {
			baseSources = append(baseSources, hookPaths...)
			for _, path := range hookPaths {
				snapshotSources = append(snapshotSources, syncBackupSource{path: path})
			}
		}
	}

	if len(snapshotSources) == 0 {
		return plan, errs
	}

	backupBasePath, restoreBaseMode, err := syncSnapshotBase(skillsTargetPath, baseSources)
	if err != nil {
		errs = append(errs, err)
		return plan, errs
	}
	plan.restoreBaseMode = restoreBaseMode
	targetRelativePath, err := snapshotPathRelativeToBase(backupBasePath, skillsTargetPath)
	if err != nil {
		errs = append(errs, fmt.Errorf("target path: %w", err))
		return plan, errs
	}
	plan.targetRelativePath = targetRelativePath.RelativePath

	for _, source := range snapshotSources {
		path, err := snapshotPathRelativeToBase(backupBasePath, source.path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		path.FollowTopSymlinks = source.followTopSymlinks
		plan.paths = append(plan.paths, path)
	}

	return plan, errs
}

func syncSnapshotBase(targetPath string, sourcePaths []string) (string, backup.SnapshotRestoreBaseMode, error) {
	cleanedTarget := filepath.Clean(strings.TrimSpace(targetPath))
	if cleanedTarget == "" || cleanedTarget == "." {
		return "", "", fmt.Errorf("snapshot base: target path is required")
	}

	commonBase, err := commonAncestorPath(sourcePaths)
	if err != nil {
		return "", "", fmt.Errorf("snapshot base: %w", err)
	}

	switch {
	case cleanedTarget == commonBase:
		return cleanedTarget, backup.SnapshotRestoreBaseTarget, nil
	case filepath.Dir(cleanedTarget) == commonBase:
		return commonBase, backup.SnapshotRestoreBaseParent, nil
	case filepath.Dir(filepath.Dir(cleanedTarget)) == commonBase:
		return commonBase, backup.SnapshotRestoreBaseGrandparent, nil
	default:
		return "", "", fmt.Errorf("snapshot base %s is not representable for target %s", commonBase, cleanedTarget)
	}
}

func managedRuleBackupSourcePaths(name string, target config.TargetConfig, projectRoot string) ([]string, error) {
	compileTarget, compileRoot, ok := resolveManagedRuleTarget(name, target, projectRoot)
	if !ok {
		return nil, nil
	}

	store := managedrules.NewStore(projectRoot)
	records, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("list managed rules: %w", err)
	}
	files, _, err := managedrules.CompileTarget(records, compileTarget, name, compileRoot)
	if err != nil {
		if errors.Is(err, managedrules.ErrUnsupportedTarget) {
			return nil, nil
		}
		return nil, fmt.Errorf("compile managed rules: %w", err)
	}

	paths := make([]string, 0, len(files)+1)
	if ownedDir, ok := managedRuleOwnedDir(compileTarget, compileRoot); ok {
		paths = append(paths, ownedDir)
	}

	for _, file := range files {
		if ownedDir, ok := managedRuleOwnedDir(compileTarget, compileRoot); ok && pathWithinDir(file.Path, ownedDir) {
			continue
		}
		paths = append(paths, file.Path)
	}

	return paths, nil
}

func managedHookBackupSourcePaths(name string, target config.TargetConfig, projectRoot string) ([]string, error) {
	compileTarget, compileRoot, ok := resolveManagedHookTarget(name, target, projectRoot)
	if !ok {
		return nil, nil
	}

	store := managedhooks.NewStore(projectRoot)
	records, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("list managed hooks: %w", err)
	}
	rawConfig, err := loadManagedHookRawConfig(compileTarget, compileRoot)
	if err != nil {
		return nil, fmt.Errorf("load managed hook config: %w", err)
	}
	files, _, err := managedhooks.CompileTarget(records, compileTarget, name, compileRoot, rawConfig)
	if err != nil {
		return nil, fmt.Errorf("compile managed hooks: %w", err)
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths, nil
}

func snapshotPathRelativeToBase(basePath, actualPath string) (backup.SnapshotPath, error) {
	relative, err := filepath.Rel(basePath, actualPath)
	if err != nil {
		return backup.SnapshotPath{}, fmt.Errorf("rel snapshot path %s: %w", actualPath, err)
	}
	return backup.SnapshotPath{
		RelativePath: relative,
		SourcePath:   actualPath,
	}, nil
}

func commonAncestorPath(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("at least one snapshot path is required")
	}

	ancestor := filepath.Clean(strings.TrimSpace(paths[0]))
	if ancestor == "" {
		return "", fmt.Errorf("snapshot path is required")
	}

	for _, candidate := range paths[1:] {
		cleaned := filepath.Clean(strings.TrimSpace(candidate))
		if cleaned == "" {
			return "", fmt.Errorf("snapshot path is required")
		}
		ancestor = sharedAncestorPath(ancestor, cleaned)
	}

	return ancestor, nil
}

func sharedAncestorPath(a, b string) string {
	ancestor := filepath.Clean(a)
	candidate := filepath.Clean(b)
	for ancestor != filepath.Dir(ancestor) {
		if ancestor == candidate || pathWithinDir(candidate, ancestor) {
			return ancestor
		}
		ancestor = filepath.Dir(ancestor)
	}
	if ancestor == candidate || pathWithinDir(candidate, ancestor) {
		return ancestor
	}
	return filepath.Dir(ancestor)
}

func resolveManagedHookTarget(name string, target config.TargetConfig, projectRoot string) (string, string, bool) {
	sc := target.SkillsConfig()
	compileTarget, ok := resolveManagedHookTool(name, sc.Path)
	if !ok {
		return "", "", false
	}
	if strings.TrimSpace(projectRoot) != "" {
		return compileTarget, projectRoot, true
	}
	return compileTarget, managedHookGlobalRoot(sc.Path), true
}

func resolveManagedRuleTool(name, targetPath string) (string, bool) {
	for _, supported := range []string{"claude", "codex", "gemini"} {
		if config.MatchesTargetName(supported, name) {
			return supported, true
		}
	}

	switch managedRulePathFamily(targetPath) {
	case "claude", "codex", "gemini":
		return managedRulePathFamily(targetPath), true
	default:
		return "", false
	}
}

func resolveManagedHookTool(name, targetPath string) (string, bool) {
	for _, supported := range []string{"claude", "codex", "gemini"} {
		if config.MatchesTargetName(supported, name) {
			return supported, true
		}
	}

	switch managedHookPathFamily(targetPath) {
	case "claude", "codex", "gemini":
		return managedHookPathFamily(targetPath), true
	default:
		return "", false
	}
}

func managedRuleGlobalRoot(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return targetPath
	}
	if strings.EqualFold(filepath.Base(cleaned), "skills") {
		return filepath.Dir(cleaned)
	}
	return cleaned
}

func managedHookGlobalRoot(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return targetPath
	}
	if strings.EqualFold(filepath.Base(cleaned), "skills") {
		cleaned = filepath.Dir(cleaned)
	}

	switch strings.ToLower(filepath.Base(cleaned)) {
	case ".claude", "claude", ".codex", "codex", ".agents", "agents", ".gemini", "gemini":
		return filepath.Dir(cleaned)
	default:
		return cleaned
	}
}

func managedRulePathFamily(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return ""
	}

	base := strings.ToLower(filepath.Base(cleaned))
	if base == "skills" {
		base = strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	}

	switch base {
	case ".claude", "claude":
		return "claude"
	case ".codex", "codex", ".agents", "agents":
		return "codex"
	case ".gemini", "gemini":
		return "gemini"
	default:
		return ""
	}
}

func managedHookPathFamily(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return ""
	}

	base := strings.ToLower(filepath.Base(cleaned))
	if base == "skills" {
		base = strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	}

	switch base {
	case ".claude", "claude":
		return "claude"
	case ".codex", "codex", ".agents", "agents":
		return "codex"
	case ".gemini", "gemini":
		return "gemini"
	default:
		return ""
	}
}

func loadManagedHookRawConfig(target, root string) (string, error) {
	path, ok := managedHookConfigPath(target, root)
	if !ok {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func managedHookConfigPath(target, root string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "claude":
		return filepath.Join(root, ".claude", "settings.json"), true
	case "codex":
		return filepath.Join(root, ".codex", "config.toml"), true
	case "gemini":
		return filepath.Join(root, ".gemini", "settings.json"), true
	default:
		return "", false
	}
}

func pruneManagedRuleOrphans(target, root string, files []adapters.CompiledFile, dryRun bool) ([]string, error) {
	ownedDir, ok := managedRuleOwnedDir(target, root)
	if !ok {
		return []string{}, nil
	}

	info, err := os.Stat(ownedDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []string{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("managed rules path is not a directory: %s", ownedDir)
	}

	keep := make(map[string]struct{}, len(files))
	for _, file := range files {
		if pathWithinDir(file.Path, ownedDir) {
			keep[filepath.Clean(file.Path)] = struct{}{}
		}
	}

	pruned := make([]string, 0)
	if err := filepath.WalkDir(ownedDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == ownedDir || d.IsDir() {
			return nil
		}

		cleaned := filepath.Clean(path)
		if _, ok := keep[cleaned]; ok {
			return nil
		}

		pruned = append(pruned, cleaned)
		if dryRun {
			return nil
		}
		return os.Remove(cleaned)
	}); err != nil {
		return nil, err
	}

	if dryRun {
		return pruned, nil
	}
	return pruned, removeEmptyRuleSubdirs(ownedDir)
}

func managedRuleOwnedDir(target, root string) (string, bool) {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "claude":
		if strings.EqualFold(filepath.Base(cleaned), ".claude") {
			return filepath.Join(cleaned, "rules"), true
		}
		return filepath.Join(cleaned, ".claude", "rules"), true
	case "gemini":
		if strings.EqualFold(filepath.Base(cleaned), ".gemini") {
			return filepath.Join(cleaned, "rules"), true
		}
		return filepath.Join(cleaned, ".gemini", "rules"), true
	default:
		return "", false
	}
}

func removeEmptyRuleSubdirs(root string) error {
	var dirs []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	}); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[i]) > len(dirs[j])
	})

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if len(entries) == 0 {
			if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func pathWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
