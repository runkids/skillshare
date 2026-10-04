package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	gitops "skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	ssync "skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

// initResult is what applyInitPlan did, for the done screen and the oplog.
type initResult struct {
	cfg            *config.Config
	gitRoot        string
	gitInit        bool
	identitySet    bool // git had no identity, a local default was set
	linked         bool
	pulled         int
	pullErr        error
	imported       int
	skillInstalled bool
	skillFallback  bool // download failed, the minimal skill was written
	warnings       []string
}

// buildInitConfig is the config a plan writes. New installs use the
// `sources:` map (v0.19.16+).
func buildInitConfig(p *initPlan) *config.Config {
	defaults := config.DefaultTargets()
	targets := make(map[string]config.TargetConfig, len(p.targets))
	for _, name := range p.targets {
		targets[name] = defaults[name]
	}
	cfg := &config.Config{
		Sources: config.GlobalSources{
			Skills: p.source(),
			Agents: filepath.Join(filepath.Dir(p.base), "agents"),
		},
		Mode:    p.mode,
		Targets: targets,
		Ignore:  []string{".DS_Store", ".git/", "__pycache__/"},
		Audit:   config.AuditConfig{BlockThreshold: "CRITICAL"},
	}
	if p.git && p.gitScope != "" && p.gitScope != "skills" {
		cfg.GitRoot = p.gitScope
	}
	return cfg
}

// gitRootFor is the directory git versions. For the skills scope it is the
// source folder before --subdir, so a repo with a skills/ folder stays whole.
func gitRootFor(p *initPlan, cfg *config.Config) string {
	if p.gitScope == "" || p.gitScope == "skills" {
		return p.base
	}
	return config.ScopeDir(cfg, p.gitScope)
}

// applyInitPlan writes the plan in an order that lets the repo win: the
// remote is pulled before imports are copied, and copying skips folders
// that already exist.
func applyInitPlan(p *initPlan) (*initResult, error) {
	cfg := buildInitConfig(p)
	res := &initResult{cfg: cfg}

	for _, dir := range []string{p.base, p.source(), cfg.Sources.Agents} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return res, fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	if p.git {
		res.gitRoot = gitRootFor(p, cfg)
		hadRepo := hasGitDir(res.gitRoot)
		if hadRepo {
			if err := gitops.WriteScopeGitignore(res.gitRoot, p.gitScope); err != nil {
				res.warnings = append(res.warnings, fmt.Sprintf("Failed to update .gitignore: %v", err))
			}
		} else {
			identitySet, err := gitops.InitScopeRepo(res.gitRoot, p.gitScope)
			if err != nil {
				return res, fmt.Errorf("failed to initialize git: %w", err)
			}
			res.gitInit, res.identitySet = true, identitySet
		}
		if p.remoteURL != "" {
			applyRemote(p, res, hadRepo)
		}
	}

	for _, s := range p.imports {
		dst := filepath.Join(p.source(), s.name)
		if _, err := os.Lstat(dst); err == nil {
			continue // already there, e.g. pulled from the repo
		}
		src, err := filepath.EvalSymlinks(s.from)
		if err != nil {
			res.warnings = append(res.warnings, fmt.Sprintf("Skipped %s: %v", s.name, err))
			continue
		}
		if err := copyDir(src, dst); err != nil {
			res.warnings = append(res.warnings, fmt.Sprintf("Failed to copy %s: %v", s.name, err))
			continue
		}
		res.imported++
	}

	if err := cfg.Save(); err != nil {
		return res, err
	}

	if p.skill && !hasBuiltinSkill(p.source()) {
		fallback, err := installBuiltinSkill(p.source())
		if err != nil {
			res.warnings = append(res.warnings, fmt.Sprintf("Failed to install the skillshare skill: %v", err))
		} else {
			res.skillInstalled, res.skillFallback = true, fallback
		}
	}

	if p.git {
		if err := commitSourceFiles(res.gitRoot); err != nil {
			res.warnings = append(res.warnings, fmt.Sprintf("Failed to create initial commit: %v", err))
		}
	}
	return res, nil
}

