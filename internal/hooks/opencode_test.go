package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestOpenCodePackageLifecycle(t *testing.T) {
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "opencode v2.3.0", nil }
	config := filepath.Join(e.home, ".config/opencode/opencode.jsonc")
	write(t, config, "{\n // keep me\n \"model\":\"unchanged\",\"plugins\":[\"other\"],\n}\n")
	m := Mutation{Name: "native", Entry: entry(t, `{"bindings":{"opencode":{"code":"export default {}\n"}}}`)}
	save(t, e.service, m)
	dir := filepath.Join(e.home, ".config/opencode/plugins/skillshare-native")
	if got := read(t, filepath.Join(dir, "package.json")); !strings.Contains(got, "@opencode/plugin") {
		t.Fatal(got)
	}
	if got := read(t, config); !strings.Contains(got, "// keep me") || !strings.Contains(got, "other") || strings.Contains(got, "index.ts") {
		t.Fatal(got)
	}
	if r := sync(t, e.service); len(r.Applied) != 0 {
		t.Fatal(r.Applied)
	}
	m.Entry.Bindings["opencode"] = Binding{Code: "export default { changed: true }\n"}
	save(t, e.service, m)
	if !strings.Contains(read(t, filepath.Join(dir, "index.ts")), "changed") {
		t.Fatal("not updated")
	}
	if r := sync(t, e.service); len(r.Applied) != 0 {
		t.Fatal(r.Applied)
	}
	save(t, e.service, Mutation{Name: "native", Remove: true})
	if exists(dir) {
		t.Fatal("package directory remains")
	}
	if got := read(t, config); strings.Contains(got, "skillshare-native") || !strings.Contains(got, "other") || !strings.Contains(got, "// keep me") {
		t.Fatal(got)
	}
}

func TestOpenCodePackageImportPreservesManifestAndOwnership(t *testing.T) {
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	original := filepath.Join(e.home, ".config/opencode/plugins/kata-user")
	manifest := "{\n  \"name\":\"kata-native-agent-hooks\", \"type\":\"module\", \"exports\":\"./index.js\",\n  \"dependencies\":{\"@opencode/plugin\":\"^2.0.0\"}, \"kata\":{\"api\":\"v2\"}\n}\n"
	write(t, filepath.Join(original, "package.json"), manifest)
	write(t, filepath.Join(original, "index.js"), "export default {}\n")
	write(t, filepath.Join(original, "lib/helper.js"), "export const helper = true\n")
	write(t, filepath.Join(original, "node_modules/ignored.js"), "not copied")
	candidates, err := e.service.Import(ImportRequest{From: "opencode", Name: "kata-user"})
	must(t, err)
	if len(candidates) != 1 || len(candidates[0].Problems) > 0 {
		t.Fatalf("%+v", candidates)
	}
	c := candidates[0]
	if c.Entry.Bindings["opencode"].Package["package.json"] != manifest || len(c.Entry.Bindings["opencode"].Package) != 3 || len(c.Warnings) == 0 {
		t.Fatalf("%+v", c)
	}
	save(t, e.service, Mutation{Name: c.Name, Entry: &c.Entry, Adopt: true})
	dir := filepath.Join(e.home, ".config/opencode/plugins/skillshare-kata-user")
	if got := read(t, filepath.Join(dir, "package.json")); got != manifest {
		t.Fatal(got)
	}
	if r := sync(t, e.service); len(r.Applied) != 0 {
		t.Fatal(r.Applied)
	}
	write(t, filepath.Join(dir, "lib/helper.js"), "outside edit")
	p, err := e.service.Preview()
	must(t, err)
	if !p.Blocked {
		t.Fatal("external package edits must conflict")
	}
	if read(t, filepath.Join(original, "package.json")) != manifest {
		t.Fatal("original modified")
	}
}

