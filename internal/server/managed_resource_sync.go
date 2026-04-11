package server

import (
	"errors"
	"fmt"
	"strings"

	"skillshare/internal/config"
	managed "skillshare/internal/resources/managed"
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
	rows := managed.Sync(managed.SyncRequest{
		ProjectRoot: s.managedRulesProjectRoot(),
		DryRun:      dryRun,
		Resources:   managed.ResourceSet{Rules: true},
		Targets: []managed.TargetSyncSpec{{
			Name:   name,
			Target: target,
		}},
	})
	return syncTargetResultFromManagedRows(name, "rules", rows)
}

func (s *Server) syncManagedHooksForTarget(name string, target config.TargetConfig, dryRun bool) (syncTargetResult, error) {
	rows := managed.Sync(managed.SyncRequest{
		ProjectRoot: s.managedHooksProjectRoot(),
		DryRun:      dryRun,
		Resources:   managed.ResourceSet{Hooks: true},
		Targets: []managed.TargetSyncSpec{{
			Name:   name,
			Target: target,
		}},
	})
	return syncTargetResultFromManagedRows(name, "hooks", rows)
}

func syncTargetResultFromManagedRows(target, resource string, rows []managed.SyncResult) (syncTargetResult, error) {
	result := newSyncTargetResult(target, resource)
	var errs []string

	for _, row := range rows {
		if row.Resource != "" {
			result.Resource = row.Resource
		}
		result.Updated = append(result.Updated, row.Updated...)
		result.Skipped = append(result.Skipped, row.Skipped...)
		result.Pruned = append(result.Pruned, row.Pruned...)
		if row.Err != nil {
			errs = append(errs, row.Err.Error())
		}
	}

	if len(errs) > 0 {
		return result, errors.New(strings.Join(errs, "; "))
	}
	return result, nil
}
