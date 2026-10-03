package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The fixtures follow scripts/pi/phase0-contract.mjs: one local package with three
// extensions, a skill and a prompt, under an isolated HOME and agent directory.
type piFixture struct {
	t        *testing.T
	home     string
	agentDir string
	pkg      string
	svc      *Service
	version  string
}

func newPiFixture(t *testing.T) *piFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	agentDir := filepath.Join(home, ".pi", "agent")
	t.Setenv("PI_CODING_AGENT_DIR", agentDir)
	f := &piFixture{t: t, home: home, agentDir: agentDir, pkg: filepath.Join(home, "pkgs", "tools"), version: "0.99.2"}
	writeTree(t, f.pkg, map[string]string{
		"package.json":           `{"name":"tools"}`,
		"extensions/a.ts":        "throw new Error('never imported')",
		"extensions/b.ts":        "throw new Error('never imported')",
		"extensions/c.ts":        "throw new Error('never imported')",
		"skills/review/SKILL.md": "---\nname: review\ndescription: fixture\n---\n",
		"prompts/commit.md":      "fixture\n",
	})
	f.svc = &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state"), Run: func(_ context.Context, _ string, _ []string, bin string, args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "--version" && f.version != "" {
			return []byte(f.version + "\n"), nil
		}
		return nil, errors.New("not found: " + bin)
	}}
	return f
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *piFixture) writeJSON(file string, v any) {
	f.t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		f.t.Fatal(err)
	}
	writeTree(f.t, filepath.Dir(file), map[string]string{filepath.Base(file): string(data)})
}

func (f *piFixture) global(v any) { f.writeJSON(filepath.Join(f.agentDir, "settings.json"), v) }

func (f *piFixture) view(target string) *PiExtensionsView {
	f.t.Helper()
	v, err := f.svc.PiExtensions(context.Background(), target)
	if err != nil {
		f.t.Fatal(err)
	}
	return v
}

// selections maps "path:selection" for one package, like the contract's pick().
func selections(p PiExtensionPackage) []string {
	out := []string{}
	for _, r := range p.Rows {
		out = append(out, r.Path+":"+r.Selection)
	}
	return out
}

func TestPiMissingManagedNpmRootDoesNotClaimLegacyIsMissing(t *testing.T) {
	for _, scope := range []string{"user", "project"} {
		t.Run(scope, func(t *testing.T) {
			f := newPiFixture(t)
			p := openPiPackage("npm:@fixture/legacy@1.2.3", f.agentDir, filepath.Join(f.home, "project", ".pi"), scope)
			if scope == "user" {
				if p.install != "unknown" || p.problem != "sourceUnknown" {
					t.Fatalf("legacy fallback was called missing: %+v", p)
				}
				f.global(map[string]any{"packages": []any{map[string]any{"source": "npm:@fixture/legacy@1.2.3", "extensions": []string{"-absent.ts"}}}})
				v := f.view("pi")
				pkg := v.Packages[0]
				if pkg.Install != "unknown" || pkg.Problem != "sourceUnknown" || len(pkg.Rows) != 0 {
					t.Fatalf("uncertain npm installation offered exact missing rows: %+v", pkg)
				}
			} else if p.install != "missing" || p.problem != "notInstalled" {
				t.Fatalf("project has no legacy fallback: %+v", p)
			}
		})
	}
}

func assertRows(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("rows %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows %v, want %v", got, want)
		}
	}
}