func TestOpenCodeVersionAndScope(t *testing.T) {
	e := newEnv(t)
	m := Mutation{Name: "native", Entry: entry(t, `{"bindings":{"opencode":{"code":"export default {}\n"}}}`)}
	version := "1.18.31"
	e.service.OpenCodeVersion = func(string) (string, error) { return version, nil }
	p, err := e.service.PreviewMutation(m)
	must(t, err)
	version = "2.3.0"
	if _, err := e.service.Mutate(m, p.Revision, true); err != ErrStaleRevision {
		t.Fatalf("version change accepted: %v", err)
	}
	version = "1.18.31"
	save(t, e.service, m)
	flat := filepath.Join(e.home, ".config/opencode/plugins/skillshare-native.ts")
	if !exists(flat) {
		t.Fatal("v1 output changed")
	}
	version = "2.3.0"
	sync(t, e.service)
	if exists(flat) {
		t.Fatal("owned v1 file not retired")
	}
	if r := sync(t, e.service); len(r.Applied) != 0 {
		t.Fatal(r.Applied)
	}
	project := e.project(t.TempDir())
	project.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	save(t, project, m)
	if !exists(filepath.Join(project.ProjectRoot, ".opencode/opencode.json")) {
		t.Fatal("missing project registration")
	}
}

func TestOpenCodePackageValidation(t *testing.T) {
	for _, name := range []string{".", "../escape.js", "/absolute.js", "nested/../../escape", "node_modules/x.js", "a\\b.js"} {
		files := map[string]string{"package.json": `{"name":"demo","exports":"./index.js"}`, "index.js": "export default {}", name: "no"}
		if err := validateOpenCodePackage(files); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
	if err := validateOpenCodePackage(map[string]string{"package.json": `{"name":"demo","exports":"./lib/index.js"}`, "lib": "file", "lib/index.js": "export default {}"}); err == nil {
		t.Fatal("file/ancestor collision accepted")
	}

}

func TestOpenCodePackageRender(t *testing.T) {
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	files, err := e.service.RenderNative(Mutation{Name: "native", Entry: entry(t, `{"bindings":{"opencode":{"code":"export default {}"}}}`)})
	must(t, err)
	if len(files) != 3 {
		t.Fatalf("%+v", files)
	}
	for _, file := range files {
		if file.Error != "" || file.Content == "" {
			t.Fatalf("%+v", file)
		}
	}
}

func TestOpenCodePackageNestedPaths(t *testing.T) {
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	files := map[string]string{"package.json": `{"name":"native","exports":"./index.js"}`, "index.js": "export default {}"}
	for _, dir := range []string{"hooks", "plugins", "extensions", "skillshare/nested", "plugins/skillshare-inner", "plugins/skillshare-native"} {
		files["lib/"+dir+"/helper.js"] = "export const helper = true"
	}
	m := Mutation{Name: "native", Entry: &Entry{Bindings: map[string]Binding{"opencode": {Package: files}}}}
	result := save(t, e.service, m)
	if r := sync(t, e.service); r.Plan.Blocked || len(r.Applied) != 0 {
		t.Fatalf("nested package is not a no-op: %+v", r.Plan)
	}
	for _, id := range result.BackupIDs {
		p, err := e.service.PreviewRestore(id)
		must(t, err)
		if p.Blocked {
			t.Fatalf("package backup parked: %+v", p)
		}
	}
	save(t, e.service, Mutation{Name: "native", Remove: true})
	if exists(filepath.Join(e.home, ".config/opencode/plugins/skillshare-native")) {
		t.Fatal("owned package remains")
	}
}

const openCodeEntry = `{"bindings":{"opencode":{"code":"export default {}\n"}}}`

// A machine without a usable opencode binary still syncs every other Agent, and
// leaves the OpenCode outputs it cannot lay out exactly as they are.
func TestOpenCodeUndetectedVersionSkipsOnlyOpenCode(t *testing.T) {
	for name, probe := range map[string]func(string) (string, error){
		"missing binary": func(string) (string, error) { return "", errors.New("executable file not found in $PATH") },
		"future version": func(string) (string, error) { return "3.0.0", nil },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			save(t, e.service, Mutation{Name: "a", Entry: entry(t, openCodeEntry)})
			save(t, e.service, Mutation{Name: "b", Entry: entry(t, openCodeEntry)})
			flat := filepath.Join(e.home, ".config/opencode/plugins/skillshare-a.ts")
			calls := 0
			e.service.OpenCodeVersion = func(bin string) (string, error) { calls++; return probe(bin) }
			r := save(t, e.service, Mutation{Name: "guard", Entry: entry(t, claudeEntry)})
			if r.Plan.Blocked || len(events(t, filepath.Join(e.home, ".claude/settings.json"), true)["PreToolUse"]) != 1 {
				t.Fatalf("claude did not sync: %+v", r.Plan)
			}
			if !exists(flat) {
				t.Fatal("owned OpenCode output removed while the version is unknown")
			}
			warnings := strings.Join(r.Plan.Warnings, "\n")
			if strings.Count(warnings, "its hook bindings are skipped") != 1 || !strings.Contains(warnings, "are left as they are") {
				t.Fatalf("warnings: %v", r.Plan.Warnings)
			}
			root := t.TempDir()
			save(t, e.service, Mutation{Project: root})
			r = save(t, e.service, Mutation{Project: root, Name: "a", Entry: entry(t, openCodeEntry)})
			if n := strings.Count(strings.Join(r.Plan.Warnings, "\n"), "its hook bindings are skipped"); n != 1 || exists(filepath.Join(root, ".opencode")) {
				t.Fatalf("project root: %d skip warnings: %v", n, r.Plan.Warnings)
			}
			calls = 0
			_, err := e.service.Preview()
			must(t, err)
			if calls != 1 {
				t.Fatalf("probe ran %d times for one plan", calls)
			}
			files, err := e.service.RenderNative(Mutation{Name: "a", Entry: entry(t, openCodeEntry)})
			must(t, err)
			if len(files) != 1 || !strings.Contains(files[0].Error, "skipped") {
				t.Fatalf("%+v", files)
			}
		})
	}
}

