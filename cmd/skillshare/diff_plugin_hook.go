package main

import (
	"os"
	hookpkg "skillshare/internal/hooks"
	pluginpkg "skillshare/internal/plugins"
)

type pluginDiffJSONEntry struct {
	Name   string   `json:"name"`
	Target string   `json:"target"`
	Synced bool     `json:"synced"`
	Items  []string `json:"items,omitempty"`
}

type hookDiffJSONEntry struct {
	Name   string   `json:"name"`
	Target string   `json:"target"`
	Synced bool     `json:"synced"`
	Items  []string `json:"items,omitempty"`
}

func collectPluginDiff(sourceRoot, projectRoot string) []pluginDiffJSONEntry {
	bundles, _ := pluginpkg.Discover(sourceRoot)
	var out []pluginDiffJSONEntry
	for _, bundle := range bundles {
		for _, target := range pluginpkg.SupportedTargets(bundle) {
			rendered := pluginpkg.RenderRoot(projectRoot, bundle.Name, target)
			_, err := os.Stat(rendered)
			out = append(out, pluginDiffJSONEntry{
				Name:   bundle.Name,
				Target: target,
				Synced: err == nil,
				Items:  diffItemsForMissing(err, rendered),
			})
		}
	}
	return out
}

func collectHookDiff(sourceRoot, projectRoot string) []hookDiffJSONEntry {
	bundles, _ := hookpkg.Discover(sourceRoot)
	var out []hookDiffJSONEntry
	for _, bundle := range bundles {
		for _, target := range hookpkg.SupportedTargets(bundle) {
			root := hookpkg.RenderRoot(projectRoot, bundle.Name, target)
			_, err := os.Stat(root)
			out = append(out, hookDiffJSONEntry{
				Name:   bundle.Name,
				Target: target,
				Synced: err == nil,
				Items:  diffItemsForMissing(err, root),
			})
		}
	}
	return out
}

func diffItemsForMissing(err error, path string) []string {
	if err == nil {
		return nil
	}
	return []string{"missing rendered state: " + path}
}