func TestPiExtensionsGlobalSelectionMatchesContract(t *testing.T) {
	cases := []struct {
		name  string
		entry any
		want  []string
	}{
		{"S1 string entry loads everything", "PKG", []string{"extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:loads"}},
		{"S2 -path excludes one exact path", map[string]any{"source": "PKG", "extensions": []string{"-extensions/b.ts"}}, []string{"extensions/a.ts:loads", "extensions/b.ts:skipped", "extensions/c.ts:loads"}},
		{"S3 [] disables every extension", map[string]any{"source": "PKG", "extensions": []string{}}, []string{"extensions/a.ts:skipped", "extensions/b.ts:skipped", "extensions/c.ts:skipped"}},
		{"S4 include glob then !glob", map[string]any{"source": "PKG", "extensions": []string{"extensions/*.ts", "!extensions/c.ts"}}, []string{"extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:skipped"}},
		{"S5 + beats !, - beats +", map[string]any{"source": "PKG", "extensions": []string{"!extensions/*.ts", "+extensions/a.ts", "+extensions/b.ts", "-extensions/b.ts"}}, []string{"extensions/a.ts:loads", "extensions/b.ts:skipped", "extensions/c.ts:skipped"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newPiFixture(t)
			entry := c.entry
			if s, ok := entry.(string); ok && s == "PKG" {
				entry = f.pkg
			} else {
				m := entry.(map[string]any)
				m["source"] = f.pkg
			}
			f.global(map[string]any{"packages": []any{entry}})
			v := f.view("pi")
			if !v.Editable || v.Scope != "global" || len(v.Packages) != 1 {
				t.Fatalf("view: %+v", v)
			}
			assertRows(t, selections(v.Packages[0]), c.want...)
			for _, r := range v.Packages[0].Rows {
				if r.Editable == (r.Origin == "emptyList") {
					t.Fatalf("%s: editable %v on a verified Pi", r.Path, r.Editable)
				}
			}
		})
	}
}

func TestPiExtensionsVersionGate(t *testing.T) {
	for _, c := range []struct{ version, readOnly string }{{"0.99.2", ""}, {"1.0.0", ""}, {"1.0.1", ""}, {"1.10.0", ""}, {"0.99.1", piReadOnlyUnsupported}, {"0.85.0", piReadOnlyUnsupported}, {"1.0.1-beta.1", piReadOnlyUnsupported}, {"pi 1.0.1", piReadOnlyUnsupported}, {"", piReadOnlyNoCLI}} {
		f := newPiFixture(t)
		f.version = c.version
		f.global(map[string]any{"packages": []any{f.pkg}})
		v := f.view("pi")
		if v.ReadOnly != c.readOnly || v.Editable != (c.readOnly == "") {
			t.Fatalf("%q: readOnly=%q editable=%v", c.version, v.ReadOnly, v.Editable)
		}
		if c.readOnly != "" && v.Packages[0].Rows[0].Editable {
			t.Fatalf("%q: a row is editable on an unsupported Pi", c.version)
		}
	}
}

func TestPiCLIIsNative(t *testing.T) {
	for _, c := range []struct {
		goos, cli string
		want      bool
	}{
		{"linux", "pi", true},
		{"linux", "/usr/local/bin/pi", true},
		{"darwin", "/opt/homebrew/bin/pi", true},
		{"linux", "pi.cmd", false},
		{"darwin", "pi.exe", false},
		{"linux", "PI", false},
		{"windows", "pi", true},
		{"windows", "pi.cmd", true},
		{"windows", "pi.exe", true},
		{"windows", `C:\Users\tester\AppData\Roaming\npm\pi.cmd`, true},
		{"windows", `C:\Program Files\Pi\PI.EXE`, true},
		{"windows", "C:/tools/Pi.CmD", true},
		{"windows", `\\server\tools\pi.exe`, true},
		{"windows", "omo.cmd", false},
		{"windows", `C:\tools\senpi.exe`, false},
		{"windows", "not-pi.exe", false},
		{"windows", "pi.exe.cmd", false},
		{"windows", "pi.ps1", false},
		{"windows", "pi.bat", false},
		{"windows", "pi.cmd.exe", false},
		{"linux", `C:\tools\pi.cmd`, false},
	} {
		t.Run(c.goos+"/"+c.cli, func(t *testing.T) {
			if got := piCLIIsNative(c.cli, c.goos); got != c.want {
				t.Fatalf("piCLIIsNative(%q, %q) = %v, want %v", c.cli, c.goos, got, c.want)
			}
		})
	}
}

