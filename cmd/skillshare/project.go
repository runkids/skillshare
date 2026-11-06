package main

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"skillshare/internal/config"
	ssync "skillshare/internal/sync"
)

var projectValueFlags = []string{"--target", "--name", "--skills-mode", "--skills-naming", "--skills-include", "--skills-exclude", "--agents-mode", "--agents-include", "--agents-exclude"}

type projectOptions struct {
	action, token           string
	json, dryRun, yes, sync bool
	machineLocal, shared    bool
	values                  map[string][]string
	toggles                 map[string]bool
}

// projectJSONRequested respects value tokens and the operand terminator, even
// when the final parser will reject a different argument.
func projectJSONRequested(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		if args[i] == "--json" {
			return true
		}
		if slices.Contains(projectValueFlags, args[i]) && i+1 < len(args) {
			i++
		}
	}
	return false
}

// Read-only, preview and JSON invocations do not run startup maintenance.
func projectSkipsMaintenance(args []string) bool {
	o, err := parseProjectArgs(args)
	if err != nil || o.json || o.dryRun || slices.Contains([]string{"help", "list", "show"}, o.action) {
		return true
	}
	if o.action == "edit" && len(o.values) == 0 && len(o.toggles) == 0 && !o.sync {
		return true
	}
	for _, flag := range []string{"--skills-mode", "--agents-mode"} {
		for _, mode := range o.values[flag] {
			if mode != "" && !config.IsValidSyncMode(mode) {
				return true
			}
		}
	}
	return false
}

func cmdProject(args []string) error {
	jsonMode := projectJSONRequested(args)
	opts, err := parseProjectArgs(args)
	if err != nil {
		if jsonMode {
			return writeJSONError(err)
		}
		return err
	}
	if opts.action == "help" {
		printProjectHelp()
		return nil
	}
	if opts.json {
		previous := ssync.DiagOutput
		ssync.DiagOutput = io.Discard
		defer func() { ssync.DiagOutput = previous }()
	}
	err = runProject(opts)
	if err != nil && opts.json {
		if _, ok := err.(*jsonSilentError); !ok {
			return writeJSONError(err)
		}
	}
	return err
}

