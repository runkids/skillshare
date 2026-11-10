package audit

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"skillshare/internal/sourcewalk"
)

// skillRoots records the directories under a scan root that discovery treats
// as skills, so a tracked repository's skills in hidden directories (such as
// .agents/skills/<name> or .curated/<name>) get the same text scan they get
// when audited one by one. Target copies inside target dot dirs are not
// skills and stay header-only.
type skillRoots struct {
	agentsSkills string // slash path of a .agents/skills anchor, or ""
	dirs         map[string]bool
}

func newSkillRoots(scanRoot string) *skillRoots {
	r := &skillRoots{dirs: map[string]bool{}}
	if dir, ok := sourcewalk.AgentsSkillsDir(scanRoot, sourcewalk.TargetDotDirs); ok {
		if rel, err := filepath.Rel(scanRoot, dir); err == nil {
			r.agentsSkills = filepath.ToSlash(rel)
		}
	}
	return r
}

// visit records dir (slash relPath below the scan root) when it holds a
// SKILL.md outside target dot dirs.
func (r *skillRoots) visit(dir, relPath string) {
	if relPath == "." {
		return
	}
	// Like discovery, any SKILL.md entry that is not a directory counts.
	if info, err := os.Lstat(filepath.Join(dir, "SKILL.md")); err != nil || info.IsDir() {
		return
	}
	// Below the .agents/skills anchor only nested target dot dirs disqualify.
	rest := relPath
	if r.agentsSkills != "" && relPath == r.agentsSkills {
		rest = ""
	} else if r.agentsSkills != "" {
		rest = strings.TrimPrefix(relPath, r.agentsSkills+"/")
	}
	for _, part := range strings.Split(rest, "/") {
		if sourcewalk.TargetDotDirs[part] {
			return
		}
	}
	r.dirs[relPath] = true
}

// owner returns the deepest recorded skill containing relPath, or "" when
// the scan root itself holds it.
func (r *skillRoots) owner(relPath string) string {
	for dir := path.Dir(relPath); dir != "."; dir = path.Dir(dir) {
		if r.dirs[dir] {
			return dir
		}
	}
	return ""
}

// within returns relPath relative to the deepest recorded skill containing it.
func (r *skillRoots) within(relPath string) string {
	if dir := r.owner(relPath); dir != "" {
		return strings.TrimPrefix(relPath, dir+"/")
	}
	return relPath
}
