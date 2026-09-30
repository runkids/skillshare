package main

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"skillshare/internal/hooks"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func runHooks(service *hooks.Service, sub string, o hooksOptions) error {
	switch sub {
	case "list":
		return runHooksList(service, o)
	case "add", "edit":
		return runHooksSave(service, sub, o)
	case "import":
		return runHooksImport(service, o)
	case "enable", "disable":
		return runHooksToggle(service, sub == "enable", o)
	case "remove":
		if o.name == "" {
			return fmt.Errorf("usage: skillshare hooks remove <name> [--sync]")
		}
		if _, err := hooksEntry(service, o.name); err != nil {
			return err
		}
		return finishHooksMutation(service, "hooks remove", hooks.Mutation{Name: o.name, Remove: true, Replace: o.replace}, o)
	case "sync":
		o.sync = true
		// A name only scopes --replace; the sync itself covers every entry.
		return finishHooksMutation(service, "hooks sync", hooks.Mutation{Name: o.name, Replace: o.replace}, o)
	case "restore":
		return runHooksRestore(service, o)
	}
	return fmt.Errorf("unknown hooks command %q; run skillshare hooks --help", sub)
}

func runHooksList(service *hooks.Service, o hooksOptions) error {
	inv, err := service.List()
	if err != nil {
		return err
	}
	if o.json {
		return writeJSON(inv)
	}
	ui.Info("Hooks source: %s", inv.Source.Path)
	for _, name := range slices.Sorted(maps.Keys(inv.Source.Entries)) {
		entry := inv.Source.Entries[name]
		state := "enabled"
		if entry.Enabled != nil && !*entry.Enabled {
			state = "disabled"
		}
		agents := strings.Join(slices.Sorted(maps.Keys(entry.Bindings)), ", ")
		if agents == "" {
			agents = "no Agents"
		}
		ui.Status(name, state, agents)
	}
	if len(inv.Source.Entries) == 0 {
		ui.Info("No hooks configured. Run 'skillshare hooks add <name> --file <entry.yaml>' to get started.")
	}
	for _, u := range inv.Unmanaged {
		ui.Status(u.Target, "not managed", u.Path+": "+strings.Join(u.Names, ", "))
	}
	if inv.PreviewError != "" {
		ui.Warning("Sync preview failed: %s", inv.PreviewError)
	} else if inv.Plan != nil && len(inv.Plan.Changes) > 0 {
		ui.Info("Pending sync changes:")
		printHooksChanges(inv.Plan)
	}
	return nil
}

func runHooksSave(service *hooks.Service, sub string, o hooksOptions) error {
	if o.name == "" || o.file == "" {
		return fmt.Errorf("usage: skillshare hooks %s <name> --file <entry.json|entry.yaml> [--sync]", sub)
	}
	data, err := os.ReadFile(o.file)
	if err != nil {
		return err
	}
	entry, err := hooks.ParseEntry(data)
	if err != nil {
		return fmt.Errorf("%s: %w", o.file, err)
	}
	_, lookupErr := hooksEntry(service, o.name)
	switch {
	case sub == "edit" && lookupErr != nil:
		return lookupErr
	case sub == "add" && lookupErr == nil && !o.replace:
		return fmt.Errorf("hook %s already exists; use 'skillshare hooks edit %s --file %s' or --replace", o.name, o.name, o.file)
	}
	return finishHooksMutation(service, "hooks "+sub, hooks.Mutation{Name: o.name, Entry: &entry, Replace: o.replace}, o)
}

func runHooksToggle(service *hooks.Service, enabled bool, o hooksOptions) error {
	command := "hooks disable"
	if enabled {
		command = "hooks enable"
	}
	if o.name == "" {
		return fmt.Errorf("usage: skillshare %s <name> [--sync]", command)
	}
	entry, err := hooksEntry(service, o.name)
	if err != nil {
		return err
	}
	entry.Enabled = &enabled
	return finishHooksMutation(service, command, hooks.Mutation{Name: o.name, Entry: &entry, Replace: o.replace}, o)
}

