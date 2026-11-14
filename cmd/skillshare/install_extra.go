package main

import (
	"fmt"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/projectdir"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type installExtraJSON struct {
	Kind            string   `json:"kind"`
	Name            string   `json:"name,omitempty"`
	Remote          string   `json:"remote,omitempty"`
	Branch          string   `json:"branch,omitempty"`
	Commit          string   `json:"commit,omitempty"`
	Action          string   `json:"action,omitempty"`
	AuditSkipped    bool     `json:"audit_skipped,omitempty"`
	AuditOverridden bool     `json:"audit_overridden,omitempty"`
	Interrupted     bool     `json:"interrupted,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Error           string   `json:"error,omitempty"`
}

func extraInstallRequested(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "--kind" && i+1 < len(args) && strings.EqualFold(args[i+1], "extra") {
			return true
		}
	}
	return false
}

func extraJSONRequested(args []string) bool {
	if extraInstallRequested(args) {
		for _, arg := range args {
			if arg == "--json" {
				return true
			}
		}
	}
	return false
}

func cmdInstallExtra(parsed *installArgs, mode runMode, cwd string, start time.Time) error {
	jsonUI := newJSONUISuppressor(parsed.jsonOutput)
	defer jsonUI.Flush()
	result, targetless, err := installExtra(parsed, mode, cwd, start)
	if parsed.jsonOutput {
		out := installExtraJSON{Kind: "extra"}
		if result != nil {
			out.Name, out.Remote, out.Branch, out.Commit, out.Action = result.Name, result.Remote, result.Branch, result.Commit, result.Action
			out.AuditSkipped, out.AuditOverridden, out.Interrupted, out.Warnings = result.AuditSkipped, result.AuditOverridden, result.Interrupted, result.Warnings
		}
		if err != nil {
			out.Error = err.Error()
		}
		jsonUI.Flush()
		return writeJSONResult(&out, err)
	}
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		ui.Warning("%s", warning)
	}
	if result.AuditSkipped {
		ui.Warning("Security audit skipped")
	}
	if result.Interrupted {
		ui.Note("Repaired an interrupted publication")
	}
	ui.Done(ui.MarkOK, fmt.Sprintf("Extra %s: %s", result.Name, result.Action), time.Since(start))
	if result.Action == "would install" {
		ui.DryRun()
		return nil
	}
	if targetless {
		ui.Next("skillshare extras "+result.Name+" --add-target <path>", "Add a target", "skillshare sync extras", "Sync extras")
	} else {
		ui.Next("skillshare sync extras", "Sync extras")
	}
	return nil
}

func installExtra(parsed *installArgs, mode runMode, cwd string, start time.Time) (result *install.RemoteExtraResult, targetless bool, retErr error) {
	raw := strings.TrimSpace(parsed.sourceArg)
	if raw == "" {
		return nil, false, fmt.Errorf("--kind extra requires a remote source")
	}
	if err := install.RejectSourceCredentials(raw); err != nil {
		return nil, false, err
	}
	var root, lockDir, configPath string
	var opts install.ParseOptions
	var threshold string
	var load func() ([]config.ExtraConfig, error)
	var save func(extraConfigUpdate) error
	if mode == modeProject {
		if _, ok := projectdir.Find(cwd); !ok {
			return nil, false, fmt.Errorf("project config not found: run 'skillshare init -p' first")
		}
		cfg, err := config.LoadProjectReadOnly(cwd)
		if err != nil {
			return nil, false, err
		}
		root, lockDir, configPath = cfg.EffectiveExtrasSource(cwd), projectdir.Resolve(cwd), config.ProjectConfigPath(cwd)
		opts, threshold = parseOptsFromProjectConfig(cfg), cfg.Audit.BlockThreshold
		load = func() ([]config.ExtraConfig, error) {
			c, err := config.LoadProjectReadOnly(cwd)
			if err != nil {
				return nil, err
			}
			return c.Extras, nil
		}
		save = func(update extraConfigUpdate) error {
			c, err := config.LoadProjectReadOnly(cwd)
			if err != nil {
				return err
			}
			extras, err := update(c.Extras)
			if err != nil || extras == nil {
				return err
			}
			c.Extras = extras
			return c.Save(cwd)
		}
	} else {
		cfg, err := config.LoadReadOnly()
		if err != nil {
			return nil, false, err
		}
		root, configPath = cfg.EffectiveExtrasSource(), config.ReadOnlyConfigPath()
		opts, threshold = parseOptsFromConfig(cfg), cfg.Audit.BlockThreshold
		load = func() ([]config.ExtraConfig, error) {
			c, err := config.LoadReadOnly()
			if err != nil {
				return nil, err
			}
			return c.Extras, nil
		}
		save = func(update extraConfigUpdate) error {
			c, err := config.LoadReadOnly()
			if err != nil {
				return err
			}
			extras, err := update(c.Extras)
			if err != nil || extras == nil {
				return err
			}
			c.Extras = extras
			candidate, err := config.PrepareSave(configPath, c)
			if err != nil {
				return err
			}
			return candidate.Write()
		}
	}
	source, err := install.ParseSourceWithOptions(raw, opts)
	if err != nil {
		return nil, false, fmt.Errorf("unsupported remote source form; use owner/repo[/subdir], a Git URL, or user@host:path")
	}
	if !source.IsGit() {
		return nil, false, fmt.Errorf("use 'skillshare extras init <name> --source <path>' for a local folder")
	}
	source.Raw = raw
	name := parsed.opts.Name
	if name == "" {
		name = source.Name
	}
	if err := config.ValidateExtraName(name); err != nil {
		return nil, false, err
	}
	branch := parsed.opts.Branch
	callbacks, err := newExtraConfigCallbacks(name, raw, load, save)
	if err != nil {
		return nil, false, err
	}
	resolvedConfig, err := utils.ConfigWritePath(configPath, mode == modeProject)
	if err != nil {
		return nil, false, err
	}
	requestOpts := parsed.opts
	requestOpts.ParseOptions = opts
	if requestOpts.AuditThreshold == "" {
		requestOpts.AuditThreshold = threshold
	}
	if mode == modeProject {
		requestOpts.AuditProjectRoot = cwd
	}
	if !parsed.jsonOutput {
		requestOpts.OnProgress = func(line string) { ui.Info("%s", line) }
	}
	if !requestOpts.DryRun {
		defer func() {
			e := oplog.NewEntry("install", statusFromErr(retErr), time.Since(start))
			e.Args = map[string]any{"kind": "extra", "mode": modeString(mode), "name": name, "source": raw}
			if result != nil && result.Commit != "" {
				e.Args["commit"] = result.Commit
			}
			if retErr != nil {
				e.Message = retErr.Error()
			}
			// Logging must not migrate unrelated embedded skill declarations.
			cfg, _ := config.LoadReadOnly()
			oplog.WriteWithLimit(configPath, oplog.OpsFile, e, configuredLogMaxEntries(cfg)) //nolint:errcheck
		}()
	}
	req := install.RemoteExtraRequest{Name: name, RawSource: raw, Branch: branch, Source: source, ExtrasRoot: root, LockDir: lockDir, ConfigPath: resolvedConfig, Opts: requestOpts, Recheck: callbacks.Recheck, WriteConfig: func() error { return callbacks.WriteConfig(source.Branch) }, RevertConfig: callbacks.RevertConfig, RepairConfig: callbacks.RepairConfig, RefreshAfterRepair: callbacks.RefreshAfterRepair, PinBranch: func() string { return callbacks.PinBranch(source.Branch) }}
	if mode == modeProject {
		req.ProjectRoot = cwd
	}
	result, retErr = install.InstallRemoteExtra(req)
	if retErr != nil {
		return result, false, retErr
	}
	entries, err := load()
	if err != nil {
		return result, false, err
	}
	index, err := extraConfigAt(entries, name)
	if err != nil {
		return result, false, err
	}
	return result, index < 0 || len(entries[index].Targets) == 0, nil
}
