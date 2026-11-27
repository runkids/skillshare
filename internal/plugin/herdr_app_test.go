package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestHerdrModLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "herdr"))
	if err := os.Mkdir(filepath.Join(home, "herdr"), 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "first")
	writePluginFile(t, source, "mods/demo/old", "old")
	writePluginFile(t, source, "mods/demo/jsconfig.json", "reserved")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
	dest := filepath.Join(home, "herdr/mods/demo")
	writePluginFile(t, dest, "jsconfig.json", "generated")
	writePluginFile(t, dest, "gpui-kit.d.ts", "generated")
	writeHerdrRuntimeFiles(t, dest)
	if err := os.Remove(filepath.Join(source, "mods/demo/old")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, source, "mods/demo/main.js", "second")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	assertHerdrRuntimeGone(t, dest)
	if _, err := os.Stat(filepath.Join(dest, "old")); !os.IsNotExist(err) {
		t.Fatal("obsolete file retained", err)
	}
	writePluginFile(t, dest, "main.js", "edited")
	r := Request{Action: "remove", Name: "demo"}
	p, err := s.Preview(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), r, p.Revision); err == nil {
		t.Fatal("removed edited file")
	}
	if data, _ := os.ReadFile(filepath.Join(dest, "main.js")); string(data) != "edited" {
		t.Fatal("lost edits")
	}
	writePluginFile(t, source, "mods/demo/main.js", "third")
	update := Request{Action: "update", Name: "demo"}
	p, err = s.Preview(context.Background(), update)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), update, p.Revision); err == nil {
		t.Fatal("updated edited folder")
	}
	writePluginFile(t, dest, "main.js", "second")
	writeHerdrRuntimeFiles(t, dest)
	applyPluginRequest(t, s, r)
}

