package sync

import (
	"os"
	"path/filepath"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

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

// unavailable returns the declared entries that make discovery incomplete.
// While any exists, nothing absent from discovery can be attributed.
func (s followScope) unavailable() []sourcewalk.Entry {
	if s.set == nil {
		return nil
	}
	return s.set.Unavailable()
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
