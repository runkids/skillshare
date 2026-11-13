package main

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

func TestArchiveCLISelectionAndNoop(t *testing.T) {
	var payload bytes.Buffer
	z := zip.NewWriter(&payload)
	f, _ := z.Create("SKILL.md")
	_, _ = f.Write([]byte("# Root"))
	for _, name := range []string{"one", "two"} {
		f, _ := z.Create("pack/" + name + "/SKILL.md")
		_, _ = f.Write([]byte("# " + name))
	}
	_ = z.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload.Bytes()) }))
	defer srv.Close()
	old := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	defer func() { http.DefaultTransport = old }()
	root := t.TempDir()
	cfg := &config.Config{Source: root}
	source, _ := install.ParseSource(srv.URL + "/pack.zip")
	opts := install.InstallOptions{Skills: []string{"one", "two"}, Into: "legal", SourceDir: root}
	if _, err := dispatchInstall(source, cfg, opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "legal", "two", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	source, _ = install.ParseSource(srv.URL + "/pack.zip#skill=pack%2Ftwo")
	opts.Skills, opts.Name = nil, "custom"
	if _, err := dispatchInstall(source, cfg, opts); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "legal", "custom")
	content, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil || string(content) != "# two" {
		t.Fatalf("selection failed: %s %v", content, err)
	}
	store := install.LoadMetadataOrNew(root)
	entry := store.GetByPath("legal/custom")
	if entry == nil || entry.Subdir != "pack/two" {
		t.Fatalf("metadata = %+v", entry)
	}
	uc := &updateContext{sourcePath: root, opts: &updateOptions{}}
	updated, result, err := updateSkillFromMeta(uc, dest, entry)
	if err != nil || result == nil || updated || result.Action != "skipped" {
		t.Fatalf("no-op counted as update: %v %+v %v", updated, result, err)
	}
	rootSource, _ := install.ParseSource(srv.URL + "/pack.zip")
	if _, err := dispatchInstall(rootSource, cfg, install.InstallOptions{Skills: []string{"pack"}, Into: "legal", SourceDir: root}); err != nil {
		t.Fatal(err)
	}
	rootEntry := install.LoadMetadataOrNew(root).GetByPath("legal/pack")
	if rootEntry == nil || rootEntry.Source != srv.URL+"/pack.zip#skill=." {
		t.Fatalf("root selection = %+v", rootEntry)
	}
	for _, tc := range []struct {
		name   string
		source *install.Source
		opts   install.InstallOptions
		direct bool
	}{
		{"single-subdir", source, install.InstallOptions{Name: "custom", Into: "legal", SourceDir: root, Update: true}, false},
		{"multi-selection", rootSource, install.InstallOptions{Skills: []string{"one", "pack"}, Into: "legal", SourceDir: root, Update: true}, false},
		{"installed-name", source, install.InstallOptions{Name: "custom", Into: "legal", SourceDir: root, Update: true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := dispatchInstall
			if tc.direct {
				run = handleDirectInstall
			}
			summary, err := run(tc.source, cfg, tc.opts)
			if err != nil || summary.SkillCount != 0 || len(summary.InstalledSkills) != 0 {
				t.Fatalf("unchanged install counted as installed: %+v %v", summary, err)
			}
		})
	}
	// Reinstall from portable declarations into fresh global and project roots.
	for _, project := range []bool{false, true} {
		fresh := t.TempDir()
		freshSource := filepath.Join(fresh, "skills")
		var ctx install.InstallContext
		if project {
			ctx = &projectInstallContext{runtime: &projectRuntime{root: fresh, sourcePath: freshSource,
				config: &config.ProjectConfig{Skills: []config.SkillEntry{{Name: "custom", Group: "legal", Source: entry.Source}, {Name: "pack", Group: "legal", Source: rootEntry.Source}}}}}
		} else {
			declarations := install.NewMetadataStore()
			declarations.Set("legal/custom", entry)
			declarations.Set("legal/pack", rootEntry)
			ctx = &globalInstallContext{cfg: &config.Config{Source: freshSource}, store: declarations}
		}
		res, err := install.InstallFromConfig(ctx, install.InstallOptions{Quiet: true})
		if err != nil || res.Installed != 2 || len(res.FailedSkills) != 0 {
			t.Fatalf("config reinstall project=%v: %+v %v", project, res, err)
		}
		content, err := os.ReadFile(filepath.Join(freshSource, "legal", "custom", "SKILL.md"))
		if err != nil || string(content) != "# two" {
			t.Fatalf("config lost member: %s %v", content, err)
		}
		content, err = os.ReadFile(filepath.Join(freshSource, "legal", "pack", "SKILL.md"))
		if err != nil || string(content) != "# Root" {
			t.Fatalf("config lost root: %s %v", content, err)
		}
		if _, err := os.Stat(filepath.Join(freshSource, "legal", "pack", "pack")); !os.IsNotExist(err) {
			t.Fatal("root config reinstall absorbed descendants")
		}
	}
	// An archive with one root skill takes the non-subdir discovery branch.
	payload.Reset()
	z = zip.NewWriter(&payload)
	f, _ = z.Create("SKILL.md")
	_, _ = f.Write([]byte("# Solo"))
	_ = z.Close()
	solo, _ := install.ParseSource(srv.URL + "/solo.zip")
	soloOpts := install.InstallOptions{SourceDir: root}
	if _, err := dispatchInstall(solo, cfg, soloOpts); err != nil {
		t.Fatal(err)
	}
	soloOpts.Update = true
	summary, err := dispatchInstall(solo, cfg, soloOpts)
	if err != nil || summary.SkillCount != 0 || len(summary.InstalledSkills) != 0 {
		t.Fatalf("unchanged single-root install counted as installed: %+v %v", summary, err)
	}
}
