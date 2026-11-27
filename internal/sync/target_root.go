package sync

import (
	"os"

	"skillshare/internal/config"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// TargetRoot says what a skills target folder is, so every command treats a
// folder that is itself a link the way sync does.
type TargetRoot int

const (
	// TargetRootPlain is not a link: a directory, a file, or nothing.
	TargetRootPlain TargetRoot = iota
	// TargetRootSourceLink links to the source: symlink mode, or left over from it.
	TargetRootSourceLink
	// TargetRootExternalLink is another tool's link (a dotfiles manager) to a
	// directory. Sync works through it, so it counts as that directory.
	TargetRootExternalLink
	// TargetRootBrokenLink is another tool's link to nothing or to a file.
	TargetRootBrokenLink
)

// ClassifyTargetRoot reports what the skills target folder at path is.
func ClassifyTargetRoot(path, sourcePath string) TargetRoot {
	if !utils.IsSymlinkOrJunction(path) {
		return TargetRootPlain
	}
	if IsSymlinkToSource(path, sourcePath) {
		return TargetRootSourceLink
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return TargetRootExternalLink
	}
	return TargetRootBrokenLink
}

// ResolveTargetRoot returns the directory a skills or agents target folder
// stands for: where another tool's link leads, even when nothing is there
// yet, or else path itself. Backup and restore use it so they act on the files
// sync wrote and keep the link, while a link to the source still has
// nothing of its own.
func ResolveTargetRoot(path, sourcePath string) string {
	switch ClassifyTargetRoot(path, sourcePath) {
	case TargetRootExternalLink:
		return sourcewalk.Canonical(path)
	case TargetRootBrokenLink:
		// A link to nothing leads where restore can make the directory; a
		// link to a file stays, so backup skips it.
		if dest, err := utils.ResolveLinkTarget(path); err == nil {
			if _, err := os.Lstat(dest); os.IsNotExist(err) {
				return sourcewalk.Canonical(dest)
			}
		}
	}
	return path
}

// rootLinkStatus is the merge and copy status of a target folder that cannot
// hold skills of that mode, with done false when the caller should count the
// skills in it: a plain directory, or the directory an external link leads to.
func rootLinkStatus(targetPath, sourcePath string, info os.FileInfo) (TargetStatus, bool) {
	switch ClassifyTargetRoot(targetPath, sourcePath) {
	case TargetRootSourceLink: // symlink mode, or left over from it
		return StatusLinked, true
	case TargetRootBrokenLink:
		return StatusConflict, true
	case TargetRootPlain:
		if !info.IsDir() {
			return StatusUnknown, true
		}
	}
	return StatusUnknown, false
}

// SkillsFolderKeptBy is config.SkillsPathKeptBy that also follows links: a
// target whose folder leads to the same directory as name's shares it, either
// way round. A folder that links to the source is never shared, since
// removing it leaves other targets' links as they are.
func SkillsFolderKeptBy(targets map[string]config.TargetConfig, name string, leaving map[string]bool, sourcePath string) string {
	if keeper := config.SkillsPathKeptBy(targets, name, leaving); keeper != "" {
		return keeper
	}
	self := targets[name]
	path := config.ExpandPath(self.SkillsConfig().Path)
	if path == "" || ClassifyTargetRoot(path, sourcePath) == TargetRootSourceLink {
		return ""
	}
	resolved := sourcewalk.Canonical(path)
	for other, target := range targets {
		sc := target.SkillsConfig()
		if other == name || leaving[other] || sc.Path == "" || !sc.IsEnabled() {
			continue
		}
		if utils.PathsEqual(sourcewalk.Canonical(config.ExpandPath(sc.Path)), resolved) {
			return other
		}
	}
	return ""
}
