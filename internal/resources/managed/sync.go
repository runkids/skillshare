package managed

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/resources/adapters"
	"skillshare/internal/resources/apply"
	managedhooks "skillshare/internal/resources/hooks"
	managedpi "skillshare/internal/resources/managed/pi"
	managedrules "skillshare/internal/resources/rules"
)

// Sync runs managed rule and hook sync for each target in the request.
func Sync(req SyncRequest) []SyncResult {
	results := make([]SyncResult, 0, len(req.Targets)*2)
	for _, target := range req.Targets {
		if req.Resources.Rules {
			if result, ok := syncRules(req, target); ok {
				results = append(results, result)
			}
		}
		if req.Resources.Hooks {
			if result, ok := syncHooks(req, target); ok {
				results = append(results, result)
			}
		}
	}
	return results
}

func syncRules(req SyncRequest, spec TargetSyncSpec) (SyncResult, bool) {
	compileTarget, compileRoot, ok := resolveRuleTarget(spec.Name, spec.Target, req.ProjectRoot)
	if !ok {
		return SyncResult{}, false
	}

	result := SyncResult{Target: spec.Name, Resource: "rules"}
	store := managedrules.NewStore(req.ProjectRoot)
	records, err := store.List()
	if err != nil {
		result.Err = fmt.Errorf("list managed rules: %w", err)
		return result, true
	}

	files, _, err := managedrules.CompileTarget(records, compileTarget, spec.Name, compileRoot)
	if err != nil {
		if errors.Is(err, managedrules.ErrUnsupportedTarget) {
			return SyncResult{}, false
		}
		result.Err = fmt.Errorf("compile managed rules: %w", err)
		return result, true
	}
	otherCurrentPaths, err := detectRuleOutputConflicts(req, spec, records, compileTarget, files)
	if err != nil {
		result.Err = fmt.Errorf("compile managed rules: %w", err)
		return result, true
	}

	updated, skipped, err := apply.CompiledFiles(files, req.DryRun)
	if err != nil {
		result.Err = fmt.Errorf("apply managed rules: %w", err)
		return result, true
	}
	state, err := loadManagedRuleSyncState(compileRoot)
	if err != nil {
		result.Err = fmt.Errorf("load managed rule state: %w", err)
		return result, true
	}

	pruned, err := pruneRuleOrphans(compileTarget, compileRoot, files, otherCurrentPaths, state, req.DryRun)
	if err != nil {
		result.Err = fmt.Errorf("prune managed rules: %w", err)
		return result, true
	}
	if !req.DryRun {
		if err := recordManagedRuleSyncState(spec.Name, compileTarget, compileRoot, files, state); err != nil {
			result.Err = fmt.Errorf("save managed rule state: %w", err)
			return result, true
		}
	}

	result.Updated = updated
	result.Skipped = skipped
	result.Pruned = pruned
	return result, true
}

func syncHooks(req SyncRequest, spec TargetSyncSpec) (SyncResult, bool) {
	compileTarget, compileRoot, ok := resolveHookTarget(spec.Name, spec.Target, req.ProjectRoot)
	if !ok {
		return SyncResult{}, false
	}

	result := SyncResult{Target: spec.Name, Resource: "hooks"}
	store := managedhooks.NewStore(req.ProjectRoot)
	records, err := store.List()
	if err != nil {
		result.Err = fmt.Errorf("list managed hooks: %w", err)
		return result, true
	}

	rawConfig, err := loadHookRawConfig(compileTarget, compileRoot)
	if err != nil {
		result.Err = fmt.Errorf("load managed hook config: %w", err)
		return result, true
	}

	files, _, err := managedhooks.CompileTarget(records, compileTarget, spec.Name, compileRoot, string(rawConfig))
	if err != nil {
		if errors.Is(err, managedhooks.ErrUnsupportedTarget) {
			return SyncResult{}, false
		}
		result.Err = fmt.Errorf("compile managed hooks: %w", err)
		return result, true
	}

	updated, skipped, err := apply.CompiledFiles(files, req.DryRun)
	if err != nil {
		result.Err = fmt.Errorf("apply managed hooks: %w", err)
		return result, true
	}

	result.Updated = updated
	result.Skipped = skipped
	return result, true
}

func resolveRuleTarget(name string, target config.TargetConfig, projectRoot string) (compileTarget, compileRoot string, ok bool) {
	sc := target.SkillsConfig()
	family, ok := ResolveManagedFamily(ResourceKindRules, name, sc.Path)
	if !ok {
		return "", "", false
	}
	if projectRoot != "" {
		return family, projectRoot, true
	}
	return family, RuleGlobalPreviewRoot(sc.Path), true
}

