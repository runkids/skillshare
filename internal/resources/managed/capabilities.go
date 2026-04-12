package managed

import (
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	managedpi "skillshare/internal/resources/managed/pi"
)

type ResourceKind string

const (
	ResourceKindRules ResourceKind = "rules"
	ResourceKindHooks ResourceKind = "hooks"
)

type FamilySpec struct {
	Name                 string
	SupportsRules        bool
	SupportsHooks        bool
	CompatibleTargets    []string
	RuleInstructionFiles []string
	HookConfigFiles      []string
}

type TargetClassification struct {
	Name        string   `json:"name"`
	RulesFamily string   `json:"rulesFamily,omitempty"`
	HooksFamily string   `json:"hooksFamily,omitempty"`
	Status      []string `json:"status"`
}

type CapabilitySnapshotPayload struct {
	Families map[string]FamilySpec
	Targets  map[string]TargetClassification
}

type capabilityRegistry struct {
	families map[string]FamilySpec
	targets  map[string]TargetClassification
}

var managedCapabilities = newCapabilityRegistry()

func newCapabilityRegistry() capabilityRegistry {
	families := map[string]FamilySpec{
		"claude": {
			Name:                 "claude",
			SupportsRules:        true,
			SupportsHooks:        true,
			CompatibleTargets:    []string{"claude", "claude-code"},
			RuleInstructionFiles: []string{"CLAUDE.md", ".claude/rules/**"},
			HookConfigFiles:      []string{".claude/settings.json"},
		},
		"codex": {
			Name:                 "codex",
			SupportsRules:        true,
			SupportsHooks:        true,
			CompatibleTargets:    []string{"codex", "universal", "agents"},
			RuleInstructionFiles: []string{"AGENTS.md", ".codex/AGENTS.md"},
			HookConfigFiles:      []string{".codex/config.toml", ".codex/hooks.json"},
		},
		"gemini": {
			Name:                 "gemini",
			SupportsRules:        true,
			SupportsHooks:        true,
			CompatibleTargets:    []string{"gemini", "gemini-cli"},
			RuleInstructionFiles: []string{"GEMINI.md", ".gemini/rules/**"},
			HookConfigFiles:      []string{".gemini/settings.json"},
		},
		"pi": {
			Name:                 "pi",
			SupportsRules:        true,
			SupportsHooks:        false,
			CompatibleTargets:    []string{"pi"},
			RuleInstructionFiles: managedpi.RuleInstructionFiles(),
		},
	}

	targets := map[string]TargetClassification{
		"claude": {
			Name:        "claude",
			RulesFamily: "claude",
			HooksFamily: "claude",
			Status:      []string{"rules", "hooks"},
		},
		"codex": {
			Name:        "codex",
			RulesFamily: "codex",
			HooksFamily: "codex",
			Status:      []string{"rules", "hooks"},
		},
		"universal": {
			Name:        "universal",
			RulesFamily: "codex",
			HooksFamily: "codex",
			Status:      []string{"rules", "hooks"},
		},
		"gemini": {
			Name:        "gemini",
			RulesFamily: "gemini",
			HooksFamily: "gemini",
			Status:      []string{"rules", "hooks"},
		},
		"pi": {
			Name:        "pi",
			RulesFamily: "pi",
			Status:      []string{"rules"},
		},
	}

	return capabilityRegistry{
		families: families,
		targets:  targets,
	}
}

func ResolveManagedFamily(kind ResourceKind, targetName, targetPath string) (string, bool) {
	switch normalizeResourceKind(kind) {
	case ResourceKindRules, ResourceKindHooks:
	default:
		return "", false
	}

	cleanName := strings.TrimSpace(targetName)
	if cleanName != "" {
		if canonical, ok := config.CanonicalTargetName(cleanName); ok {
			if classification, ok := managedCapabilities.targets[canonical]; ok {
				if family, ok := classification.familyForKind(kind); ok {
					return family, true
				}
				return "", false
			}
			return "", false
		}

		// Preserve compatibility for custom, non-canonical targets that point at
		// a native managed path surface intentionally.
		for family, spec := range managedCapabilities.families {
			if !spec.supportsKind(kind) {
				continue
			}
			if config.MatchesTargetName(family, cleanName) {
				return family, true
			}
		}
	}

	family := managedCapabilities.familyForPath(targetPath)
	if family == "" {
		return "", false
	}
	if !managedCapabilities.supportsKind(family, kind) {
		return "", false
	}
	return family, true
}

