package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

const mcpCheckUsage = "skillshare mcp check [name...] [--json] [--no-dns] [--live [--timeout <duration>] [--accept-tools]]"

// runMCPCheck writes no config file. Only --live starts servers or sends requests to them, and
// only --live reads or writes the tool baseline in the state dir. Only --accept-tools, which
// approves changed tool definitions, writes an operation log entry.
func runMCPCheck(service *mcp.Service, args []string) error {
	opts := mcp.CheckOptions{ClientVersion: version}
	asJSON := false
	timeout := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			asJSON = true
		case arg == "--no-dns":
			opts.SkipDNS = true
		case arg == "--live":
			opts.Live = true
		case arg == "--accept-tools":
			opts.AcceptTools = true
		case arg == "--timeout":
			if i+1 >= len(args) {
				return fmt.Errorf("--timeout requires a duration such as 10s")
			}
			i++
			timeout = args[i]
		case strings.HasPrefix(arg, "--timeout="):
			timeout = strings.TrimPrefix(arg, "--timeout=")
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown mcp check argument %q; usage: %s", arg, mcpCheckUsage)
		default:
			opts.Names = append(opts.Names, arg)
		}
	}
	if opts.AcceptTools && !opts.Live {
		return fmt.Errorf("--accept-tools only applies with --live")
	}
	if timeout != "" {
		if !opts.Live {
			return fmt.Errorf("--timeout only applies with --live")
		}
		d, err := time.ParseDuration(timeout)
		if err != nil || d <= 0 {
			return fmt.Errorf("--timeout needs a positive duration such as 10s or 1m, got %q", timeout)
		}
		opts.Timeout = d
	}
	start := time.Now()
	report, err := service.Check(opts)
	if opts.AcceptTools {
		logMCPAcceptTools(service.ConfigPath, start, report, err)
	}
	if err != nil {
		if asJSON {
			return writeJSONError(err)
		}
		return err
	}
	var failed error
	if report.Summary.Errors > 0 {
		failed = fmt.Errorf("MCP check found %d error(s)", report.Summary.Errors)
	}
	if asJSON {
		return writeJSONResult(report, failed)
	}
	printMCPCheck(report)
	if failed != nil {
		// The summary line already says so.
		return &jsonSilentError{cause: failed}
	}
	return nil
}

func printMCPCheck(report *mcp.CheckReport) {
	printMCPSource(report.SourcePath)
	marks := map[string]string{"error": ui.MarkFail, "warning": ui.MarkWarn}
	for _, server := range report.Servers {
		heading := server.Name
		if server.Project != "" {
			heading += "  (project " + shortenPath(server.Project) + ")"
		}
		mark := ui.MarkOK
		switch {
		case !server.OK:
			mark = ui.MarkFail
		case mcpCheckHasWarning(server):
			mark = ui.MarkWarn
		}
		fmt.Println(ui.StyledMark(mark) + " " + heading)
		for _, f := range server.Findings {
			message := f.Message
			if f.Target != "" {
				message = f.Target + ": " + message
			}
			if m, ok := marks[f.Level]; ok {
				fmt.Println("  " + ui.StyledMark(m) + " " + message)
			} else {
				ui.Note("  " + message)
			}
		}
	}

	mark, text := ui.MarkOK, "Checked "+plural(len(report.Servers), "server")
	if report.Summary.Errors > 0 {
		mark, text = ui.MarkFail, fmt.Sprintf("%s: %s", text, plural(report.Summary.Errors, "error"))
	}
	if report.Summary.Warnings > 0 {
		if mark == ui.MarkOK {
			mark, text = ui.MarkWarn, text+": "+plural(report.Summary.Warnings, "warning")
		} else {
			text += ", " + plural(report.Summary.Warnings, "warning")
		}
	}
	fmt.Println()
	ui.Done(mark, text, 0)
}

func mcpCheckHasWarning(server mcp.CheckServer) bool {
	for _, f := range server.Findings {
		if f.Level == "warning" {
			return true
		}
	}
	return false
}

// logMCPAcceptTools records which servers' tool baselines --accept-tools recorded, so a later
// review can tell when a changed definition was approved. A first pin counts too. A server
// with an error, or whose baseline could not be saved, is listed as failed, and so is the entry.
func logMCPAcceptTools(path string, start time.Time, report *mcp.CheckReport, err error) {
	entry := oplog.NewEntry("mcp check --accept-tools", statusFromErr(err), time.Since(start))
	if report != nil {
		var accepted, failed []string
		for _, server := range report.Servers {
			name := strings.TrimSpace(server.Name + " " + server.Project)
			recorded, ok := false, true
			for _, f := range server.Findings {
				pin := f.Check == "tool-change"
				recorded = recorded || pin && (strings.HasPrefix(f.Message, "accepted ") || strings.HasPrefix(f.Message, "pinned "))
				ok = ok && f.Level != "error" && !(pin && strings.HasPrefix(f.Message, "could not save the tool baseline"))
			}
			switch {
			case !ok:
				failed = append(failed, name)
			case recorded:
				accepted = append(accepted, name)
			}
		}
		entry.Args = map[string]any{"accepted": accepted}
		if len(failed) > 0 {
			entry.Args["failed"] = failed
			entry.Status = "error"
		}
	}
	if logErr := oplog.Write(path, oplog.OpsFile, entry); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write MCP operation log")
	}
}
