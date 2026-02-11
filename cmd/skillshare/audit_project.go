package main

import (
	"fmt"

	"skillshare/internal/audit"
)

func cmdAuditProject(root, specificSkill string) (auditRunSummary, bool, error) {
	if !projectConfigExists(root) {
		return auditRunSummary{}, false, fmt.Errorf("no project config found; run 'skillshare init -p' first")
	}

	rt, err := loadProjectRuntime(root)
	if err != nil {
		return auditRunSummary{}, false, err
	}

	threshold, err := audit.NormalizeThreshold(rt.config.Audit.BlockThreshold)
	if err != nil {
		threshold = audit.DefaultThreshold()
	}

	if specificSkill != "" {
		_, summary, err := auditSkillByName(rt.sourcePath, specificSkill, "project", root, threshold, false)
		return summary, summary.Failed > 0, err
	}

	_, summary, err := auditInstalled(rt.sourcePath, "project", root, threshold, false)
	return summary, summary.Failed > 0, err
}
