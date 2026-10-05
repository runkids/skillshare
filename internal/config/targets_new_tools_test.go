package config

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestTargets_DeepSeekHarnessAndGitLabDuo(t *testing.T) {
	for _, key := range []string{"DSH_HOME", "GLAB_CONFIG_DIR", "XDG_CONFIG_HOME", "APPDATA"} {
		t.Setenv(key, "")
	}
	for _, tt := range []struct {
		name, global, project     string
		globalScans, projectScans []string
	}{
		{"deepseek-harness", "~/.dsh/skills", ".dsh/skills", []string{"~/.agents/skills"}, []string{".agents/skills"}},
		{"gitlab-duo", "~/.gitlab/duo/skills", "skills", []string{"~/.agents/skills"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			global, ok := LookupGlobalTarget(tt.name)
			if !ok || global.Path != normalizeTargetPath(tt.global) {
				t.Errorf("global target = %+v, %v; want %s", global, ok, normalizeTargetPath(tt.global))
			}
			project, ok := LookupProjectTarget(tt.name)
			if !ok || project.Path != filepath.FromSlash(tt.project) {
				t.Errorf("project target = %+v, %v; want %s", project, ok, tt.project)
			}
			if !slices.Contains(KnownTargetNames(), tt.name) {
				t.Errorf("missing known target %s", tt.name)
			}
			spec := alsoScansSpec(t, tt.name)
			if !slices.Equal(spec.Global, tt.globalScans) || !slices.Equal(spec.Project, tt.projectScans) {
				t.Errorf("also_scans = %+v, want %v / %v", spec, tt.globalScans, tt.projectScans)
			}
		})
	}
	if !ProjectTargetDotDirs()[".dsh"] {
		t.Error("missing .dsh discovery exclusion")
	}
}

func TestTargetConfigHome(t *testing.T) {
	base := t.TempDir()
	for _, tt := range []struct {
		name, goos, dsh, glab, xdg, appdata, want string
	}{
		{"gitlab-duo", "linux", "", "", "", "", ""},
		{"gitlab-duo", "linux", "", base + "/glab", base + "/xdg", base + "/appdata", base + "/glab"},
		{"gitlab-duo", "darwin", "", "", base + "/xdg", "", filepath.Join(base, "xdg", "gitlab", "duo")},
		{"gitlab-duo", "windows", "", "", "", base + "/appdata", filepath.Join(base, "appdata", "GitLab", "duo")},
		{"gitlab-duo", "windows", "", base + "/glab", base + "/xdg", base + "/appdata", base + "/glab"},
		{"gitlab-duo", "windows", "", "", base + "/xdg", base + "/appdata", filepath.Join(base, "xdg", "gitlab", "duo")},
		{"deepseek-harness", "linux", " \t ", "", "", "", ""},
		{"deepseek-harness", "windows", base + "/dsh", "", "", "", filepath.Join(base, "dsh")},
		{"claude", "linux", base, base, base, base, ""},
	} {
		t.Run(tt.name+"/"+tt.goos+"/"+tt.want, func(t *testing.T) {
			t.Setenv("DSH_HOME", tt.dsh)
			t.Setenv("GLAB_CONFIG_DIR", tt.glab)
			t.Setenv("XDG_CONFIG_HOME", tt.xdg)
			t.Setenv("APPDATA", tt.appdata)
			if got := targetConfigHome(tt.name, tt.goos); got != tt.want {
				t.Errorf("config home = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTargets_ConfigHomeOverrides(t *testing.T) {
	for _, tt := range []struct {
		name, env, root string
	}{
		{"deepseek-harness", "DSH_HOME", "dsh"},
		{"gitlab-duo", "GLAB_CONFIG_DIR", "duo"},
		{"gitlab-duo", "XDG_CONFIG_HOME", "xdg/gitlab/duo"},
	} {
		t.Run(tt.env, func(t *testing.T) {
			for _, key := range []string{"DSH_HOME", "GLAB_CONFIG_DIR", "XDG_CONFIG_HOME"} {
				t.Setenv(key, "")
			}
			base := t.TempDir()
			value := filepath.Join(base, tt.root)
			if tt.env == "XDG_CONFIG_HOME" {
				value = filepath.Join(base, "xdg")
			}
			t.Setenv(tt.env, value)
			want := filepath.Join(base, tt.root, "skills")
			if got := DefaultTargets()[tt.name].Path; got != want {
				t.Errorf("global path = %q, want %q", got, want)
			}
			if got := RuntimeScanPaths(tt.name, false); !slices.Contains(got, want) {
				t.Errorf("runtime scans = %v, missing %s", got, want)
			}
			cfg, root, err := loadWithTargets(t, "  "+tt.name+":\n    skills:\n      path: $ROOT/pinned/skills\n")
			if err != nil {
				t.Fatal(err)
			}
			target := cfg.Targets[tt.name]
			if got := target.SkillsConfig().Path; got != filepath.Join(root, "pinned", "skills") {
				t.Errorf("explicit path = %q, want pinned path", got)
			}
		})
	}
}
