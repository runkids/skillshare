package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"skillshare/internal/resource"
)

func findAgent(agentsSource, name string) (resource.DiscoveredResource, error) {
	discovered, err := resource.AgentKind{}.DiscoverAll(agentsSource)
	if err != nil {
		return resource.DiscoveredResource{}, fmt.Errorf("failed to discover agents: %w", err)
	}
	if d, ok := resource.FindAgent(discovered, name); ok {
		return d, nil
	}
	return resource.DiscoveredResource{}, fmt.Errorf("agent not found: %s", name)
}

func resolveAgentResource(agentsSource, name string) (resource.DiscoveredResource, error) {
	return findAgent(agentsSource, name)
}

func agentMetaKey(relPath string) string {
	return strings.TrimSuffix(filepath.ToSlash(relPath), ".md")
}
