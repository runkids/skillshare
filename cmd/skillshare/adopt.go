package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/trash"
)

// adoptAgentsTargetNames are the canonical name + alias of the universal target
// (~/.agents/skills) that external CLI tools write into.
var adoptAgentsTargetNames = []string{"universal", "agents"}

func parseAdoptOptions(args []string) adoptOptions {
	opts := adoptOptions{}
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			opts.dryRun = true
		case "--force", "-f":
			opts.force = true
		case "--all", "-a":
			opts.all = true
		case "--json":
			opts.jsonOutput = true
		default:
			if opts.targetName == "" && !strings.HasPrefix(arg, "-") {
				opts.targetName = arg
			}
		}
	}
	if opts.jsonOutput {
		opts.force = true
	}
	return opts
}

func cmdAdopt(args []string) error {
	if wantsHelp(args) {
		printAdoptHelp()
		return nil
	}

	start := time.Now()

	mode, rest, err := parseModeArgs(args)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	if mode == modeAuto {
		if projectConfigExists(cwd) {
			mode = modeProject
		} else {
			mode = modeGlobal
		}
	}

	applyModeLabel(mode)

	opts := parseAdoptOptions(rest)

	if mode == modeProject {
		return cmdAdoptProject(opts, cwd, start)
	}
	return cmdAdoptGlobal(opts, start)
}

// cmdAdoptGlobal builds the adoptContext from the global config and runs adopt.
func cmdAdoptGlobal(opts adoptOptions, start time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return adoptCommandError(err, opts.jsonOutput)
	}

	agentsTarget, ok := findAgentsTarget(cfg.Targets)
	if !ok {
		return adoptCommandError(fmt.Errorf("universal/agents target not configured; nothing to adopt"), opts.jsonOutput)
	}
	sc := agentsTarget.SkillsConfig()

	allTargets := make(map[string]string, len(cfg.Targets))
	for name, t := range cfg.Targets {
		allTargets[name] = t.SkillsConfig().Path
	}

	actx := adoptContext{
		agentsPath: sc.Path,
		sourcePath: cfg.EffectiveSkillsSource(),
		syncMode:   adoptSyncMode(sc.Mode, cfg.Mode),
		allTargets: allTargets,
		targets:    cfg.Targets,
		trashBase:  trash.TrashDir(),
		configPath: config.ConfigPath(),
	}

	return runAdoptCommand(actx, opts, start)
}

// findAgentsTarget locates the universal/agents target in a target map.
func findAgentsTarget(targets map[string]config.TargetConfig) (config.TargetConfig, bool) {
	for _, name := range adoptAgentsTargetNames {
		if t, ok := targets[name]; ok {
			return t, true
		}
	}
	return config.TargetConfig{}, false
}

// adoptSyncMode resolves the effective sync mode for the agents target.
func adoptSyncMode(targetMode, globalMode string) string {
	mode := sync.EffectiveMode(targetMode)
	if targetMode == "" && globalMode != "" {
		mode = globalMode
	}
	return mode
}

func printAdoptHelp() {
	fmt.Println(`Usage: skillshare adopt [target] [options]

Adopt CLI-bundled skills that external tools (e.g. firecrawl/cli,
googleworkspace/cli) drop into the universal target (~/.agents/skills),
bypassing skillshare's source-of-truth model.

Adopt migrates the canonical files into skillshare's source, removes the
external tool's orphan symlinks, re-syncs to all targets, and warns about any
lingering entries in the tool's lockfile (~/.agents/.skill-lock.json). The
lockfile is never modified — release those entries from the owning tool.

Arguments:
  [target]          Reserved for future per-target scoping (optional)

Options:
  --all, -a         Adopt all detected skills
  --dry-run, -n     Preview changes without applying
  --force, -f       Overwrite same-name skills in source and skip confirmation
  --json            Output results as JSON (implies --force)
  --project, -p     Use project-level config (.agents/skills)
  --global, -g      Use global config (~/.agents/skills)
  --help, -h        Show this help

Examples:
  skillshare adopt                 Detect and interactively adopt skills
  skillshare adopt --all --force   Adopt everything without prompting
  skillshare adopt --dry-run       Preview what would be adopted
  skillshare adopt --json          Adopt and emit JSON`)
}