func TestPiExtensionsAccountLauncherGate(t *testing.T) {
	launchers := []string{"pi", "omo", "senpi", "pi.ps1", "pi.exe.cmd"}
	if runtime.GOOS == "windows" {
		launchers = append(launchers, "pi.cmd", "pi.exe", `C:\tools\PI.CMD`, `C:\tools\pi.exe`, "omo.cmd", "senpi.exe")
	} else {
		launchers = append(launchers, "/usr/local/bin/pi", "pi.cmd", "pi.exe")
	}
	for _, cli := range launchers {
		for _, version := range []string{"0.99.2", "1.0.0", "0.99.1", ""} {
			t.Run(cli+"/"+version, func(t *testing.T) {
				f := newPiFixture(t)
				f.version = version
				f.svc.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: f.agentDir, CLI: cli}}
				run := f.svc.Run
				calls := 0
				f.svc.Run = func(ctx context.Context, dir string, env []string, bin string, args ...string) ([]byte, error) {
					calls++
					if bin != cli || !slices.Equal(args, []string{"--version"}) {
						t.Fatalf("unexpected execution: %s %v", bin, args)
					}
					return run(ctx, dir, env, bin, args...)
				}
				gotVersion, readOnly := f.svc.piGate(context.Background(), "pi-work")
				if !piCLIIsNative(cli, runtime.GOOS) {
					if calls != 0 || gotVersion != "" || readOnly != piReadOnlyFork {
						t.Fatalf("fork was probed: calls=%d version=%q readOnly=%q", calls, gotVersion, readOnly)
					}
					return
				}
				want := ""
				if version == "" {
					want = piReadOnlyNoCLI
				} else if version == "0.99.1" {
					want = piReadOnlyUnsupported
				}
				if calls != 1 || gotVersion != version || readOnly != want {
					t.Fatalf("native gate: calls=%d version=%q readOnly=%q, want %q/%q", calls, gotVersion, readOnly, version, want)
				}
			})
		}
	}
}

func TestPiExtensionsAccountUsesItsOwnDirectoryAndCLI(t *testing.T) {
	f := newPiFixture(t)
	work := filepath.Join(f.home, ".pi-work", "agent")
	f.writeJSON(filepath.Join(work, "settings.json"), map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"-extensions/a.ts"}}}})
	f.svc.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: work}}
	v := f.view("pi-work")
	if v.Scope != "account" || !v.Editable || v.SettingsPath != filepath.Join(work, "settings.json") {
		t.Fatalf("account view: %+v", v)
	}
	assertRows(t, selections(v.Packages[0]), "extensions/a.ts:skipped", "extensions/b.ts:loads", "extensions/c.ts:loads")

	// A fork is known unsupported from its name, so it is never run to ask its version.
	f.svc.Accounts = map[string]Account{"pi-work": {Agent: "pi", Dir: work, CLI: "/usr/local/bin/omo"}}
	f.svc.Run = func(_ context.Context, _ string, _ []string, bin string, _ ...string) ([]byte, error) {
		t.Fatalf("the fork %s was run", bin)
		return nil, nil
	}
	if v := f.view("pi-work"); v.ReadOnly != piReadOnlyFork || v.Packages[0].Rows[0].Editable {
		t.Fatalf("a fork CLI must be read-only: %+v", v)
	}
}

func TestPiExtensionsRefusesNonStrictJSON(t *testing.T) {
	f := newPiFixture(t)
	writeTree(t, f.agentDir, map[string]string{"settings.json": "{\n  // comment\n  \"packages\": []\n}\n"})
	v := f.view("pi")
	if v.Problem != "invalidJson" || v.ReadOnly != piReadOnlySettings || len(v.Packages) != 0 {
		t.Fatalf("view: %+v", v)
	}
}

func TestPiExtensionsSymlinkedSettingsAreRefused(t *testing.T) {
	f := newPiFixture(t)
	real := filepath.Join(f.home, "dotfiles", "settings.json")
	writeTree(t, filepath.Dir(real), map[string]string{"settings.json": `{"packages":[]}`})
	if err := os.MkdirAll(f.agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(f.agentDir, "settings.json")); err != nil {
		t.Fatal(err)
	}
	if v := f.view("pi"); v.Problem != "symlink" || v.Editable {
		t.Fatalf("view: %+v", v)
	}
}

