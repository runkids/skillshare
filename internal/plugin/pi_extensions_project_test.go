package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// projectFixture registers a project with no .pi folder; the global settings and
// trust.json are written by each test and must never change.
func projectFixture(t *testing.T) (*piFixture, string) {
	t.Helper()
	f := newPiFixture(t)
	root := filepath.Join(f.home, "code", "acme")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	f.svc.ProjectRoot = root
	return f, root
}

func (f *piFixture) projectFile(root string) string {
	return filepath.Join(root, ".pi", "settings.json")
}

// unchanged returns a check that the files still hold the bytes they hold now
// (or are still absent).
func unchanged(t *testing.T, files ...string) func() {
	t.Helper()
	before := map[string][]byte{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		before[file] = data
	}
	return func() {
		t.Helper()
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(data, before[file]) {
				t.Fatalf("%s changed:\n%s", file, data)
			}
		}
	}
}

func (f *piFixture) projectApply(changes ...PiExtensionChange) (*PiExtensionsPlan, error) {
	f.t.Helper()
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", changes)
	if err != nil {
		return nil, err
	}
	return f.svc.ApplyPiExtensions(context.Background(), "pi", changes, plan.Revision)
}

func readJSON(t *testing.T, file string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func findPackage(t *testing.T, v *PiExtensionsView, scope, source string) PiExtensionPackage {
	t.Helper()
	for _, p := range v.Packages {
		if p.Scope == scope && p.Source == source {
			return p
		}
	}
	t.Fatalf("no %s package %s in %+v", scope, source, v.Packages)
	return PiExtensionPackage{}
}

// A global package seen from a project is overridden the way pi config does it:
// a project entry {source, autoload: false, extensions} whose local source is
// relative to the project's .pi. Preview writes nothing; Apply creates only the
// project file, and the global settings and trust.json stay byte for byte.
func TestPiProjectOverrideOfAGlobalPackage(t *testing.T) {
	f, root := projectFixture(t)
	f.global(map[string]any{"packages": []any{f.pkg}})
	f.writeJSON(filepath.Join(f.agentDir, "trust.json"), map[string]any{root: false})
	same := unchanged(t, f.settingsPath(), filepath.Join(f.agentDir, "trust.json"))

	v := f.view("pi")
	if !v.Editable || v.ReadOnly != "" {
		t.Fatalf("project view: editable=%v readOnly=%q", v.Editable, v.ReadOnly)
	}
	pkg := findPackage(t, v, "global", f.pkg)
	if pkg.Shape != "global" || len(pkg.Rows) != 3 || !pkg.Rows[0].Editable {
		t.Fatalf("inherited package: %+v", pkg)
	}
	change := PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".pi")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview created .pi: %v", err)
	}
	if plan.SettingsPath != f.projectFile(root) || len(plan.Entries) != 1 || !plan.Entries[0].Created {
		t.Fatalf("plan: %+v", plan)
	}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); err != nil {
		t.Fatal(err)
	}
	same()
	rel, _ := filepath.Rel(filepath.Join(root, ".pi"), f.pkg)
	want := map[string]any{"packages": []any{map[string]any{"source": filepath.ToSlash(rel), "autoload": false, "extensions": []any{"-extensions/a.ts"}}}}
	if got := readJSON(t, f.projectFile(root)); !reflect.DeepEqual(got, want) {
		t.Fatalf("project settings:\n got %v\nwant %v", got, want)
	}
	delta := f.view("pi").Packages[0]
	if delta.Scope != "project" || delta.Shape != "delta" {
		t.Fatalf("after apply: %+v", delta)
	}
	assertRows(t, selections(delta), "extensions/a.ts:skipped", "extensions/b.ts:loads", "extensions/c.ts:loads")
}

