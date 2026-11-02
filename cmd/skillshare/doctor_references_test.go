package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"skillshare/internal/audit"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
)

func TestDoctorBundledReferences(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	root := t.TempDir()
	skill := filepath.Join(root, "fixture")
	if err := os.Mkdir(skill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# Fixture\n![image](missing.png)\n[external](../outside.md)"), 0644); err != nil {
		t.Fatal(err)
	}
	skills := []sync.DiscoveredSkill{{SourcePath: skill, RelPath: "fixture"}}
	for _, project := range []bool{false, true} {
		t.Run(map[bool]string{false: "global", true: "project"}[project], func(t *testing.T) {
			projectRoot := ""
			if project {
				projectRoot = t.TempDir()
			}
			result := &doctorResult{}
			captureStdout(t, func() { checkSkillReferences(result, skills, projectRoot, nil) })
			if result.errors != 1 || result.warnings != 1 || len(result.checks) != 2 {
				t.Fatalf("doctor parity: %+v", result)
			}
			for _, rule := range []string{"dangling-link", "reference-escapes-root"} {
				found := false
				for _, c := range result.checks {
					if strings.Contains(c.Message, rule) {
						found = true
					}
				}
				if !found {
					t.Errorf("missing %s: %+v", rule, result.checks)
				}
			}
			if !project {
				return
			}
			if err := os.Mkdir(filepath.Join(projectRoot, ".skillshare"), 0755); err != nil {
				t.Fatal(err)
			}
			rules := "rules:\n  - id: reference-escapes-root\n    enabled: false\n  - id: dangling-link\n    enabled: false\n"
			if err := os.WriteFile(filepath.Join(projectRoot, ".skillshare", "audit-rules.yaml"), []byte(rules), 0644); err != nil {
				t.Fatal(err)
			}
			result = &doctorResult{}
			captureStdout(t, func() { checkSkillReferences(result, skills, projectRoot, nil) })
			if len(result.checks) != 1 || result.checks[0].Status != checkPass {
				t.Fatalf("doctor ignored project controls: %+v", result)
			}
		})
	}
}

func TestDoctorBundledReferencesCleanPass(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# Clean\nText[^1]\n\n[^1]: Some note text\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result := &doctorResult{}
	captureStdout(t, func() {
		checkSkillReferences(result, []sync.DiscoveredSkill{{SourcePath: root, RelPath: "clean"}}, "", nil)
	})
	if len(result.checks) != 1 || result.checks[0].Name != "bundled_references" || result.checks[0].Status != checkPass {
		t.Fatalf("clean references: %+v", result.checks)
	}
}

func TestDoctorBundledReferencesFollowedSource(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	root := t.TempDir()
	source, checkout := filepath.Join(root, "source"), filepath.Join(root, "checkout")
	if err := os.MkdirAll(filepath.Join(checkout, "skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	logical := filepath.Join(source, "_linked")
	if err := os.Symlink(checkout, logical); err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "skill", "ok.md"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	ref := filepath.ToSlash(filepath.Join(logical, "skill", "ok.md"))
	if err := os.WriteFile(filepath.Join(checkout, "skill", "SKILL.md"), []byte("[ok]("+ref+")"), 0644); err != nil {
		t.Fatal(err)
	}
	result := &doctorResult{}
	captureStdout(t, func() {
		checkSkillReferences(result, []sync.DiscoveredSkill{{SourcePath: filepath.Join(logical, "skill"), RelPath: "_linked/skill"}}, "", sourcewalk.NewFollow(source, nil))
	})
	if result.errors != 0 || result.warnings != 0 || len(result.checks) != 1 || result.checks[0].Status != checkPass {
		t.Fatalf("followed source: %+v", result.checks)
	}
}

func TestDoctorBundledReferencesDisabledSkipsScanning(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	data := "rules:\n  - id: dangling-link\n    enabled: false\n  - id: reference-escapes-root\n    enabled: false\n  - id: reference-check-incomplete\n    enabled: false\n"
	if err := os.WriteFile(filepath.Join(project, ".skillshare", "audit-rules.yaml"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	result := &doctorResult{}
	captureStdout(t, func() {
		checkSkillReferences(result, []sync.DiscoveredSkill{{SourcePath: filepath.Join(project, "missing"), RelPath: "missing"}}, project, nil)
	})
	if result.errors != 0 || result.warnings != 0 || len(result.checks) != 1 || result.checks[0].Status != checkInfo {
		t.Fatalf("disabled references: %+v", result.checks)
	}
}

func TestDoctorBundledReferencesInvalidProjectRulesWithoutSkills(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	audit.ResetGlobalCache()
	t.Cleanup(audit.ResetGlobalCache)
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".skillshare"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".skillshare", "audit-rules.yaml"), []byte("rules: ["), 0644); err != nil {
		t.Fatal(err)
	}
	result := &doctorResult{}
	captureStdout(t, func() { checkSkillReferences(result, nil, project, nil) })
	if len(result.checks) != 1 || result.checks[0].Status != checkWarning {
		t.Fatalf("invalid rules reported clean: %+v", result.checks)
	}
}
