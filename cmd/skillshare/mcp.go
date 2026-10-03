package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"skillshare/internal/mcp"
	"skillshare/internal/oplog"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type mcpOptions struct {
	name, url, from, file, revision string
	// toolsAllow and toolsDeny are nil unless their --tools-* flag was given; an empty
	// value clears that part of the tool policy.
	toolsAllow, toolsDeny *string
	// piOptions is nil unless --pi-options was given.
	piOptions                                    mcp.PiOptions
	targets                                      []string
	command                                      []string
	sync, dryRun, json, replace, noTUI, disabled bool
	keepFiles                                    bool
}

// removedMCPFlags were Pi settings until 0.23.0. Naming one says what replaced it rather
// than calling the flag unknown.
var removedMCPFlags = map[string]string{
	"--pi-extension":     "--pi-extension was removed in 0.23.0: Pi always uses its built-in MCP (~/.pi/agent/mcp.json or .pi/mcp.json); drop the flag",
	"--pi-options-prune": "--pi-options-prune was removed in 0.23.0: sync always removes Pi fields Skillshare wrote earlier that are unchanged; drop the flag",
	"--direct-tools":     "--direct-tools was removed in 0.23.0: use --pi-options '{\"exposure\":\"direct\"}' for every tool, or --pi-options '{\"toolExposure\":{\"TOOL\":\"direct\"}}' for single tools in Pi",
}

