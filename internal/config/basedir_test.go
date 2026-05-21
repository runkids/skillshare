package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBaseDir_DefaultFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, _ := os.UserHomeDir()

	got := BaseDir()

	if runtime.GOOS == "windows" {
		winDir, _ := os.UserConfigDir()
		want := filepath.Join(winDir, "skillshare")
		if got != want {
			t.Errorf("BaseDir() = %q, want %q", got, want)
		}
	} else {
		want := filepath.Join(home, ".config", "skillshare")
		if got != want {
			t.Errorf("BaseDir() = %q, want %q", got, want)
		}
	}
}

func TestBaseDir_RespectsXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	got := BaseDir()
	want := filepath.Join("/custom/config", "skillshare")
	if got != want {
		t.Errorf("BaseDir() = %q, want %q", got, want)
	}
}

func TestConfigPath_RespectsXDGConfigHome(t *testing.T) {
	t.Setenv("SKILLSHARE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	got := ConfigPath()
	want := filepath.Join("/custom/config", "skillshare", "config.yaml")
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestEffectiveAgentsSource_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	cfg := &Config{}

	got := cfg.EffectiveAgentsSource()
	want := filepath.Join(BaseDir(), "agents")
	if got != want {
		t.Errorf("EffectiveAgentsSource() = %q, want %q", got, want)
	}
}

func TestEffectiveAgentsSource_Explicit(t *testing.T) {
	cfg := &Config{AgentsSource: "/custom/agents"}

	got := cfg.EffectiveAgentsSource()
	if got != "/custom/agents" {
		t.Errorf("EffectiveAgentsSource() = %q, want %q", got, "/custom/agents")
	}
}

func TestEffectiveAgentsSource_SourcesPrefersOverLegacy(t *testing.T) {
	cfg := &Config{
		AgentsSource: "/legacy/agents",
		Sources:      GlobalSources{Agents: "/new/agents"},
	}

	got := cfg.EffectiveAgentsSource()
	if got != "/new/agents" {
		t.Errorf("EffectiveAgentsSource() = %q, want %q", got, "/new/agents")
	}
}

func TestEffectiveSkillsSource_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	cfg := &Config{}

	got := cfg.EffectiveSkillsSource()
	want := filepath.Join(BaseDir(), "skills")
	if got != want {
		t.Errorf("EffectiveSkillsSource() = %q, want %q", got, want)
	}
}

func TestEffectiveSkillsSource_LegacySource(t *testing.T) {
	cfg := &Config{Source: "/legacy/skills"}

	got := cfg.EffectiveSkillsSource()
	if got != "/legacy/skills" {
		t.Errorf("EffectiveSkillsSource() = %q, want %q", got, "/legacy/skills")
	}
}

func TestEffectiveSkillsSource_SourcesPrefersOverLegacy(t *testing.T) {
	cfg := &Config{
		Source:  "/legacy/skills",
		Sources: GlobalSources{Skills: "/new/skills"},
	}

	got := cfg.EffectiveSkillsSource()
	if got != "/new/skills" {
		t.Errorf("EffectiveSkillsSource() = %q, want %q", got, "/new/skills")
	}
}

func TestEffectiveExtrasSource_DerivedFromSkills(t *testing.T) {
	cfg := &Config{Source: "/work/skills"}

	got := cfg.EffectiveExtrasSource()
	want := filepath.Join("/work", "extras")
	if got != want {
		t.Errorf("EffectiveExtrasSource() = %q, want %q", got, want)
	}
}

func TestEffectiveExtrasSource_LegacyExtrasSource(t *testing.T) {
	cfg := &Config{
		Source:       "/work/skills",
		ExtrasSource: "/custom/extras",
	}

	got := cfg.EffectiveExtrasSource()
	if got != "/custom/extras" {
		t.Errorf("EffectiveExtrasSource() = %q, want %q", got, "/custom/extras")
	}
}

func TestEffectiveExtrasSource_SourcesPrefersOverLegacy(t *testing.T) {
	cfg := &Config{
		Source:       "/work/skills",
		ExtrasSource: "/legacy/extras",
		Sources:      GlobalSources{Extras: "/new/extras"},
	}

	got := cfg.EffectiveExtrasSource()
	if got != "/new/extras" {
		t.Errorf("EffectiveExtrasSource() = %q, want %q", got, "/new/extras")
	}
}

func TestConfigPath_SKILLSHARECONFIGTakesPriority(t *testing.T) {
	t.Setenv("SKILLSHARE_CONFIG", "/override/config.yaml")
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	got := ConfigPath()
	want := "/override/config.yaml"
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}
