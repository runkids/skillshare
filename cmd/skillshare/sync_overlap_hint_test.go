package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/sync"
)

// loadProjectConfig loads a global config registering root with targets.
func loadProjectConfig(t *testing.T, root string, targets string) *config.Config {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := fmt.Sprintf("source: %q\ntargets: {}\nprojects:\n  %q:\n    targets: [%s]\n    skills: {}\n", filepath.Join(dir, "skills"), root, targets)
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadReadOnlyPath(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSkillsOffCommand_ProjectTargetKeepsOtherTargets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "RUNNER~1", "app")
	cfg := loadProjectConfig(t, root, "claude, cursor")
	got := skillsOffCommand("app@claude", false, cfg.Targets, cfg.Projects)
	if want := "skillshare project edit " + root + " --target cursor"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkillsOffCommand_QuotesMultipleRemainingTargets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app")
	cfg := loadProjectConfig(t, root, "claude, cursor, codex")
	got := skillsOffCommand("app@claude", false, cfg.Targets, cfg.Projects)
	list := "'cursor,codex'"
	if runtime.GOOS == "windows" {
		list = `"cursor,codex"`
	}
	if want := "skillshare project edit " + root + " --target " + list; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkillsOffCommand_QuotesLeadingTildeFallback(t *testing.T) {
	want := "skillshare project show '~app'"
	if runtime.GOOS == "windows" {
		want = `skillshare project show "~app"`
	}
	if got := skillsOffCommand("~app@claude", false, nil, nil); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkillsOffCommand_OnlyProjectTargetTurnsSkillsOff(t *testing.T) {
	root := filepath.Join(t.TempDir(), "RUNNER~1", "app")
	cfg := loadProjectConfig(t, root, "claude")
	got := skillsOffCommand("app@claude", false, cfg.Targets, cfg.Projects)
	if want := "skillshare project edit " + root + " --no-skills"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkillsOffCommand_QuotesRootWithSpaces(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my app")
	cfg := loadProjectConfig(t, root, "claude")
	got := skillsOffCommand("my app@claude", false, cfg.Targets, cfg.Projects)
	quoted := "'" + root + "'"
	if runtime.GOOS == "windows" {
		quoted = `"` + root + `"`
	}
	if want := "skillshare project edit " + quoted + " --no-skills"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSkillsOffCommand_UsesExplicitProjectRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "~app")
	cfg := loadProjectConfig(t, root, "claude")
	got := skillsOffCommand("~app@claude", false, cfg.Targets, cfg.Projects)
	if !strings.Contains(got, root) {
		t.Fatal("hint cannot resolve a root-looking name:", got)
	}
}

func TestSkillsOffCommand_WindowsExpansionRootLabelsPowerShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows shell hint")
	}
	for _, folder := range []string{"$tmp", "a`b", "it's $app"} {
		t.Run(folder, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), folder, "app")
			cfg := loadProjectConfig(t, root, "claude")
			want := "PowerShell: skillshare project edit '" + strings.ReplaceAll(root, "'", "''") + "' --no-skills"
			if got := skillsOffCommand("app@claude", false, cfg.Targets, cfg.Projects); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestSkillsOffCommand_OrdinaryTargets(t *testing.T) {
	for _, tc := range []struct {
		name      string
		isProject bool
		want      string
	}{
		{"codex", false, "skillshare target codex --skills=false"},
		{"codex", true, "skillshare target codex --skills=false -p"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillsOffCommand(tc.name, tc.isProject, nil, nil); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShellArg(t *testing.T) {
	cases := map[string]string{
		"/a/b": "/a/b", "/tmp/RUNNER~1/app": "/tmp/RUNNER~1/app",
		"~app": "'~app'", "claude,cursor": "'claude,cursor'", "a b": "'a b'", "": "''",
		`a"$b`: `'a"$b'`, "a`b!": "'a`b!'", "a'b": `'a'\''b'`,
	}
	if runtime.GOOS == "windows" {
		cases = map[string]string{
			`C:\work\RUNNER~1\app`: `C:\work\RUNNER~1\app`,
			"~app":                 `"~app"`, "claude,cursor": `"claude,cursor"`, "a b": `"a b"`, "": `""`,
			`C:\work\$tmp\app`:   `'C:\work\$tmp\app'`,
			"C:\\work\\a`b\\app": "'C:\\work\\a`b\\app'",
			`C:\work\it's $app`:  `'C:\work\it''s $app'`,
		}
	}
	for in, want := range cases {
		if got := shellArg(in); got != want {
			t.Errorf("shellArg(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintSyncOverlapHint_NamesConflictAndFix(t *testing.T) {
	keepPathsUnfolded(t)
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills", Exclude: []string{"feature-radar*"}}},
		"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
	}
	out := captureStdout(t, func() { printSyncOverlapHint(targets, nil, true, false, "", nil) })

	for _, want := range []string{
		"codex and universal sync skills to " + filepath.FromSlash("/tmp/agents/skills") + " with different settings, so each sync undoes the other",
		"keep one: skillshare target codex --skills=false -p",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "share skill folders") {
		t.Errorf("conflict fully explains the overlap, generic line should be gone:\n%s", out)
	}
}

func TestPrintSyncOverlapHint_SameSettingsKeepsGenericLine(t *testing.T) {
	targets := map[string]config.TargetConfig{
		"universal": {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
		"codex":     {Skills: &config.ResourceTargetConfig{Path: "/tmp/agents/skills"}},
	}
	out := captureStdout(t, func() { printSyncOverlapHint(targets, nil, false, false, "", nil) })

	if !strings.Contains(out, "2 targets share skill folders") {
		t.Errorf("expected generic overlap line:\n%s", out)
	}
	if strings.Contains(out, "undoes") {
		t.Errorf("same settings is not a conflict:\n%s", out)
	}
}

func TestPrintSyncOverlapHint_OpenCodeSameSkillsIsQuiet(t *testing.T) {
	cfg := opencodeAndClaude(t)
	discovered := []sync.DiscoveredSkill{{FlatName: "shared"}}
	out := captureStdout(t, func() { printSyncOverlapHint(cfg.Targets, cfg.Projects, false, false, cfg.Mode, discovered) })

	if strings.Contains(out, "share skill folders") {
		t.Errorf("opencode loads nothing extra from claude's folder, expected no hint:\n%s", out)
	}
}
