package main

import (
	"skillshare/internal/audit"
	"skillshare/internal/install"
)

// updateContext holds mode-specific configuration for update operations.
// projectRoot == "" means global mode.
type updateContext struct {
	sourcePath  string
	projectRoot string
	opts        *updateOptions
}

func (uc *updateContext) isProject() bool {
	return uc.projectRoot != ""
}

func (uc *updateContext) auditScanFn() auditScanFunc {
	if uc.isProject() {
		return func(path string) (*audit.Result, error) {
			return audit.ScanSkillForProject(path, uc.projectRoot)
		}
	}
	return audit.ScanSkill
}

func (uc *updateContext) makeInstallOpts() install.InstallOptions {
	opts := install.InstallOptions{
		Force:          true,
		Update:         true,
		SkipAudit:      uc.opts.skipAudit,
		AuditThreshold: uc.opts.threshold,
	}
	if uc.isProject() {
		opts.AuditProjectRoot = uc.projectRoot
	}
	return opts
}
