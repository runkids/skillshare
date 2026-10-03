package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/plugin"
	"skillshare/internal/ui"
)

type pluginOptions struct {
	request             plugin.Request
	json, dryRun, noTUI bool
	revision            string
}

func parsePluginOptions(args []string) (pluginOptions, error) {
	o := pluginOptions{request: plugin.Request{Action: "list"}}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		o.request.Action = args[0]
		args = args[1:]
	}
	positional := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			o.json = true
		case "--dry-run", "-n":
			o.dryRun = true
		case "--no-tui":
			o.noTUI = true
		case "--target", "--from", "--plugin", "--name", "--revision", "--source-ref", "--entry":
			if i+1 == len(args) {
				return o, fmt.Errorf("%s requires a value", a)
			}
			i++
			v := args[i]
			if v == "" || strings.HasPrefix(v, "--") {
				return o, fmt.Errorf("%s requires a value", a)
			}
			switch a {
			case "--target":
				o.request.Targets = append(o.request.Targets, plugin.NormalizeTarget(v))
			case "--from":
				o.request.From = plugin.NormalizeTarget(v)
			case "--plugin":
				o.request.Plugin = v
			case "--name":
				o.request.Name = v
			case "--entry":
				o.request.Entry = v
			case "--source-ref":
				o.request.SourceRef = v
			case "--revision":
				o.revision = v
			}
		default:
			if strings.HasPrefix(a, "-") || positional != "" {
				return o, fmt.Errorf("unknown plugin argument %q", a)
			}
			positional = a
		}
	}
	switch o.request.Action {
	case "add", "discover":
		o.request.Source = positional
	case "import":
		o.request.Plugin = positional
	case "list", "sync", "check", "inspect", "remove", "update", "enable", "disable":
		if positional != "" {
			if o.request.Name != "" {
				return o, fmt.Errorf("specify one package name")
			}
			o.request.Name = positional
		}
	default:
		return o, fmt.Errorf("unknown plugin command %q; run skillshare plugin --help", o.request.Action)
	}
	return o, nil
}

// pluginAccounts are the configured targets that are another config directory of an
// Agent, such as a second Claude account. They are plugin targets of their own.
func pluginAccounts(cfg *config.Config) map[string]plugin.Account {
	accounts := map[string]plugin.Account{}
	for name, target := range cfg.Targets {
		if target.Agent != "" && target.ConfigDir != "" {
			accounts[name] = plugin.Account{Agent: target.Agent, Dir: target.ConfigDir, CLI: target.CLI}
		}
	}
	return accounts
}

func pluginContext(args []string) (*plugin.Service, []string, error) {
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
	s := &plugin.Service{ConfigPath: config.ConfigPath(), StateDir: config.StateDir()}
	if mode == modeProject {
		s.ConfigPath = config.ProjectConfigPath(cwd)
		s.ProjectRoot = cwd
	} else if cfg, err := config.Load(); err == nil {
		s.Accounts = pluginAccounts(cfg)
	}
	applyModeLabel(mode)
	return s, rest, nil
}

func cmdPlugin(args []string) (resultErr error) {
	defer func() {
		if errors.Is(resultErr, errMCPCancelled) {
			resultErr = nil
		}
	}()
	if wantsHelp(args) {
		printPluginHelp()
		return nil
	}
	s, rest, err := pluginContext(args)
	if err != nil {
		return err
	}
	o, err := parsePluginOptions(rest)
	if err != nil {
		return err
	}
	interactive := mcpInteractive(mcpOptions{json: o.json, noTUI: o.noTUI})
	if interactive && !o.dryRun {
		if o.request.Action == "list" {
			return runPluginManager(s)
		}
		if o.request.Action == "add" || o.request.Action == "import" || o.request.Action == "remove" || o.request.Action == "update" || o.request.Action == "enable" || o.request.Action == "disable" || o.request.Action == "sync" {
			return pluginWizard(s, o)
		}
	}
	return executePlugin(s, o)
}

