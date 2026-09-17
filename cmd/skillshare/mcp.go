package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
)

type mcpOptions struct {
	name, url, from, file, revision string
	targets                         []string
	command                         []string
	sync, dryRun, json, replace     bool
}

func parseMCPOptions(args []string) (mcpOptions, error) {
	var o mcpOptions
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--":
			o.command = args[i+1:]
			return o, nil
		case "--url", "--target", "--from", "--file", "--revision":
			if i+1 == len(args) {
				return o, fmt.Errorf("%s requires a value", a)
			}
			i++
			value := args[i]
			switch a {
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

func cmdMCP(args []string) error {
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
	if len(args) == 0 {
		return cmdMCP([]string{"list"})
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
		p, err := service.Preview()
		if err != nil {
			return err
		}
		return printMCPPlan(p, o.json)
	case "add":
		return runMCPAdd(service, o)
	case "import":
		return runMCPImport(service, o)
	case "remove":
		if o.name == "" {
			return fmt.Errorf("usage: skillshare mcp remove <name> [--sync]")
		}
		return finishMCPMutation(service, mcp.Mutation{Name: o.name, Remove: true}, o, start)
	case "restore":
		p, err := service.PreviewRestore(o.name)
		if err != nil {
			return err
		}
		if o.dryRun {
			return printMCPPlan(p, o.json)
		}
		result, err := service.Restore(o.name, p.Revision)
		logMCPOp(service.ConfigPath, "mcp restore", start, err)
		if result != nil {
			if outputErr := printMCPResult(result, o.json); outputErr != nil {
				return outputErr
			}
		}
		return err
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
	if o.name != "" || o.url != "" || o.from != "" || o.file != "" || len(o.command) > 0 || len(o.targets) > 0 || o.replace || o.sync {
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
	fmt.Printf("MCP source: %s\n", p.SourcePath)
	for _, c := range p.Changes {
		fmt.Printf("  %-10s %-8s %s", c.Action, c.Target, c.Name)
		if c.Message != "" {
			fmt.Printf(" — %s", c.Message)
		}
		fmt.Println()
	}
	if len(p.Changes) == 0 {
		fmt.Println("No MCP servers configured. Run 'skillshare mcp add' to get started.")
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
		fmt.Printf("Backup: %s\n", id)
	}
	if result.Plan == nil {
		fmt.Println("MCP source saved. Run 'skillshare sync mcp' when ready.")
	} else if !result.Plan.Blocked {
		fmt.Printf("MCP files applied: %d. Reload your Agent after synchronization and complete any required login.\n", len(result.Applied))
	}
	return nil
}

func logMCPOp(path, command string, start time.Time, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	if logErr := oplog.Write(path, oplog.OpsFile, oplog.NewEntry(command, status, time.Since(start))); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write MCP operation log")
	}
}

func printMCPHelp() {
	fmt.Println(`Usage: skillshare mcp <command> [options]

Commands:
  add [name]        Guided setup, or --url URL / -- command args...
  import [name]     Import --from <client> or --file <JSON/TOML file>
  list              Show per-client sync status
  remove <name>     Remove from source (sync to remove managed native entries)
  restore <id>      Restore MCP entries from a backup

Options:
  --target <client>  Receiving client; repeat for multiple clients
  --from <client>    claude, codex, cursor, vscode, opencode, or grok
  --file <path>      Native configuration file to import
  --url <url>        Streamable HTTP endpoint
  --sync            Save and synchronize (non-interactive default: save only)
  --replace         Replace an existing source entry; on import, also rewrite
                    the imported client's entry when it differs
  --dry-run, -n     Preview without writing
  --json            Machine-readable, credential-free sync results
  --revision <id>   Require the matching preview revision
  --global, -g      Global configuration
  --project, -p     Project configuration

Sync: skillshare sync mcp [--dry-run] [--json] [-g|-p]
Import does not start MCP servers or copy OAuth credentials.`)
}
