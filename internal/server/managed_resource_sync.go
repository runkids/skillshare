package server

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

type serverSyncResources struct {
	skills bool
	rules  bool
	hooks  bool
}

func defaultServerSyncResources() serverSyncResources {
	return serverSyncResources{
		skills: true,
		rules:  true,
		hooks:  true,
	}
}

func parseServerSyncResources(values []string) (serverSyncResources, error) {
	if len(values) == 0 {
		return defaultServerSyncResources(), nil
	}

	var resources serverSyncResources
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			switch strings.ToLower(strings.TrimSpace(part)) {
			case "":
				continue
			case "skills":
				resources.skills = true
			case "rules":
				resources.rules = true
			case "hooks":
				resources.hooks = true
			default:
				return serverSyncResources{}, fmt.Errorf("unsupported sync resource %q", strings.TrimSpace(part))
			}
		}
	}

	if !resources.skills && !resources.rules && !resources.hooks {
		return serverSyncResources{}, errors.New("at least one sync resource is required")
	}

	return resources, nil
}

func (s *Server) syncManagedRulesForTarget(name string, target config.TargetConfig, dryRun bool) (syncTargetResult, error) {
	result := newSyncTargetResult(name, "rules")

	compileTarget, ok := resolveManagedRulePreviewTool(name, target.SkillsConfig().Path)
	if !ok {
		return result, nil
	}

	store := managedrules.NewStore(s.managedRulesProjectRoot())
	records, err := store.List()
	if err != nil {
		return result, fmt.Errorf("list managed rules: %w", err)
	}

	_, compileRoot := s.resolveManagedRulePreviewTarget(name, target)
	files, _, err := managedrules.CompileTarget(records, compileTarget, compileRoot)
	if err != nil {
		if errors.Is(err, managedrules.ErrUnsupportedTarget) {
			return result, nil
		}
		return result, fmt.Errorf("compile managed rules: %w", err)
	}

	updated, skipped, err := apply.CompiledFiles(files, dryRun)
	if err != nil {
		return result, fmt.Errorf("apply managed rules: %w", err)
	}
	pruned, err := pruneManagedRuleOrphans(compileTarget, compileRoot, files, dryRun)
	if err != nil {
		return result, fmt.Errorf("prune managed rules: %w", err)
	}

	result.Updated = updated
	result.Skipped = skipped
	result.Pruned = pruned
	return result, nil
}

func (s *Server) syncManagedHooksForTarget(name string, target config.TargetConfig, dryRun bool) (syncTargetResult, error) {
	result := newSyncTargetResult(name, "hooks")

	compileTarget, compileRoot, ok := s.resolveManagedHookPreviewTarget(name, target)
	if !ok {
		return result, nil
	}

	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	records, err := store.List()
	if err != nil {
		return result, fmt.Errorf("list managed hooks: %w", err)
	}

	rawConfig, err := loadManagedHookRawConfig(compileTarget, compileRoot)
	if err != nil {
		return result, fmt.Errorf("load managed hook config: %w", err)
	}
	files, _, err := managedhooks.CompileTarget(records, compileTarget, compileRoot, rawConfig)
	if err != nil {
		return result, fmt.Errorf("compile managed hooks: %w", err)
	}

	updated, skipped, err := apply.CompiledFiles(files, dryRun)
	if err != nil {
		return result, fmt.Errorf("apply managed hooks: %w", err)
	}

	result.Updated = updated
	result.Skipped = skipped
	return result, nil
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
