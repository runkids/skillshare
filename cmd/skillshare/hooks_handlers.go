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
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
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
			return fmt.Errorf("usage: skillshare hooks remove <name> [--sync | --keep-files]")
		}
		if _, err := hooksEntry(service, o.name); err != nil {
			return err
		}
		return finishHooksMutation(service, "hooks remove", hooks.Mutation{Name: o.name, Remove: true, Replace: o.replace, Unmanage: o.keepFiles}, o)
	case "sync":
		o.sync = true
		// Names only scope --replace; the sync itself covers every entry.
		return finishHooksMutation(service, "hooks sync", hooks.Mutation{ReplaceNames: o.names}, o)
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
	printHooksSource(inv.Source.Path)
	var labels []string
	labels = append(labels, slices.Collect(maps.Keys(inv.Source.Entries))...)
	for _, u := range inv.Unmanaged {
		labels = append(labels, u.Target)
	}
	width := ui.RowWidth(labels...)
	for _, name := range slices.Sorted(maps.Keys(inv.Source.Entries)) {
		entry := inv.Source.Entries[name]
		agents := strings.Join(slices.Sorted(maps.Keys(entry.Bindings)), ", ")
		if agents == "" {
			agents = "no Agents"
		}
		if entry.Enabled != nil && !*entry.Enabled {
			ui.Row(ui.MarkNone, name, ui.DimText("disabled · "+agents), width)
		} else {
			ui.Row(ui.MarkOK, name, agents, width)
		}
	}
	for _, u := range inv.Unmanaged {
		mark, status := ui.MarkWarn, "not managed "
		if !u.Importable {
			mark, status = ui.MarkNone, "left as is "
		}
		ui.Row(mark, u.Target, status+ui.DimText("· "+utils.FoldHomePath(u.Path)+": "+strings.Join(u.Names, ", ")), width)
	}
	if len(inv.Source.Entries) == 0 {
		fmt.Println()
		ui.Done(ui.MarkNone, "No hooks configured", 0)
		ui.Next("skillshare hooks add <name> --file <entry.yaml>", "add one")
	}
	if inv.PreviewError != "" {
		fmt.Println()
		ui.Warning("Sync preview failed: %s", inv.PreviewError)
	} else if inv.Plan != nil {
		pending := *inv.Plan
		pending.Changes = slices.DeleteFunc(slices.Clone(pending.Changes), func(c hooks.Change) bool {
			return c.Action == "unchanged"
		})
		if len(pending.Changes) > 0 {
			ui.Section("Pending sync")
			printHooksChanges(&pending, nil)
			ui.Next("skillshare hooks sync", "write them into Agent files")
		}
	}
	return nil
}

// printHooksSource names the hooks source file above what follows.
func printHooksSource(path string) {
	fmt.Println(theme.Primary().Bold(true).Render("Hooks source") + "  " + utils.FoldHomePath(path))
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
		adoptSource := ""
		if o.file == "" {
			adoptSource = c.Source
		}
		return finishHooksMutation(service, "hooks import", hooks.Mutation{Name: c.Name, Entry: &c.Entry, Replace: o.replace, Adopt: true, AdoptSource: adoptSource}, o)
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
	if err := checkHooksPreview(p, o, mutation.ReplaceNames...); err != nil || o.dryRun {
		return err
	}
	start := time.Now()
	result, err := service.Mutate(mutation, p.Revision, o.sync)
	logHooksOp(service.ConfigPath, command, start, err)
	if err != nil || !o.trust {
		return printHooksResult(result, err, o)
	}
	// The sync result prints first, so the question comes after what was written.
	if !o.json {
		if err := printHooksResult(result, nil, o); err != nil {
			return err
		}
	}
	trust, declined, err := trustCodexHooks(service, o)
	result.CodexTrust = trust
	if err != nil {
		// A home that was written says so in the error itself.
		if slices.ContainsFunc(trust, func(h hooks.CodexTrust) bool { return h.Action == "trusted" }) {
			err = fmt.Errorf("hook files were written: %w", err)
		} else {
			err = fmt.Errorf("hook files were written, but Codex trust was not recorded: %w", err)
		}
	}
	if o.json {
		if service.ProjectRoot != "" && err != nil {
			result.CodexTrustError = err.Error()
		}
		return writeJSONResult(result, err)
	}
	if !declined {
		printHooksTrust(trust)
	}
	return err
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
func checkHooksPreview(p *hooks.Plan, o hooksOptions, replacing ...string) error {
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
	printHooksPlan(p, replacing...)
	if o.dryRun {
		fmt.Println()
		ui.DryRun()
	}
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

func logHooksOp(path, command string, start time.Time, err error) {
	if logErr := oplog.Write(path, oplog.OpsFile, oplog.NewEntry(command, statusFromErr(err), time.Since(start))); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write hooks operation log")
	}
}
