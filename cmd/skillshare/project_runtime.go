package main

import (
	"path/filepath"

	"skillshare/internal/config"
)

type projectRuntime struct {
	root       string
	config     *config.ProjectConfig
	registry   *config.Registry
	sourcePath string
	targets    map[string]config.TargetConfig
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

	reg, err := config.LoadRegistry(filepath.Join(root, ".skillshare"))
	if err != nil {
		return nil, err
	}

	return &projectRuntime{
		root:       root,
		config:     cfg,
		registry:   reg,
		sourcePath: filepath.Join(root, ".skillshare", "skills"),
		targets:    targets,
	}, nil
}
