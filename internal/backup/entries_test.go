package backup

import (
	"errors"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
)

func TestMovedEntryName(t *testing.T) {
	if got := MovedEntryName("goose", ".goose/./skills"); got != "goose-moved-2e676f6f73652f736b696c6c73" {
		t.Fatalf("normalized key = %q", got)
	}
	if MovedEntryName("goose", "a/b") == MovedEntryName("goose", "a__b") {
		t.Fatal("slash and literal underscores collide")
	}
}

func TestSkillsEntryDir(t *testing.T) {
	for _, root := range []string{t.TempDir(), t.TempDir()} {
		targets := map[string]config.TargetConfig{
			"goose": {Skills: &config.ResourceTargetConfig{Path: filepath.Join(root, ".agents", "skills")}},
		}
		for _, rel := range []string{".agents/skills", ".goose/skills"} {
			key := "goose"
			if rel == ".goose/skills" {
				key = MovedEntryName("goose", rel)
			}
			want := filepath.Join(root, filepath.FromSlash(rel))
			// Neither folder exists: recovery cannot depend on cleanup eligibility.
			got, err := SkillsEntryDir(root, key, targets)
			if err != nil || got != want {
				t.Fatalf("%s: got %q, %v; want %q", key, got, err, want)
			}
		}
		for _, key := range []string{"missing", "../goose", MovedEntryName("goose", "../escape"), MovedEntryName("goose", "/tmp/escape"), "goose-moved-00"} {
			if _, err := SkillsEntryDir(root, key, targets); err == nil {
				t.Fatalf("accepted undeclared key %q", key)
			}
		}
	}
}

func TestSkillsEntryDir_AmbiguousNames(t *testing.T) {
	root := t.TempDir()
	key := MovedEntryName("goose", ".goose/skills")
	for _, name := range []string{key, "claude-agents"} {
		targets := map[string]config.TargetConfig{
			"goose": {Skills: &config.ResourceTargetConfig{Path: filepath.Join(root, "current")}},
		}
		targets[name] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: filepath.Join(root, "other")}}
		if name == "claude-agents" {
			targets["claude"] = config.TargetConfig{}
		}
		if _, err := SkillsEntryDir(root, name, targets); !errors.Is(err, ErrAmbiguousEntry) {
			t.Fatalf("collision %q: %v", name, err)
		}
	}
}

func TestSkillsEntryDir_DeclarationOrderAndUnresolvedNames(t *testing.T) {
	root := t.TempDir()
	key := MovedEntryName("goose", ".goose/skills")
	for _, names := range [][]string{{"goose", "claude"}, {"claude", "goose"}} {
		targets := make(map[string]config.TargetConfig)
		for _, name := range names {
			targets[name] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: filepath.Join(root, name)}}
		}
		if got, err := SkillsEntryDir(root, key, targets); err != nil || got != filepath.Join(root, ".goose", "skills") {
			t.Fatalf("order %v: %q, %v", names, got, err)
		}
		targets[key] = config.TargetConfig{}
		if _, err := SkillsEntryDir(root, key, targets); !errors.Is(err, ErrAmbiguousEntry) {
			t.Fatalf("unresolved configured name did not reserve alias: %v", err)
		}
	}
}

func TestSkillsEntryDir_AgentsSuffixWithoutAgentDestination(t *testing.T) {
	root := t.TempDir()
	targets := map[string]config.TargetConfig{
		"local": {Skills: &config.ResourceTargetConfig{Path: filepath.Join(root, "local")}},
	}
	dest := filepath.Join(root, "local-agents")
	targets["local-agents"] = config.TargetConfig{Skills: &config.ResourceTargetConfig{Path: dest}}
	if got, err := SkillsEntryDir(root, "local-agents", targets); err != nil || got != dest {
		t.Fatalf("non-agent suffix was treated as an agent destination: %q, %v", got, err)
	}
}