// A linked home volume or dotfiles folder above the agent directory is the
// user's own layout; only links inside the agent directory are refused.
func TestPiExtensionsAgentDirUnderSymlinkedAncestorIsEditable(t *testing.T) {
	f := newPiFixture(t)
	volume := filepath.Join(f.home, "volume")
	if err := os.MkdirAll(volume, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(f.home, "linked")
	if err := os.Symlink(volume, linked); err != nil {
		t.Fatal(err)
	}
	f.agentDir = filepath.Join(linked, "agent")
	t.Setenv("PI_CODING_AGENT_DIR", f.agentDir)
	f.global(map[string]any{"packages": []any{f.pkg}})
	if v := f.view("pi"); v.Problem != "" || !v.Editable {
		t.Fatalf("view: %+v", v)
	}
}

func TestPiExtensionsUnsupportedGlobIsUnknownAndReadOnly(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"!extensions/**", "+extensions/a.ts"}}}})
	rows := f.view("pi").Packages[0].Rows
	assertRows(t, selections(PiExtensionPackage{Rows: rows}), "extensions/a.ts:loads", "extensions/b.ts:unknown", "extensions/c.ts:unknown")
	if !rows[0].Editable || rows[1].Editable || rows[2].Editable {
		t.Fatalf("only the row an exact rule decides can change: %+v", rows)
	}
}

func TestPiExtensionsDiscoveryRefusesWhatItCannotReproduce(t *testing.T) {
	cases := map[string]struct {
		files   map[string]string
		problem string
	}{
		"ignore files":      {map[string]string{"extensions/.gitignore": "c.ts\n"}, piProblemIgnoreFiles},
		"manifest glob":     {map[string]string{"package.json": `{"pi":{"extensions":["extensions/*.ts"]}}`}, piProblemManifestGlob},
		"manifest escapes":  {map[string]string{"package.json": `{"pi":{"extensions":["../outside.ts"]}}`}, piProblemPathEscapes},
		"absolute manifest": {map[string]string{"package.json": `{"pi":{"extensions":["/etc/passwd"]}}`}, piProblemPathEscapes},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			writeTree(t, f.pkg, c.files)
			f.global(map[string]any{"packages": []any{f.pkg}})
			p := f.view("pi").Packages[0]
			if p.Problem != c.problem || len(p.Rows) != 0 {
				t.Fatalf("package: %+v", p)
			}
		})
	}
}

func TestPiExtensionsLinkOutOfPackageIsUnsafe(t *testing.T) {
	f := newPiFixture(t)
	outside := filepath.Join(f.home, "outside.ts")
	writeTree(t, f.home, map[string]string{"outside.ts": "x"})
	if err := os.Symlink(outside, filepath.Join(f.pkg, "extensions", "d.ts")); err != nil {
		t.Fatal(err)
	}
	f.global(map[string]any{"packages": []any{f.pkg}})
	if p := f.view("pi").Packages[0]; p.Problem != piProblemUnsafeLink || len(p.Rows) != 0 {
		t.Fatalf("package: %+v", p)
	}
}

func TestPiExtensionsManifestAndInstallLocations(t *testing.T) {
	f := newPiFixture(t)
	npm := filepath.Join(f.agentDir, "npm", "node_modules", "@acme", "tools")
	writeTree(t, npm, map[string]string{
		"package.json":          `{"name":"@acme/tools","pi":{"extensions":["./src/main.ts","./plugins","!plugins/skip.ts"]}}`,
		"src/main.ts":           "x",
		"plugins/one.ts":        "x",
		"plugins/skip.ts":       "x",
		"plugins/dir/index.ts":  "x",
		"extensions/ignored.ts": "x",
	})
	f.global(map[string]any{"packages": []any{"npm:@acme/tools@1.2.0", "git:github.com/acme/none@v1"}})
	v := f.view("pi")
	if p := v.Packages[0]; p.Kind != "npm" || p.Identity != "npm:@acme/tools" || p.Install != "present" {
		t.Fatalf("npm package: %+v", p)
	}
	assertRows(t, selections(v.Packages[0]), "src/main.ts:loads", "plugins/dir/index.ts:loads", "plugins/one.ts:loads")
	if p := v.Packages[1]; p.Kind != "git" || p.Identity != "git:github.com/acme/none" || p.Install != "missing" || p.Problem != "notInstalled" || len(p.Rows) != 0 {
		t.Fatalf("git package: %+v", p)
	}
}

