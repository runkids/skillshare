package main

import (
	"strings"

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
		relPath := install.KeyToRelPath(name, entry)
		group, bareName := splitMetadataRelPath(relPath)
		dtos = append(dtos, install.SkillEntryDTO{
			Name:    bareName,
			Source:  entry.Source,
			Tracked: entry.Tracked,
			Group:   group,
			Branch:  entry.Branch,
		})
	}
	return dtos
}

func splitMetadataRelPath(relPath string) (group, name string) {
	relPath = strings.Trim(relPath, "/")
	idx := strings.LastIndex(relPath, "/")
	if idx < 0 {
		return "", relPath
	}
	return relPath[:idx], relPath[idx+1:]
}

// ---------------------------------------------------------------------------
// globalInstallContext
// ---------------------------------------------------------------------------

// globalInstallContext implements install.InstallContext for global mode.
type globalInstallContext struct {
	cfg   *config.Config
	store *install.MetadataStore
}

func (g *globalInstallContext) SourcePath() string { return g.cfg.EffectiveSkillsSource() }
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
	skills := p.runtime.config.Skills
	dtos := make([]install.SkillEntryDTO, 0, len(skills))
	for _, s := range skills {
		dtos = append(dtos, install.SkillEntryDTO{
			Name:    s.Name,
			Source:  s.Source,
			Tracked: s.Tracked,
			Group:   s.Group,
			Branch:  s.Branch,
		})
	}
	return dtos
}
func (p *projectInstallContext) Reconcile() error {
	return reconcileProjectRemoteSkills(p.runtime)
}
func (p *projectInstallContext) PostInstallSkill(displayName string) error {
	gitDir, prefix := config.ProjectGitignoreTarget(p.runtime.root, p.runtime.sourcePath)
	if gitDir == "" {
		return nil
	}
	return install.UpdateGitIgnore(gitDir, prefix+"/"+displayName)
}
func (p *projectInstallContext) Mode() string { return "project" }
func (p *projectInstallContext) GitLabHosts() []string {
	return p.runtime.config.EffectiveGitLabHosts()
}
func (p *projectInstallContext) AzureHosts() []string {
	return p.runtime.config.EffectiveAzureHosts()
}