func runHooksImport(service *hooks.Service, o hooksOptions) error {
	if o.from == "" {
		return fmt.Errorf("usage: skillshare hooks import --from <agent> [--file <native file>] [name] [--dry-run]")
	}
	request := hooks.ImportRequest{From: o.from, Name: o.name}
	if o.file != "" {
		data, err := os.ReadFile(o.file)
		if err != nil {
			return err
		}
		request.Content = string(data)
	}
	candidates, err := service.Import(request)
	if err != nil {
		return err
	}
	if o.name == "" {
		return printHooksCandidates(candidates, o.json)
	}
	for _, c := range candidates {
		if c.Name != o.name {
			continue
		}
		if len(c.Problems) > 0 {
			return fmt.Errorf("cannot import %s: %s", c.Name, strings.Join(c.Problems, "; "))
		}
		if !o.json {
			for _, w := range c.Warnings {
				ui.Warning("%s", w)
			}
		}
		if _, err := hooksEntry(service, c.Name); err == nil && !o.replace {
			return fmt.Errorf("hook %s already exists; use --replace to overwrite its definition", c.Name)
		}
		// Importing takes over the registrations it read, so they are not reported as
		// unmanaged duplicates on the next sync.
		return finishHooksMutation(service, "hooks import", hooks.Mutation{Name: c.Name, Entry: &c.Entry, Replace: o.replace, Adopt: true}, o)
	}
	return fmt.Errorf("hook %q not found in %s", o.name, o.from)
}

func runHooksRestore(service *hooks.Service, o hooksOptions) error {
	if o.name == "" {
		return fmt.Errorf("usage: skillshare hooks restore <backup-id> [--dry-run]; 'skillshare hooks list --json' shows backups")
	}
	p, err := service.PreviewRestore(o.name)
	if err != nil {
		return err
	}
	o.sync = true
	if err := checkHooksPreview(p, o); err != nil || o.dryRun {
		return err
	}
	start := time.Now()
	result, err := service.Restore(o.name, p.Revision)
	logHooksOp(service.ConfigPath, "hooks restore", start, err)
	return printHooksResult(result, err, o)
}

// finishHooksMutation previews first and applies with that preview's revision, so
// native files edited in between are reported instead of being overwritten.
func finishHooksMutation(service *hooks.Service, command string, mutation hooks.Mutation, o hooksOptions) error {
	p, err := service.PreviewMutation(mutation)
	if err != nil {
		return err
	}
	if err := checkHooksPreview(p, o); err != nil || o.dryRun {
		return err
	}
	start := time.Now()
	result, err := service.Mutate(mutation, p.Revision, o.sync)
	logHooksOp(service.ConfigPath, command, start, err)
	return printHooksResult(result, err, o)
}

// applyHooksSync previews the whole source and applies with that revision, so sync
// --all never writes past a conflict that appeared after its preflight.
func applyHooksSync(service *hooks.Service, start time.Time) (*hooks.Result, error) {
	p, err := service.Preview()
	if err != nil {
		return nil, err
	}
	result := &hooks.Result{Plan: p, Applied: []string{}, BackupIDs: []string{}}
	if p.Blocked {
		err = hooks.ErrConflict
	} else {
		result, err = service.Mutate(hooks.Mutation{}, p.Revision, true)
	}
	logHooksOp(service.ConfigPath, "sync hooks", start, err)
	return result, err
}

// checkHooksPreview prints a dry-run or blocked plan and enforces --revision. Conflicts
// block only writes to Agent files; saving the source alone always proceeds.
func checkHooksPreview(p *hooks.Plan, o hooksOptions) error {
	if o.revision != "" && o.revision != p.Revision {
		return hooks.ErrStaleRevision
	}
	blocked := error(nil)
	if o.sync && p.Blocked {
		blocked = hooks.ErrConflict
	}
	if !o.dryRun && blocked == nil {
		return nil
	}
	if o.json {
		return writeJSONResult(p, blocked)
	}
	printHooksPlan(p)
	return blocked
}

