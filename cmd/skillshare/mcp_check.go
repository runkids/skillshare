package main

import (
	"fmt"
	"strings"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/ui"
)

const mcpCheckUsage = "skillshare mcp check [name...] [--json] [--no-dns] [--live [--timeout <duration>]]"

// runMCPCheck is read-only: it writes no file and no operation log entry. Only --live
// starts servers or sends requests to them.
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
	report, err := service.Check(opts)
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