// applyRemote links origin and, when chosen, moves the new repo onto the
// remote's history. A repo that existed before init keeps its history; its
// owner pulls by hand.
func applyRemote(p *initPlan, res *initResult, hadRepo bool) {
	if out, err := gitOutput(res.gitRoot, "remote", "get-url", "origin"); err == nil && out != "" {
		if out != p.remoteURL {
			res.warnings = append(res.warnings, fmt.Sprintf("Git remote already exists: %s (to change: git remote set-url origin %s)", out, p.remoteURL))
		}
		return
	}
	if err := gitops.SetOrAddRemote(res.gitRoot, p.remoteURL); err != nil {
		res.warnings = append(res.warnings, fmt.Sprintf("Failed to add remote: %v", err))
		return
	}
	res.linked = true
	if !p.pull {
		return
	}
	if hadRepo {
		res.warnings = append(res.warnings, fmt.Sprintf("%s already had a git repo, so the remote's skills were not pulled. Run: skillshare pull", utils.FoldHomePath(res.gitRoot)))
		return
	}
	if err := pullRemote(res.gitRoot, p.remoteURL); err != nil {
		res.pullErr = err
		return
	}
	if discovered, err := ssync.DiscoverSourceSkills(p.source()); err == nil {
		res.pulled = len(discovered)
	}
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func hasBuiltinSkill(sourcePath string) bool {
	_, err := os.Stat(filepath.Join(sourcePath, "skillshare", "SKILL.md"))
	return err == nil
}

// installBuiltinSkill downloads the skillshare skill, falling back to a
// minimal copy when GitHub can't be reached. Both paths write through the
// source handle, so a skillshare folder that is a link is refused instead of
// replaced, and the fallback cannot write through it either.
func installBuiltinSkill(sourcePath string) (fallback bool, err error) {
	src, err := sourcefs.Create(sourcePath)
	if err != nil {
		return false, err
	}
	defer src.Close()
	if err := src.CheckNoLink("skillshare"); err != nil {
		return false, err
	}
	dir := filepath.Join(sourcePath, "skillshare")
	source, err := install.ParseSource(skillshareSkillSource)
	if err == nil {
		source.Name = "skillshare"
		_, err = install.Install(source, dir, install.InstallOptions{Force: true})
	}
	if err == nil {
		return false, nil
	}
	if err := src.MkdirAll("skillshare", 0o755); err != nil {
		return true, err
	}
	return true, src.WriteFile(filepath.Join("skillshare", "SKILL.md"), []byte(fallbackSkillContent), 0o644)
}

// printInitDone prints what was set up.
func printInitDone(p *initPlan, res *initResult) {
	dim := theme.Dim()
	ui.Answered("Config", utils.FoldHomePath(config.ConfigPath()))

	var skills []string
	if p.connect {
		skills = append(skills, fmt.Sprintf("%d from the repo", res.pulled))
		skills = append(skills, fmt.Sprintf("%d kept from this machine", res.imported))
	} else if res.imported > 0 {
		skills = append(skills, fmt.Sprintf("%d imported", res.imported))
	}
	if res.skillInstalled {
		note := "skillshare skill installed"
		if res.skillFallback {
			note += " " + dim.Render("(minimal; run skillshare upgrade --skill)")
		}
		skills = append(skills, note)
	}
	if len(skills) == 0 {
		skills = append(skills, "empty "+dim.Render("(add some: skillshare install <repo>)"))
	}
	ui.Answered("Skills", strings.Join(skills, " · "))

	if p.git {
		ui.Answered("Git", "history on: "+describeGit(p))
	}
	if res.linked {
		ui.Answered("Remote", "linked "+p.remoteURL)
	}
	if p.pull && !p.connect && res.pulled > 0 {
		pulled := plural(res.pulled, "skill") + " from the repo"
		if same := p.sameNameAsRemote(); len(same) > 0 {
			pulled += " " + dim.Render(fmt.Sprintf("(repo version used for %s)", strings.Join(same, ", ")))
		}
		ui.Answered("Pulled", pulled)
	}
	if res.pullErr != nil {
		ui.Warning("Could not pull the repo: %v", res.pullErr)
		ui.Note("Try again: skillshare pull")
	}
	for _, w := range res.warnings {
		ui.Warning("%s", w)
	}
	if res.identitySet {
		printGitIdentityNote(res.gitRoot)
	}
	fmt.Println()
}

// firstSync replaces target copies identical to the source with links, then
// syncs skills (and agents, if any) quietly. It returns the folders left as
// they were because they differ from the source.
func firstSync(cfg *config.Config) (skills int, kept []string, err error) {
	start := time.Now()
	spinner := ui.StartSpinner("Syncing…")
	discovered, err := ssync.DiscoverSourceSkills(cfg.EffectiveSkillsSource())
	if err != nil {
		spinner.Fail("Sync failed")
		return 0, nil, err
	}
	kept = replaceIdenticalCopies(cfg, discovered)

	var entries []syncTargetEntry
	for name, target := range cfg.Targets {
		entries = append(entries, syncTargetEntry{name: name, target: target, mode: getTargetMode(target.SkillsConfig().Mode, cfg.Mode)})
	}
	_, failed := runParallelSyncQuiet(entries, cfg.EffectiveSkillsSource(), discovered, ssync.EffectiveFileIgnorePatterns(cfg.Ignore), false, false, "")
	if _, agentErr := syncAgentsGlobal(cfg, false, false, true, start); agentErr != nil && err == nil {
		err = agentErr
	}
	spinner.Stop()
	if failed > 0 {
		err = fmt.Errorf("%s failed to sync; run skillshare sync for details", plural(failed, "tool"))
	}
	ui.Answered("Synced", fmt.Sprintf("%s to %s", plural(len(discovered), "skill"), plural(len(cfg.Targets), "tool"))+ui.Took(time.Since(start)))
	return len(discovered), kept, err
}

// replaceIdenticalCopies removes real skill folders in merge-mode targets
// whose content matches the source byte for byte, so sync can link them.
// Folders that differ are kept and returned as "tool/name".
func replaceIdenticalCopies(cfg *config.Config, skills []ssync.DiscoveredSkill) []string {
	var kept []string
	for name, target := range cfg.Targets {
		sc := target.SkillsConfig()
		if getTargetMode(sc.Mode, cfg.Mode) != "merge" {
			continue
		}
		for _, s := range skills {
			dst := filepath.Join(sc.Path, s.FlatName)
			info, err := os.Lstat(dst)
			if err != nil || !info.IsDir() {
				continue // missing, or already a link
			}
			if sameTree(s.SourcePath, dst) {
				os.RemoveAll(dst) //nolint:errcheck // sync reports a folder it can't link
			} else {
				kept = append(kept, name+"/"+s.FlatName)
			}
		}
	}
	return kept
}

// sameTree reports whether two folders hold the same files with the same
// bytes.
func sameTree(a, b string) bool {
	files := func(root string) (map[string]string, error) {
		out := map[string]string{}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			out[rel] = path
			return nil
		})
		return out, err
	}
	fa, errA := files(a)
	fb, errB := files(b)
	if errA != nil || errB != nil || len(fa) != len(fb) {
		return false
	}
	for rel, pa := range fa {
		pb, ok := fb[rel]
		if !ok {
			return false
		}
		da, errA := os.ReadFile(pa)
		db, errB := os.ReadFile(pb)
		if errA != nil || errB != nil || !bytes.Equal(da, db) {
			return false
		}
	}
	return true
}