func parseMCPOptions(args []string) (mcpOptions, error) {
	var o mcpOptions
	for i := 0; i < len(args); i++ {
		flag, _, _ := strings.Cut(args[i], "=")
		if message, removed := removedMCPFlags[flag]; removed {
			return o, errors.New(message)
		}
		switch a := args[i]; a {
		case "--":
			o.command = args[i+1:]
			i = len(args)
		case "--url", "--target", "--from", "--file", "--revision", "--tools-allow", "--tools-deny", "--pi-options":
			if i+1 == len(args) {
				return o, fmt.Errorf("%s requires a value", a)
			}
			i++
			value := args[i]
			switch a {
			case "--tools-allow":
				o.toolsAllow = &value
			case "--tools-deny":
				o.toolsDeny = &value
			case "--pi-options":
				if err := json.Unmarshal([]byte(value), &o.piOptions); err != nil || o.piOptions == nil {
					return o, fmt.Errorf("--pi-options takes a JSON object, such as '{\"timeout\":30}'")
				}
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
		case "--disabled":
			o.disabled = true
		case "--keep-files":
			o.keepFiles = true
		default:
			if strings.HasPrefix(a, "-") || o.name != "" {
				return o, fmt.Errorf("unknown MCP argument %q", a)
			}
			o.name = a
		}
	}
	// none is an empty list: the server stays in Skillshare and no Agent receives it.
	if slices.Contains(o.targets, "none") {
		if len(o.targets) > 1 {
			return o, fmt.Errorf("--target none cannot be combined with a client")
		}
		o.targets = []string{}
	}
	return o, nil
}

// toolFlags reports whether any --tools-* flag was given.
func (o mcpOptions) toolFlags() bool {
	return o.toolsAllow != nil || o.toolsDeny != nil
}

// applyToolFlags sets the parts of a tool policy that --tools-* flags name. Lists are
// comma-separated; an empty value clears the part.
func (o mcpOptions) applyToolFlags(t mcp.ToolPolicy) mcp.ToolPolicy {
	list := func(value string) []string {
		var tools []string
		for _, tool := range strings.Split(value, ",") {
			if tool = strings.TrimSpace(tool); tool != "" {
				tools = append(tools, tool)
			}
		}
		return tools
	}
	if o.toolsAllow != nil {
		t.Allow = list(*o.toolsAllow)
	}
	if o.toolsDeny != nil {
		t.Deny = list(*o.toolsDeny)
	}
	return t
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
	if sub == "check" {
		return runMCPCheck(service, rest)
	}
	o, err := parseMCPOptions(rest)
	if err != nil {
		return err
	}
	if o.disabled && sub != "add" {
		return fmt.Errorf("--disabled only applies to mcp add")
	}
	if o.keepFiles && (sub != "remove" || o.sync) {
		return fmt.Errorf("--keep-files only applies to mcp remove without --sync: it leaves Agent files as they are")
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
		if o.json {
			return printMCPPlan(p, true)
		}
		source, err := mcp.LoadSource(service.ConfigPath)
		if err != nil {
			return err
		}
		// The plan lists what a sync would touch, which leaves out a server no Agent receives.
		parked := mcpServersWithoutTargets(source, p)
		if len(parked) == 0 || len(p.Changes) > 0 {
			if err := printMCPPlan(p, false); err != nil {
				return err
			}
		} else {
			printMCPSource(p.SourcePath)
		}
		names := make([]string, len(parked))
		for i, server := range parked {
			names[i] = server[0]
		}
		width := ui.RowWidth(names...)
		for _, server := range parked {
			ui.Row(ui.MarkNone, server[0], "kept  "+ui.DimText(server[1]), width)
		}
		return nil
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
			return fmt.Errorf("usage: skillshare mcp remove <name> [--sync | --keep-files]")
		}
		return finishMCPMutation(service, mcp.Mutation{Name: o.name, Remove: true, Unmanage: o.keepFiles}, o, start)
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
	if o.piOptions != nil || o.toolFlags() || o.name != "" || o.url != "" || o.from != "" || o.file != "" || len(o.command) > 0 || o.targets != nil || o.replace || o.sync || o.disabled || o.keepFiles {
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

// mcpServersWithoutTargets lists the servers kept in Skillshare only, as a name and a note
// that adds a project's root. One that a sync still has to remove is already in the plan.
func mcpServersWithoutTargets(source *mcp.Source, p *mcp.Plan) [][2]string {
	planned := map[string]bool{}
	for _, c := range p.Changes {
		planned[c.Root+"\x00"+c.Name] = true
	}
	var names [][2]string
	add := func(root string, servers map[string]mcp.Server) {
		for _, name := range slices.Sorted(maps.Keys(servers)) {
			if server := servers[name]; server.Targets != nil && len(server.Targets) == 0 && !planned[root+"\x00"+name] {
				note := "no targets"
				if root != "" {
					note += " (" + root + ")"
				}
				names = append(names, [2]string{name, note})
			}
		}
	}
	add("", source.Servers)
	for _, root := range slices.Sorted(maps.Keys(source.Projects)) {
		add(root, source.Projects[root].Servers)
	}
	return names
}

func printMCPPlan(p *mcp.Plan, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(p)
	}
	printMCPSource(p.SourcePath)
	for _, notice := range p.Notices {
		ui.Warning("%s", notice)
	}
	// mcp.projects puts one server into several roots; name the file only then.
	seen := map[string]int{}
	names := make([]string, len(p.Changes))
	for i, c := range p.Changes {
		seen[c.Name+"\x00"+c.Target]++
		names[i] = c.Name
	}
	width := ui.RowWidth(names...)
	for _, c := range p.Changes {
		detail := c.Target
		if seen[c.Name+"\x00"+c.Target] > 1 {
			detail += " (" + c.Path + ")"
		}
		if c.Message != "" {
			detail += " — " + c.Message
		}
		mark := ui.MarkNone
		if c.Action == "conflict" {
			mark = ui.MarkFail
		}
		ui.Row(mark, c.Name, c.Action+"  "+ui.DimText(detail), width)
	}
	if len(p.Changes) == 0 {
		fmt.Println()
		ui.Done(ui.MarkNone, "No MCP servers configured", 0)
		ui.Next("skillshare mcp add", "add one")
	}
	return nil
}

// printMCPSource names the MCP source file above what follows.
func printMCPSource(path string) {
	fmt.Println(theme.Primary().Bold(true).Render("MCP source") + "  " + utils.FoldHomePath(path))
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
		ui.Note("backup " + id)
	}
	printMCPMigrated(result)
	if result.Plan == nil {
		if len(result.BackupIDs) > 0 || len(result.Migrated) > 0 {
			fmt.Println()
		}
		ui.Done(ui.MarkOK, "Saved the MCP source", 0)
		ui.Next("skillshare sync mcp", "write it into Agent files")
	} else if !result.Plan.Blocked {
		fmt.Println()
		ui.Done(ui.MarkOK, "Applied "+plural(len(result.Applied), "MCP file"), 0)
		ui.Note("Reload your Agent and complete any required login")
	}
	return nil
}

// printMCPMigrated says the sync also saved the config without the settings 0.23.0 retired.
func printMCPMigrated(result *mcp.Result) {
	for _, file := range result.Migrated {
		ui.Note(fmt.Sprintf("Updated %s for 0.23.0 (backup: %s)", filepath.Base(file.Path), file.Backup))
	}
}

func logMCPOp(path, command string, start time.Time, err error) {
	if logErr := oplog.Write(path, oplog.OpsFile, oplog.NewEntry(command, statusFromErr(err), time.Since(start))); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write MCP operation log")
	}
}

