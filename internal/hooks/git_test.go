package hooks

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func gitDraft(t *testing.T, e *env, m Mutation) *Plan {
	t.Helper()
	_, err := e.service.Mutate(m, "", false)
	must(t, err)
	p, err := e.service.Preview()
	must(t, err)
	return p
}

func TestGitHooksFileDecisions(t *testing.T) {
	for _, tc := range []struct {
		name, current, record, action string
		remove, replace, other        bool
	}{
		{"new", "", "", "add", false, false, false},
		{"unmanaged identical", "desired", "", "conflict", false, false, false},
		{"adopt identical", "desired", "", "adopt", false, true, false},
		{"unmanaged changed", "other", "", "conflict", false, false, false},
		{"replace unmanaged", "other", "", "update", false, true, false},
		{"owned unchanged", "desired", "desired", "unchanged", false, false, false},
		{"owned update", "other", "other", "update", false, false, false},
		{"owned edited", "other", "desired", "conflict", false, false, false},
		{"owned missing", "", "desired", "conflict", false, false, false},
		{"replace edited", "other", "desired", "update", false, true, false},
		{"remove owned", "desired", "desired", "remove", true, false, false},
		{"remove edited", "other", "desired", "conflict", true, false, false},
		{"replace remove edited", "other", "desired", "remove", true, true, false},
		{"other owner", "desired", "desired", "conflict", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := gitEnv(t)
			_, err := e.service.Mutate(Mutation{Name: "guard", Entry: entry(t, gitEntry)}, "", false)
			must(t, err)
			source, err := LoadSource(e.config)
			must(t, err)
			d, err := e.service.render(source)
			must(t, err)
			var w gitWant
			for _, v := range d.git {
				w = v
			}
			content := func(which string) []byte {
				if which == "desired" {
					return w.content
				}
				return []byte("# outside edit\n")
			}
			if tc.current != "" {
				write(t, w.destination.hooksFile, string(content(tc.current)))
			}
			if tc.record != "" {
				owner := e.config
				if tc.other {
					owner = filepath.Join(t.TempDir(), "config.yaml")
					write(t, owner, "targets: {}\n")
				}
				state := ledger{Version: 1, Records: map[string]record{fileKey(w.destination.hooksFile): {Owner: owner, Target: "git", Path: w.destination.hooksFile, Hash: digest(content(tc.record))}}}
				c := w.commands[0]
				if tc.record == "other" {
					c.command.Command = "echo before"
				}
				state.Records[elementKey(owner, w.destination.identity, w.destination.hooksFile, "hook."+c.name, c.entry, 0)] = record{Owner: owner, Target: "git", Path: w.destination.hooksFile, Entry: c.entry, Event: "hook." + c.name, Hash: elementHash(c.command)}
				must(t, writeJSONFile(e.service.statePath(), state))
			}
			m := Mutation{Name: "guard", Replace: tc.replace}
			if tc.remove {
				m.Remove = true
			} else {
				m.Entry = entry(t, gitEntry)
			}
			p, err := e.service.PreviewMutation(m)
			must(t, err)
			found := false
			for _, c := range p.Changes {
				if c.Target == "git" && c.Name == "guard" && c.Path == w.destination.hooksFile {
					found = true
					if c.Action != tc.action {
						t.Fatalf("want %s: %+v", tc.action, c)
					}
				}
			}
			if !found {
				t.Fatalf("missing name row: %+v", p.Changes)
			}
			if (tc.action == "conflict") != p.Blocked {
				t.Fatalf("blocked: %+v", p)
			}
		})
	}
}

func TestGitIncludeAndCollisionPreview(t *testing.T) {
	for _, tc := range []struct {
		name, config                           string
		replace, conflict, warning, addInclude bool
	}{
		{"foreign command", "[hook \"check\"]\n command = echo foreign\n", false, true, false, true},
		{"foreign event", "[hook \"check\"]\n event = pre-commit\n", false, true, false, true},
		{"replace current section", "[hook \"check\"]\n command = echo foreign\n", true, false, false, true},
		{"enabled override", "[hook \"check\"]\n enabled = false\n", false, false, true, true},
		{"manual include", "[include]\n path = ~/.config/git/skillshare/hooks.gitconfig\n", false, false, false, false},
		{"conditional manual include", "[includeIf \"gitdir:/never/\"]\n path = ~/.config/git/skillshare/hooks.gitconfig\n", false, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := gitEnv(t)
			write(t, filepath.Join(e.home, ".gitconfig"), tc.config+"[credential]\n helper = SECRET\n")
			p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry), Replace: tc.replace})
			must(t, err)
			if p.Blocked != tc.conflict {
				t.Fatalf("blocked: %+v", p.Changes)
			}
			if tc.warning && len(p.Warnings) == 0 {
				t.Fatal("warning missing")
			}
			add := false
			for _, f := range p.files {
				if f.kind == kindGitInclude && f.changed() {
					add = true
				}
			}
			if add != tc.addInclude {
				t.Fatalf("include addition: %+v", p.files)
			}
			for _, f := range p.Files() {
				if strings.Contains(f.Before+f.After, "SECRET") {
					t.Fatal("private config in preview")
				}
			}
		})
	}
}

