package main

import (
	"path/filepath"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

type projectRuntime struct {
	root             string
	config           *config.ProjectConfig
	skillsStore      *install.MetadataStore
	agentsStore      *install.MetadataStore
	sourcePath       string
	agentsSourcePath string
	targets          map[string]config.TargetConfig
}

func loadProjectRuntime(root string) (*projectRuntime, error) {
	cfg, err := config.LoadProject(root)
	if err != nil {
		return nil, err
	}

	targets, err := config.ResolveProjectTargets(root, cfg)
	if err != nil {
		return nil, err
	}

	skillsDir := filepath.Join(root, ".skillshare", "skills")
	agentsDir := filepath.Join(root, ".skillshare", "agents")

	skillsStore, err := install.LoadMetadataWithMigration(skillsDir, "")
	if err != nil {
		return nil, err
	}

	agentsStore, err := install.LoadMetadataWithMigration(agentsDir, "agent")
	if err != nil {
		return nil, err
	}

	return &projectRuntime{
		root:             root,
		config:           cfg,
		skillsStore:      skillsStore,
		agentsStore:      agentsStore,
		sourcePath:       skillsDir,
		agentsSourcePath: agentsDir,
		targets:          targets,
	}, nil
}

// configFromProjectRuntime builds a minimal global Config from a project runtime,
// carrying host lists so that source parsing uses the project's azure_hosts / gitlab_hosts.
func configFromProjectRuntime(r *projectRuntime) *config.Config {
	return &config.Config{
		Source:       r.sourcePath,
		AgentsSource: r.agentsSourcePath,
		GitLabHosts:  r.config.GitLabHosts,
		AzureHosts:   r.config.AzureHosts,
	}
}
