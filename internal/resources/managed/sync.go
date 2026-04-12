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

	updated, skipped, err := apply.CompiledFiles(files, req.DryRun)
	if err != nil {
		result.Err = fmt.Errorf("apply managed rules: %w", err)
		return result, true
	}
	pruned, err := pruneRuleOrphans(compileTarget, compileRoot, files, req.DryRun)
	if err != nil {
		result.Err = fmt.Errorf("prune managed rules: %w", err)
		return result, true
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
	return family, managedRuleGlobalPreviewRoot(sc.Path), true
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

func managedRuleGlobalPreviewRoot(targetPath string) string {
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

func pruneRuleOrphans(target, root string, files []adapters.CompiledFile, dryRun bool) ([]string, error) {
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
