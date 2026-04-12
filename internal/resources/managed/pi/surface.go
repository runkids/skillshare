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
		id:                 ManagedAgentsID,
		bareName:           "AGENTS.md",
		instructionFile:    "AGENTS.md",
		projectCompileRel:  "AGENTS.md",
		globalCompileRel:   "AGENTS.md",
		discoveryGlobalRel: ".pi/agent/AGENTS.md",
	},
	{
		id:                  ManagedSystemID,
		bareName:            "SYSTEM.md",
		instructionFile:     ".pi/SYSTEM.md",
		projectCompileRel:   ".pi/SYSTEM.md",
		globalCompileRel:    "SYSTEM.md",
		discoveryProjectRel: ".pi/SYSTEM.md",
		discoveryGlobalRel:  ".pi/agent/SYSTEM.md",
	},
	{
		id:                  ManagedAppendSystemID,
		bareName:            "APPEND_SYSTEM.md",
		instructionFile:     ".pi/APPEND_SYSTEM.md",
		projectCompileRel:   ".pi/APPEND_SYSTEM.md",
		globalCompileRel:    "APPEND_SYSTEM.md",
		discoveryProjectRel: ".pi/APPEND_SYSTEM.md",
		discoveryGlobalRel:  ".pi/agent/APPEND_SYSTEM.md",
	},
}

type ruleSurface struct {
	id                  string
	bareName            string
	instructionFile     string
	projectCompileRel   string
	globalCompileRel    string
	discoveryProjectRel string
	discoveryGlobalRel  string
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
	for _, surface := range managedRuleSurfaces {
		for _, rel := range []string{surface.discoveryProjectRel, surface.discoveryGlobalRel} {
			if rel == "" {
				continue
			}
			normalizedRel := strings.ToLower(filepath.ToSlash(rel))
			if normalized == normalizedRel || strings.HasSuffix(normalized, "/"+normalizedRel) {
				return surface.id, true
			}
		}
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

func DiscoveryGlobalPaths(home string) []string {
	out := make([]string, 0, len(managedRuleSurfaces))
	for _, surface := range managedRuleSurfaces {
		if surface.discoveryGlobalRel == "" {
			continue
		}
		out = append(out, filepath.Join(home, filepath.FromSlash(surface.discoveryGlobalRel)))
	}
	return out
}

func OwnedCompilePaths(root string) []string {
	ids := []string{ManagedSystemID, ManagedAppendSystemID}
	if isGlobalOutputRoot(root) {
		ids = append([]string{ManagedAgentsID}, ids...)
	}

	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if path, ok := CompilePath(root, id); ok {
			out = append(out, filepath.Clean(path))
		}
	}
	return out
}

func CompilePath(root, id string) (string, bool) {
	surface, ok := surfaceByManagedID(id)
	if !ok {
		return "", false
	}
	compileRel := surface.projectCompileRel
	if isGlobalOutputRoot(root) {
		compileRel = surface.globalCompileRel
	}
	if compileRel == "" {
		return "", false
	}
	return filepath.Join(root, filepath.FromSlash(compileRel)), true
}

func isGlobalOutputRoot(root string) bool {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if cleaned == "" || cleaned == "." {
		return false
	}

	base := strings.ToLower(filepath.Base(cleaned))
	parent := strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	return base == ".pi" || (base == "agent" && parent == ".pi")
}

func surfaceByManagedID(id string) (ruleSurface, bool) {
	for _, surface := range managedRuleSurfaces {
		if id == surface.id {
			return surface, true
		}
	}
	return ruleSurface{}, false
}