func TestPiExtensionsListsARuleForAMissingFile(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"-extensions/gone.ts"}}}})
	rows := f.view("pi").Packages[0].Rows
	last := rows[len(rows)-1]
	if last.Path != "extensions/gone.ts" || last.File != "missing" || last.Selection != "none" || last.Rule != "-extensions/gone.ts" || !last.Editable {
		t.Fatalf("missing row: %+v", last)
	}
}

func TestPiExtensionsMarksAnExtraInPisFolder(t *testing.T) {
	f := newPiFixture(t)
	extra := filepath.Join(f.home, "extras", "pi-ext")
	writeTree(t, extra, map[string]string{"guard.ts": "x"})
	writeTree(t, filepath.Join(f.agentDir, "extensions"), map[string]string{"scratch.ts": "x"})
	if err := os.Symlink(filepath.Join(extra, "guard.ts"), filepath.Join(f.agentDir, "extensions", "guard.ts")); err != nil {
		t.Fatal(err)
	}
	f.global(map[string]any{"extensions": []string{"-extensions/scratch.ts"}})
	f.svc.ExtrasSources = map[string]string{"pi-ext": extra}
	folder := f.view("pi").Folders[0]
	if len(folder.Rows) != 2 || folder.Rows[0].Provenance != "extras" || folder.Rows[0].Extra != "pi-ext" || folder.Rows[0].Selection != "loads" ||
		folder.Rows[1].Provenance != "native" || folder.Rows[1].Selection != "skipped" {
		t.Fatalf("folder: %+v", folder)
	}
}

func TestPiExtensionsProjectScopeShapes(t *testing.T) {
	f := newPiFixture(t)
	root := filepath.Join(f.home, "code", "acme")
	only := filepath.Join(f.home, "pkgs", "lint")
	writeTree(t, only, map[string]string{"extensions/lint.ts": "x"})
	other := filepath.Join(f.home, "pkgs", "other")
	writeTree(t, other, map[string]string{"extensions/o.ts": "x"})
	f.global(map[string]any{
		"defaultProjectTrust": "ask",
		"packages": []any{
			map[string]any{"source": f.pkg, "extensions": []string{"-extensions/c.ts"}},
			map[string]any{"source": other, "extensions": []string{"-extensions/o.ts"}},
		},
	})
	f.writeJSON(filepath.Join(root, ".pi", "settings.json"), map[string]any{"packages": []any{
		map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"+extensions/c.ts", "-extensions/a.ts"}},
		map[string]any{"source": only, "autoload": false, "extensions": []string{"+extensions/lint.ts"}},
	}})
	f.writeJSON(filepath.Join(f.agentDir, "trust.json"), map[string]any{root: true})
	f.svc.ProjectRoot = root
	v := f.view("pi")
	if v.Scope != "project" || !v.Editable || v.ReadOnly != "" {
		t.Fatalf("project view: %+v", v)
	}
	if v.Trust == nil || v.Trust.Saved != "trusted" || v.Trust.Default != "ask" {
		t.Fatalf("trust hints: %+v", v.Trust)
	}
	// S6: the delta decides a and c, b inherits the global selection.
	delta := v.Packages[0]
	if delta.Shape != "delta" {
		t.Fatalf("delta: %+v", delta)
	}
	assertRows(t, selections(delta), "extensions/a.ts:skipped", "extensions/c.ts:loads", "extensions/b.ts:loads")
	if delta.Rows[2].Origin != "inherited" {
		t.Fatalf("b should be inherited: %+v", delta.Rows[2])
	}
	// S7: a delta with no global entry names only its own paths.
	if v.Packages[1].Shape != "deltaOnly" {
		t.Fatalf("delta only: %+v", v.Packages[1])
	}
	assertRows(t, selections(v.Packages[1]), "extensions/lint.ts:loads")
	if v.Packages[2].Shape != "global" {
		t.Fatalf("global-only: %+v", v.Packages[2])
	}
	assertRows(t, selections(v.Packages[2]), "extensions/o.ts:skipped")
	// Trust is shown, never required: every shape here can be switched.
	for _, p := range v.Packages {
		for _, r := range p.Rows {
			if !r.Editable {
				t.Fatalf("%s row %s is read-only", p.Shape, r.Path)
			}
		}
	}
}

