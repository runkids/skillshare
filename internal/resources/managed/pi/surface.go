package pi

import (
	"path"
	"path/filepath"
	"strings"
)

const (
	ManagedAgentsID       = "pi/AGENTS.md"
	ManagedSystemID       = "pi/SYSTEM.md"
	ManagedAppendSystemID = "pi/APPEND_SYSTEM.md"
)

var managedRuleSurfaces = []ruleSurface{
	{
		id:              ManagedAgentsID,
		bareName:        "AGENTS.md",
		instructionFile: "AGENTS.md",
	},
	{
		id:                  ManagedSystemID,
		bareName:            "SYSTEM.md",
		instructionFile:     ".pi/SYSTEM.md",
		discoveryProjectRel: ".pi/SYSTEM.md",
	},
	{
		id:                  ManagedAppendSystemID,
		bareName:            "APPEND_SYSTEM.md",
		instructionFile:     ".pi/APPEND_SYSTEM.md",
		discoveryProjectRel: ".pi/APPEND_SYSTEM.md",
	},
}

type ruleSurface struct {
	id                  string
	bareName            string
	instructionFile     string
	discoveryProjectRel string
}

func ManagedRuleIDs() []string {
	out := make([]string, 0, len(managedRuleSurfaces))
	for _, surface := range managedRuleSurfaces {
		out = append(out, surface.id)
	}
	return out
}

func RuleInstructionFiles() []string {
	out := make([]string, 0, len(managedRuleSurfaces))
	for _, surface := range managedRuleSurfaces {
		out = append(out, surface.instructionFile)
	}
	return out
}

func IsManagedRuleID(id string) bool {
	_, ok := surfaceByManagedID(id)
	return ok
}

func NormalizeManagedRuleID(ref string) (string, bool) {
	normalized := path.Clean(strings.ReplaceAll(strings.TrimSpace(ref), "\\", "/"))
	for _, surface := range managedRuleSurfaces {
		if normalized == surface.id || normalized == surface.bareName {
			return surface.id, true
		}
	}
	return "", false
}

func ManagedRuleIDForDiscoveredPath(filePath string) (string, bool) {
	normalized := strings.ToLower(filepath.ToSlash(strings.TrimSpace(filePath)))
	switch {
	case strings.HasSuffix(normalized, "/.pi/system.md"):
		return ManagedSystemID, true
	case strings.HasSuffix(normalized, "/.pi/append_system.md"):
		return ManagedAppendSystemID, true
	}
	return "", false
}

func DiscoveryProjectPaths(projectRoot string) []string {
	out := make([]string, 0, len(managedRuleSurfaces))
	for _, surface := range managedRuleSurfaces {
		if surface.discoveryProjectRel == "" {
			continue
		}
		out = append(out, filepath.Join(projectRoot, filepath.FromSlash(surface.discoveryProjectRel)))
	}
	return out
}

func CompilePath(root, id string) (string, bool) {
	surface, ok := surfaceByManagedID(id)
	if !ok {
		return "", false
	}
	switch surface.id {
	case ManagedAgentsID:
		return filepath.Join(root, "AGENTS.md"), true
	case ManagedSystemID:
		return filepath.Join(outputBaseDir(root), "SYSTEM.md"), true
	case ManagedAppendSystemID:
		return filepath.Join(outputBaseDir(root), "APPEND_SYSTEM.md"), true
	default:
		return "", false
	}
}

func outputBaseDir(root string) string {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if cleaned == "" || cleaned == "." {
		return cleaned
	}

	base := strings.ToLower(filepath.Base(cleaned))
	parent := strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	if base == ".pi" || (base == "agent" && parent == ".pi") {
		return cleaned
	}
	return filepath.Join(cleaned, ".pi")
}

func surfaceByManagedID(id string) (ruleSurface, bool) {
	for _, surface := range managedRuleSurfaces {
		if id == surface.id {
			return surface, true
		}
	}
	return ruleSurface{}, false
}
