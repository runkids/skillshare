package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"skillshare/internal/codexskills"
	"skillshare/internal/config"
)

func cmdDedup(args []string) (err error) {
	if wantsHelp(args) {
		fmt.Println(`Usage: skillshare dedup codex [--apply] [--json]

Preview identical local skills in Codex's actual catalog. Requires Codex CLI.
--apply  Back up Codex config, disable redundant paths, and verify the catalog.
--json   Output the plan, skipped variants, applied paths, and verification state.

Only identical user skill folders are eligible. Plugins, repository/system
skills, differing variants, symlinks inside skills and parent-relative Markdown
references require manual review. Files and sync settings are never removed.
Existing Codex sessions may need a restart to refresh their skill picker.`)
		return nil
	}
	jsonOutput := slices.Contains(args, "--json")
	defer func() {
		var silent *jsonSilentError
		if err != nil && jsonOutput && !errors.As(err, &silent) {
			err = writeJSONError(err)
		}
	}()
	if len(args) == 0 || args[0] != "codex" {
		return fmt.Errorf("usage: skillshare dedup codex [--apply] [--json]")
	}
	var apply bool
	for _, arg := range args[1:] {
		switch arg {
		case "--apply":
			apply = true
		case "--json":
			jsonOutput = true
		default:
			return fmt.Errorf("unknown dedup option: %s", arg)
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	source, err := filepath.EvalSymlinks(cfg.EffectiveSkillsSource())
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	client, err := codexskills.Start(context.Background())
	if err != nil {
		return err
	}
	defer client.Close()
	skills, err := client.List(cwd)
	if err != nil {
		return err
	}
	plan, err := codexskills.BuildPlan(skills, source)
	if err != nil {
		return err
	}
	if apply {
		plan, err = codexskills.Apply(client, cwd, source, client.ConfigPath, plan)
	}
	if jsonOutput {
		if err != nil {
			plan.Error = err.Error()
		}
		return writeJSONResult(plan, err)
	}
	for _, group := range plan.Groups {
		fmt.Printf("%s: keep %s\n", group.Name, group.Keep)
		for _, path := range group.Disable {
			fmt.Printf("  disable %s\n", path)
		}
	}
	for _, skipped := range plan.Skipped {
		fmt.Printf("%s: manual review — %s\n", skipped.Name, skipped.Reason)
	}
	if plan.Backup != "" {
		fmt.Printf("Config backup: %s\n", plan.Backup)
	}
	if apply {
		fmt.Printf("Applied %d overrides; verified: %t\n", len(plan.Applied), plan.Verified)
	} else {
		fmt.Println("Preview only. Use --apply to write Codex overrides.")
	}
	return err
}
