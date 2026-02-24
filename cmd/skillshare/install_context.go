package main

import (
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

// Compile-time interface satisfaction checks.
var (
	_ install.InstallContext = (*globalInstallContext)(nil)
	_ install.InstallContext = (*projectInstallContext)(nil)
)

// toSkillEntryDTOs converts config.SkillEntry (or its alias ProjectSkill)
// to install.SkillEntryDTO to avoid circular imports between install and config.
func toSkillEntryDTOs(skills []config.SkillEntry) []install.SkillEntryDTO {
	dtos := make([]install.SkillEntryDTO, len(skills))
	for i, s := range skills {
		dtos[i] = install.SkillEntryDTO{
			Name:    s.Name,
			Source:  s.Source,
			Tracked: s.Tracked,
			Group:   s.Group,
		}
	}
	return dtos
}

// ---------------------------------------------------------------------------
// globalInstallContext
// ---------------------------------------------------------------------------

// globalInstallContext implements install.InstallContext for global mode.
type globalInstallContext struct {
	cfg *config.Config
}

func (g *globalInstallContext) SourcePath() string { return g.cfg.Source }
func (g *globalInstallContext) ConfigSkills() []install.SkillEntryDTO {
	return toSkillEntryDTOs(g.cfg.Skills)
}
func (g *globalInstallContext) Reconcile() error              { return config.ReconcileGlobalSkills(g.cfg) }
func (g *globalInstallContext) PostInstallSkill(string) error { return nil }
func (g *globalInstallContext) Mode() string                  { return "global" }

// ---------------------------------------------------------------------------
// projectInstallContext
// ---------------------------------------------------------------------------

// projectInstallContext implements install.InstallContext for project mode.
type projectInstallContext struct {
	runtime *projectRuntime
}

func (p *projectInstallContext) SourcePath() string { return p.runtime.sourcePath }
func (p *projectInstallContext) ConfigSkills() []install.SkillEntryDTO {
	return toSkillEntryDTOs(p.runtime.config.Skills)
}
func (p *projectInstallContext) Reconcile() error {
	return reconcileProjectRemoteSkills(p.runtime)
}
func (p *projectInstallContext) PostInstallSkill(displayName string) error {
	return install.UpdateGitIgnore(
		filepath.Join(p.runtime.root, ".skillshare"),
		filepath.Join("skills", displayName),
	)
}
func (p *projectInstallContext) Mode() string { return "project" }