// Pi loads its global extensions folder whether or not it trusts the project, so
// the project view lists it next to the project's own folder, each with its scope
// and evaluated by that scope's own rules.
func TestPiExtensionsProjectListsGlobalAndProjectFolders(t *testing.T) {
	f := newPiFixture(t)
	root := filepath.Join(f.home, "code", "acme")
	extra := filepath.Join(f.home, "extras", "pi-ext")
	writeTree(t, extra, map[string]string{"guard.ts": "x"})
	writeTree(t, filepath.Join(f.agentDir, "extensions"), map[string]string{"scratch.ts": "x"})
	if err := os.Symlink(filepath.Join(extra, "guard.ts"), filepath.Join(f.agentDir, "extensions", "guard.ts")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, filepath.Join(root, ".pi", "extensions"), map[string]string{"local.ts": "x"})
	f.global(map[string]any{"extensions": []string{"-extensions/scratch.ts"}})
	f.writeJSON(filepath.Join(root, ".pi", "settings.json"), map[string]any{"extensions": []string{"-extensions/local.ts"}})
	f.svc.ExtrasSources = map[string]string{"pi-ext": extra}
	f.svc.ProjectRoot = root
	got := []string{}
	for _, folder := range f.view("pi").Folders {
		for _, r := range folder.Rows {
			got = append(got, folder.Scope+":"+r.Path+":"+r.Selection+":"+r.Provenance)
		}
	}
	want := []string{
		"project:extensions/local.ts:skipped:native",
		"global:extensions/guard.ts:loads:extras",
		"global:extensions/scratch.ts:skipped:native",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("folders:\n got %q\nwant %q", got, want)
	}
}

func TestPiExtensionsProjectReplacingEntries(t *testing.T) {
	for name, entry := range map[string]func(string) any{
		"S8 object without autoload": func(src string) any { return map[string]any{"source": src, "extensions": []string{"+extensions/c.ts"}} },
		"S9 string":                  func(src string) any { return src },
	} {
		t.Run(name, func(t *testing.T) {
			f := newPiFixture(t)
			root := filepath.Join(f.home, "code", "acme")
			f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"-extensions/c.ts"}}}})
			f.writeJSON(filepath.Join(root, ".pi", "settings.json"), map[string]any{"packages": []any{entry(f.pkg)}})
			f.svc.ProjectRoot = root
			p := f.view("pi").Packages[0]
			if p.Shape != "replaces" || len(p.GlobalRules) != 1 {
				t.Fatalf("package: %+v", p)
			}
			assertRows(t, selections(p), "extensions/a.ts:loads", "extensions/b.ts:loads", "extensions/c.ts:loads")
		})
	}
}

// A URL or git source can carry credentials in its userinfo or its query; neither
// may reach the dashboard. Local paths and npm names are shown as written.
func TestPiExtensionsSourceDisplayCarriesNoURLCredentials(t *testing.T) {
	hidden := map[string]string{
		"git:https://dummy-user:dummy-pass@example.com/acme/tools?access_token=dummy-token": "git:https://***@example.com/acme/tools?***",
		"https://example.com/acme/tools.git?access_token=dummy-token":                       "https://example.com/acme/tools.git?***",
		"git:example.com/acme/tools?token=dummy-token":                                      "git:example.com/acme/tools?***",
	}
	for source, want := range hidden {
		if got := redactSource(source); got != want {
			t.Errorf("redactSource(%q) = %q, want %q", source, got, want)
		}
	}
	for _, kept := range []string{"/home/me/pkgs/what?.ts", "./pkgs/a?b", "npm:@acme/tools", "git@github.com:acme/tools"} {
		if got := redactSource(kept); got != kept {
			t.Errorf("redactSource(%q) = %q, want it unchanged", kept, got)
		}
	}
}

