package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"skillshare/internal/profile"
)

func TestProfileHintShellRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell round trip; Windows shell labels are covered separately")
	}
	for _, tc := range []struct{ name, child string }{
		{"space", "space child"},
		{"apostrophe", "child's review"},
		{"separator", `child; printf injected > "$PROFILE_HINT_SENTINEL"; #`},
		{"substitution", `child$(printf injected > "$PROFILE_HINT_SENTINEL")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _, _ := profileDiagnosticFixture(t, false, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
			writeHintSkill(t, cfg.Source, "_repo/"+tc.child)
			hint := installedProfileHint(cfg, modeGlobal, cfg.Source, cfg.SkillsWalk(), []string{filepath.Join(cfg.Source, "_repo")})
			const prefix = `Not in active profile "work": `
			if !strings.HasPrefix(hint, prefix) {
				t.Fatal("missing hint for discovered child", hint)
			}

			// The copied command calls only this recorder, never the product CLI.
			// Any unintended execution can write only the harmless fixture sentinel.
			root := t.TempDir()
			argsPath, sentinel := filepath.Join(root, "args"), filepath.Join(root, "sentinel")
			script := `skillshare() { printf '%s\000' "$@" > "$PROFILE_HINT_ARGS"; }; ` + strings.TrimPrefix(hint, prefix)
			cmd := exec.Command("sh", "-c", script)
			cmd.Dir = root
			cmd.Env = []string{"PROFILE_HINT_ARGS=" + argsPath, "PROFILE_HINT_SENTINEL=" + sentinel, "LC_ALL=C"}
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("copied hint must execute as one literal argument: %v: %s", err, out)
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Errorf("hint executed child text instead of passing it literally: %v", err)
			}
			data, err := os.ReadFile(argsPath)
			if err != nil {
				t.Fatal("argument recorder did not run", err)
			}
			got := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
			want := []string{"profile", "add", "work", "_repo__" + tc.child, "-g"}
			if !slices.Equal(got, want) {
				t.Fatalf("copied hint arguments = %q, want %q", got, want)
			}
		})
	}
}

func TestProfileHintWindowsShellContract(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows shell quoting and expansion label contract")
	}
	for _, tc := range []struct{ child, command string }{
		{"space child", `skillshare profile add work "_repo__space child"`},
		{"child's review", `skillshare profile add work "_repo__child's review"`},
		{"$child", `PowerShell: skillshare profile add work '_repo__$child'`},
		{"it's $child", `PowerShell: skillshare profile add work '_repo__it''s $child'`},
		{"%CHILD%", `PowerShell: skillshare profile add work '_repo__%CHILD%'`},
		{"!child!", `PowerShell: skillshare profile add work '_repo__!child!'`},
		{"child`name", "PowerShell: skillshare profile add work '_repo__child`name'"},
	} {
		t.Run(tc.child, func(t *testing.T) {
			cfg, _, _ := profileDiagnosticFixture(t, false, "profiles:\n  work:\n    skills:\n      include: [keep]\n", profile.State{Version: 1, Active: "work"})
			writeHintSkill(t, cfg.Source, "_repo/"+tc.child)
			got := installedProfileHint(cfg, modeGlobal, cfg.Source, cfg.SkillsWalk(), []string{filepath.Join(cfg.Source, "_repo")})
			if want := `Not in active profile "work": ` + tc.command + " -g"; got != want {
				t.Fatalf("hint = %q, want %q", got, want)
			}
		})
	}
}