func TestGitDisabledOverrideAliases(t *testing.T) {
	for _, name := range []string{"check", "pre-commit"} {
		for _, tc := range []struct {
			setting  string
			disabled bool
		}{
			{"enabled = false", true}, {"enabled = FALSE", true},
			{"enabled = no", true}, {"enabled = NO", true},
			{"enabled = off", true}, {"enabled = OFF", true},
			{"enabled = 0", true}, {"enabled =", true},
			{"enabled", false}, {"enabled = true", false},
			{"enabled = yes", false}, {"enabled = on", false},
			{"enabled = 1", false},
		} {
			t.Run(name+"/"+tc.setting, func(t *testing.T) {
				e := gitEnv(t)
				write(t, filepath.Join(e.home, ".gitconfig"), "[hook \""+name+"\"]\n "+tc.setting+"\n")
				p, err := e.service.PreviewMutation(Mutation{Name: "guard", Entry: entry(t, gitEntry)})
				must(t, err)
				warning := false
				for _, w := range p.Warnings {
					warning = warning || strings.Contains(w, "hook."+name+" is disabled in ")
				}
				if warning != tc.disabled {
					t.Fatalf("disabled warning=%v, want %v: %v", warning, tc.disabled, p.Warnings)
				}
				if p.Blocked {
					t.Fatal("enabled override should warn without blocking sync")
				}
			})
		}
	}
}

type versionGit struct {
	GitRunner
	version string
	missing bool
}

func (g versionGit) Run(dir string, stdin []byte, args ...string) ([]byte, error) {
	if err := allowGit(args); err != nil {
		return nil, err
	}
	if g.missing {
		return nil, exec.ErrNotFound
	}
	if slices.Equal(args, []string{"version"}) {
		return []byte("git version " + g.version + "\n"), nil
	}
	return g.GitRunner.Run(dir, stdin, args...)
}

func TestGitInactiveAndRevision(t *testing.T) {
	e := gitEnv(t)
	e.service.Git = versionGit{GitRunner: e.service.gitRunner(), version: "2.53.0"}
	p := gitDraft(t, e, Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	if p.Blocked || !strings.Contains(actions(p), "inactive") || len(p.Files()) == 0 {
		t.Fatalf("old Git still writes: %+v", p)
	}
	e.service.Git = versionGit{missing: true}
	p, err := e.service.Preview()
	must(t, err)
	if p.Blocked || !strings.Contains(actions(p), "inactive") || len(p.Files()) != 0 {
		t.Fatalf("missing Git skips: %+v", p)
	}
	e.service.Git = nil
	p, err = e.service.Preview()
	must(t, err)
	write(t, filepath.Join(e.home, ".gitconfig"), "[user]\n name = later\n")
	q, err := e.service.Preview()
	must(t, err)
	if p.Revision != q.Revision {
		t.Fatal("unrelated settings changed revision")
	}
	write(t, filepath.Join(e.home, ".gitconfig"), "[hook \"check\"]\n enabled = false\n")
	q, err = e.service.Preview()
	must(t, err)
	if p.Revision == q.Revision {
		t.Fatal("override must change revision")
	}
	_, err = e.service.Mutate(Mutation{}, p.Revision, true)
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale: %v", err)
	}
	// No Git records can be released by a temporary missing repository.
	state := ledger{Version: 1, Records: map[string]record{"test": {Owner: e.config, Target: "git", Path: "/missing/.git/skillshare/hooks.gitconfig", Root: "/missing", Hash: "x"}}}
	must(t, writeJSONFile(e.service.statePath(), state))
	p, err = e.service.Preview()
	must(t, err)
	b, _ := json.Marshal(p.state.Records)
	if !strings.Contains(string(b), `"test"`) {
		t.Fatal("inactive root lost ownership")
	}
}

func gitEnv(t *testing.T) *env {
	t.Helper()
	e := newEnv(t)
	t.Setenv("HOME", e.home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(e.home, ".config"))
	t.Setenv("XDG_STATE_HOME", e.service.StateDir)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(e.home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(e.home, "system.gitconfig"))
	return e
}