func TestHerdrBatchAdd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	ctx := context.Background()
	home := t.TempDir()
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.Mkdir(herdr, 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	for _, name := range []string{"one", "two", "unchecked"} {
		writePluginFile(t, source, "mods/"+name+"/main.js", name)
	}
	writePluginFile(t, source, "layouts/custom.json", "{}")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	r := Request{Action: "add", Source: source, Targets: []string{"herdr-app"}, Plugins: []string{"one", "two"}}
	for _, invalid := range [][]string{{"one", "ONE"}, {"one", "missing"}} {
		request := r
		request.Plugins = invalid
		if _, err := s.Preview(ctx, request); err == nil {
			t.Fatalf("accepted invalid selection %v", invalid)
		}
	}
	plan, err := s.Preview(ctx, r)
	if err != nil || plan.Blocked || len(plan.Changes) != 2 || plan.HerdrPanel != nil {
		t.Fatalf("expected a single review of both mods, got %+v, %v", plan, err)
	}
	for i, name := range []string{"one", "two"} {
		if c := plan.Changes[i]; c.Name != name || c.Action != "install" || c.Target != "herdr-app" {
			t.Fatalf("unexpected preview row %+v", c)
		}
	}
	if _, err := os.Stat(filepath.Join(herdr, "mods")); !os.IsNotExist(err) {
		t.Fatalf("preview created mods: %v", err)
	}
	changed := r
	changed.Plugins = []string{"one", "unchecked"}
	if _, err := s.Apply(ctx, changed, plan.Revision); err == nil {
		t.Fatal("accepted a different selection than the reviewed selection")
	}
	// A conflict must block the entire batch before any mod is written.
	writePluginFile(t, herdr, "mods/two/main.js", "existing")
	blocked, err := s.Preview(ctx, r)
	if err != nil || !blocked.Blocked {
		t.Fatalf("expected a blocked batch, got %+v, %v", blocked, err)
	}
	if _, err := s.Apply(ctx, r, blocked.Revision); err == nil {
		t.Fatal("applied a blocked batch")
	}
	if _, err := os.Stat(filepath.Join(herdr, "mods/one")); !os.IsNotExist(err) {
		t.Fatalf("blocked preview wrote another mod: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(herdr, "mods/two")); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, r)
	for _, name := range []string{"one", "two"} {
		if data, err := os.ReadFile(filepath.Join(herdr, "mods", name, "main.js")); err != nil || string(data) != name {
			t.Fatalf("%s was not installed: %q %v", name, data, err)
		}
	}
	for _, path := range []string{"mods/unchecked", "layouts"} {
		if _, err := os.Stat(filepath.Join(herdr, path)); !os.IsNotExist(err) {
			t.Fatalf("installed unselected path %s: %v", path, err)
		}
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "one"})
	if _, err := os.Stat(filepath.Join(herdr, "mods/two/main.js")); err != nil {
		t.Fatalf("removing one mod touched the other: %v", err)
	}
}

func TestHerdrGitHubLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file URL fixture uses a POSIX path")
	}
	ctx := context.Background()
	t.Chdir(t.TempDir())
	repo := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if _, err := runCommand(ctx, repo, nil, "git", args...); err != nil {
			t.Fatalf("git %s: %v", args[0], err)
		}
	}
	commit := func() {
		git("add", ".")
		git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "mod")
	}
	git("init", "-b", "main")
	writePluginFile(t, repo, "gallery/package/mods/demo/main.js", "first")
	writePluginFile(t, repo, "gallery/package/mods/other/main.js", "other")
	writePluginFile(t, repo, "mods/unrelated/main.js", "outside the package")
	commit()
	git("branch", "gallery/stable")
	writePluginFile(t, repo, "gallery/package/mods/demo/main.js", "default branch")
	commit()
	const remote = "https://github.com/example/mod-gallery"
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	writePluginFile(t, filepath.Dir(cfg), filepath.Base(cfg), "[url \"file://"+repo+"\"]\n  insteadOf = "+remote+".git\n")
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)

	folder := remote + "/tree/gallery/stable/gallery/package"
	list, err := Discover(ctx, folder)
	if err != nil || len(list.Candidates) != 2 {
		t.Fatalf("package link did not scope discovery: %+v %v", list, err)
	}
	file := remote + "/blob/gallery/stable/gallery/package/mods/demo/main.js"
	d, err := Discover(ctx, file+"#L1")
	if err != nil || len(d.Candidates) != 1 || d.Candidates[0].Name != "demo" || d.Source != file || d.Commit == "" {
		t.Fatalf("mod link did not resolve to one mod: %+v %v", d, err)
	}
	for _, tc := range []struct{ source, entry, key string }{
		{file, "bin/launcher", "plugins.error.herdrModLink"},
		{remote + "/blob/main/README.md", "", "plugins.error.herdrModLink"},
		{remote + "/tree/main/.git", "", "plugins.error.sourceFolder"},
		{remote + "/tree/main/gallery/missing", "", "plugins.error.sourceFolder"},
	} {
		_, err := DiscoverOptions(ctx, tc.source, "", tc.entry)
		if key, _ := ErrorKey(err); key != tc.key {
			t.Fatalf("discovery of %s: wanted %s, got %v", tc.source, tc.key, err)
		}
	}

	home := t.TempDir()
	t.Setenv("HERDR_APP_HOME", home)
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: d.Source, Targets: []string{"herdr-app"}})
	installed := filepath.Join(home, "mods/demo/main.js")
	if data, err := os.ReadFile(installed); err != nil || string(data) != "first" {
		t.Fatalf("did not install from the linked branch: %q %v", data, err)
	}
	git("checkout", "gallery/stable")
	writePluginFile(t, repo, "gallery/package/mods/demo/main.js", "updated")
	commit()
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo", Targets: []string{"herdr-app"}})
	if data, err := os.ReadFile(installed); err != nil || string(data) != "updated" {
		t.Fatalf("did not update from the linked branch: %q %v", data, err)
	}
}

func TestHerdrSymlinkedHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	resolved := t.TempDir()
	link := filepath.Join(home, "herdr")
	if err := os.Symlink(resolved, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_APP_HOME", link)
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "demo")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	req := Request{Action: "add", Source: source, Targets: []string{"herdr-app"}}
	preview, err := s.Preview(context.Background(), req)
	if err != nil || preview.Blocked {
		t.Fatalf("preview blocked: %+v %v", preview, err)
	}
	if _, err := s.Apply(context.Background(), req, preview.Revision); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(resolved, "mods/demo/main.js")); err != nil || string(data) != "demo" {
		t.Fatalf("mod not installed under resolved home: %q %v", data, err)
	}
}

func TestHerdrDiscovery(t *testing.T) {
	root := t.TempDir()
	writePluginFile(t, root, "mods/alpha/main.js", "a")
	writePluginFile(t, root, "mods/beta/main.js", "b")
	d, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 2 || d.Candidates[0].pathFor("herdr-app") != "mods/alpha" || d.Candidates[1].pathFor("herdr-app") != "mods/beta" {
		t.Fatalf("unexpected candidates: %+v", d.Candidates)
	}
}

func TestHerdrLauncherDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launchers require a POSIX shell")
	}
	root := t.TempDir()
	writePluginFile(t, root, "bin/lantern-app", "#!/bin/sh\n")
	if _, err := Discover(context.Background(), root); err == nil {
		t.Fatal("launcher discovered without explicit entry")
	}
	if _, err := DiscoverOptions(context.Background(), root, "", "bin/lantern-app"); err == nil {
		t.Fatal("non-executable launcher accepted")
	}
	if err := os.Chmod(filepath.Join(root, "bin/lantern-app"), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := DiscoverOptions(context.Background(), root, "", "bin/lantern-app")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || d.Candidates[0].Name != "lantern-app" {
		t.Fatalf("unexpected launcher: %+v", d.Candidates)
	}
}

func TestHerdrMissingHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "missing"))
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	h := s.host(context.Background(), "herdr-app")
	if h.Status != HostMissing || h.Error != "" || h.NoteKey != "plugins.note.herdrHomeMissing" || h.NoteArgs["path"] != filepath.Join(home, "missing") {
		t.Fatalf("unexpected host: %+v", h)
	}
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "demo")
	p, err := s.Preview(context.Background(), Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked || len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.herdrAddHomeMissing" || p.Changes[0].MessageArgs["path"] != filepath.Join(home, "missing") {
		t.Fatalf("add should be blocked with a translated path: %+v", p.Changes)
	}
	if _, err := os.Stat(filepath.Join(home, "missing")); !os.IsNotExist(err) {
		t.Fatal("created absent home", err)
	}
}

func TestHerdrLauncherLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "herdr"))
	if err := os.Mkdir(filepath.Join(home, "herdr"), 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "bin/lantern-app", "#!/bin/sh\necho first\n")
	if err := os.Chmod(filepath.Join(source, "bin/lantern-app"), 0755); err != nil {
		t.Fatal(err)
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	req := Request{Action: "add", Source: source, Name: "lantern", Entry: "bin/lantern-app", Targets: []string{"herdr-app"}}
	applyPluginRequest(t, s, req)
	runtime := filepath.Join(s.managedRoot(), "herdr-app/lantern-app")
	writeHerdrRuntimeFiles(t, runtime)
	writePluginFile(t, source, "bin/lantern-app", "#!/bin/sh\necho second\n")
	if err := os.Chmod(filepath.Join(source, "bin/lantern-app"), 0755); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "update", Name: "lantern"})
	assertHerdrRuntimeGone(t, runtime)
	writeHerdrRuntimeFiles(t, runtime)
	applyPluginRequest(t, s, Request{Action: "remove", Name: "lantern"})
	if _, err := os.Stat(runtime); !os.IsNotExist(err) {
		t.Fatalf("runtime remains: %v", err)
	}
}

func TestHerdrStoreIdentityChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "first"))
	if err := os.Mkdir(filepath.Join(home, "first"), 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "demo")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "second"))
	if err := os.Mkdir(filepath.Join(home, "second"), 0755); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview(context.Background(), Request{Action: "remove", Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked {
		t.Fatal("changed store did not block removal")
	}
	if _, err := os.Stat(filepath.Join(home, "first/mods/demo/main.js")); err != nil {
		t.Fatal(err)
	}
}

func TestHerdrRejectsUnsafeSourceLink(t *testing.T) {
	root := t.TempDir()
	writePluginFile(t, root, "mods/demo/main.js", "mod")
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(root, "mods/demo/link")); err != nil {
		t.Skip(err)
	}
	if _, err := Discover(context.Background(), root); err != nil {
		t.Fatal(err)
	}
}

func TestHerdrPackageJSONLauncherDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launchers require a POSIX shell")
	}
	root := t.TempDir()
	writePluginFile(t, root, "package.json", `{"name":"app","version":"1.0.0"}`)
	writePluginFile(t, root, "bin/app", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(root, "bin/app"), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := DiscoverOptions(context.Background(), root, "", "bin/app")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Candidates) != 1 || !slices.Contains(d.Candidates[0].Targets, "herdr-app") || d.Candidates[0].TargetInfo["herdr-app"].Entry != "bin/app" {
		t.Fatalf("candidate: %+v", d.Candidates)
	}
}

func TestHerdrFolderEditsAndReadd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "herdr"))
	if err := os.Mkdir(filepath.Join(home, "herdr"), 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "first")
	writePluginFile(t, source, "mods/demo/.assets/settings.json", "original")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
	dest := filepath.Join(home, "herdr/mods/demo")
	if data, err := os.ReadFile(filepath.Join(dest, ".assets/settings.json")); err != nil || string(data) != "original" {
		t.Fatalf("hidden source file not copied: %q %v", data, err)
	}
	writePluginFile(t, dest, "stray", "edited")
	req := Request{Action: "remove", Name: "demo"}
	p, err := s.Preview(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Apply(context.Background(), req, p.Revision); err == nil {
		t.Fatal("removed edited folder")
	}
	if err := os.Remove(filepath.Join(dest, "stray")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, dest, "gpui-kit.d.ts", "generated")
	applyPluginRequest(t, s, req)
	if _, err = os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("folder remains: %v", err)
	}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
}

func TestHerdrModReplacementChangesFileToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override the Windows user profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("HERDR_APP_HOME", filepath.Join(home, "herdr"))
	if err := os.Mkdir(filepath.Join(home, "herdr"), 0755); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writePluginFile(t, root, "mods/demo/main.js", "main")
	writePluginFile(t, root, "mods/demo/change", "file")
	if err := os.Symlink("main.js", filepath.Join(root, "mods/demo/link")); err != nil {
		t.Fatal(err)
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: root, Targets: []string{"herdr-app"}})
	dest := filepath.Join(home, "herdr/mods/demo")
	if target, err := os.Readlink(filepath.Join(dest, "link")); err != nil || target != "main.js" {
		t.Fatalf("link: %q %v", target, err)
	}
	if err := os.Remove(filepath.Join(root, "mods/demo/change")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, root, "mods/demo/change/child", "child")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	if data, err := os.ReadFile(filepath.Join(dest, "change/child")); err != nil || string(data) != "child" {
		t.Fatalf("replacement: %q %v", data, err)
	}
	if err := os.RemoveAll(filepath.Join(root, "mods/demo/change")); err != nil {
		t.Fatal(err)
	}
	writePluginFile(t, root, "mods/demo/change", "again")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	if data, err := os.ReadFile(filepath.Join(dest, "change")); err != nil || string(data) != "again" {
		t.Fatalf("replacement: %q %v", data, err)
	}
}