// A non-local source is written as the global settings have it, version and ref
// included, so pi config recognises the override; one carrying credentials or a
// query is never copied into the project.
func TestPiProjectOverrideReferences(t *testing.T) {
	f, root := projectFixture(t)
	writeTree(t, filepath.Join(f.agentDir, "npm", "node_modules", "@acme", "tools"), map[string]string{"package.json": `{"name":"@acme/tools"}`, "extensions/a.ts": "x"})
	writeTree(t, filepath.Join(f.agentDir, "git", "example.com", "acme", "tools"), map[string]string{"package.json": `{"name":"tools"}`, "extensions/a.ts": "x"})
	writeTree(t, filepath.Join(f.agentDir, "git", "example.com", "acme", "secret"), map[string]string{"package.json": `{"name":"secret"}`, "extensions/a.ts": "x"})
	secret := "git:https://dummy-user:dummy-pass@example.com/acme/secret"
	f.global(map[string]any{"packages": []any{"npm:@acme/tools@1.2.0", "git:example.com/acme/tools@v1", secret}})
	same := unchanged(t, f.settingsPath())

	v := f.view("pi")
	if p := findPackage(t, v, "global", redactSource(secret)); p.ReadOnly != "credentials" || p.Rows[0].Editable {
		t.Fatalf("credential source: %+v", p)
	}
	if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{{Scope: "global", Index: 2, Source: redactSource(secret), Path: "extensions/a.ts", Action: "exclude"}}); err == nil {
		t.Fatal("previewed an override that would copy credentials into the project")
	}
	if _, err := f.projectApply(
		PiExtensionChange{Scope: "global", Index: 0, Source: "npm:@acme/tools@1.2.0", Path: "extensions/a.ts", Action: "exclude"},
		PiExtensionChange{Scope: "global", Index: 1, Source: "git:example.com/acme/tools@v1", Path: "extensions/a.ts", Action: "exclude"},
	); err != nil {
		t.Fatal(err)
	}
	same()
	got, _ := json.Marshal(readJSON(t, f.projectFile(root))["packages"])
	want := `[{"autoload":false,"extensions":["-extensions/a.ts"],"source":"npm:@acme/tools@1.2.0"},{"autoload":false,"extensions":["-extensions/a.ts"],"source":"git:example.com/acme/tools@v1"}]`
	if string(got) != want || strings.Contains(string(got), "dummy") {
		t.Fatalf("packages: %s", got)
	}
}

// Removing an override's last rule removes the entry when only source and
// autoload: false are left, and otherwise only its extensions key.
func TestPiProjectDeltaDefaults(t *testing.T) {
	f, root := projectFixture(t)
	other := filepath.Join(f.home, "pkgs", "other")
	writeTree(t, other, map[string]string{"extensions/o.ts": "x"})
	f.global(map[string]any{"packages": []any{f.pkg, other}})
	f.writeJSON(f.projectFile(root), map[string]any{"x-top": 1, "packages": []any{
		map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"-extensions/a.ts"}},
		map[string]any{"source": other, "autoload": false, "extensions": []string{"-extensions/o.ts"}, "x-note": "keep"},
	}})
	same := unchanged(t, f.settingsPath())
	if _, err := f.projectApply(
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "default"},
		PiExtensionChange{Scope: "project", Index: 1, Source: other, Path: "extensions/o.ts", Action: "default"},
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "exclude"},
	); err != nil {
		t.Fatal(err)
	}
	same()
	want := map[string]any{"x-top": float64(1), "packages": []any{
		map[string]any{"source": f.pkg, "autoload": false, "extensions": []any{"-extensions/b.ts"}},
		map[string]any{"source": other, "autoload": false, "x-note": "keep"},
	}}
	if got := readJSON(t, f.projectFile(root)); !reflect.DeepEqual(got, want) {
		t.Fatalf("project settings:\n got %v\nwant %v", got, want)
	}
	if _, err := f.projectApply(PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "default"}); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, f.projectFile(root))["packages"]; !reflect.DeepEqual(got, []any{map[string]any{"source": other, "autoload": false, "x-note": "keep"}}) {
		t.Fatalf("emptied override not removed: %v", got)
	}
}

// A project entry without autoload: false replaces the global one and stays a
// replacement; one that would need converting while its package has convention
// folders the manifest leaves out stays read-only.
func TestPiProjectReplacementEntries(t *testing.T) {
	f, root := projectFixture(t)
	partial := filepath.Join(f.home, "pkgs", "partial")
	otherResourcePackage(t, partial, `{"extensions":["extensions/a.ts"]}`)
	f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"-extensions/c.ts"}}}})
	f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []string{"+extensions/c.ts"}}, partial}})
	same := unchanged(t, f.settingsPath())
	v := f.view("pi")
	if p := findPackage(t, v, "project", partial); p.ReadOnly != "otherResources" || p.Rows[0].Editable {
		t.Fatalf("partial: %+v", p)
	}
	if _, err := f.projectApply(PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}); err != nil {
		t.Fatal(err)
	}
	same()
	got, _ := json.Marshal(readJSON(t, f.projectFile(root))["packages"])
	if want := `[{"extensions":["+extensions/c.ts","-extensions/a.ts"],"source":"` + f.pkg + `"},"` + partial + `"]`; string(got) != want {
		t.Fatalf("packages:\n got %s\nwant %s", got, want)
	}
}