// printInitNext prints the commands to try next and, on platforms with a
// build, the desktop app.
func printInitNext(p *initPlan, synced bool, interactive bool) {
	cmd := func(c, note string) {
		fmt.Printf("  %s %s\n", theme.Accent().Render(fmt.Sprintf("%-27s", c)), theme.Dim().Render(note))
	}
	fmt.Println()
	fmt.Println(theme.Primary().Bold(true).Render("Next"))
	if !synced && len(p.targets) > 0 {
		cmd("skillshare sync", "sync skills to your tools")
	}
	switch {
	case p.connect && len(p.imports) > 0:
		cmd("skillshare push", fmt.Sprintf("upload the %s you kept to the repo", plural(len(p.imports), "skill")))
	case p.remoteURL != "":
		cmd("skillshare push", "upload this machine's skills to the repo")
	}
	cmd("skillshare install <repo>", "add skills from GitHub")
	cmd("skillshare ui", "open the dashboard")
	if !interactive {
		return
	}
	if hint := desktopAppHint(); hint != "" {
		fmt.Println()
		fmt.Println(theme.Primary().Bold(true).Render("Desktop app") + " " + theme.Dim().Render("— manage skills without the terminal"))
		fmt.Println("  " + theme.Accent().Render(hint))
	}
}
