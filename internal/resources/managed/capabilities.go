package managed

import (
	"path/filepath"
	"strings"

	"skillshare/internal/config"
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
			RuleInstructionFiles: []string{"AGENTS.md", ".pi/SYSTEM.md", ".pi/APPEND_SYSTEM.md"},
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
	for name, target := range targets {
		snapshotTargets[name] = managedCapabilities.classificationForTarget(name, target.Path)
	}

	snapshotFamilies := make(map[string]FamilySpec, len(managedCapabilities.families))
	for name, family := range managedCapabilities.families {
		snapshotFamilies[name] = family
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
	if base == "skills" || base == "agents" || base == "rules" {
		base = strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
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
