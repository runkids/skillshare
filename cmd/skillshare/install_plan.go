package main

import (
	"path/filepath"

	"skillshare/internal/install"
)

// installPlannedSkill identifies a selected skill without claiming it was installed.
// Git paths are relative to the repository root, including an explicit subdirectory.
type installPlannedSkill struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (s *installLogSummary) planSkills(source *install.Source, skills []install.SkillInfo) {
	if !s.DryRun {
		return
	}
	s.PlannedSkills = make([]installPlannedSkill, 0, len(skills))
	for _, skill := range skills {
		path := skill.Path
		if (source.IsGit() || source.Type == install.SourceTypeArchive) && source.HasSubdir() {
			path = filepath.ToSlash(filepath.Join(source.Subdir, path))
		}
		s.PlannedSkills = append(s.PlannedSkills, installPlannedSkill{Name: skill.Name, Path: path})
	}
}