func printMCPHelp() {
	printHelp("skillshare mcp [command] [options]",
		"With no command, opens the MCP manager in a terminal; otherwise prints status.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"list", "Browse connections and per-client sync status (default)"},
			{"add [name]", "Guided setup, or --url URL / -- command args..."},
			{"edit [name]", "Interactive editor, or update --url / --target / -- command"},
			{"import [name]", "Import --from <client> or --file <JSON/TOML/YAML file>"},
			{"check [name...]", "Verify variables, commands, hosts and sync state (--no-dns);\n--live also starts or calls each server [--timeout 10s]"},
			{"remove [name]", "Select and remove a source entry; optionally sync removal,\nor stop managing it and keep its Agent entries (--keep-files)"},
			{"restore [id]", "Browse backups, preview and restore Agent entries"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--target <client>", "Receiving client; repeat for several, or none to keep the\nserver in Skillshare without writing it to any Agent"},
			{"--from <client>", "Native client ID or account target (see mcp documentation)"},
			{"--file <path>", "Native configuration file to import"},
			{"--url <url>", "Streamable HTTP endpoint"},
			{"--tools-allow <tools>", "Only these tools, separated by commas; * matches any\ncharacters (\"\" clears)"},
			{"--tools-deny <tools>", "Never these tools, separated by commas; beats allow (\"\" clears).\nThe plan names each Agent that cannot hold a part of the policy"},
			{"--pi-options <json>", "Other Pi built-in per-server fields as a JSON object"},
			{"--disabled", "Project mode: turn off a server from the Agent's global config\n(add NAME --disabled --target opencode; claude, opencode, kilocode)"},
			{"--sync", "Save and synchronize (non-interactive default: save only)"},
			{"--keep-files", "remove only: stop managing the server; its Agent entries stay\nand sync no longer removes or updates them"},
			{"--replace", "Replace an existing source entry; on import, also rewrite\nthe imported client's entry when it differs"},
			{"-n, --dry-run", "Preview without writing"},
			{"--json", "Machine-readable, credential-free sync results"},
			{"--no-tui", "Disable interactive menus (also honors tui: false)"},
			{"--revision <id>", "Require the matching preview revision"},
			{"-g, --global", "Global configuration"},
			{"-p, --project", "Project configuration"},
		}},
		helpNotes("Notes",
			"In a terminal, mcp opens a manager; its keys are at the bottom of the screen.",
			"JSON and non-TTY output never open a TUI. Scripted edits require a name.",
			"Sync: skillshare sync mcp [--dry-run] [--json] [--no-tui] [-g|-p]",
			"Import does not start MCP servers or copy OAuth credentials.",
		),
	)
}