func TestOpenCodePackageOnV1IsSkipped(t *testing.T) {
	e := newEnv(t)
	pkg := &Entry{Bindings: map[string]Binding{"opencode": {Package: map[string]string{"package.json": `{"name":"native","exports":"./index.js"}`, "index.js": "export default {}"}}}}
	r := save(t, e.service, Mutation{Name: "native", Entry: pkg})
	if r.Plan.Blocked || !strings.Contains(strings.Join(r.Plan.Warnings, "\n"), "requires OpenCode v2") {
		t.Fatalf("%+v", r.Plan)
	}
	if exists(filepath.Join(e.home, ".config/opencode/plugins/skillshare-native")) {
		t.Fatal("package written for OpenCode v1")
	}
}

func TestOpenCodeDependencyWarningOnlyWhenWritten(t *testing.T) {
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	save(t, e.service, Mutation{Name: "a", Entry: entry(t, openCodeEntry)})
	r := save(t, e.service, Mutation{Name: "b", Entry: entry(t, openCodeEntry)})
	if n := strings.Count(strings.Join(r.Plan.Warnings, "\n"), openCodeDependencyWarning); n != 1 {
		t.Fatalf("warned %d times: %v", n, r.Plan.Warnings)
	}
	p, err := e.service.Preview()
	must(t, err)
	if len(p.Warnings) != 0 {
		t.Fatalf("synced packages still warn: %v", p.Warnings)
	}
}

func TestOpenCodePackageUpdateAndDowngrade(t *testing.T) {
	e := newEnv(t)
	version := "2.3.0"
	e.service.OpenCodeVersion = func(string) (string, error) { return version, nil }
	manifest := `{"name":"native","exports":"./index.js"}`
	m := Mutation{Name: "native", Entry: &Entry{Bindings: map[string]Binding{"opencode": {Package: map[string]string{"package.json": manifest, "index.js": "export default {}", "lib/old.js": "old"}}}}}
	save(t, e.service, m)
	dir := filepath.Join(e.home, ".config/opencode/plugins/skillshare-native")
	m.Entry.Bindings["opencode"] = Binding{Package: map[string]string{"package.json": manifest, "index.js": "export default { changed: true }"}}
	save(t, e.service, m)
	if !strings.Contains(read(t, filepath.Join(dir, "index.js")), "changed") || exists(filepath.Join(dir, "lib/old.js")) {
		t.Fatal("package not updated in place")
	}
	save(t, e.service, Mutation{Name: "flat", Entry: entry(t, openCodeEntry)})
	version = "1.18.31"
	r := sync(t, e.service)
	config := filepath.Join(e.home, ".config/opencode/opencode.json")
	if exists(dir) || exists(filepath.Join(e.home, ".config/opencode/plugins/skillshare-flat")) || !exists(filepath.Join(e.home, ".config/opencode/plugins/skillshare-flat.ts")) {
		t.Fatalf("v2 outputs not retired on v1: %s", actions(r.Plan))
	}
	if exists(config) && strings.Contains(read(t, config), "skillshare-") {
		t.Fatal(read(t, config))
	}
	if r := sync(t, e.service); len(r.Applied) != 0 {
		t.Fatal(r.Applied)
	}
}

