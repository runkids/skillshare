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

// storeToSkillEntryDTOs converts MetadataStore entries to []install.SkillEntryDTO.
func storeToSkillEntryDTOs(store *install.MetadataStore) []install.SkillEntryDTO {
	names := store.List() // sorted
	dtos := make([]install.SkillEntryDTO, 0, len(names))
	for _, name := range names {
		entry := store.Get(name)
		if entry == nil {
			continue
		}
		dtos = append(dtos, install.SkillEntryDTO{
			Name:    name,
			Source:  entry.Source,
			Tracked: entry.Tracked,
			Group:   entry.Group,
			Branch:  entry.Branch,
		})
	}
	return dtos
}

// ---------------------------------------------------------------------------
// globalInstallContext
// ---------------------------------------------------------------------------

// globalInstallContext implements install.InstallContext for global mode.
type globalInstallContext struct {
	cfg   *config.Config
	store *install.MetadataStore
}

func (g *globalInstallContext) SourcePath() string { return g.cfg.Source }
func (g *globalInstallContext) ConfigSkills() []install.SkillEntryDTO {
	return storeToSkillEntryDTOs(g.store)
}
func (g *globalInstallContext) Reconcile() error {
	return config.ReconcileGlobalSkills(g.cfg, g.store)
}
func (g *globalInstallContext) PostInstallSkill(string) error { return nil }
func (g *globalInstallContext) Mode() string                  { return "global" }
func (g *globalInstallContext) GitLabHosts() []string         { return g.cfg.EffectiveGitLabHosts() }
func (g *globalInstallContext) AzureHosts() []string          { return g.cfg.EffectiveAzureHosts() }

// ---------------------------------------------------------------------------
// projectInstallContext
// ---------------------------------------------------------------------------

// projectInstallContext implements install.InstallContext for project mode.
type projectInstallContext struct {
	runtime *projectRuntime
}

func (p *projectInstallContext) SourcePath() string { return p.runtime.sourcePath }
func (p *projectInstallContext) ConfigSkills() []install.SkillEntryDTO {
	return storeToSkillEntryDTOs(p.runtime.skillsStore)
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
func (p *projectInstallContext) GitLabHosts() []string {
	return p.runtime.config.EffectiveGitLabHosts()
}
func (p *projectInstallContext) AzureHosts() []string {
	return p.runtime.config.EffectiveAzureHosts()
}