func executePlugin(s *plugin.Service, o pluginOptions) error {
	ctx := context.Background()
	r := o.request
	switch r.Action {
	case "discover":
		d, err := s.Discover(ctx, r.Source, r.SourceRef, r.Entry)
		if err != nil {
			return err
		}
		if o.json {
			return json.NewEncoder(os.Stdout).Encode(d)
		}
		for _, warning := range d.Warnings {
			ui.Warning("%s", warning)
		}
		names := make([]string, len(d.Candidates))
		for i, c := range d.Candidates {
			names[i] = c.Name
		}
		width := ui.RowWidth(names...)
		for _, c := range d.Candidates {
			mark := ui.MarkNone
			if c.Problem != "" {
				mark = ui.MarkFail
			}
			value := strings.Join(c.Targets, ", ")
			if len(c.Components) > 0 {
				value += ui.DimText(" · " + strings.Join(c.Components, ", "))
			}
			ui.Row(mark, c.Name, value, width)
			if c.Problem != "" {
				ui.Note(c.Problem)
			}
		}
		return nil
	case "list", "inspect":
		inventory, err := s.Inventory(ctx)
		if err != nil {
			return err
		}
		if r.Name != "" {
			p, ok := inventory.Packages[r.Name]
			if !ok {
				return fmt.Errorf("plugin package %q not found", r.Name)
			}
			inventory.Packages = map[string]plugin.Package{r.Name: p}
		}
		if o.json {
			return json.NewEncoder(os.Stdout).Encode(inventory)
		}
		width := ui.RowWidth(slices.Collect(maps.Keys(inventory.Packages))...)
		for _, name := range slices.Sorted(maps.Keys(inventory.Packages)) {
			p := inventory.Packages[name]
			if len(p.Bindings) == 0 {
				ui.Row(ui.MarkNone, name, "no targets"+ui.DimText(" · "+p.Source), width)
			}
			for _, definition := range inventory.TargetDefinitions {
				target := definition.Target
				if b, ok := p.Bindings[target]; ok {
					mark, state := ui.MarkWarn, "not installed"
					for _, h := range inventory.Hosts {
						if h.Target != target {
							continue
						}
						if h.Error != "" {
							mark, state = ui.MarkFail, h.Error
						}
						for _, i := range h.Installed {
							if i.ID == b.ID {
								mark, state = ui.MarkOK, "registered (loading unverified)"
								if i.EnabledKnown && !i.Enabled {
									mark, state = ui.MarkNone, "disabled"
								}
							}
						}
					}
					if b.Pending != "" {
						mark, state = ui.MarkWarn, "pending "+b.Pending
					}
					ui.Row(mark, name, target+" · "+state+ui.DimText(" · "+b.ID), width)
				}
			}
		}
		for _, h := range inventory.Hosts {
			if h.Error != "" {
				ui.Warning("%s: %s", h.Target, h.Error)
			}
		}
		if len(inventory.Packages) == 0 {
			fmt.Println()
			ui.Done(ui.MarkNone, "No plugins managed yet", 0)
			ui.Next("skillshare plugin add", "add one", "skillshare plugin import", "take over one an Agent already has")
		}
		return nil
	}
	p, err := s.Preview(ctx, r)
	if err != nil {
		return err
	}
	if o.revision != "" && o.revision != p.Revision {
		return fmt.Errorf("plugin state changed; preview again")
	}
	if o.dryRun || r.Action == "check" {
		if o.json {
			err = json.NewEncoder(os.Stdout).Encode(p)
		} else {
			printPluginPlan(p)
			if o.dryRun {
				fmt.Println()
				ui.DryRun()
			}
		}
		if err != nil {
			return err
		}
		if p.Blocked {
			return fmt.Errorf("plugin changes are blocked")
		}
		return nil
	}
	start := time.Now()
	result, err := s.Apply(ctx, r, p.Revision)
	entry := oplog.NewEntry("plugin "+r.Action, statusFromErr(err), time.Since(start))
	if logErr := oplog.Write(s.ConfigPath, oplog.OpsFile, entry); logErr != nil {
		fmt.Fprintln(os.Stderr, "Warning: could not write plugin operation log")
	}
	if result != nil {
		if o.json {
			if outErr := json.NewEncoder(os.Stdout).Encode(result); outErr != nil {
				return outErr
			}
		} else {
			names := make([]string, len(result.Results))
			for i, r := range result.Results {
				names[i] = r.Name
			}
			width := ui.RowWidth(names...)
			for _, r := range result.Results {
				mark := ui.MarkNone
				switch r.Status {
				case "installed", "saved":
					mark = ui.MarkOK
				case "skipped":
					mark = ui.MarkWarn
				case "failed":
					mark = ui.MarkFail
				}
				ui.Row(mark, r.Name, pluginTargetLabel(r.Target)+" · "+r.Status, width)
				if r.Message != "" {
					ui.Note(r.Message)
				}
			}
		}
	}
	return err
}

// printPluginPlan prints one row per change, marking blocked ones as
// failures, with what each includes and why beneath.
func printPluginPlan(p *plugin.Plan) {
	names := make([]string, len(p.Changes))
	for i, c := range p.Changes {
		names[i] = c.Name
	}
	width := ui.RowWidth(names...)
	for _, c := range p.Changes {
		mark := ui.MarkNone
		if c.Action == "blocked" {
			mark = ui.MarkFail
		}
		value := pluginTargetLabel(c.Target) + " · " + c.Action
		if c.ID != "" {
			value += ui.DimText(" · " + c.ID)
		}
		ui.Row(mark, c.Name, value, width)
		if len(c.Components) > 0 {
			ui.Note("includes " + strings.Join(c.Components, ", "))
		}
		if c.Message != "" {
			ui.Note(c.Message)
		}
	}
	ui.Note("revision " + p.Revision)
}

// pluginTargetLabel names package-level changes, which apply to Skillshare rather than an Agent.
func pluginTargetLabel(target string) string {
	if target == "" {
		return "skillshare"
	}
	return target
}
