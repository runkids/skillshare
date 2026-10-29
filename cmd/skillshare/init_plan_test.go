package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"skillshare/internal/config"
)

func TestDetectCLIDirectories_SkillsFirstThenByName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, dir := range []string{".cursor", ".claude/skills/a", ".claude/skills/b", ".gemini/skills/c"} {
		os.MkdirAll(filepath.Join(home, dir), 0o755)
	}

	var names []string
	for _, d := range detectCLIDirectories() {
		names = append(names, d.name)
	}

	want := []string{"claude", "gemini", "cursor", "universal"}
	if !slices.Equal(names, want) {
		t.Errorf("order = %v, want %v", names, want)
	}
}

func TestDetectCLIDirectories_DoesNotCreateFolders(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".cursor"), 0o755)

	detectCLIDirectories()

	if _, err := os.Stat(filepath.Join(home, ".cursor", "skills")); err == nil {
		t.Error("detection created ~/.cursor/skills")
	}
}

func TestNewInitPlan_DefaultsMatchPressingEnter(t *testing.T) {
	detected := []detectedDir{{name: "claude", path: "/x", skillCount: 1, hasSkills: true}, {name: "universal"}}

	p := newInitPlan(&initOptions{sourcePath: "/src"}, detected, "/home")

	if !slices.Equal(p.targets, []string{"claude", "universal"}) || !p.git || !p.skill || p.mode != "merge" || p.gitScope != "skills" {
		t.Errorf("plan = %+v, want every tool, git on (skills), skill on, merge", p)
	}
}

func TestNewInitPlan_FlagsOverrideDefaults(t *testing.T) {
	detected := []detectedDir{{name: "claude"}, {name: "universal"}}

	p := newInitPlan(&initOptions{sourcePath: "/src", noTargets: true, noGit: true, noSkill: true, mode: "copy"}, detected, "/home")

	if len(p.targets) != 0 || p.git || p.skill || p.mode != "copy" {
		t.Errorf("plan = %+v, want flags to win", p)
	}
}

func TestDefaultGitScope_RemoteVersionsEverything(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := &initPlan{base: filepath.Join(config.BaseDir(), "skills"), remoteURL: "git@example.com:me/skills.git"}

	if got := p.defaultGitScope(); got != "root" {
		t.Errorf("scope = %q, want root", got)
	}
}

func TestDefaultGitScope_CustomSourceKeepsSkillsScope(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := &initPlan{base: filepath.Join(t.TempDir(), "my-skills"), remoteURL: "git@example.com:me/skills.git"}

	if got := p.defaultGitScope(); got != "skills" {
		t.Errorf("scope = %q, want skills: the skillshare folder holds none of a custom source", got)
	}
}

func TestInspectRemote_FindsSkillsFolder(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "skills", "pdf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "skills", "pdf", "SKILL.md"), []byte("# pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "skills")

	r := inspectRemote(repo)

	if r.subdir != "skills" || !slices.Equal(r.names, []string{"pdf"}) {
		t.Errorf("subdir = %q, names = %v; want the skills/ folder and pdf", r.subdir, r.names)
	}
}

func TestSameNameAsRemote_ListsImportsTheRepoAlsoHas(t *testing.T) {
	p := &initPlan{
		pull:    true,
		remote:  &remoteRepo{names: []string{"pdf", "tdd"}},
		imports: []importedSkill{{name: "pdf"}, {name: "mine"}},
	}

	if got := p.sameNameAsRemote(); !slices.Equal(got, []string{"pdf"}) {
		t.Errorf("same = %v, want [pdf]", got)
	}
}

func TestSameTree_ComparesContent(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	write := func(dir, name, body string) {
		os.MkdirAll(filepath.Join(dir, name), 0o755)
		os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(body), 0o644)
	}
	write(src, "same", "x")
	write(dst, "same", "x")
	write(src, "edited", "x")
	write(dst, "edited", "y")

	if !sameTree(filepath.Join(src, "same"), filepath.Join(dst, "same")) {
		t.Error("identical folders should match")
	}
	if sameTree(filepath.Join(src, "edited"), filepath.Join(dst, "edited")) {
		t.Error("edited folders should not match")
	}
}

func TestDetectCLIDirectories_OMPRequiresOwnInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	piHome := filepath.Join(home, "pi-home")
	if err := os.MkdirAll(filepath.Join(piHome, "skills", "pi-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", piHome)
	for _, d := range detectCLIDirectories() {
		if d.name == "omp" {
			t.Fatalf("Pi-only home detected as OMP: %+v", d)
		}
	}
	if err := os.MkdirAll(filepath.Join(home, ".omp"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, d := range detectCLIDirectories() {
		if d.name == "omp" && d.path == filepath.Join(piHome, "skills") {
			return
		}
	}
	t.Fatal("installed OMP did not retain its environment-selected skills path")
}

func TestDiscover_OMPAccountAndPiOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	account := filepath.Join(home, ".omp-work", "agent")
	if err := os.MkdirAll(account, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PI_CODING_AGENT_DIR", account)
	cfg := &config.Config{Targets: map[string]config.TargetConfig{"omp-work": {Agent: "omp", ConfigDir: account}}}
	for _, agent := range detectNewAgents(cfg) {
		if agent.name == "omp" {
			t.Fatal("Pi-only home offered as OMP by discover")
		}
	}
	if err := os.MkdirAll(filepath.Join(home, ".omp", "agent"), 0755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".omp", "agent", "skills")
	found := false
	for _, agent := range detectNewAgents(cfg) {
		if agent.name == "omp" {
			found = true
			if agent.path != want {
				t.Fatalf("discover path = %q; want %q", agent.path, want)
			}
		}
	}
	if !found {
		t.Fatal("discover did not find installed OMP")
	}
	t.Setenv("SKILLSHARE_CONFIG", filepath.Join(home, "config.yaml"))
	if err := saveAddedAgents(cfg, []string{"omp"}, false, false, ""); err != nil {
		t.Fatal(err)
	}
	target := cfg.Targets["omp"]
	if target.SkillsConfig().Path != want {
		t.Fatalf("saved default = %q; want %q", target.SkillsConfig().Path, want)
	}
}