// One apply may override several global packages and change existing entries;
// every requested path is written once and later indexes stay right.
func TestPiProjectBatch(t *testing.T) {
	f, root := projectFixture(t)
	other := filepath.Join(f.home, "pkgs", "other")
	third := filepath.Join(f.home, "pkgs", "third")
	writeTree(t, other, map[string]string{"extensions/o.ts": "x"})
	writeTree(t, third, map[string]string{"extensions/t.ts": "x"})
	f.global(map[string]any{"packages": []any{f.pkg, other, third}})
	f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{
		map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"-extensions/a.ts"}},
		map[string]any{"source": other, "autoload": false, "extensions": []string{"-extensions/o.ts"}},
	}})
	if _, err := f.projectApply(
		PiExtensionChange{Scope: "global", Index: 2, Source: third, Path: "extensions/t.ts", Action: "exclude"},
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "default"},
		PiExtensionChange{Scope: "project", Index: 1, Source: other, Path: "extensions/o.ts", Action: "select"},
	); err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(filepath.Join(root, ".pi"), third)
	got, _ := json.Marshal(readJSON(t, f.projectFile(root))["packages"])
	want := `[{"autoload":false,"extensions":["+extensions/o.ts"],"source":"` + other + `"},{"autoload":false,"extensions":["-extensions/t.ts"],"source":"` + filepath.ToSlash(rel) + `"}]`
	if string(got) != want {
		t.Fatalf("packages:\n got %s\nwant %s", got, want)
	}
}

// Anything that changes after the preview (the project file appearing or changing,
// the global settings) makes the apply stale; nothing is written.
func TestPiProjectApplyIsBoundToBothFiles(t *testing.T) {
	cases := map[string]func(f *piFixture, root string){
		"project file appeared": func(f *piFixture, root string) { f.writeJSON(f.projectFile(root), map[string]any{"theme": "dark"}) },
		"global changed":        func(f *piFixture, root string) { f.global(map[string]any{"packages": []any{f.pkg}, "theme": "dark"}) },
	}
	for name, after := range cases {
		t.Run(name, func(t *testing.T) {
			f, root := projectFixture(t)
			f.global(map[string]any{"packages": []any{f.pkg}})
			change := PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}
			plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
			if err != nil {
				t.Fatal(err)
			}
			after(f, root)
			same := unchanged(t, f.settingsPath(), f.projectFile(root))
			if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); !errors.Is(err, ErrPiExtensionsStale) {
				t.Fatalf("err = %v", err)
			}
			same()
		})
	}
	t.Run("project file changed", func(t *testing.T) {
		f, root := projectFixture(t)
		f.global(map[string]any{"packages": []any{f.pkg}})
		f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"-extensions/a.ts"}}}})
		change := PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "exclude"}
		plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
		if err != nil {
			t.Fatal(err)
		}
		f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"-extensions/c.ts"}}}})
		same := unchanged(t, f.projectFile(root))
		if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); !errors.Is(err, ErrPiExtensionsStale) {
			t.Fatalf("err = %v", err)
		}
		same()
	})
}

// A change must name the package as the view showed it, in its own scope; a
// project preview never applies to the global target.
func TestPiProjectRefusesMisaddressedChanges(t *testing.T) {
	f, root := projectFixture(t)
	f.global(map[string]any{"packages": []any{f.pkg}})
	f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"-extensions/a.ts"}}}})
	same := unchanged(t, f.settingsPath(), f.projectFile(root))
	for name, c := range map[string]PiExtensionChange{
		"shadowed global package": {Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "exclude"},
		"unknown scope":           {Scope: "account", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "exclude"},
		"wrong source":            {Scope: "project", Index: 0, Source: "npm:@acme/other", Path: "extensions/b.ts", Action: "exclude"},
	} {
		if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{c}); err == nil {
			t.Errorf("%s: previewed", name)
		}
	}
	change := PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "exclude"}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
	if err != nil {
		t.Fatal(err)
	}
	f.svc.ProjectRoot = ""
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); err == nil {
		t.Fatal("applied a project preview to the global target")
	}
	same()
}