func TestOpenCodeImportKeepsOtherCandidatesBesideUnreadablePackage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	plugins := filepath.Join(e.home, ".config/opencode/plugins")
	write(t, filepath.Join(plugins, "flat.ts"), "export default {}\n")
	for _, name := range []string{"good", "linked"} {
		write(t, filepath.Join(plugins, name, "package.json"), `{"name":"demo","exports":"./index.js"}`)
		write(t, filepath.Join(plugins, name, "index.js"), "export default {}\n")
	}
	must(t, os.Symlink(filepath.Join(e.home, "elsewhere"), filepath.Join(plugins, "linked", "bin")))
	candidates, err := e.service.Import(ImportRequest{From: "opencode"})
	must(t, err)
	problems := map[string][]string{}
	for _, c := range candidates {
		problems[c.Name] = c.Problems
	}
	if len(candidates) != 3 || len(problems["flat"]) != 0 || len(problems["good"]) != 0 {
		t.Fatalf("%+v", candidates)
	}
	if len(problems["linked"]) != 1 || !strings.Contains(problems["linked"][0], `"bin" cannot be imported`) {
		t.Fatalf("%v", problems["linked"])
	}
}

func TestOpenCodeInvalidPackageProblemReportedOnce(t *testing.T) {
	e := newEnv(t)
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	write(t, filepath.Join(e.home, ".config/opencode/plugins/broken/readme.md"), "no entry\n")
	candidates, err := e.service.Import(ImportRequest{From: "opencode"})
	must(t, err)
	if len(candidates) != 1 || len(candidates[0].Problems) != 1 {
		t.Fatalf("%+v", candidates)
	}
}

func TestOpenCodeV1DoesNotListDirectories(t *testing.T) {
	e := newEnv(t)
	plugins := filepath.Join(e.home, ".config/opencode/plugins")
	write(t, filepath.Join(plugins, "lib/helper.js"), "export const helper = true\n")
	write(t, filepath.Join(plugins, "mine.ts"), "export const Mine = async () => ({})\n")
	inv, err := e.service.List()
	must(t, err)
	for _, u := range inv.Unmanaged {
		if u.Target == "opencode" && !slices.Equal(u.Names, []string{"mine.ts"}) {
			t.Fatalf("%+v", u)
		}
	}
	e.service.OpenCodeVersion = func(string) (string, error) { return "2.3.0", nil }
	inv, err = e.service.List()
	must(t, err)
	if !slices.ContainsFunc(inv.Unmanaged, func(u Unmanaged) bool { return u.Target == "opencode" && slices.Contains(u.Names, "lib") }) {
		t.Fatalf("%+v", inv.Unmanaged)
	}
}

func TestOpenCodeAccountProbesItsOwnCLI(t *testing.T) {
	e := newEnv(t)
	work := filepath.Join(e.home, "work")
	must(t, os.MkdirAll(work, 0755))
	e.service.Accounts = map[string]Account{"work": {Agent: "opencode", Dir: work, CLI: "/opt/opencode-next"}}
	e.service.OpenCodeVersion = func(bin string) (string, error) {
		if bin == "/opt/opencode-next" {
			return "2.3.0", nil
		}
		return "1.18.31", nil
	}
	save(t, e.service, Mutation{Name: "native", Entry: &Entry{Bindings: map[string]Binding{"work": {Code: "export default {}\n"}, "opencode": {Code: "export default {}\n"}}}})
	if !exists(filepath.Join(work, "plugins/skillshare-native/package.json")) || !exists(filepath.Join(e.home, ".config/opencode/plugins/skillshare-native.ts")) {
		t.Fatal("each OpenCode home must follow its own CLI version")
	}
}
