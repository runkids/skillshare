package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnimportRejectsInvalidTargets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		targets []string
		install bool
		key     string
	}{
		{"no target", nil, false, "plugins.error.unimportTargetRequired"},
		{"no receipt", []string{"claude"}, false, "plugins.error.unimportReceiptRequired"},
		{"not imported", []string{"claude"}, true, "plugins.error.unimportReceiptRequired"},
		{"missing target", []string{"codex"}, false, "plugins.error.unimportTargetMissing"},
		{"mixed targets", []string{"claude", "codex"}, true, "plugins.error.unimportTargetMissing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			native := &fakeAgents{}
			s := native.service(t)
			source := fixture(t)
			writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    review:\n      source: "+source+"\n      plugin: demo\n      bindings:\n        claude: {}\n")
			if tc.install {
				applyPluginRequest(t, s, Request{Action: "sync", Name: "review"})
			}
			same := unchanged(t, s.ConfigPath)
			_, err := s.Preview(context.Background(), Request{Action: "unimport", Name: "review", Targets: tc.targets})
			if key, _ := ErrorKey(err); key != tc.key {
				t.Fatalf("error = %v (key %q), want %q", err, key, tc.key)
			}
			same()
		})
	}
}

func TestUnimportPreservesOtherHostsSourceInstall(t *testing.T) {
	for _, scope := range []string{"global", "project", "account"} {
		t.Run(scope, func(t *testing.T) {
			source := fixture(t)
			for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "fixture"}, {"branch", "next"}} {
				cmd := exec.Command("git", args...)
				cmd.Dir = source
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			}
			cfg := filepath.Join(t.TempDir(), "gitconfig")
			writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+source+"\"]\n  insteadOf = https://fixture.invalid/round3.git\n")
			t.Setenv("GIT_CONFIG_GLOBAL", cfg)
			source = "https://fixture.invalid/round3.git"
			nativeA := &fakeAgents{version: "1.0.0"}
			a := nativeA.service(t)
			nativeB := &fakeAgents{version: "1.0.0", installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
			b := nativeB.service(t)
			if scope == "project" {
				a.ProjectRoot, b.ProjectRoot = t.TempDir(), t.TempDir()
				a.ConfigPath = filepath.Join(a.ProjectRoot, ".skillshare", "config.yaml")
				b.ConfigPath = filepath.Join(b.ProjectRoot, ".skillshare", "config.yaml")
				nativeB.installed["claude"][0].Scope = "project"
				nativeB.installed["claude"][0].ProjectPath = b.ProjectRoot
			}
			target := "claude"
			if scope == "account" {
				target = "claude-work"
				a.Accounts = map[string]Account{target: {Agent: "claude", Dir: filepath.Join(filepath.Dir(a.StateDir), "account")}}
				b.Accounts = map[string]Account{target: {Agent: "claude", Dir: filepath.Join(filepath.Dir(b.StateDir), "account")}}
			}

			apply := func(s *Service, r Request) { t.Setenv("HOME", filepath.Dir(s.StateDir)); applyPluginRequest(t, s, r) }
			packages := func(s *Service) map[string]Package {
				t.Setenv("HOME", filepath.Dir(s.StateDir))
				return hostPackages(t, s)
			}
			copyConfig := func(from, to *Service) {
				raw, err := os.ReadFile(from.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				writePluginFile(t, filepath.Dir(to.ConfigPath), filepath.Base(to.ConfigPath), string(raw))
			}
			apply(a, Request{Action: "add", Source: source, Name: "review", Targets: []string{target}})
			copyConfig(a, b)
			apply(b, Request{Action: "import", Plugin: "demo@team", From: target, Name: "review"})
			apply(a, Request{Action: "update", Name: "review", SourceRef: "next"})
			copyConfig(a, b)
			same := unchanged(t, b.ConfigPath)
			nativeB.commands = nil
			targets, err := b.UnimportTargets("review")
			if err != nil || len(targets) != 1 || targets[0] != target {
				t.Fatalf("unimport targets = %v, %v; want %s", targets, err, target)
			}
			apply(b, Request{Action: "unimport", Name: "review", Targets: []string{target}})
			same()
			if len(nativeB.installed["claude"]) != 0 {
				t.Fatalf("local import still shadows source: %v", nativeB.commands)
			}
			if got := packages(b)["review"].Bindings[target]; got.ID != "" || got.Imported {
				t.Fatalf("receipt retained: %+v", got)
			}
			apply(b, Request{Action: "sync"})
			same()
			if got := packages(b)["review"].Bindings[target]; got.Source == "" || got.Imported || got.SourceRef != "next" {
				t.Fatalf("source not installed: %+v", got)
			}
			copyConfig(b, a)
			apply(a, Request{Action: "sync"})
			if got := packages(a)["review"].Bindings[target]; got.ID == "" || got.Cleanup || got.Imported {
				t.Fatalf("A lost source binding: %+v", got)
			}
			if len(nativeA.installed["claude"]) != 1 {
				t.Fatalf("A source removed: %v", nativeA.commands)
			}
		})
	}
}

