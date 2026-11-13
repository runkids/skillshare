package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"skillshare/internal/mcp"
	"skillshare/internal/mcpchecklog"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

const mcpCheckUsage = "skillshare mcp check [name...] [--json] [--no-dns] [--live [--timeout <duration>] [--strict-tools] [--root <path>] [--verify-key <file>] [--sign-unsigned] [--accept-tools --revision <id> [--signing-key <file>]]]"

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
		case arg == "--strict-tools":
			opts.StrictTools = true
		case arg == "--sign-unsigned":
			opts.SignUnsigned = true
		case arg == "--verify-key" || arg == "--signing-key":
			if i+1 >= len(args) || args[i+1] == "" {
				return fmt.Errorf("%s requires a key file", arg)
			}
			i++
			if arg == "--verify-key" {
				opts.VerifyKeyPath = args[i]
			} else {
				opts.SigningKeyPath = args[i]
			}
		case strings.HasPrefix(arg, "--verify-key="):
			opts.VerifyKeyPath = strings.TrimPrefix(arg, "--verify-key=")
			if opts.VerifyKeyPath == "" {
				return fmt.Errorf("--verify-key requires a key file")
			}
		case strings.HasPrefix(arg, "--signing-key="):
			opts.SigningKeyPath = strings.TrimPrefix(arg, "--signing-key=")
			if opts.SigningKeyPath == "" {
				return fmt.Errorf("--signing-key requires a key file")
			}
		case arg == "--revision" || arg == "--root":
			if i+1 >= len(args) {
				return fmt.Errorf("%s requires a value", arg)
			}
			i++
			if arg == "--revision" {
				opts.Revision = args[i]
			} else {
				root := args[i]
				opts.Root = &root
			}
		case strings.HasPrefix(arg, "--revision="):
			opts.Revision = strings.TrimPrefix(arg, "--revision=")
		case strings.HasPrefix(arg, "--root="):
			root := strings.TrimPrefix(arg, "--root=")
			opts.Root = &root
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
	if (opts.VerifyKeyPath != "" || opts.SigningKeyPath != "" || opts.SignUnsigned) && !opts.Live {
		return fmt.Errorf("--verify-key, --signing-key and --sign-unsigned require --live")
	}
	if opts.SigningKeyPath != "" && (!opts.AcceptTools || opts.VerifyKeyPath == "") {
		return fmt.Errorf("--signing-key requires --accept-tools and --verify-key")
	}
	if opts.SignUnsigned && opts.VerifyKeyPath == "" {
		return fmt.Errorf("--sign-unsigned requires --verify-key")
	}
	if opts.AcceptTools && !opts.Live {
		return fmt.Errorf("--accept-tools only applies with --live")
	}
	if opts.AcceptTools && (len(opts.Names) != 1 || opts.Revision == "") {
		return fmt.Errorf("--accept-tools requires one server name and --revision from a complete live review")
	}
	if opts.Revision != "" && !opts.AcceptTools {
		return fmt.Errorf("--revision only applies to --accept-tools")
	}
	if opts.StrictTools && !opts.Live {
		return fmt.Errorf("--strict-tools only applies with --live")
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
		if logErr := mcpchecklog.WriteAudit(service.ConfigPath, start, report, err); logErr != nil {
			fmt.Fprintln(os.Stderr, "Warning: could not write MCP security scan log")
		}
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
		ensureEmptySlices(report)
		return writeJSONResult(mcpCheckJSON{report}, failed)
	}
	printMCPCheck(report)
	if failed != nil {
		// The summary line already says so.
		return &jsonSilentError{cause: failed}
	}
	return nil
}

// JSON escaping is a display boundary, independent of canonical fingerprinting.
// Preserve number lexemes and complete snapshots while preventing Unicode
// formatting/C1 controls from acting on a terminal displaying --json output.
type mcpCheckJSON struct{ report *mcp.CheckReport }

func (v mcpCheckJSON) MarshalJSON() ([]byte, error) {
	raw, err := json.Marshal(v.report)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, r := range string(raw) {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			if r <= 0xffff {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				a, z := utf16.EncodeRune(r)
				fmt.Fprintf(&b, "\\u%04x\\u%04x", a, z)
			}
		} else {
			b.WriteRune(r)
		}
	}
	return []byte(b.String()), nil
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
		fmt.Println(ui.StyledMark(mark) + " " + safeMCPCheckText(heading))
		for _, f := range server.Findings {
			message := f.Message
			if f.Target != "" {
				message = f.Target + ": " + message
			}
			if m, ok := marks[f.Level]; ok {
				fmt.Println("  " + ui.StyledMark(m) + " " + safeMCPCheckText(message))
			} else {
				ui.Note("  " + safeMCPCheckText(message))
			}
		}
		if review := server.Contract; review != nil {
			for _, change := range review.Changes {
				ui.Note("  " + safeMCPCheckText(change.Grade+" "+change.Tool+" "+change.Area+change.Path+": "+change.Message))
			}
			for _, finding := range review.Audit {
				ui.Note("  " + safeMCPCheckText("audit "+finding.Severity+" "+finding.Tool+finding.Path+" "+finding.RuleID+": "+finding.Message))
			}
			if review.Omitted > 0 {
				ui.Note(fmt.Sprintf("  %d additional change summaries omitted; inspect the complete inventory with --json", review.Omitted))
			}
			if review.Revision != "" {
				ui.Note("  Review revision: " + review.Revision)
				ui.Note("  Inspect full tool definitions with --json before explicit --accept-tools --revision approval")
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
			if server.Contract != nil && server.Contract.Status == "accepted" {
				accepted = append(accepted, name)
			} else {
				failed = append(failed, name)
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

// Tool metadata and diagnostics are untrusted terminal text, including bidi controls.
func safeMCPCheckText(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			if r <= 0xffff {
				fmt.Fprintf(&b, "\\u%04x", r)
			} else {
				a, z := utf16.EncodeRune(r)
				fmt.Fprintf(&b, "\\u%04x\\u%04x", a, z)
			}
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
