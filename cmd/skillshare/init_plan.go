package main

import (
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/ui"
)

// initPlan is everything init will do. It is built from flags, detection,
// and answers without touching the disk; applyInitPlan carries it out.
type initPlan struct {
	connect   bool   // connecting an existing repo (second machine)
	base      string // source folder before --subdir
	subdir    string
	mode      string
	targets   []string // tool names, in detection order
	imports   []importedSkill
	dupes     []importedSkill // same name in a second tool; the first tool's copy wins
	importAll bool            // imports came from every tool (vs. one tool or none)
	git       bool
	gitScope  string // skills, agents, extras, or root
	remote    *remoteRepo
	remoteURL string
	pull      bool // bring the remote's skills to this machine
	skill     bool // install the built-in skillshare skill
	sync      bool
	dryRun    bool
}

func (p *initPlan) source() string {
	if p.subdir == "" {
		return p.base
	}
	return filepath.Join(p.base, p.subdir)
}

// sameNameAsRemote lists imported skills the remote also has; the remote's
// version is used for those.
func (p *initPlan) sameNameAsRemote() []string {
	if !p.pull || p.remote == nil {
		return nil
	}
	remote := map[string]bool{}
	for _, n := range p.remote.names {
		remote[n] = true
	}
	var same []string
	for _, s := range p.imports {
		if remote[s.name] {
			same = append(same, s.name)
		}
	}
	return same
}

// newInitPlan applies flags on top of the defaults: every detected tool,
// every existing skill, git on, the built-in skill installed. These are the
// answers Enter gives in the interactive flow, so a run without a terminal
// ends up with the same setup.
func newInitPlan(opts *initOptions, detected []detectedDir, home string) *initPlan {
	p := &initPlan{
		base:      opts.sourcePath,
		subdir:    opts.subdir,
		mode:      opts.mode,
		git:       !opts.noGit,
		gitScope:  opts.gitRootScope,
		remoteURL: opts.remoteURL,
		skill:     !opts.noSkill,
		dryRun:    opts.dryRun,
	}
	if p.base == "" {
		p.base = filepath.Join(config.BaseDir(), "skills")
	}
	if p.mode == "" {
		p.mode = "merge"
	}
	if p.gitScope == "" {
		p.gitScope = p.defaultGitScope()
	}

	switch {
	case opts.noTargets:
	case opts.targetsArg != "":
		known := config.DefaultTargets()
		for _, name := range strings.Split(opts.targetsArg, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := known[name]; !ok {
				ui.Warning("Unknown target: %s (skipped)", name)
				continue
			}
			p.targets = append(p.targets, name)
		}
	default:
		for _, d := range detected {
			p.targets = append(p.targets, d.name)
		}
	}

	switch {
	case opts.noCopy:
	case opts.copyFrom != "":
		if dir, ok := copyFromDir(opts.copyFrom, detected, home); ok {
			p.imports, p.dupes = collectImports([]detectedDir{dir})
		} else {
			ui.Warning("Copy source not found: %s", opts.copyFrom)
		}
	default:
		p.imports, p.dupes = collectImports(withSkills(detected))
		p.importAll = true
	}
	return p
}

// defaultGitScope picks what git versions when no flag chose it: the whole
// skillshare folder when a remote is linked (so agents and extras reach
// other machines too), otherwise only the skills. A source outside that
// folder keeps the skills scope, since the folder would hold none of them.
func (p *initPlan) defaultGitScope() string {
	if p.remote != nil && p.remote.reachable && !p.remote.rootScope && p.remote.skills > 0 {
		return "skills"
	}
	if p.remoteURL != "" && filepath.Dir(p.base) == config.BaseDir() {
		return "root"
	}
	return "skills"
}

// copyFromDir resolves --copy-from as a detected tool name or a path.
func copyFromDir(arg string, detected []detectedDir, home string) (detectedDir, bool) {
	for _, d := range detected {
		if strings.EqualFold(d.name, arg) {
			return d, true
		}
	}
	path := arg
	if strings.HasPrefix(path, "~") {
		path = filepath.Join(home, path[1:])
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return detectedDir{name: filepath.Base(path), path: path, exists: true}, true
	}
	return detectedDir{}, false
}

func withSkills(detected []detectedDir) []detectedDir {
	var out []detectedDir
	for _, d := range detected {
		if d.hasSkills {
			out = append(out, d)
		}
	}
	return out
}