// Setup alone may create repositories; management and its tests never run hooks.
func gitSetup(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = sanitizedGitEnv(os.Environ())
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git setup %v: %v: %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}

func TestGitRunnerAllowlist(t *testing.T) {
	for _, args := range [][]string{
		{"version"}, {"rev-parse", "--path-format=absolute", "--show-toplevel", "--git-common-dir"},
		{"config", "--global", "--includes", "--null", "--show-origin", "--get-regexp", "^hook\\."},
		{"config", "--file", "/tmp/config", "--add", "include.path", "skillshare/hooks.gitconfig"},
		{"config", "--file", "/tmp/config", "--fixed-value", "--unset", "include.path", "value"},
		{"config", "--file", "/tmp/config", "--remove-section", "hook.check"},
		{"config", "--file", "-", "--null", "--list"},
		{"config", "--file", "/tmp/config", "--path", "--no-includes", "--get-regexp", "^include"}, {"hook", "list", "--show-scope", "pre-commit"},
	} {
		if err := allowGit(args); err != nil {
			t.Errorf("allowed %v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"commit"}, {"push"}, {"hook", "run", "pre-commit"}, {"config", "hook.check.command", "evil"},
		{"-c", "core.hooksPath=evil", "version"}, {"config", "--global", "--add", "include.path", "x"},
		{"config", "--file", "/tmp/config", "--path", "--add", "include.path", "x"},
		{"config", "--file", "relative", "--add", "include.path", "x"},
		{"config", "--file", "/tmp/config", "--add", "hook.check.command", "x"},
		{"config", "--file", "/tmp/config", "--remove-section", "user"},
		{"config", "--file", "/tmp/config", "--remove-section", "hook.pre-commit"},
		{"rev-parse", "--git-path", "hooks"}, {"hook", "list", "pre-commit", "--to-stdin"},
	} {
		if err := allowGit(args); err == nil {
			t.Errorf("unsafe command accepted: %v", args)
		}
	}
}

func TestGitEnvironment(t *testing.T) {
	got := sanitizedGitEnv([]string{"HOME=/home/test", "XDG_CONFIG_HOME=/config", "GIT_CONFIG_GLOBAL=/global", "GIT_CONFIG_NOSYSTEM=1", "GIT_DIR=/host", "GIT_WORK_TREE=/host", "GIT_COMMON_DIR=/host", "GIT_INDEX_FILE=/host", "GIT_OBJECT_DIRECTORY=/host", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/host", "GIT_PREFIX=x", "GIT_CONFIG_PARAMETERS=evil", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=x", "GIT_CONFIG_VALUE_0=y", "LC_ALL=zh_TW"})
	if !slices.Contains(got, "LC_ALL=C") || !slices.Contains(got, "HOME=/home/test") || !slices.Contains(got, "GIT_CONFIG_GLOBAL=/global") {
		t.Fatalf("missing preserved environment: %v", got)
	}
	for _, v := range got {
		if strings.Contains(v, "/host") || strings.Contains(v, "evil") || strings.HasPrefix(v, "GIT_CONFIG_KEY_") || strings.HasPrefix(v, "GIT_CONFIG_VALUE_") {
			t.Fatalf("unsafe environment: %v", got)
		}
	}
}

func TestGitDestinations(t *testing.T) {
	e := gitEnv(t)
	s := e.service
	d, err := s.gitDestination("")
	must(t, err)
	if d.includeTarget != filepath.Join(e.home, ".gitconfig") || d.includeValue != "~/.config/git/skillshare/hooks.gitconfig" {
		t.Fatalf("default: %+v", d)
	}
	write(t, filepath.Join(e.home, ".config", "git", "config"), "[user]\n name = test\n")
	d, err = s.gitDestination("")
	must(t, err)
	if d.includeTarget != filepath.Join(e.home, ".config", "git", "config") {
		t.Fatalf("xdg fallback: %+v", d)
	}
	write(t, filepath.Join(e.home, ".gitconfig"), "")
	d, err = s.gitDestination("")
	must(t, err)
	if d.includeTarget != filepath.Join(e.home, ".gitconfig") {
		t.Fatalf("home precedence: %+v", d)
	}
	s.GitGlobalConfig = filepath.Join(e.home, "explicit")
	d, err = s.gitDestination("")
	must(t, err)
	if d.includeTarget != s.GitGlobalConfig {
		t.Fatalf("explicit: %+v", d)
	}
	root := t.TempDir()
	gitSetup(t, root, "init", "--quiet")
	d, err = s.scoped(root).gitDestination(root)
	must(t, err)
	if d.base != filepath.Join(root, ".git") || d.includeValue != "skillshare/hooks.gitconfig" {
		t.Fatalf("project: %+v", d)
	}
	sub := filepath.Join(root, "sub")
	must(t, os.Mkdir(sub, 0755))
	if _, err := s.scoped(sub).gitDestination(sub); err == nil {
		t.Fatal("subdirectory must be inactive")
	}
	bare := t.TempDir()
	gitSetup(t, bare, "init", "--bare", "--quiet")
	if _, err := s.scoped(bare).gitDestination(bare); err == nil {
		t.Fatal("bare repository must be inactive")
	}
	// --orphan creates a linked worktree without creating a commit or running hooks.
	linked := filepath.Join(t.TempDir(), "linked")
	gitSetup(t, root, "worktree", "add", "--orphan", "-b", "test-linked", linked)
	ld, err := s.scoped(linked).gitDestination(linked)
	must(t, err)
	if ld.identity != d.identity || ld.hooksFile != d.hooksFile {
		t.Fatalf("linked destinations differ: %+v / %+v", d, ld)
	}
}

const gitEntry = `{"bindings":{"git":{"commands":{"check":{"events":["pre-commit"],"command":"{files}/check.sh"}},"files":{"check.sh":"#!/bin/sh\nexit 0\n"}}}}`

func TestGitRender(t *testing.T) {
	e := gitEnv(t)
	e.service.ConfigDirs["xdg"] = filepath.Join(e.home, "space ' quote")
	r, err := e.service.RenderNative(Mutation{Name: "guard", Entry: entry(t, gitEntry)})
	must(t, err)
	if len(r) != 2 {
		t.Fatalf("outputs: %+v", r)
	}
	for _, f := range r {
		if f.Error != "" {
			t.Fatal(f.Error)
		}
		if strings.HasSuffix(f.Path, "hooks.gitconfig") {
			out, err := e.service.gitRunner().Run("", []byte(f.Content), "config", "--file", "-", "--get", "hook.check.command")
			must(t, err)
			if !strings.Contains(string(out), "space '\\'' quote") || !strings.Contains(string(out), "'/check.sh") {
				t.Fatalf("shell quoting lost: %s", out)
			}
		}
	}
	a := *entry(t, gitEntry)
	source := &Source{ConfigPath: e.config, Entries: map[string]Entry{"a": a, "b": a}}
	if _, err := e.service.render(source); err == nil || !strings.Contains(err.Error(), "two hooks define hook.check") {
		t.Fatalf("duplicate name: %v", err)
	}
	b := *entry(t, strings.ReplaceAll(gitEntry, `"check":`, `"zed":`))
	source.Entries = map[string]Entry{"a": b, "z": a}
	d, err := e.service.render(source)
	must(t, err)
	for _, w := range d.git {
		if strings.Index(string(w.content), `[hook "zed"]`) > strings.Index(string(w.content), `[hook "check"]`) {
			t.Fatal("entries must sort before friendly names")
		}
	}
	if got := shellQuote(`C:\Users\Test Space\files`); got != "'C:/Users/Test Space/files'" {
		t.Fatal(got)
	}
}

func FuzzGitQuoteRoundTrip(f *testing.F) {
	for _, s := range []string{"echo hi", "quotes \\\"'", "tab\tvalue", "$(literal)", "unicode 世界"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if strings.ContainsAny(value, "\x00\r\n") {
			t.Skip()
		}
		out, err := (execGitRunner{}).Run("", []byte("[hook \"test\"]\n command = "+gitQuote(value)+"\n"), "config", "--file", "-", "--get", "hook.test.command")
		must(t, err)
		if string(out) != value+"\n" {
			t.Fatalf("round trip: %q != %q", out, value)
		}
	})
}

func TestGitShellQuoteKeepsPOSIXBackslashes(t *testing.T) {
	path := `/home/space and '\literal/files`
	if got, want := shellQuote(path), "'/home/space and '\\''\\literal/files'"; got != want {
		t.Fatalf("POSIX filename changed: %q != %q", got, want)
	}
}

func TestGitEnvSanitizesCaseVariants(t *testing.T) {
	got := sanitizedGitEnv([]string{"git_dir=unsafe", "Git_Config_Count=1", "Git_Config_Key_0=hook.test.command", "git_config_value_0=echo unsafe", "Git_Config_Global=/safe"})
	if !slices.Equal(got, []string{"Git_Config_Global=/safe", "LC_ALL=C"}) {
		t.Fatalf("case-insensitive environment leaked: %v", got)
	}
}
