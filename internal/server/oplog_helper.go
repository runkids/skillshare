package server

import (
	"time"

	"skillshare/internal/oplog"
)

func (s *Server) writeOpsLog(cmd, status string, start time.Time, args map[string]any, msg string) {
	e := oplog.NewEntry(cmd, status, time.Since(start))
	if len(args) > 0 {
		e.Args = args
	}
	if msg != "" {
		e.Message = msg
	}
	oplog.Write(s.configPath(), oplog.OpsFile, e) //nolint:errcheck
}

func (s *Server) writeAuditLog(status string, start time.Time, args map[string]any, msg string) {
	e := oplog.NewEntry("audit", status, time.Since(start))
	if len(args) > 0 {
		e.Args = args
	}
	if msg != "" {
		e.Message = msg
	}
	oplog.Write(s.configPath(), oplog.AuditFile, e) //nolint:errcheck
}

// writeAuditLogTo writes an audit log entry using the given config path (no lock needed).
// Used by handlers that snapshot cfgPath under RLock and write the log after releasing.
func writeAuditLogTo(cfgPath, status string, start time.Time, args map[string]any, msg string) {
	e := oplog.NewEntry("audit", status, time.Since(start))
	if len(args) > 0 {
		e.Args = args
	}
	if msg != "" {
		e.Message = msg
	}
	oplog.Write(cfgPath, oplog.AuditFile, e) //nolint:errcheck
}
