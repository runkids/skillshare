package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/hooks"
)

type hooksOptions struct {
	name, file, from, revision  string
	sync, dryRun, json, replace bool
	keepFiles                   bool
}

func parseHooksOptions(args []string) (hooksOptions, error) {
	var o hooksOptions
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--file", "--from", "--revision":
			if i+1 == len(args) {
				return o, fmt.Errorf("%s requires a value", a)
			}
			i++
			switch a {
			case "--file":
				o.file = args[i]
			case "--from":
				o.from = args[i]
			case "--revision":
				o.revision = args[i]
			}
		case "--sync":
			o.sync = true
		case "--dry-run", "-n":
			o.dryRun = true
		case "--json":
			o.json = true
		case "--replace":
			o.replace = true
		case "--keep-files":
			o.keepFiles = true
		default:
			if strings.HasPrefix(a, "-") || o.name != "" {
				return o, fmt.Errorf("unknown hooks argument %q", a)
			}
			o.name = a
		}
	}
	return o, nil
}

func hooksContext(args []string) (*hooks.Service, []string, error) {
	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return nil, nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	if mode == modeAuto {
		mode = modeGlobal
		if projectConfigExists(cwd) {
			mode = modeProject
		}
	}
	service := &hooks.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir(), ConfigDirs: hooks.ConfigDirsFromEnv()}
	if mode == modeProject {
		service.ConfigPath = config.ProjectConfigPath(cwd)
		service.ProjectRoot = cwd
	} else if cfg, err := config.Load(); err == nil {
		service.Accounts = hooksAccounts(cfg)
	}
	applyModeLabel(mode)
	return service, rest, nil
}

func cmdHooks(args []string) error {
	if wantsHelp(args) {
		printHooksHelp()
		return nil
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		args = append([]string{"list"}, args...)
	}
	sub, args := args[0], args[1:]
	service, rest, err := hooksContext(args)
	if err != nil {
		return err
	}
	o, err := parseHooksOptions(rest)
	if err != nil {
		return err
	}
	if err := checkHooksFlags(sub, o); err != nil {
		return err
	}
	return hooksJSONError(runHooks(service, sub, o), o)
}

// cmdSyncHooks is "skillshare sync hooks", the same as "skillshare hooks sync".
func cmdSyncHooks(args []string) error {
	if wantsHelp(args) {
		printHooksHelp()
		return nil
	}
	service, rest, err := hooksContext(args)
	if err != nil {
		return err
	}
	o, err := parseHooksOptions(rest)
	if err != nil {
		return err
	}
	if o.name != "" || o.file != "" || o.from != "" || o.sync || o.replace || o.keepFiles {
		return fmt.Errorf("sync hooks accepts only --dry-run, --json, --revision and scope flags; use 'skillshare hooks sync <name> --replace' to take over conflicts")
	}
	return hooksJSONError(runHooks(service, "sync", o), o)
}

// hooksJSONError keeps --json failures machine-readable.
func hooksJSONError(err error, o hooksOptions) error {
	var silent *jsonSilentError
	if err != nil && o.json && !errors.As(err, &silent) {
		return writeJSONError(err)
	}
	return err
}

// checkHooksFlags rejects flags a subcommand would otherwise ignore silently.
func checkHooksFlags(sub string, o hooksOptions) error {
	allowed := map[string]string{
		"list":    "",
		"add":     "name file sync replace",
		"edit":    "name file sync replace",
		"import":  "name file from sync replace",
		"enable":  "name sync replace",
		"disable": "name sync replace",
		"remove":  "name sync replace",
		"sync":    "name replace",
		"restore": "name",
	}
	spec, ok := allowed[sub]
	if !ok {
		return fmt.Errorf("unknown hooks command %q; run skillshare hooks --help", sub)
	}
	used := map[string]bool{"name": o.name != "", "file": o.file != "", "from": o.from != "", "sync": o.sync, "replace": o.replace}
	for flag, set := range used {
		if set && !strings.Contains(" "+spec+" ", " "+flag+" ") {
			if flag == "name" {
				return fmt.Errorf("hooks %s takes no name", sub)
			}
			return fmt.Errorf("--%s does not apply to hooks %s", flag, sub)
		}
	}
	if o.keepFiles && (sub != "remove" || o.sync) {
		return fmt.Errorf("--keep-files only applies to hooks remove without --sync: it leaves Agent files as they are")
	}
	if sub == "sync" && o.name != "" && !o.replace {
		return fmt.Errorf("hooks sync takes a name only with --replace, to take over that hook's conflicting Agent entries")
	}
	if sub == "list" && (o.dryRun || o.revision != "") {
		return fmt.Errorf("hooks list accepts only --json and scope flags")
	}
	return nil
}

func printHooksHelp() {
	fmt.Println(`Usage: skillshare hooks [command] [options]

Commands:
  list              Show hook entries, their Agents and sync status (default)
  add <name>        Add an entry from --file (Entry JSON or YAML)
  edit <name>       Replace an existing entry's definition from --file
  import            Read native hooks with --from <agent> (or --file <native file>);
                    list candidates, or save one with a name
  enable <name>     Publish a disabled entry again on the next sync
  disable <name>    Keep the definition; the next sync removes its owned outputs
  remove <name>     Delete the entry; the next sync prunes its owned outputs,
                    or stop managing it and keep its Agent entries (--keep-files)
  sync [name]       Write hooks to each Agent's native configuration; with a name
                    and --replace, take over that hook's conflicting Agent entries
  restore <id>      Preview and restore an Agent file from a hooks backup

Options:
  Account targets (agent + config_dir under targets) are hook targets in global mode.
  --file <path>     Entry document for add/edit, or native file for import
  --from <agent>    Agent or account target to import from, such as claude, codex or codex-2
  --sync            Save and synchronize (default: save the source only)
  --replace         Replace an existing entry, or take over its conflicting
                    Agent entries (unmanaged duplicates or outside edits)
  --keep-files      remove only: stop managing the hook; its Agent entries stay
  --dry-run, -n     Preview without writing
  --json            Machine-readable output
  --revision <id>   Require the matching preview revision
  --global, -g      Global configuration
  --project, -p     Project configuration

Sync alias: skillshare sync hooks [--dry-run] [--json] [-g|-p]; sync --all includes hooks.
Import reads configuration and code only; it never runs hook commands.`)
}

// hooksAccounts are the global targets with their own native config homes.
func hooksAccounts(cfg *config.Config) map[string]hooks.Account {
	accounts := map[string]hooks.Account{}
	for name, target := range cfg.Targets {
		if target.Agent != "" && target.ConfigDir != "" {
			accounts[name] = hooks.Account{Agent: target.Agent, Dir: target.ConfigDir}
		}
	}
	return accounts
}
