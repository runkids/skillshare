package hooks

import (
	"path/filepath"
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
	version = "3.0.0"
	if _, err := e.service.Preview(); err == nil {
		t.Fatal("future API guessed")
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
