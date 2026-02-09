package main

import (
	"fmt"
)

func cmdAuditProject(root, specificSkill string) (auditRunSummary, bool, error) {
	if !projectConfigExists(root) {
		return auditRunSummary{}, false, fmt.Errorf("no project config found; run 'skillshare init -p' first")
	}

	rt, err := loadProjectRuntime(root)
	if err != nil {
		return auditRunSummary{}, false, err
	}

	if specificSkill != "" {
		summary, blocked, err := auditSingleSkill(rt.sourcePath, specificSkill)
		summary.Mode = "project"
		return summary, blocked, err
	}

	summary, blocked, err := auditAllSkills(rt.sourcePath)
	summary.Mode = "project"
	return summary, blocked, err
}
