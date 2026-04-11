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

func (s *Server) syncManagedResourcesForTarget(name string, target config.TargetConfig, resources serverSyncResources, dryRun bool) ([]syncTargetResult, error) {
	rows := managed.Sync(managed.SyncRequest{
		ProjectRoot: s.managedRulesProjectRoot(),
		DryRun:      dryRun,
		Resources: managed.ResourceSet{
			Rules: resources.rules,
			Hooks: resources.hooks,
		},
		Targets: []managed.TargetSyncSpec{{
			Name:   name,
			Target: target,
		}},
	})
	return syncTargetResultsFromManagedRows(name, rows)
}

func syncTargetResultsFromManagedRows(target string, rows []managed.SyncResult) ([]syncTargetResult, error) {
	results := make([]syncTargetResult, 0, len(rows))
	var errs []string

	for _, row := range rows {
		result := newSyncTargetResult(target, row.Resource)
		result.Updated = append(result.Updated, row.Updated...)
		result.Skipped = append(result.Skipped, row.Skipped...)
		result.Pruned = append(result.Pruned, row.Pruned...)
		results = append(results, result)
		if row.Err != nil {
			errs = append(errs, row.Err.Error())
		}
	}

	if len(errs) > 0 {
		return results, errors.New(strings.Join(errs, "; "))
	}
	return results, nil
}