func writeHerdrRuntimeFiles(t *testing.T, dir string) {
	t.Helper()
	writePluginFile(t, dir, ".finder.lock", "runtime")
	writePluginFile(t, dir, ".venv/data", "runtime")
	writePluginFile(t, dir, "node_modules/data", "runtime")
	writePluginFile(t, dir, "__pycache__/data", "runtime")
	if err := os.Symlink("missing", filepath.Join(dir, "node_modules", "broken")); err != nil {
		t.Fatal(err)
	}
}

func assertHerdrRuntimeGone(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{".finder.lock", ".venv", "node_modules", "__pycache__"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s retained: %v", name, err)
		}
	}
}

func TestHerdrInventoryIgnoresUnrelatedFolders(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX FIFO")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.MkdirAll(filepath.Join(herdr, "mods/large"), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(herdr, "mods/large/data"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(101 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	neighbor := filepath.Join(s.managedRoot(), "herdr-app/neighbor")
	if err := os.MkdirAll(neighbor, 0755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("mkfifo", filepath.Join(neighbor, "pipe")).Run(); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "first")
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
	if p, err := s.Preview(context.Background(), Request{Action: "check", Name: "demo"}); err != nil || p.Blocked {
		t.Fatalf("check blocked: %+v %v", p, err)
	}
	writePluginFile(t, source, "mods/demo/main.js", "second")
	applyPluginRequest(t, s, Request{Action: "update", Name: "demo"})
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
}

func TestHerdrMissingHomeDoesNotBlockOtherTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.Mkdir(herdr, 0755); err != nil {
		t.Fatal(err)
	}
	mod := t.TempDir()
	writePluginFile(t, mod, "mods/demo/main.js", "first")
	cursor := t.TempDir()
	writePluginFile(t, cursor, ".cursor-plugin/plugin.json", `{"name":"other"}`)
	writePluginFile(t, cursor, "file", "first")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: mod, Targets: []string{"herdr-app"}})
	applyPluginRequest(t, s, Request{Action: "add", Source: cursor, Targets: []string{"cursor"}})
	if err := os.RemoveAll(herdr); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"sync", "check", "update"} {
		p, err := s.Preview(context.Background(), Request{Action: action, All: action == "update"})
		if err != nil || p.Blocked {
			t.Fatalf("%s: %+v %v", action, p, err)
		}
		found := false
		for _, c := range p.Changes {
			if c.Target == "herdr-app" && c.Action == "skip" {
				if c.MessageKey != "plugins.note.herdrHomeSkipped" {
					t.Fatalf("%s skip lacks translation key: %+v", action, c)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("%s did not skip Herdr: %+v", action, p.Changes)
		}
	}
	writePluginFile(t, cursor, "file", "second")
	applyPluginRequest(t, s, Request{Action: "update", All: true})
	root, _, err := s.localRoot("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(root, "other/file")); err != nil || string(data) != "second" {
		t.Fatalf("other target did not update: %s %v", data, err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "demo"})
	if _, err := os.Stat(herdr); !os.IsNotExist(err) {
		t.Fatalf("created Herdr home: %v", err)
	}
}

func TestHerdrLauncherRemoveWithMissingHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launchers require POSIX")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.Mkdir(herdr, 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "bin/app", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(source, "bin/app"), 0755); err != nil {
		t.Fatal(err)
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Entry: "bin/app", Targets: []string{"herdr-app"}})
	root := filepath.Join(s.managedRoot(), "herdr-app")
	if err := os.Remove(herdr); err != nil {
		t.Fatal(err)
	}
	applyPluginRequest(t, s, Request{Action: "remove", Name: "app"})
	for _, p := range []string{filepath.Join(root, "app"), filepath.Join(root, ".skillshare-app.json")} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("launcher artifact remains: %s: %v", p, err)
		}
	}
}

