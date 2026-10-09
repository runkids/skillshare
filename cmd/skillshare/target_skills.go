package main

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
)

// stripNoSkillsFlag removes --no-skills from target add arguments.
func stripNoSkillsFlag(args []string) ([]string, bool) {
	rest := make([]string, 0, len(args))
	noSkills := false
	for _, arg := range args {
		if arg == "--no-skills" {
			noSkills = true
			continue
		}
		rest = append(rest, arg)
	}
	return rest, noSkills
}

// setTargetSkillsGlobal switches skills sync for a global target. Turning it
// off saves the config, then removes the folder's links into the source.
func setTargetSkillsGlobal(cfg *config.Config, name string, target config.TargetConfig, enabled, dryRun bool) error {
	start := time.Now()
	if enabled {
		// An off target may keep a naming its mode cannot sync.
		sc := target.SkillsConfig()
		if err := config.TargetNamingModeError(sc.TargetNaming, cmp.Or(sc.Mode, cfg.Mode)); err != nil {
			return fmt.Errorf("%w; set --mode copy first", err)
		}
	}
	res, err := switchTargetSkills(name, enabled, dryRun, func() error {
		target.EnsureSkills().SetEnabled(enabled)
		cfg.Targets[name] = target
		return cfg.Save()
	}, func() (map[string]config.TargetConfig, error) {
		return cfg.Targets, nil
	}, cfg.EffectiveSkillsSource())
	logTargetSkillsOp(config.ConfigPath(), name, enabled, dryRun, res, start, err)
	return err
}

// setTargetSkillsProject is setTargetSkillsGlobal for a project target.
func setTargetSkillsProject(cfg *config.ProjectConfig, idx int, enabled, dryRun bool, root string) error {
	start := time.Now()
	name := cfg.Targets[idx].Name
	if enabled {
		sc := cfg.Targets[idx].SkillsConfig()
		if err := config.TargetNamingModeError(sc.TargetNaming, sc.Mode); err != nil {
			return fmt.Errorf("%w; set --mode copy first", err)
		}
	}
	res, err := switchTargetSkills(name, enabled, dryRun, func() error {
		cfg.Targets[idx].EnsureSkills().SetEnabled(enabled)
		return cfg.Save(root)
	}, func() (map[string]config.TargetConfig, error) {
		return config.ResolveProjectTargets(root, cfg)
	}, cfg.EffectiveSkillsSource(root))
	logTargetSkillsOp(config.ProjectConfigPath(root), name, enabled, dryRun, res, start, err)
	return err
}

func switchTargetSkills(name string, enabled, dryRun bool, save func() error, targets func() (map[string]config.TargetConfig, error), sourcePath string) (*sync.SkillsOffResult, error) {
	if !dryRun {
		if err := save(); err != nil {
			return nil, err
		}
	}
	if enabled {
		if dryRun {
			ui.Done(ui.MarkNone, "Would turn skills on for "+name, 0)
			fmt.Println()
			ui.DryRun()
		} else {
			ui.Done(ui.MarkOK, "Skills on for "+name, 0)
			ui.Next("skillshare sync", "sync skills to this target")
		}
		return nil, nil
	}

	all, err := targets()
	if err != nil {
		return nil, err
	}
	res, err := sync.DetachSkills(all, name, sourcePath, dryRun)
	if err != nil {
		return nil, fmt.Errorf("%s: skills off saved, but removing links failed: %w", name, err)
	}
	renderSkillsOff(name, dryRun, res)
	return res, nil
}

func renderSkillsOff(name string, dryRun bool, res *sync.SkillsOffResult) {
	removeVerb, keepVerb, removeMark := "Removed", "Kept", ui.MarkOK
	if dryRun {
		removeVerb, keepVerb, removeMark = "Would remove", "Would keep", ui.MarkNone
	}
	width := ui.RowWidth(removeVerb, keepVerb)
	if len(res.Removed) > 0 {
		ui.Row(removeMark, removeVerb, plural(len(res.Removed), "link")+"  "+ui.DimText(strings.Join(res.Removed, ", ")), width)
	}
	if len(res.Kept) > 0 {
		ui.Row(ui.MarkNone, keepVerb, plural(len(res.Kept), "local skill")+"  "+ui.DimText(strings.Join(res.Kept, ", ")), width)
	}
	if len(res.Copies) > 0 {
		ui.Row(ui.MarkWarn, keepVerb, plural(len(res.Copies), "copied skill")+"  "+ui.DimText(strings.Join(res.Copies, ", ")), width)
	}
	if len(res.Removed)+len(res.Kept)+len(res.Copies) > 0 {
		fmt.Println()
	}

	if dryRun {
		ui.Done(ui.MarkNone, "Would turn skills off for "+name, 0)
	} else {
		ui.Done(ui.MarkOK, "Skills off for "+name, 0)
	}
	if res.SharedWith != "" {
		ui.Note(fmt.Sprintf("The skills folder is shared with %s, left as is", res.SharedWith))
	}
	if len(res.Copies) > 0 {
		ui.Note("The tool still loads these copies; delete them if it reads the same skills elsewhere")
	}
	ui.Note("Agents, MCP servers and instructions are still managed")
	if dryRun {
		fmt.Println()
		ui.DryRun()
	}
}

func logTargetSkillsOp(cfgPath, name string, enabled, dryRun bool, res *sync.SkillsOffResult, start time.Time, err error) {
	e := oplog.NewEntry("target", statusFromErr(err), time.Since(start))
	e.Args = map[string]any{
		"action":  "skills",
		"name":    name,
		"enabled": enabled,
		"dry_run": dryRun,
	}
	if res != nil {
		e.Args["removed"] = len(res.Removed)
		e.Args["kept"] = len(res.Kept)
		e.Args["copies"] = len(res.Copies)
	}
	if err != nil {
		e.Message = err.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}
