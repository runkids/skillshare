package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

type mcpOptions struct {
	name, url, from, file, revision, piExtension string
	targets                                      []string
	command                                      []string
	sync, dryRun, json, replace, noTUI           bool
}

func parseMCPOptions(args []string) (mcpOptions, error) {
	var o mcpOptions
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--":
			o.command = args[i+1:]
			return o, nil
		case "--url", "--target", "--from", "--file", "--revision", "--pi-extension":
			if i+1 == len(args) {
				return o, fmt.Errorf("%s requires a value", a)
			}
			i++
			value := args[i]
			switch a {
			case "--pi-extension":
				o.piExtension = value
			case "--url":
				o.url = value
			case "--target":
				o.targets = append(o.targets, value)
			case "--from":
				o.from = value
			case "--file":
				o.file = value
			case "--revision":
				o.revision = value
			}
		case "--sync":
			o.sync = true
		case "--dry-run", "-n":
			o.dryRun = true
		case "--json":
			o.json = true
		case "--no-tui":
			o.noTUI = true
		case "--replace":
			o.replace = true
		default:
			if strings.HasPrefix(a, "-") || o.name != "" {
				return o, fmt.Errorf("unknown MCP argument %q", a)
			}
			o.name = a
		}
	}
	return o, nil
}

func cmdMCP(args []string) (resultErr error) {
	defer func() {
		if errors.Is(resultErr, errMCPCancelled) {
			resultErr = nil
		}
	}()
	helpArgs := args
	for i, arg := range args {
		if arg == "--" {
			helpArgs = args[:i]
			break
		}
	}
	if wantsHelp(helpArgs) {
		printMCPHelp()
		return nil
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		args = append([]string{"list"}, args...)
	}
	sub, args := args[0], args[1:]
	service, rest, err := mcpContext(args)
	if err != nil {
		return err
	}
	o, err := parseMCPOptions(rest)
	if err != nil {
		return err
	}
	start := time.Now()
	switch sub {
	case "list":
		if mcpInteractive(o) && !o.dryRun {
			return runMCPManager(service)
		}
		p, err := service.Preview()
		if err != nil {
			return err
		}
		return printMCPPlan(p, o.json)
	case "add":
		return runMCPAdd(service, o)
	case "edit":
		return runMCPEdit(service, o)
	case "import":
		return runMCPImport(service, o)
	case "remove":
		if mcpInteractive(o) {
			return mcpRemoveWizard(service, o, terminalMCPPrompts{})
		}
		if o.name == "" {
			return fmt.Errorf("usage: skillshare mcp remove <name> [--sync]")
		}
		return finishMCPMutation(service, mcp.Mutation{Name: o.name, Remove: true}, o, start)
	case "restore":
		return runMCPRestore(service, o, terminalMCPPrompts{})
	default:
		return fmt.Errorf("unknown MCP command %q; run skillshare mcp --help", sub)
	}
}

func cmdSyncMCP(args []string) error {
	if wantsHelp(args) {
		printMCPHelp()
		return nil
	}
	service, rest, err := mcpContext(args)
	if err != nil {
		return err
	}
	o, err := parseMCPOptions(rest)
	if err != nil {
		return err
	}
	if o.piExtension != "" || o.name != "" || o.url != "" || o.from != "" || o.file != "" || len(o.command) > 0 || len(o.targets) > 0 || o.replace || o.sync {
		return fmt.Errorf("sync mcp accepts only --dry-run, --json, --revision and scope flags")
	}
	if o.dryRun {
		p, err := service.Preview()
		if err != nil {
			return err
		}
		if err := printMCPPlan(p, o.json); err != nil {
			return err
		}
		if p.Blocked {
			return fmt.Errorf("MCP conflicts found; no files changed")
		}
		return nil
	}
	start := time.Now()
	result, err := service.Apply(o.revision)
	logMCPOp(service.ConfigPath, "sync mcp", start, err)
	if result != nil {
		if outputErr := printMCPResult(result, o.json); outputErr != nil {
			return outputErr
		}
	}
	return err
}

func printMCPPlan(p *mcp.Plan, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	ui.Info("MCP source: %s", p.SourcePath)
	for _, c := range p.Changes {
		detail := c.Target
		if c.Message != "" {
			detail += " — " + c.Message
		}
		ui.Status(c.Name, c.Action, detail)
	}
	if len(p.Changes) == 0 {
		ui.Info("No MCP servers configured. Run 'skillshare mcp add' to get started.")
	}
	return nil
}

func printMCPResult(result *mcp.Result, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	if result.Plan != nil {
		if err := printMCPPlan(result.Plan, false); err != nil {
			return err
		}
	}
	for _, id := range result.BackupIDs {
		ui.Info("Backup: %s", id)
	}
	if result.Plan == nil {
		ui.Success("MCP source saved. Run 'skillshare sync mcp' when ready.")
	} else if !result.Plan.Blocked {
		ui.Success("MCP files applied: %d. Reload your Agent after synchronization and complete any required login.", len(result.Applied))
	}
	return nil
}

func logMCPOp(path, command string, start time.Time, err error) {
	if logErr := oplog.Write(path, oplog.OpsFile, oplog.NewEntry(command, statusFromErr(err), time.Since(start))); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write MCP operation log")
	}
}

func printMCPHelp() {
	fmt.Println(`Usage: skillshare mcp [command] [options]

Commands:
  add [name]        Guided setup, or --url URL / -- command args...
  edit [name]       Interactive editor, or update --url / --target / -- command
  import [name]     Import --from <client> or --file <JSON/TOML/YAML file>
  list             Browse connections and per-client sync status (default)
  remove [name]     Select and remove a source entry; optionally sync removal
  restore [id]      Browse backups, preview and restore Agent entries

Options:
  --pi-extension <package>  pi-mcp-adapter or pi-mcp-extension (requires installation in Pi)
  --target <client>  Receiving client; repeat for multiple clients
  --from <client>    Native client ID (see mcp documentation for destinations)
  --file <path>      Native configuration file to import
  --url <url>        Streamable HTTP endpoint
  --sync            Save and synchronize (non-interactive default: save only)
  --replace         Replace an existing source entry; on import, also rewrite
                    the imported client's entry when it differs
  --dry-run, -n     Preview without writing
  --json            Machine-readable, credential-free sync results
  --no-tui          Disable interactive menus (also honors tui: false)
  --revision <id>   Require the matching preview revision
  --global, -g      Global configuration
  --project, -p     Project configuration

With no command, opens the MCP manager in a terminal; otherwise prints status.
Manager keys: / search, Enter details, a add, i import, e edit, x remove,
              s sync, b backups, r refresh, q quit.
JSON and non-TTY output never open a TUI. Scripted edits require a name.
Sync: skillshare sync mcp [--dry-run] [--json] [--no-tui] [-g|-p]
Import does not start MCP servers or copy OAuth credentials.`)
}