func parseProjectArgs(args []string) (projectOptions, error) {
	o := projectOptions{json: projectJSONRequested(args), values: map[string][]string{}, toggles: map[string]bool{}}
	mode, rest, err := parseModeArgs(args, projectValueFlags...)
	if err != nil {
		return o, err
	}
	if mode == modeProject {
		return o, fmt.Errorf("projects belong in the global config")
	}
	var operands []string
	literal := false
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if literal {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			literal = true
			continue
		}
		flag, value, equal := strings.Cut(arg, "=")
		if slices.Contains(projectValueFlags, flag) {
			if !equal {
				if i+1 >= len(rest) {
					return o, fmt.Errorf("%s requires a value", flag)
				}
				i++
				value = rest[i]
			}
			o.values[flag] = append(o.values[flag], value)
			continue
		}
		switch arg {
		case "--help", "-h":
			o.action = "help"
			return o, nil
		case "help":
			if len(operands) == 0 {
				o.action = "help"
				return o, nil
			}
			operands = append(operands, arg)
		case "--json":
			o.json = true
		case "--dry-run", "-n":
			o.dryRun = true
		case "--yes":
			o.yes = true
		case "--sync":
			o.sync = true
		case "--machine-local":
			o.machineLocal = true
		case "--shared":
			o.shared = true
		case "--skills", "--no-skills", "--agents", "--no-agents":
			o.toggles[arg] = true
		default:
			if strings.HasPrefix(arg, "-") {
				return o, fmt.Errorf("unknown flag: %s", arg)
			}
			operands = append(operands, arg)
		}
	}
	o.action = "list"
	if len(operands) > 0 {
		o.action = operands[0]
	}
	if !slices.Contains([]string{"list", "show", "add", "edit", "sync", "remove", "localize", "share"}, o.action) {
		return o, fmt.Errorf("unknown project command: %s", o.action)
	}
	want := 2
	if o.action == "list" {
		want = 1
	}
	if want == 1 && len(operands) > 1 {
		return o, fmt.Errorf("project list takes no project argument")
	}
	if want == 2 && len(operands) != 2 {
		return o, fmt.Errorf("project %s requires exactly one project argument", o.action)
	}
	if want == 2 {
		o.token = operands[1]
	}
	if o.action != "add" && o.action != "edit" && (len(o.values) > 0 || len(o.toggles) > 0 || o.sync) {
		return o, fmt.Errorf("settings and --sync require project add or edit")
	}
	if o.yes && o.action != "remove" && o.action != "share" {
		return o, fmt.Errorf("--yes requires project remove or share")
	}
	if o.machineLocal && o.action != "add" {
		return o, fmt.Errorf("--machine-local requires project add; use project localize for a saved project")
	}
	if o.shared && o.action != "add" && o.action != "remove" {
		return o, fmt.Errorf("--shared requires project add or remove; use project share for a saved project")
	}
	if o.machineLocal && o.shared {
		return o, fmt.Errorf("--machine-local and --shared are contradictory")
	}
	if o.dryRun && (o.action == "list" || o.action == "show") {
		return o, fmt.Errorf("--dry-run requires project add, edit, sync, remove, localize or share")
	}
	for _, kind := range []string{"skills", "agents"} {
		if !o.toggles["--no-"+kind] {
			continue
		}
		if o.toggles["--"+kind] {
			return o, fmt.Errorf("--%s and --no-%s are contradictory", kind, kind)
		}
		for flag := range o.values {
			if strings.HasPrefix(flag, "--"+kind+"-") {
				return o, fmt.Errorf("--no-%s and %s are contradictory", kind, flag)
			}
		}
	}
	if o.action == "add" && len(o.values["--target"]) == 0 {
		return o, fmt.Errorf("project add requires --target")
	}
	if o.action == "remove" && o.json && !o.yes && !o.dryRun {
		return o, fmt.Errorf("project remove --json requires --yes")
	}
	return o, nil
}

func printProjectHelp() {
	printHelp("skillshare project [list|show|add|edit|sync|remove|localize|share] [project] [options]", "Manage the project folders the global config syncs to. A project is saved in config.yaml unless it is kept on this machine only. No project config is changed.",
		helpGroup{title: "Commands", rows: []helpRow{
			{"list", "List registered projects (default)"}, {"show <project>", "Show one project by name or root"}, {"add <root> --target T", "Add an existing project folder"}, {"edit <project>", "Change only the specified settings, where they are saved"}, {"localize <project>", "Keep a copy on this machine only; config.yaml stays"}, {"share <project>", "Move a machine-only project into config.yaml"}, {"sync <project>", "Sync skills and agents to this project"}, {"remove <project>", "Detach skills and remove the project; agents stay"},
		}},
		helpGroup{title: "Options", rows: []helpRow{
			{"--target T", "Replace targets; repeat or use a comma list"}, {"--name N", "Set name; --name= restores the basename"}, {"--skills, --no-skills", "Turn skills on (add default) or off"}, {"--agents, --no-agents", "Turn agents on or off (add default)"}, {"--skills-mode, --agents-mode MODE", "merge, symlink or copy; empty inherits"}, {"--skills-naming NAMING", "flat, standard or prefixed; empty inherits"}, {"--skills-include, --skills-exclude P", "Replace filter list; repeat; empty clears"}, {"--agents-include, --agents-exclude P", "Replace filter list; repeat; empty clears"}, {"--machine-local", "Add: save on this machine only, not in config.yaml"}, {"--shared", "Add: save in config.yaml despite projects_storage; remove: also remove config.yaml's copy"}, {"--sync", "Sync after add/edit; force is off"}, {"-n, --dry-run", "Preview without writing or prompting"}, {"--yes", "Skip the remove or share confirmation (remove --json requires it)"}, {"--json", "Output JSON"}, {"-g, --global", "Use the global config (always)"},
		}},
	)
}