// Skillshare has not verified how Pi resolves a git source with a query, so such a
// source is Unknown: no identity carries the query and nothing becomes editable.
func TestPiExtensionsGitSourceWithAQueryIsUnknown(t *testing.T) {
	for _, source := range []string{"git:example.com/acme/tools?token=dummy-token", "https://example.com/acme/tools?access_token=dummy-token"} {
		if src := resolvePiSource(source, "/agent", "", "user"); src.kind != "unknown" || src.identity != "" {
			t.Errorf("%s: %+v", source, src)
		}
	}
}

// The display source is redacted, but the file keeps the raw registration: a string
// entry converted to an object still holds the source exactly as the user wrote it.
func TestPiExtensionsConvertingAnEntryKeepsItsRawSource(t *testing.T) {
	f := newPiFixture(t)
	raw := "git:https://dummy-user:dummy-pass@example.com/acme/tools"
	writeTree(t, filepath.Join(f.agentDir, "git", "example.com", "acme", "tools"), map[string]string{"package.json": `{"name":"tools"}`, "extensions/a.ts": "x"})
	f.rawGlobal(`{"packages": ["` + raw + `"]}`)
	view, err := json.Marshal(f.view("pi"))
	if err != nil {
		t.Fatal(err)
	}
	ch := PiExtensionChange{Index: 0, Source: "git:https://***@example.com/acme/tools", Path: "extensions/a.ts", Action: "exclude"}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{ch})
	if err != nil {
		t.Fatal(err)
	}
	shown, _ := json.Marshal(plan)
	if strings.Contains(string(view)+string(shown), "dummy-") {
		t.Fatalf("a credential reached the dashboard:\n%s\n%s", view, shown)
	}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{ch}, plan.Revision); err != nil {
		t.Fatal(err)
	}
	if got, want := f.read(f.settingsPath()), `{"packages": [{"source":"`+raw+`","extensions":["-extensions/a.ts"]}]}`; got != want {
		t.Fatalf("file:\n%s\nwant:\n%s", got, want)
	}
}

func TestPiExtensionsRedactsSourceCredentials(t *testing.T) {
	f := newPiFixture(t)
	f.global(map[string]any{"packages": []any{"https://user:secret@example.com/acme/tools.git"}})
	p := f.view("pi").Packages[0]
	if p.Source != "https://***@example.com/acme/tools.git" || p.Identity != "git:example.com/acme/tools" {
		t.Fatalf("package: %+v", p)
	}
}

// PiMinVersion must be a version scripts/pi/version-matrix.sh ran and passed, and every
// recorded version must have passed: the contract against dist/core and against the
// CLI's bundle, and the lock test against that version's own proper-lockfile.
func TestPiMinVersionHasExecutedEvidence(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "pi", "version-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Package  string `json:"package"`
		Versions []struct {
			Version string `json:"version"`
			Passed  bool   `json:"passed"`
			Checks  []struct {
				Name string `json:"name"`
				Exit int    `json:"exit"`
			} `json:"checks"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.Package != "@earendil-works/pi-coding-agent" {
		t.Fatalf("evidence: %v %s", err, doc.Package)
	}
	passed := []string{}
	for _, v := range doc.Versions {
		names := []string{}
		ok := v.Passed
		for _, c := range v.Checks {
			names = append(names, c.Name)
			ok = ok && c.Exit == 0
		}
		slices.Sort(names)
		if ok && slices.Equal(names, []string{"contract/bundle", "contract/core", "native-lock", "project-native"}) {
			passed = append(passed, v.Version)
		}
	}
	if len(passed) != len(doc.Versions) || !slices.Contains(passed, PiMinVersion) {
		t.Fatalf("PiMinVersion = %s, recorded %d versions, executed and passed: %v", PiMinVersion, len(doc.Versions), passed)
	}
}
