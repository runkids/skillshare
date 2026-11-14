package skillserve

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/profile"
)

func hostedSelector(name string, include ...string) profile.Selector {
	return profile.Resolve(profile.Catalog{Definitions: map[string]profile.Definition{
		name: {Name: name, Usable: true, Skills: &profile.SkillSelection{Include: include, IncludePresent: true}},
	}}, profile.State{Version: 1, Active: name}, "fixture.yaml", nil)
}

func catalogURIs(c *Catalog) []string {
	var uris []string
	for _, s := range c.Skills {
		uris = append(uris, s.URI)
	}
	return uris
}

// Omitting explicit membership leaks the other account's skills, with or without a target.
func TestHostedCatalog_ExplicitSelection(t *testing.T) {
	src := t.TempDir()
	for _, n := range []string{"alpha", "beta", "shared"} {
		writeFile(t, filepath.Join(src, n, "SKILL.md"), skillMD(n))
	}
	for _, name := range []string{"writing", "coding"} {
		own, foreign := "alpha", "beta"
		if name == "coding" {
			own, foreign = foreign, own
		}
		selector := hostedSelector(name, own, "shared")
		local := hostedSelector("local", foreign)
		for _, targeted := range []bool{false, true} {
			b := &Builder{Source: src, Profile: &selector}
			want := []string{"skill://" + own + "/SKILL.md", "skill://shared/SKILL.md"}
			sc := config.ResourceTargetConfig{Exclude: []string{"shared"}, Profile: local}
			if targeted {
				b.Target, b.Skills = "claude", &sc
				want = want[:1]
			}
			b.ResolveProfile = func() (profile.Selector, error) { return local, errors.New("local state must not be read") }
			c, err := b.Build()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(catalogURIs(c), want) {
				t.Fatalf("%s target=%t: got %v want %v", name, targeted, catalogURIs(c), want)
			}
			if _, err := c.Read("skill://" + foreign + "/SKILL.md"); err == nil {
				t.Fatal("foreign file exposed")
			}
			if sc.Profile.Name() != "local" {
				t.Fatal("mutated caller target")
			}
		}
	}
}

func TestHostedCatalog_ContainmentAndUnavailable(t *testing.T) {
	src := t.TempDir()
	for _, n := range []string{"alpha", "off", "ignored", "suite", "suite/child", "_repo/skills/claude/fix", "_repo/skills/codex/fix"} {
		writeFile(t, filepath.Join(src, n, "SKILL.md"), skillMD(filepath.Base(n)))
	}
	writeFile(t, filepath.Join(src, "off/.skillshare-disabled"), "")
	writeFile(t, filepath.Join(src, ".skillignore"), "ignored\noff\n")
	for _, tc := range []struct {
		name          string
		include, want []string
	}{
		{"empty", []string{}, nil},
		{"hidden", []string{"alpha", "off", "ignored", "suite"}, []string{"skill://alpha/SKILL.md"}},
		{"variants", []string{"*fix"}, []string{"skill://_repo/skills/claude/fix/SKILL.md", "skill://_repo/skills/codex/fix/SKILL.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sel := hostedSelector(tc.name, tc.include...)
			c, err := (&Builder{Source: src, Profile: &sel}).Build()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(catalogURIs(c), tc.want) {
				t.Fatalf("got %v want %v", catalogURIs(c), tc.want)
			}
			for _, uri := range []string{"skill://suite/child/SKILL.md", "skill://off/SKILL.md", "skill://ignored/SKILL.md"} {
				if _, err := c.Read(uri); err == nil {
					t.Fatalf("exposed %s", uri)
				}
			}
		})
	}
	bad := profile.Resolve(profile.Catalog{}, profile.State{Active: "missing"}, "fixture.yaml", nil)
	if _, err := (&Builder{Source: src, Profile: &bad}).Build(); !errors.Is(err, profile.ErrProfileUnavailable) {
		t.Fatalf("unavailable: %v", err)
	}
}