// The destination is the project's own .pi/settings.json: a linked .pi or file,
// or a lock Pi holds, refuses the write.
func TestPiProjectWriteProtections(t *testing.T) {
	change := func(f *piFixture) PiExtensionChange {
		return PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}
	}
	t.Run("linked .pi", func(t *testing.T) {
		f, root := projectFixture(t)
		f.global(map[string]any{"packages": []any{f.pkg}})
		elsewhere := filepath.Join(f.home, "elsewhere")
		writeTree(t, elsewhere, map[string]string{"settings.json": "{}"})
		if err := os.Symlink(elsewhere, filepath.Join(root, ".pi")); err != nil {
			t.Fatal(err)
		}
		same := unchanged(t, filepath.Join(elsewhere, "settings.json"))
		if f.view("pi").Editable {
			t.Fatal("a linked .pi is editable")
		}
		if _, err := f.projectApply(change(f)); err == nil {
			t.Fatal("wrote through a linked .pi")
		}
		same()
	})
	t.Run("Pi holds the lock", func(t *testing.T) {
		f, root := projectFixture(t)
		f.global(map[string]any{"packages": []any{f.pkg}})
		f.writeJSON(f.projectFile(root), map[string]any{})
		if err := os.Mkdir(f.projectFile(root)+".lock", 0o755); err != nil {
			t.Fatal(err)
		}
		same := unchanged(t, f.projectFile(root))
		if _, err := f.projectApply(change(f)); !errors.Is(err, ErrPiExtensionsBusy) {
			t.Fatalf("err = %v", err)
		}
		same()
		if _, err := os.Stat(f.projectFile(root) + ".lock"); err != nil {
			t.Fatalf("Pi's lock was removed: %v", err)
		}
	})
	t.Run("a refused apply leaves no .pi behind", func(t *testing.T) {
		f, root := projectFixture(t)
		f.global(map[string]any{"packages": []any{f.pkg}})
		plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change(f)})
		if err != nil {
			t.Fatal(err)
		}
		f.global(map[string]any{"packages": []any{f.pkg}, "theme": "dark"})
		if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change(f)}, plan.Revision); err == nil {
			t.Fatal("applied a stale preview")
		}
		if _, err := os.Lstat(filepath.Join(root, ".pi")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf(".pi left behind: %v", err)
		}
	})
}

// An override with no global entry to inherit from loads only the paths it names
// (contract scenarios "project-only delta"): the others are listed as not loaded
// and can be selected; one left with no rule is dropped.
func TestPiProjectDeltaMissingExactRules(t *testing.T) {
	for _, hasBase := range []bool{false, true} {
		for _, rulePath := range []string{"extensions/gone.ts", "./extensions/gone.ts", "absolute"} {
			for _, sign := range []string{"+", "-"} {
				t.Run(fmt.Sprintf("base=%v/%s/%s", hasBase, rulePath, sign), func(t *testing.T) {
					f, root := projectFixture(t)
					if hasBase {
						f.global(map[string]any{"packages": []any{f.pkg}})
					}
					exactPath := rulePath
					if exactPath == "absolute" {
						exactPath = filepath.ToSlash(filepath.Join(f.pkg, "extensions/gone.ts"))
					}
					f.writeJSON(filepath.Join(f.agentDir, "trust.json"), map[string]any{root: false})
					f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{
						"source": f.pkg, "autoload": false, "extensions": []string{"-extensions/a.ts", sign + exactPath},
						"skills": []string{"skills/review"}, "x-note": "keep",
					}}})
					same := unchanged(t, f.settingsPath(), filepath.Join(f.agentDir, "trust.json"))
					previewSame := unchanged(t, f.projectFile(root))
					p := findPackage(t, f.view("pi"), "project", f.pkg)
					missingPath := strings.TrimPrefix(exactPath, "./")
					row, ok := findRow(p.Rows, missingPath)
					if !ok || row.File != "missing" || row.Selection != "none" || row.Origin != "project" || row.Rule != sign+exactPath || !row.Editable {
						t.Fatalf("missing project rule: %+v; rows: %+v", row, p.Rows)
					}
					change := PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: missingPath, Action: "default"}
					for _, action := range []string{"select", "exclude"} {
						invalid := change
						invalid.Action = action
						if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{invalid}); err == nil {
							t.Fatalf("missing file accepted %s", action)
						}
					}
					plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
					if err != nil {
						t.Fatal(err)
					}
					previewSame()
					if len(plan.Rows) != 1 || plan.Rows[0].Before != "none" || plan.Rows[0].After != "none" || !reflect.DeepEqual(plan.Rows[0].Removed, []string{sign + exactPath}) {
						t.Fatalf("plan: %+v", plan)
					}
					if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); err != nil {
						t.Fatal(err)
					}
					same()
					want := map[string]any{"packages": []any{map[string]any{
						"source": f.pkg, "autoload": false, "extensions": []any{"-extensions/a.ts"},
						"skills": []any{"skills/review"}, "x-note": "keep",
					}}}
					if got := readJSON(t, f.projectFile(root)); !reflect.DeepEqual(got, want) {
						t.Fatalf("settings: got %v, want %v", got, want)
					}
					if _, ok := findRow(f.view("pi").Packages[0].Rows, missingPath); ok {
						t.Fatal("removed project rule still has a row")
					}
				})
			}
		}
	}
}

