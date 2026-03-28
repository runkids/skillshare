package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillEntry_EffectiveKind_Default(t *testing.T) {
	e := SkillEntry{Name: "test", Source: "s"}
	if e.EffectiveKind() != "skill" {
		t.Fatalf("EffectiveKind() = %q, want skill", e.EffectiveKind())
	}
}

func TestSkillEntry_EffectiveKind_Explicit(t *testing.T) {
	e := SkillEntry{Name: "test", Source: "s", Kind: "agent"}
	if e.EffectiveKind() != "agent" {
		t.Fatalf("EffectiveKind() = %q, want agent", e.EffectiveKind())
	}
}

func TestRegistry_KindField_Persisted(t *testing.T) {
	dir := t.TempDir()
	reg := &Registry{
		Skills: []SkillEntry{
			{Name: "my-agent", Source: "test", Kind: "agent"},
			{Name: "my-skill", Source: "test2"},
		},
	}
	if err := reg.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(loaded.Skills) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Skills))
	}

	// Find agent entry
	var agent, skill *SkillEntry
	for i := range loaded.Skills {
		if loaded.Skills[i].Name == "my-agent" {
			agent = &loaded.Skills[i]
		}
		if loaded.Skills[i].Name == "my-skill" {
			skill = &loaded.Skills[i]
		}
	}

	if agent == nil || agent.Kind != "agent" {
		t.Fatalf("agent entry kind = %q", agent.Kind)
	}
	if agent.EffectiveKind() != "agent" {
		t.Fatalf("agent EffectiveKind() = %q", agent.EffectiveKind())
	}

	// Skill without explicit kind
	if skill.Kind != "" {
		t.Fatalf("skill should have empty kind, got %q", skill.Kind)
	}
	if skill.EffectiveKind() != "skill" {
		t.Fatalf("skill EffectiveKind() = %q", skill.EffectiveKind())
	}
}

func TestRegistry_OldFormat_NoKind(t *testing.T) {
	dir := t.TempDir()
	// Write old-format registry without kind field
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(`skills:
  - name: old-skill
    source: test
`), 0644)

	loaded, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if loaded.Skills[0].EffectiveKind() != "skill" {
		t.Fatalf("old format EffectiveKind() = %q", loaded.Skills[0].EffectiveKind())
	}
}
