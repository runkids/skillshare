package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/plugin"
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
		d, err := plugin.DiscoverOptions(ctx, r.Source, r.SourceRef, r.Entry)
		if err != nil {
			return err
		}
		if o.json {
			return json.NewEncoder(os.Stdout).Encode(d)
		}
		for _, warning := range d.Warnings {
			fmt.Println("Warning:", warning)
		}
		for _, c := range d.Candidates {
			fmt.Printf("%s · %s · %s\n", c.Name, strings.Join(c.Targets, ", "), strings.Join(c.Components, ", "))
			if c.Problem != "" {
				fmt.Println(c.Problem)
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
		if len(inventory.Packages) == 0 {
			fmt.Println("No plugins managed yet. Run 'skillshare plugin add' or 'skillshare plugin import'.")
		}
		for name, p := range inventory.Packages {
			for _, target := range plugin.Targets {
				if b, ok := p.Bindings[target]; ok {
					state := "not installed"
					for _, h := range inventory.Hosts {
						if h.Target != target {
							continue
						}
						if h.Error != "" {
							state = h.Error
						}
						for _, i := range h.Installed {
							if i.ID == b.ID {
								state = "registered (loading unverified)"
								if i.EnabledKnown && !i.Enabled {
									state = "disabled"
								}
							}
						}
					}
					if b.Pending != "" {
						state = "pending " + b.Pending
					}
					fmt.Printf("%s · %s · %s · %s\n", name, target, b.ID, state)
				}
			}
		}
		for _, h := range inventory.Hosts {
			if h.Error != "" {
				fmt.Printf("%s: %s\n", h.Target, h.Error)
			}
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
			for _, r := range result.Results {
				fmt.Printf("%s · %s · %s\n%s\n", r.Name, r.Target, r.Status, r.Message)
			}
		}
	}
	return err
}

func printPluginPlan(p *plugin.Plan) {
	for _, c := range p.Changes {
		fmt.Printf("%s · %s · %s · %s\n", c.Name, c.Target, c.Action, c.ID)
		if len(c.Components) > 0 {
			fmt.Println("Includes:", strings.Join(c.Components, ", "))
		}
		if c.Message != "" {
			fmt.Println(c.Message)
		}
	}
	fmt.Println("Revision:", p.Revision)
}