func TestPiProjectDeltaWithoutGlobalBase(t *testing.T) {
	f, root := projectFixture(t)
	f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{"source": f.pkg, "autoload": false, "extensions": []string{"+extensions/a.ts"}}}})
	p := f.view("pi").Packages[0]
	if p.Shape != "deltaOnly" || p.ReadOnly != "" || !p.Rows[1].Editable || p.Rows[1].Origin != "unnamed" {
		t.Fatalf("package: %+v", p)
	}
	assertRows(t, selections(p), "extensions/a.ts:loads", "extensions/b.ts:skipped", "extensions/c.ts:skipped")
	if _, err := f.projectApply(
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "select"},
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"},
	); err != nil {
		t.Fatal(err)
	}
	assertRows(t, selections(f.view("pi").Packages[0]), "extensions/a.ts:skipped", "extensions/b.ts:loads", "extensions/c.ts:skipped")
	if _, err := f.projectApply(
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "default"},
		PiExtensionChange{Scope: "project", Index: 0, Source: f.pkg, Path: "extensions/b.ts", Action: "default"},
	); err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, f.projectFile(root))["packages"]; !reflect.DeepEqual(got, []any{}) {
		t.Fatalf("packages: %v", got)
	}
}

// An existing project entry keeps its source and unknown fields byte for byte,
// credentials included, and neither reaches the view or the plan.
func TestPiProjectKeepsAnEntrysCredentialsAndFields(t *testing.T) {
	f, root := projectFixture(t)
	writeTree(t, filepath.Join(f.agentDir, "git", "example.com", "acme", "tools"), map[string]string{"package.json": `{"name":"tools"}`, "extensions/a.ts": "x", "extensions/b.ts": "x"})
	f.global(map[string]any{"packages": []any{"git:example.com/acme/tools"}})
	raw := `{"packages": [{"source": "git:https://dummy-user:dummy-pass@example.com/acme/tools", "autoload": false, "x-opaque": {"keep": [1, 2]}, "extensions": ["-extensions/a.ts"]}]}`
	writeTree(t, filepath.Join(root, ".pi"), map[string]string{"settings.json": raw})
	change := PiExtensionChange{Scope: "project", Index: 0, Source: "git:https://***@example.com/acme/tools", Path: "extensions/b.ts", Action: "exclude"}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
	if err != nil {
		t.Fatal(err)
	}
	shown, _ := json.Marshal([]any{f.view("pi"), plan})
	if strings.Contains(string(shown), "dummy") || plan.Entries[0].Identity != "git:example.com/acme/tools" {
		t.Fatalf("view and plan: %s", shown)
	}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(raw, `["-extensions/a.ts"]`, `["-extensions/a.ts","-extensions/b.ts"]`, 1)
	if got := f.read(f.projectFile(root)); got != want {
		t.Fatalf("project settings:\n got %s\nwant %s", got, want)
	}
}

