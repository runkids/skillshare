package main

import (
	"maps"
	"slices"
	"strings"

	"skillshare/internal/plugin"
	"skillshare/internal/ui"
)

func printPluginDiscovery(d *plugin.Discovery) {
	for _, warning := range d.Warnings {
		ui.Warning("%s", warning)
	}
	names := make([]string, len(d.Candidates))
	for i, c := range d.Candidates {
		names[i] = c.Name
	}
	width := ui.RowWidth(names...)
	for _, c := range d.Candidates {
		mark := ui.MarkNone
		if c.Problem != "" {
			mark = ui.MarkFail
		}
		groups := map[string][]string{}
		for _, target := range c.Targets {
			path := c.TargetInfo[target].Path
			if path == "" {
				path = c.Path
			}
			groups[path] = append(groups[path], target)
		}
		if len(groups) == 0 {
			groups[c.Path] = c.Targets
		}
		for _, path := range slices.Sorted(maps.Keys(groups)) {
			components := []string{}
			for _, target := range groups[path] {
				for _, component := range c.TargetInfo[target].Components {
					if !slices.Contains(components, component) {
						components = append(components, component)
					}
				}
			}
			slices.Sort(components)
			value := strings.Join(groups[path], ", ")
			if len(components) > 0 {
				value += ui.DimText(" · " + strings.Join(components, ", "))
			}
			if len(groups) > 1 {
				value += ui.DimText(" · " + path)
			}
			ui.Row(mark, c.Name, value, width)
		}
		if c.Problem != "" {
			ui.Note(c.Problem)
		}
	}
}