func CapabilitySnapshot() CapabilitySnapshotPayload {
	targets := config.DefaultTargets()
	snapshotTargets := make(map[string]TargetClassification, len(targets))
	compatibleTargets := make(map[string][]string, len(managedCapabilities.families))
	for name, target := range targets {
		classification := managedCapabilities.classificationForTarget(name, target.Path)
		snapshotTargets[name] = classification
		if classification.RulesFamily != "" {
			compatibleTargets[classification.RulesFamily] = append(compatibleTargets[classification.RulesFamily], name)
		}
		if classification.HooksFamily != "" && classification.HooksFamily != classification.RulesFamily {
			compatibleTargets[classification.HooksFamily] = append(compatibleTargets[classification.HooksFamily], name)
		}
	}

	snapshotFamilies := make(map[string]FamilySpec, len(managedCapabilities.families))
	for name, family := range managedCapabilities.families {
		clone := family
		clone.CompatibleTargets = dedupeSortedStrings(compatibleTargets[name])
		snapshotFamilies[name] = clone
	}

	return CapabilitySnapshotPayload{
		Families: snapshotFamilies,
		Targets:  snapshotTargets,
	}
}

func normalizeResourceKind(kind ResourceKind) ResourceKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case string(ResourceKindRules):
		return ResourceKindRules
	case string(ResourceKindHooks):
		return ResourceKindHooks
	default:
		return ""
	}
}

func (r capabilityRegistry) classificationForTarget(name, targetPath string) TargetClassification {
	classification := TargetClassification{Name: name}
	if explicit, ok := r.targets[name]; ok {
		classification = explicit
	}

	if classification.RulesFamily == "" {
		if family, ok := ResolveManagedFamily(ResourceKindRules, name, targetPath); ok {
			classification.RulesFamily = family
		}
	}
	if classification.HooksFamily == "" {
		if family, ok := ResolveManagedFamily(ResourceKindHooks, name, targetPath); ok {
			classification.HooksFamily = family
		}
	}

	switch {
	case classification.RulesFamily != "" && classification.HooksFamily != "":
		classification.Status = []string{"rules", "hooks"}
	case classification.RulesFamily != "":
		classification.Status = []string{"rules"}
	case classification.HooksFamily != "":
		classification.Status = []string{"hooks"}
	default:
		classification.Status = []string{"skills"}
	}
	return classification
}

func (r capabilityRegistry) familyForPath(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return ""
	}

	base := strings.ToLower(filepath.Base(cleaned))
	parent := strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	grandparent := strings.ToLower(filepath.Base(filepath.Dir(filepath.Dir(cleaned))))
	if base == "skills" || base == "agents" || base == "rules" {
		if parent == "agent" && grandparent == ".pi" {
			return "pi"
		}
		base = parent
	}

	switch base {
	case ".claude", "claude":
		return "claude"
	case ".codex", "codex", ".agents", "agents":
		return "codex"
	case ".gemini", "gemini":
		return "gemini"
	case ".pi", "pi":
		return "pi"
	default:
		return ""
	}
}

func (r capabilityRegistry) supportsKind(family string, kind ResourceKind) bool {
	spec, ok := r.families[family]
	if !ok {
		return false
	}
	switch kind {
	case ResourceKindRules:
		return spec.SupportsRules
	case ResourceKindHooks:
		return spec.SupportsHooks
	default:
		return false
	}
}

func (s FamilySpec) supportsKind(kind ResourceKind) bool {
	switch normalizeResourceKind(kind) {
	case ResourceKindRules:
		return s.SupportsRules
	case ResourceKindHooks:
		return s.SupportsHooks
	default:
		return false
	}
}

func (c TargetClassification) familyForKind(kind ResourceKind) (string, bool) {
	switch normalizeResourceKind(kind) {
	case ResourceKindRules:
		if c.RulesFamily != "" {
			return c.RulesFamily, true
		}
	case ResourceKindHooks:
		if c.HooksFamily != "" {
			return c.HooksFamily, true
		}
	}
	return "", false
}

func dedupeSortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