func TestHerdrLauncherGeneratedNamesAreEdits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launchers require POSIX")
	}
	for _, file := range []string{"jsconfig.json", "types.d.ts"} {
		t.Run(file, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			herdr := filepath.Join(home, "herdr")
			t.Setenv("HERDR_APP_HOME", herdr)
			if err := os.Mkdir(herdr, 0755); err != nil {
				t.Fatal(err)
			}
			source := t.TempDir()
			writePluginFile(t, source, "bin/app", "#!/bin/sh\n")
			if err := os.Chmod(filepath.Join(source, "bin/app"), 0755); err != nil {
				t.Fatal(err)
			}
			s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
			applyPluginRequest(t, s, Request{Action: "add", Source: source, Entry: "bin/app", Targets: []string{"herdr-app"}})
			dest := filepath.Join(s.managedRoot(), "herdr-app/app")
			writePluginFile(t, dest, file, "local edit")
			r := Request{Action: "remove", Name: "app"}
			p, err := s.Preview(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Apply(context.Background(), r, p.Revision); err == nil {
				t.Fatal("removed launcher with local edit")
			}
			if data, err := os.ReadFile(filepath.Join(dest, file)); err != nil || string(data) != "local edit" {
				t.Fatalf("lost edit: %q %v", data, err)
			}
		})
	}
}

func TestHerdrModDisableEnableSync(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("isolated HOME does not override Windows profile")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.Mkdir(herdr, 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "mods/demo/main.js", "demo")
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	applyPluginRequest(t, s, Request{Action: "add", Source: source, Targets: []string{"herdr-app"}})
	installed := filepath.Join(herdr, "mods/demo/main.js")
	applyPluginRequest(t, s, Request{Action: "disable", Name: "demo"})
	applyPluginRequest(t, s, Request{Action: "sync"})
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatalf("disabled mod remains: %v", err)
	}
	applyPluginRequest(t, s, Request{Action: "enable", Name: "demo"})
	applyPluginRequest(t, s, Request{Action: "sync"})
	if data, err := os.ReadFile(installed); err != nil || string(data) != "demo" {
		t.Fatalf("mod not restored: %q %v", data, err)
	}
}

func TestHerdrLauncherOutcomeMessages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("launchers require POSIX")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	herdr := filepath.Join(home, "herdr")
	t.Setenv("HERDR_APP_HOME", herdr)
	if err := os.Mkdir(herdr, 0755); err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	writePluginFile(t, source, "bin/app", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(source, "bin/app"), 0755); err != nil {
		t.Fatal(err)
	}
	s := &Service{ConfigPath: filepath.Join(home, "config.yaml"), StateDir: filepath.Join(home, "state")}
	apply := func(r Request, want, key, arg, value string) {
		t.Helper()
		p, err := s.Preview(context.Background(), r)
		if err != nil || p.Blocked {
			t.Fatalf("preview: %+v %v", p, err)
		}
		if r.Action == "update" && (len(p.Changes) != 1 || p.Changes[0].MessageKey != "plugins.note.herdrLauncherUpdate" || p.Changes[0].MessageArgs["id"] != "app") {
			t.Fatalf("update preview: %+v", p.Changes)
		}
		result, err := s.Apply(context.Background(), r, p.Revision)
		if err != nil || len(result.Results) != 1 || !strings.Contains(result.Results[0].Message, want) || result.Results[0].MessageKey != key || (arg != "" && result.Results[0].MessageArgs[arg] != value) {
			t.Fatalf("outcome: %+v %v", result, err)
		}
	}
	apply(Request{Action: "add", Source: source, Entry: "bin/app", Targets: []string{"herdr-app"}}, "Installed at ", "plugins.outcome.herdrLauncherInstalled", "path", filepath.Join(s.managedRoot(), "herdr-app/app/bin/app"))
	writePluginFile(t, source, "bin/app", "#!/bin/sh\necho updated\n")
	apply(Request{Action: "update", Name: "app"}, "Updated app; close and reopen", "plugins.outcome.herdrLauncherUpdated", "id", "app")
	apply(Request{Action: "remove", Name: "app"}, "Removed.", "plugins.outcome.herdrRemoved", "", "")
}