// A preview that is already stale is refused before anything is created.
func TestPiProjectStaleApplyHasNoSideEffects(t *testing.T) {
	f, root := projectFixture(t)
	f.global(map[string]any{"packages": []any{f.pkg}})
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}}, "stale"); !errors.Is(err, ErrPiExtensionsStale) {
		t.Fatalf("err = %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(f.svc.StateDir, "pi-extensions", "backups", "*"))
	if _, err := os.Lstat(filepath.Join(root, ".pi")); !errors.Is(err, os.ErrNotExist) || len(backups) != 0 {
		t.Fatalf(".pi: %v, backups: %v", err, backups)
	}
}

// A project file Skillshare can't read as Pi does stays read-only, and so does
// one with an entry it can't read: Pi may keep that entry over the package's
// others. An empty file is written as a new file.
func TestPiProjectSettingsForms(t *testing.T) {
	for name, raw := range map[string]string{
		"null":           "null",
		"duplicate key":  `{"packages": [], "packages": []}`,
		"comment":        "{\n// note\n}",
		"lone surrogate": `{"packages": [{"source": "PKG", "autoload": false, "extensions": ["-extensions/\ud800a.ts"]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			f, root := projectFixture(t)
			f.global(map[string]any{"packages": []any{f.pkg}})
			writeTree(t, filepath.Join(root, ".pi"), map[string]string{"settings.json": strings.ReplaceAll(raw, "PKG", f.pkg)})
			same := unchanged(t, f.projectFile(root), f.settingsPath())
			if v := f.view("pi"); v.Editable || v.ReadOnly != piReadOnlySettings {
				t.Fatalf("view: editable=%v readOnly=%q", v.Editable, v.ReadOnly)
			}
			if _, err := f.projectApply(PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}); err == nil {
				t.Fatal("applied")
			}
			same()
		})
	}
	t.Run("empty", func(t *testing.T) {
		f, root := projectFixture(t)
		f.global(map[string]any{"packages": []any{f.pkg}})
		writeTree(t, filepath.Join(root, ".pi"), map[string]string{"settings.json": ""})
		if _, err := f.projectApply(PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}); err != nil {
			t.Fatal(err)
		}
		if got := readJSON(t, f.projectFile(root)); len(got["packages"].([]any)) != 1 {
			t.Fatalf("project settings: %v", got)
		}
	})
}

// The package's files are part of what was previewed: a changed file list makes
// the apply refuse.
func TestPiProjectApplyIsBoundToDiscovery(t *testing.T) {
	f, root := projectFixture(t)
	f.global(map[string]any{"packages": []any{f.pkg}})
	change := PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}
	plan, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.pkg, "extensions", "a.ts")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ApplyPiExtensions(context.Background(), "pi", []PiExtensionChange{change}, plan.Revision); err == nil {
		t.Fatal("applied after the package changed")
	}
	if _, err := os.Lstat(filepath.Join(root, ".pi")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf(".pi left behind: %v", err)
	}
}

// Losing Pi's lock between the check and the write refuses the write, for a new
// file too, and leaves no .pi behind.
func TestPiProjectLockLostMidApply(t *testing.T) {
	for name, takeover := range map[string]func(t *testing.T, dir string){
		"replaced": func(t *testing.T, dir string) {
			if err := os.Remove(dir); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"touched": func(t *testing.T, dir string) {
			old := time.Now().Add(-time.Minute)
			if err := os.Chtimes(dir, old, old); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, root := projectFixture(t)
			f.global(map[string]any{"packages": []any{f.pkg}})
			piBeforeWrite = func(dir string) { takeover(t, dir) }
			t.Cleanup(func() { piBeforeWrite = func(string) {} })
			if _, err := f.projectApply(PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}); !errors.Is(err, ErrPiExtensionsBusy) {
				t.Fatalf("err = %v", err)
			}
			if _, err := os.Stat(f.projectFile(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("settings written: %v", err)
			}
			// The lock is no longer provably ours, so it stays, and with it .pi.
			if _, err := os.Stat(f.projectFile(root) + ".lock"); err != nil {
				t.Fatalf("the other owner's lock was removed: %v", err)
			}
		})
	}
}

// Pi keeps the first global entry of a package. When Skillshare can't read that
// entry, a later one of the same package doesn't govern and isn't offered for
// editing, nor is a project override on top of it; when it can't tell which
// package an entry is, the whole project view is read-only.
func TestPiProjectFirstGlobalEntryGoverns(t *testing.T) {
	for name, first := range map[string]string{
		"unreadable rules":  `{"source": "%s", "extensions": ["*", "!extensions/\ud800.ts"]}`,
		"autoload false":    `{"source": "%s", "autoload": false, "extensions": ["+extensions/b.ts"]}`,
		"unreadable source": `"./pkgs/\udc00"`,
	} {
		t.Run(name, func(t *testing.T) {
			f, root := projectFixture(t)
			if strings.Contains(first, "%s") {
				first = strings.ReplaceAll(first, "%s", f.pkg)
			}
			f.rawGlobal(`{"packages": [` + first + `, {"source": "` + f.pkg + `", "extensions": ["-extensions/a.ts"]}]}`)
			for _, p := range f.view("pi").Packages {
				for _, r := range p.Rows {
					if r.Editable {
						t.Fatalf("editable row under an unread first entry: %+v", p)
					}
				}
			}
			change := PiExtensionChange{Scope: "global", Index: 1, Source: f.pkg, Path: "extensions/b.ts", Action: "exclude"}
			if _, err := f.svc.PreviewPiExtensions(context.Background(), "pi", []PiExtensionChange{change}); err == nil {
				t.Fatal("previewed an override of an entry Pi ignores")
			}

			rel, _ := filepath.Rel(filepath.Join(root, ".pi"), f.pkg)
			f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{map[string]any{"source": filepath.ToSlash(rel), "autoload": false, "extensions": []any{"+extensions/a.ts"}}}})
			v := f.view("pi")
			p := findPackage(t, v, "project", filepath.ToSlash(rel))
			if v.Editable && p.Problem == "" && p.ReadOnly == "" {
				t.Fatalf("project override over an unread first entry: %+v", p)
			}
			for _, r := range p.Rows {
				if r.Editable {
					t.Fatalf("editable project row: %+v", p)
				}
			}
		})
	}
}

// The global view keeps the same rule: an unread first entry still makes later
// entries of its package duplicates.
func TestPiExtensionsUnreadFirstEntryStillDedupes(t *testing.T) {
	f := newPiFixture(t)
	f.rawGlobal(`{"packages": [{"source": "` + f.pkg + `", "extensions": ["*", "!extensions/\ud800.ts"]}, {"source": "` + f.pkg + `", "extensions": ["-extensions/a.ts"]}]}`)
	if p := f.view("pi").Packages[1]; p.Problem != "duplicate" || len(p.Rows) != 0 {
		t.Fatalf("second entry: %+v", p)
	}
}

// Apply revalidates both settings files and the package at the write itself, not
// only when it takes the lock: a change made meanwhile to what was reviewed (the
// entries written and the rows changed) refuses the write.
func TestPiProjectApplyRevalidatesAtTheWrite(t *testing.T) {
	for name, tc := range map[string]struct {
		project bool
		change  func(f *piFixture)
	}{
		"global settings, new file": {change: func(f *piFixture) {
			f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []any{"-extensions/b.ts"}}}})
		}},
		"changed file left the package, new file": {change: func(f *piFixture) {
			if err := os.Remove(filepath.Join(f.pkg, "extensions", "a.ts")); err != nil {
				f.t.Fatal(err)
			}
		}},
		"global settings, existing file": {project: true, change: func(f *piFixture) {
			f.global(map[string]any{"packages": []any{map[string]any{"source": f.pkg, "extensions": []any{"-extensions/b.ts"}}}})
		}},
	} {
		t.Run(name, func(t *testing.T) {
			f, root := projectFixture(t)
			f.global(map[string]any{"packages": []any{f.pkg}})
			if tc.project {
				f.writeJSON(f.projectFile(root), map[string]any{"theme": "dark"})
			}
			same := unchanged(t, f.projectFile(root))
			piBeforeWrite = func(string) { tc.change(f) }
			t.Cleanup(func() { piBeforeWrite = func(string) {} })
			if _, err := f.projectApply(PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"}); !errors.Is(err, ErrPiExtensionsStale) {
				t.Fatalf("err = %v", err)
			}
			same()
			if _, err := os.Lstat(filepath.Join(root, ".pi")); !tc.project && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf(".pi left behind: %v", err)
			}
		})
	}
}

// Pi keeps a string package's skills, prompts and themes under a project
// override even when its manifest lists only extensions (contract scenario 24),
// so such a package can be overridden; the override carries no resource key but
// extensions.
func TestPiProjectOverrideOfAPartialManifestPackage(t *testing.T) {
	f, root := projectFixture(t)
	partial := filepath.Join(f.home, "pkgs", "partial")
	otherResourcePackage(t, partial, `{"extensions":["extensions/a.ts"]}`)
	f.global(map[string]any{"packages": []any{partial}})
	if _, err := f.projectApply(PiExtensionChange{Scope: "global", Index: 0, Source: partial, Path: "extensions/a.ts", Action: "exclude"}); err != nil {
		t.Fatal(err)
	}
	entry := readJSON(t, f.projectFile(root))["packages"].([]any)[0].(map[string]any)
	if len(entry) != 3 || entry["autoload"] != false || entry["extensions"] == nil {
		t.Fatalf("override: %v", entry)
	}
}

// What the dashboard shows after a project change is what Pi resolves: the pi
// CLI's own package manager reads the files this editor wrote, with the project
// trusted and untrusted. Skills and prompts, the partial-manifest package's
// included, are the same whether Pi reads the project's overrides or not.
// It needs Pi's install and Node, so it runs only with PI_ROOT set
// (scripts/pi/version-matrix.sh).
func TestPiProjectOverridesResolveInPi(t *testing.T) {
	piRoot := os.Getenv("PI_ROOT")
	if piRoot == "" {
		t.Skip("PI_ROOT not set")
	}
	f, root := projectFixture(t)
	var manifest struct{ Version string }
	if err := json.Unmarshal([]byte(f.read(filepath.Join(piRoot, "package.json"))), &manifest); err != nil {
		t.Fatal(err)
	}
	f.version = manifest.Version
	partial := filepath.Join(f.home, "pkgs", "partial")
	otherResourcePackage(t, partial, `{"extensions":["extensions/a.ts"]}`)
	third := filepath.Join(f.home, "pkgs", "third")
	writeTree(t, third, map[string]string{"extensions/t.ts": "throw new Error('never imported')", "extensions/u.ts": "throw new Error('never imported')"})
	solo := filepath.Join(f.home, "pkgs", "solo")
	writeTree(t, solo, map[string]string{"extensions/s1.ts": "throw new Error('never imported')", "extensions/s2.ts": "throw new Error('never imported')", "extensions/s3.ts": "throw new Error('never imported')"})
	f.global(map[string]any{"packages": []any{f.pkg, partial, map[string]any{"source": third, "extensions": []string{"-extensions/t.ts"}}}})
	f.writeJSON(f.projectFile(root), map[string]any{"packages": []any{
		map[string]any{"source": third, "extensions": []string{"-extensions/u.ts"}},
		map[string]any{"source": solo, "autoload": false, "extensions": []string{"+extensions/s1.ts", "-extensions/s3.ts"}},
	}})
	if _, err := f.projectApply(
		PiExtensionChange{Scope: "global", Index: 0, Source: f.pkg, Path: "extensions/a.ts", Action: "exclude"},
		PiExtensionChange{Scope: "global", Index: 1, Source: partial, Path: "extensions/a.ts", Action: "exclude"},
		PiExtensionChange{Scope: "project", Index: 0, Source: third, Path: "extensions/u.ts", Action: "default"},
		PiExtensionChange{Scope: "project", Index: 1, Source: solo, Path: "extensions/s2.ts", Action: "select"},
	); err != nil {
		t.Fatal(err)
	}
	shown := func(v *PiExtensionsView) map[string]bool {
		m := map[string]bool{}
		for _, p := range v.Packages {
			for _, r := range p.Rows {
				if r.File == "present" {
					m[filepath.Join(filepath.FromSlash(strings.TrimPrefix(p.Identity, "local:")), filepath.FromSlash(r.Path))] = r.Selection == "loads"
				}
			}
		}
		return m
	}
	resolved := func(trust string) (map[string]bool, map[string]bool) {
		cmd := exec.Command("node", filepath.Join("..", "..", "scripts", "pi", "resolve-probe.mjs"), root, f.agentDir, trust)
		cmd.Env = append(os.Environ(), "PI_ROOT="+piRoot)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("probe: %v %s", err, out)
		}
		var r struct{ Extensions, Skills, Prompts map[string]bool }
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatal(err)
		}
		others := map[string]bool{}
		for p, on := range r.Skills {
			others[p] = on
		}
		for p, on := range r.Prompts {
			others[p] = on
		}
		return r.Extensions, others
	}
	// Pi lists a path a project-only override doesn't name nowhere; shown, it is not loaded.
	same := func(pi, shown map[string]bool) bool {
		for p, on := range pi {
			if _, ok := shown[p]; !ok || shown[p] != on {
				return false
			}
		}
		for p, on := range shown {
			if pi[p] != on {
				return false
			}
		}
		return true
	}
	trusted, others := resolved("trusted")
	if want := shown(f.view("pi")); !same(trusted, want) || trusted[filepath.Join(solo, "extensions", "s2.ts")] != true {
		t.Fatalf("trusted project:\n Pi %v\nshown %v", trusted, want)
	}
	untrusted, othersUntrusted := resolved("untrusted")
	if len(others) == 0 || !reflect.DeepEqual(others, othersUntrusted) {
		t.Fatalf("skills and prompts:\n trusted %v\nuntrusted %v", others, othersUntrusted)
	}
	f.svc.ProjectRoot = ""
	if want := shown(f.view("pi")); !same(untrusted, want) {
		t.Fatalf("untrusted project:\n Pi %v\nshown %v", untrusted, want)
	}
}
