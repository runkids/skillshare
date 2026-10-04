package sync

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// FollowSetFor snapshots source's declarations against the enabled skills
// targets and the staging boundary gitRoot. It returns nil without declarations
// or diagnostics, which preserves legacy traversal and output.
func FollowSetFor(source string, targets map[string]config.TargetConfig, gitRoot string) *sourcewalk.FollowSet {
	var paths []string
	for _, target := range targets {
		skills := target.SkillsConfig()
		if skills.IsEnabled() && skills.Path != "" {
			paths = append(paths, skills.Path)
		}
	}
	sort.Strings(paths)
	set := sourcewalk.Follow(source, sourcewalk.FollowOptions{TargetPaths: paths, GitRoot: gitRoot})
	if !set.Active() && len(set.Warnings()) == 0 {
		return nil
	}
	return &set
}

// followScope binds one operation's FollowSet to the skills source root, so
// link identity, ownership, and status agree on the same canonical roots.
// A nil set keeps every legacy rule.
type followScope struct {
	set         *sourcewalk.FollowSet
	source      string // absolute logical source root
	canonSource string // canonical source root; source when it cannot be resolved
}

func newFollowScope(sourcePath string, set *sourcewalk.FollowSet) followScope {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		abs = filepath.Clean(sourcePath)
	}
	canon, err := sourcewalk.Canonicalize(abs)
	if err != nil {
		canon = abs
	}
	return followScope{set: set, source: abs, canonSource: canon}
}

// PrunePaused lists the unavailable followed entries, as "name (state)", that
// pause prune for an operation using set. Diff calls it to preview sync.
func PrunePaused(set *sourcewalk.FollowSet) []string {
	var names []string
	for _, entry := range set.Unavailable() {
		names = append(names, entry.Name+" ("+string(entry.State)+")")
	}
	return names
}

// KeepsManagedCopies reports whether copy sync keeps an existing managed copy
// it would otherwise overwrite or replace. In standard naming a target name
// says nothing about the skill's origin, so while an entry is unavailable an
// existing managed copy may be the only copy of its content. Flat names carry
// the logical prefix and proceed.
func KeepsManagedCopies(targetNaming string, set *sourcewalk.FollowSet) bool {
	return len(set.Unavailable()) > 0 &&
		config.EffectiveTargetNaming(targetNaming) != "flat"
}

// SameSkillLink reports whether linkPath already points at skill, as merge
// sync decides it. Diff calls it to preview sync.
func SameSkillLink(linkPath string, skill DiscoveredSkill, sourcePath string, set *sourcewalk.FollowSet) bool {
	return sameSkillLink(linkPath, skill, newFollowScope(sourcePath, set))
}

// OwnsSourceLink reports whether linkPath resolves inside the source or a
// currently followed entry's resolved target, as merge sync's orphan prune
// decides it. Diff calls it to preview sync.
func OwnsSourceLink(linkPath, sourcePath string, set *sourcewalk.FollowSet) bool {
	return newFollowScope(sourcePath, set).ownsLink(linkPath)
}

// skillLinkTarget returns where a skill link points. Relative link text is read
// against the link's canonical parent, which is where the OS resolves it.
func skillLinkTarget(linkPath string) (string, error) {
	if raw, err := os.Readlink(linkPath); err == nil && !filepath.IsAbs(raw) {
		parent, err := sourcewalk.Canonicalize(filepath.Dir(linkPath))
		if err != nil {
			return "", err
		}
		return filepath.Join(parent, raw), nil
	}
	return utils.ResolveLinkTarget(linkPath)
}

// skillPath is the link destination for skill: its root and logical tail.
func (s followScope) skillPath(skill DiscoveredSkill) skillLinkPath {
	return skillLinkPath{root: s.source, tail: skill.RelPath}
}

// sameSkillLink reports whether linkPath already points at skill. It accepts
// exactly three forms: the skill's logical path, the canonical source root plus
// the logical tail, or the skill's fully resolved path. A link that merely sits
// under an owned root is a different skill.
func sameSkillLink(linkPath string, skill DiscoveredSkill, scope followScope) bool {
	dest, err := skillLinkTarget(linkPath)
	if err != nil {
		return false
	}
	logical, err := filepath.Abs(skill.SourcePath)
	if err != nil {
		return false
	}
	if utils.PathsEqual(dest, logical) || utils.PathsEqual(dest, filepath.Join(scope.canonSource, filepath.FromSlash(skill.RelPath))) {
		return true
	}
	resolved, err := sourcewalk.Canonicalize(logical)
	return err == nil && utils.PathsEqual(dest, resolved)
}

// ownsLink reports whether a link resolves inside the logical source, the
// canonical source root, or the resolved target of a currently followed entry.
// This is containment, not provenance: prune still requires the manifest.
func (s followScope) ownsLink(linkPath string) bool {
	dest, err := skillLinkTarget(linkPath)
	if err != nil {
		return false
	}
	if pathUnder(dest, s.source) || pathUnder(dest, s.canonSource) {
		return true
	}
	return s.inFollowedTarget(dest)
}

// inFollowedTarget reports whether dest resolves inside a currently followed
// entry's resolved target.
func (s followScope) inFollowedTarget(dest string) bool {
	if s.set == nil {
		return false
	}
	canon, err := sourcewalk.Canonicalize(dest)
	return err == nil && s.set.FollowedTargetContains(canon)
}

// followedLink reports whether a link resolves inside a currently followed
// entry's resolved target.
func followedLink(linkPath, sourcePath string, set *sourcewalk.FollowSet) bool {
	dest, err := skillLinkTarget(linkPath)
	return err == nil && newFollowScope(sourcePath, set).inFollowedTarget(dest)
}

// pathUnder reports whether path is strictly below root.
func pathUnder(path, root string) bool {
	return utils.PathHasPrefix(path, strings.TrimRight(root, string(filepath.Separator))+string(filepath.Separator))
}