func resolveHookTarget(name string, target config.TargetConfig, projectRoot string) (compileTarget, compileRoot string, ok bool) {
	sc := target.SkillsConfig()
	family, ok := ResolveManagedFamily(ResourceKindHooks, name, sc.Path)
	if !ok {
		return "", "", false
	}
	if projectRoot != "" {
		return family, projectRoot, true
	}
	return family, managedHookGlobalPreviewRoot(sc.Path), true
}

func RuleGlobalPreviewRoot(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return targetPath
	}
	if strings.EqualFold(filepath.Base(cleaned), "skills") {
		return filepath.Dir(cleaned)
	}
	return cleaned
}

func managedHookGlobalPreviewRoot(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return targetPath
	}
	if strings.EqualFold(filepath.Base(cleaned), "skills") {
		cleaned = filepath.Dir(cleaned)
	}

	switch strings.ToLower(filepath.Base(cleaned)) {
	case ".claude", "claude", ".codex", "codex", ".agents", "agents":
		return filepath.Dir(cleaned)
	default:
		return cleaned
	}
}

func loadHookRawConfig(compileTarget, compileRoot string) ([]byte, error) {
	path, ok := managedHookConfigPath(compileTarget, compileRoot)
	if !ok {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func managedHookConfigPath(target, root string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "claude":
		return filepath.Join(root, ".claude", "settings.json"), true
	case "codex":
		return filepath.Join(root, ".codex", "config.toml"), true
	default:
		return "", false
	}
}

func managedRuleOwnedFiles(target, root string) []string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "pi":
		return managedpi.OwnedCompilePaths(root)
	default:
		return nil
	}
}

func detectRuleOutputConflicts(req SyncRequest, current TargetSyncSpec, records []managedrules.Record, currentFamily string, currentFiles []adapters.CompiledFile) (map[string]struct{}, error) {
	currentPaths := make(map[string]string, len(currentFiles))
	for _, file := range currentFiles {
		currentPaths[filepath.Clean(file.Path)] = file.Content
	}
	otherCurrentPaths := make(map[string]struct{})

	for _, other := range conflictAnalysisTargets(req) {
		if other.Name == current.Name {
			continue
		}
		otherFamily, otherRoot, ok := resolveRuleTarget(other.Name, other.Target, req.ProjectRoot)
		if !ok {
			continue
		}

		otherFiles, _, err := managedrules.CompileTarget(records, otherFamily, other.Name, otherRoot)
		if err != nil {
			if errors.Is(err, managedrules.ErrUnsupportedTarget) {
				continue
			}
			return nil, err
		}
		for _, otherFile := range otherFiles {
			cleaned := filepath.Clean(otherFile.Path)
			otherCurrentPaths[cleaned] = struct{}{}
			if currentContent, ok := currentPaths[cleaned]; ok && currentContent != otherFile.Content {
				return nil, fmt.Errorf("managed rule output conflict: %s is produced by %s (%s) and %s (%s)", cleaned, current.Name, currentFamily, other.Name, otherFamily)
			}
		}
	}

	return otherCurrentPaths, nil
}

func conflictAnalysisTargets(req SyncRequest) []TargetSyncSpec {
	if len(req.AllTargets) > 0 {
		return req.AllTargets
	}
	return req.Targets
}

func pruneRuleOrphans(target, root string, files []adapters.CompiledFile, otherCurrentPaths map[string]struct{}, state *managedRuleSyncState, dryRun bool) ([]string, error) {
	ownedDir, ok := managedRuleOwnedDir(target, root)
	ownedFiles := managedRuleOwnedFiles(target, root)
	if !ok && len(ownedFiles) == 0 && !managedRuleHasTrackedOutputs(state) {
		return []string{}, nil
	}

	if ok {
		info, err := os.Stat(ownedDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				ok = false
			} else {
				return nil, err
			}
		}
		if ok && !info.IsDir() {
			return nil, fmt.Errorf("managed rules path is not a directory: %s", ownedDir)
		}
	}

	keep := make(map[string]struct{}, len(files))
	for _, file := range files {
		if ok && pathWithinDir(file.Path, ownedDir) {
			keep[filepath.Clean(file.Path)] = struct{}{}
		}
		for _, ownedPath := range ownedFiles {
			if filepath.Clean(file.Path) == ownedPath {
				keep[ownedPath] = struct{}{}
			}
		}
	}
	for path := range otherCurrentPaths {
		keep[path] = struct{}{}
	}

	pruned := make([]string, 0)
	if ok {
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
	}
	if err := pruneTrackedManagedRuleOutputs(root, keep, state, dryRun, &pruned); err != nil {
		return nil, err
	}
	for _, ownedPath := range ownedFiles {
		if _, ok := keep[ownedPath]; ok {
			continue
		}
		info, err := os.Stat(ownedPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if info.IsDir() {
			continue
		}

		pruned = append(pruned, ownedPath)
		if dryRun {
			continue
		}
		if err := os.Remove(ownedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	if dryRun {
		return pruned, nil
	}
	if !ok {
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
