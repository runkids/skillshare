package install

import "strings"

// SkillEntryDTO is a dependency-free copy of config.SkillEntry, used to avoid
// a circular import between install and config.  The cmd layer converts
// config.SkillEntry → SkillEntryDTO before passing data through InstallContext.
type SkillEntryDTO struct {
	Name    string
	Source  string
	Tracked bool
	Group   string
}

// FullName returns the full relative path for the skill entry.
// If Group is set, returns "group/name"; otherwise returns Name.
func (s SkillEntryDTO) FullName() string {
	if s.Group != "" {
		return s.Group + "/" + s.Name
	}
	return s.Name
}

// EffectiveParts returns the effective (group, bareName) for this entry.
// If Group is set, returns (Group, Name).
// For backward compat, if Name contains "/" and Group is empty,
// splits at the last "/" to derive group and bare name.
func (s SkillEntryDTO) EffectiveParts() (group, name string) {
	if s.Group != "" {
		return s.Group, s.Name
	}
	if idx := strings.LastIndex(s.Name, "/"); idx >= 0 {
		return s.Name[:idx], s.Name[idx+1:]
	}
	return "", s.Name
}

// InstallContext abstracts the behavioral differences between global and
// project install modes so that InstallFromConfig can be written once.
type InstallContext interface {
	// SourcePath returns the skills directory
	// (e.g. ~/.config/skillshare/skills or .skillshare/skills).
	SourcePath() string

	// ConfigSkills returns the remote skill entries from the config file.
	ConfigSkills() []SkillEntryDTO

	// Reconcile performs post-install config reconciliation
	// (e.g. ReconcileGlobalSkills or ReconcileProjectSkills).
	Reconcile() error

	// PostInstallSkill is a per-skill hook called after a successful install.
	// For example, project mode uses this to update .skillshare/.gitignore.
	PostInstallSkill(displayName string) error

	// Mode returns "global" or "project".
	Mode() string
}

// ConfigInstallResult summarises the outcome of InstallFromConfig.
type ConfigInstallResult struct {
	Installed       int
	InstalledSkills []string
	FailedSkills    []string
}