func hooksEntry(service *hooks.Service, name string) (hooks.Entry, error) {
	inv, err := service.List()
	if err != nil {
		return hooks.Entry{}, err
	}
	entry, ok := inv.Source.Entries[name]
	if !ok {
		return entry, fmt.Errorf("hook %s not found; run 'skillshare hooks list'", name)
	}
	return entry, nil
}

func printHooksPlan(p *hooks.Plan) {
	ui.Info("Hooks source: %s", p.SourcePath)
	printHooksChanges(p)
	if len(p.Changes) == 0 {
		ui.Info("No hook changes.")
	}
}

func printHooksChanges(p *hooks.Plan) {
	home, _ := os.UserHomeDir()
	conflicts := []string{}
	for _, c := range p.Changes {
		detail := c.Target + "  " + hooksDisplayPath(c.Path, home)
		if c.Events != nil {
			detail += "  " + hooksEventDetail(c.Events)
		}
		if c.Message != "" {
			detail += " — " + c.Message
		}
		ui.Status(c.Name, c.Action, detail)
		if c.Action == "conflict" && c.Name != "" && c.Root == "" && !slices.Contains(conflicts, c.Name) {
			conflicts = append(conflicts, c.Name)
		}
	}
	for _, w := range p.Warnings {
		ui.Warning("%s", w)
	}
	for _, name := range conflicts {
		ui.Info("To take over %s's conflicting Agent entries: skillshare hooks sync %s --replace", name, name)
	}
}

// hooksDisplayPath shortens a path under the home directory to ~/...
func hooksDisplayPath(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

// hooksEventDetail renders events added (+), updated (~) and removed (−).
func hooksEventDetail(e *hooks.EventChanges) string {
	var parts []string
	for _, group := range []struct {
		mark   string
		events []string
	}{{"+", e.Added}, {"~", e.Updated}, {"−", e.Removed}} {
		if len(group.events) > 0 {
			parts = append(parts, group.mark+" "+strings.Join(group.events, ", "))
		}
	}
	return strings.Join(parts, "  ")
}

func printHooksResult(result *hooks.Result, err error, o hooksOptions) error {
	if o.json {
		if result == nil {
			return err
		}
		return writeJSONResult(result, err)
	}
	if result != nil {
		if result.Plan != nil {
			printHooksPlan(result.Plan)
		}
		for _, id := range result.BackupIDs {
			ui.Info("Backup: %s", id)
		}
		if err == nil && !o.sync {
			ui.Success("Hooks source saved. Run 'skillshare hooks sync' when ready.")
		} else if err == nil {
			ui.Success("Hook files applied: %d. Restart or reload the Agent to load them.", len(result.Applied))
		}
	}
	return err
}

func printHooksCandidates(candidates []hooks.Candidate, asJSON bool) error {
	if asJSON {
		return writeJSON(candidates)
	}
	for _, c := range candidates {
		switch {
		case len(c.Problems) > 0:
			ui.Status(c.Name, "blocked", strings.Join(c.Problems, "; "))
		case len(c.Warnings) > 0:
			ui.Status(c.Name, "importable", strings.Join(c.Warnings, "; "))
		default:
			ui.Status(c.Name, "importable", "")
		}
	}
	if len(candidates) == 0 {
		ui.Info("No hooks found to import.")
	} else {
		ui.Info("Import one with 'skillshare hooks import --from <agent> <name>' (add --dry-run to preview).")
	}
	return nil
}

func logHooksOp(path, command string, start time.Time, err error) {
	if logErr := oplog.Write(path, oplog.OpsFile, oplog.NewEntry(command, statusFromErr(err), time.Since(start))); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write hooks operation log")
	}
}