func TestUnimportTargetsRequiresTargetRecipeAndLocalImport(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{
		"codex": {{PluginID: "demo@team", Installed: true}},
	}}
	s := native.service(t)
	applyPluginRequest(t, s, Request{Action: "import", From: "codex", Plugin: "demo@team", Name: "review"})
	applyPluginRequest(t, s, Request{Action: "add", Source: fixture(t), Name: "review", Targets: []string{"claude"}})
	targets, err := s.UnimportTargets("review")
	if err != nil || len(targets) != 0 {
		t.Fatalf("native-only import and source-managed install must not be offered: %v, %v", targets, err)
	}
}

func TestMatchingAddDoesNotAcknowledgeRetainedImport(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
	s := native.service(t)
	source := fixture(t)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    review:\n      source: "+source+"\n      plugin: demo\n      bindings:\n        claude: {}\n")
	applyPluginRequest(t, s, Request{Action: "import", From: "claude", Plugin: "demo@team", Name: "review"})
	original := readState(t, s).state.Bindings["review"]["claude"].Applied.Intent
	replacement := fixture(t)
	raw, _ := os.ReadFile(s.ConfigPath)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), strings.Replace(string(raw), source, replacement, 1))
	applyPluginRequest(t, s, Request{Action: "add", Source: replacement, Name: "review", Targets: []string{"claude"}})
	if got := readState(t, s).state.Bindings["review"]["claude"].Applied.Intent; got != original {
		t.Fatal("matching add acknowledged an unapplied recipe")
	}
	p, err := s.Preview(context.Background(), Request{Action: "sync"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.importRecipeChanged" {
		t.Fatalf("guidance lost: %+v", p)
	}
}

func TestImportWithoutRecipeIdentityDoesNotFetch(t *testing.T) {
	native := &fakeAgents{installed: map[string][]Installed{"claude": {{ID: "demo@team", Scope: "user", Installed: true}}}}
	s := native.service(t)
	bin := t.TempDir()
	marker := filepath.Join(bin, "fetched")
	writePluginFile(t, bin, "git", "#!/bin/sh\nprintf fetched > '"+marker+"'\nexit 1\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), "plugins:\n  version: 2\n  packages:\n    alias:\n      source: https://fixture.invalid/offline.git\n      bindings:\n        claude: {}\n")
	_, err := s.Preview(context.Background(), Request{Action: "import", From: "claude", Plugin: "demo@team", Name: "alias"})
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatal("import fetched a source on an offline machine")
	}
	if key, _ := ErrorKey(err); key != "plugins.error.importRecipeIdentityRequired" {
		t.Fatalf("missing identity guidance: %v", err)
	}
	raw, _ := os.ReadFile(s.ConfigPath)
	writePluginFile(t, filepath.Dir(s.ConfigPath), filepath.Base(s.ConfigPath), strings.Replace(string(raw), "      bindings:", "      plugin: demo\n      bindings:", 1))
	applyPluginRequest(t, s, Request{Action: "import", From: "claude", Plugin: "demo@team", Name: "alias"})
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatal("explicit recipe identity still fetched source")
	}
}
