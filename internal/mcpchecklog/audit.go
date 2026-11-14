// Package mcpchecklog records contract scan outcomes at CLI/API boundaries.
// It deliberately excludes all server-controlled text and credential material.
package mcpchecklog

import (
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
)

// WriteAudit records an explicit approval attempt separately from the trust
// mutation's operations.log entry. Ordinary read-only checks do not call it.
// A logging failure never changes the result of an already committed approval.
func WriteAudit(path string, start time.Time, report *mcp.CheckReport, checkErr error) error {
	accepted := checkErr == nil && report != nil && report.ToolsAcceptance == mcp.ToolsAcceptanceAccepted
	status := "error"
	if accepted {
		status = "ok"
	}
	servers, tools, findings := 0, 0, 0
	if report != nil {
		servers = len(report.Servers)
		for _, server := range report.Servers {
			if review := server.Contract; review != nil {
				tools += len(review.Tools)
				findings += len(review.Audit)
			}
		}
	}
	e := oplog.NewEntry("mcp contract review", status, time.Since(start))
	e.Args = map[string]any{"servers": servers, "tools": tools, "findings": findings, "accepted": accepted}
	return oplog.Write(path, oplog.AuditFile, e)
}
