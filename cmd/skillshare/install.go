package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
	"skillshare/internal/validate"
)

// installArgs holds parsed install command arguments
type installArgs struct {
	sourceArg string
	opts      install.InstallOptions
}

type installLogSummary struct {
	Source          string
	Mode            string
	SkillCount      int
	InstalledSkills []string
	FailedSkills    []string
	DryRun          bool
	Tracked         bool
	Into            string
	SkipAudit       bool
	AuditVerbose    bool
	AuditThreshold  string
}

type installBatchSummary struct {
	InstalledSkills []string
	FailedSkills    []string
}

// parseInstallArgs parses install command arguments
func parseInstallArgs(args []string) (*installArgs, bool, error) {
	result := &installArgs{}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--name":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--name requires a value")
			}
			i++
			result.opts.Name = args[i]
		case arg == "--force" || arg == "-f":
			result.opts.Force = true
		case arg == "--update" || arg == "-u":
			result.opts.Update = true
		case arg == "--dry-run" || arg == "-n":
			result.opts.DryRun = true
		case arg == "--skip-audit":
			result.opts.SkipAudit = true
		case arg == "--audit-verbose":
			result.opts.AuditVerbose = true
		case arg == "--audit-threshold" || arg == "--threshold" || arg == "-T":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("%s requires a value", arg)
			}
			i++
			threshold, err := normalizeInstallAuditThreshold(args[i])
			if err != nil {
				return nil, false, err
			}
			result.opts.AuditThreshold = threshold
		case arg == "--track" || arg == "-t":
			result.opts.Track = true
		case arg == "--skill" || arg == "-s":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--skill requires a value")
			}
			i++
			result.opts.Skills = strings.Split(args[i], ",")
		case arg == "--exclude":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--exclude requires a value")
			}
			i++
			result.opts.Exclude = strings.Split(args[i], ",")
		case arg == "--into":
			if i+1 >= len(args) {
				return nil, false, fmt.Errorf("--into requires a value")
			}
			i++
			result.opts.Into = args[i]
		case arg == "--all":
			result.opts.All = true
		case arg == "--yes" || arg == "-y":
			result.opts.Yes = true
		case arg == "--help" || arg == "-h":
			return nil, true, nil // showHelp = true
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown option: %s", arg)
		default:
			if result.sourceArg != "" {
				return nil, false, fmt.Errorf("unexpected argument: %s", arg)
			}
			result.sourceArg = arg
		}
		i++
	}

	// Clean --skill input
	if len(result.opts.Skills) > 0 {
		cleaned := make([]string, 0, len(result.opts.Skills))
		for _, s := range result.opts.Skills {
			s = strings.TrimSpace(s)
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		if len(cleaned) == 0 {
			return nil, false, fmt.Errorf("--skill requires at least one skill name")
		}
		result.opts.Skills = cleaned
	}

	// Clean --exclude input
	if len(result.opts.Exclude) > 0 {
		cleaned := make([]string, 0, len(result.opts.Exclude))
		for _, s := range result.opts.Exclude {
			s = strings.TrimSpace(s)
			if s != "" {
				cleaned = append(cleaned, s)
			}
		}
		result.opts.Exclude = cleaned
	}

	// Validate mutual exclusion
	if result.opts.HasSkillFilter() && result.opts.All {
		return nil, false, fmt.Errorf("--skill and --all cannot be used together")
	}
	if result.opts.HasSkillFilter() && result.opts.Yes {
		return nil, false, fmt.Errorf("--skill and --yes cannot be used together")
	}
	if result.opts.HasSkillFilter() && result.opts.Track {
		return nil, false, fmt.Errorf("--skill cannot be used with --track")
	}
	if result.opts.ShouldInstallAll() && result.opts.Track {
		return nil, false, fmt.Errorf("--all/--yes cannot be used with --track")
	}

	if result.opts.Into != "" {
		if err := validate.IntoPath(result.opts.Into); err != nil {
			return nil, false, err
		}
	}

	return result, false, nil
}

// destWithInto returns the destination path, prepending opts.Into if set.
func destWithInto(sourceDir string, opts install.InstallOptions, skillName string) string {
	if opts.Into != "" {
		return filepath.Join(sourceDir, opts.Into, skillName)
	}
	return filepath.Join(sourceDir, skillName)
}

// ensureIntoDirExists creates the Into subdirectory if opts.Into is set.
func ensureIntoDirExists(sourceDir string, opts install.InstallOptions) error {
	if opts.Into == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(sourceDir, opts.Into), 0755)
}

// resolveSkillFromName resolves a skill name to source using metadata
func resolveSkillFromName(skillName string, cfg *config.Config) (*install.Source, error) {
	skillPath := filepath.Join(cfg.Source, skillName)

	meta, err := install.ReadMeta(skillPath)
	if err != nil {
		return nil, fmt.Errorf("skill '%s' not found or has no metadata", skillName)
	}
	if meta == nil {
		return nil, fmt.Errorf("skill '%s' has no metadata, cannot update", skillName)
	}

	source, err := install.ParseSource(meta.Source)
	if err != nil {
		return nil, fmt.Errorf("invalid source in metadata: %w", err)
	}

	source.Name = skillName
	return source, nil
}

// resolveInstallSource parses and resolves the install source
func resolveInstallSource(sourceArg string, opts install.InstallOptions, cfg *config.Config) (*install.Source, bool, error) {
	source, err := install.ParseSource(sourceArg)
	if err == nil {
		return source, false, nil
	}

	// Try resolving from installed skill metadata if update/force
	if opts.Update || opts.Force {
		resolvedSource, resolveErr := resolveSkillFromName(sourceArg, cfg)
		if resolveErr != nil {
			return nil, false, fmt.Errorf("invalid source: %w", err)
		}
		ui.Info("Resolved '%s' from installed skill metadata", sourceArg)
		return resolvedSource, true, nil // resolvedFromMeta = true
	}

	return nil, false, fmt.Errorf("invalid source: %w", err)
}

// dispatchInstall routes to the appropriate install handler
func dispatchInstall(source *install.Source, cfg *config.Config, opts install.InstallOptions) (installLogSummary, error) {
	if opts.Track {
		return handleTrackedRepoInstall(source, cfg, opts)
	}

	if source.IsGit() {
		return handleGitInstall(source, cfg, opts)
	}

	return handleDirectInstall(source, cfg, opts)
}

func cmdInstall(args []string) error {
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

	if mode == modeProject {
		summary, err := cmdInstallProject(rest, cwd)
		if summary.Mode == "" {
			summary.Mode = "project"
		}
		logInstallOp(config.ProjectConfigPath(cwd), rest, start, err, summary)
		return err
	}

	parsed, showHelp, parseErr := parseInstallArgs(rest)
	if showHelp {
		printInstallHelp()
		return parseErr
	}
	if parseErr != nil {
		return parseErr
	}

	// When no source is given, only bare "install" is valid — reject incompatible flags
	if parsed.sourceArg == "" {
		hasSourceFlags := parsed.opts.Name != "" || parsed.opts.Into != "" ||
			parsed.opts.Track || len(parsed.opts.Skills) > 0 ||
			len(parsed.opts.Exclude) > 0 || parsed.opts.All || parsed.opts.Yes || parsed.opts.Update
		if hasSourceFlags {
			return fmt.Errorf("flags --name, --into, --track, --skill, --exclude, --all, --yes, and --update require a source argument")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if parsed.opts.AuditThreshold == "" {
		parsed.opts.AuditThreshold = cfg.Audit.BlockThreshold
	}

	// No source argument: install from global config
	if parsed.sourceArg == "" {
		summary, err := installFromGlobalConfig(cfg, parsed.opts)
		if err == nil && !parsed.opts.DryRun && len(summary.InstalledSkills) > 0 {
			discoveryCache.Invalidate(cfg.Source)
		}
		logInstallOp(config.ConfigPath(), rest, start, err, summary)
		return err
	}

	source, resolvedFromMeta, err := resolveInstallSource(parsed.sourceArg, parsed.opts, cfg)
	if err != nil {
		logInstallOp(config.ConfigPath(), rest, start, err, installLogSummary{
			Source: parsed.sourceArg,
			Mode:   "global",
		})
		return err
	}

	summary := installLogSummary{
		Source:         parsed.sourceArg,
		Mode:           "global",
		DryRun:         parsed.opts.DryRun,
		Tracked:        parsed.opts.Track,
		Into:           parsed.opts.Into,
		SkipAudit:      parsed.opts.SkipAudit,
		AuditVerbose:   parsed.opts.AuditVerbose,
		AuditThreshold: parsed.opts.AuditThreshold,
	}

	// If resolved from metadata with update/force, go directly to install
	if resolvedFromMeta {
		summary, err = handleDirectInstall(source, cfg, parsed.opts)
		if summary.Mode == "" {
			summary.Mode = "global"
		}
		if summary.Source == "" {
			summary.Source = parsed.sourceArg
		}
		if err == nil && !parsed.opts.DryRun && len(summary.InstalledSkills) > 0 {
			discoveryCache.Invalidate(cfg.Source)
			reg, regErr := config.LoadRegistry(filepath.Dir(config.ConfigPath()))
			if regErr != nil {
				ui.Warning("Failed to load registry: %v", regErr)
			} else if rErr := config.ReconcileGlobalSkills(cfg, reg); rErr != nil {
				ui.Warning("Failed to reconcile global skills config: %v", rErr)
			}
		}
		logInstallOp(config.ConfigPath(), rest, start, err, summary)
		return err
	}

	summary, err = dispatchInstall(source, cfg, parsed.opts)
	if summary.Mode == "" {
		summary.Mode = "global"
	}
	if summary.Source == "" {
		summary.Source = parsed.sourceArg
	}
	if err == nil && !parsed.opts.DryRun && len(summary.InstalledSkills) > 0 {
		discoveryCache.Invalidate(cfg.Source)
		reg, regErr := config.LoadRegistry(filepath.Dir(config.ConfigPath()))
		if regErr != nil {
			ui.Warning("Failed to load registry: %v", regErr)
		} else if rErr := config.ReconcileGlobalSkills(cfg, reg); rErr != nil {
			ui.Warning("Failed to reconcile global skills config: %v", rErr)
		}
	}
	logInstallOp(config.ConfigPath(), rest, start, err, summary)
	return err
}

func logInstallOp(cfgPath string, args []string, start time.Time, cmdErr error, summary installLogSummary) {
	status := statusFromErr(cmdErr)
	if len(summary.InstalledSkills) > 0 && len(summary.FailedSkills) > 0 {
		status = "partial"
	}
	e := oplog.NewEntry("install", status, time.Since(start))
	fields := map[string]any{}
	source := summary.Source
	if len(args) > 0 {
		source = args[0]
	}
	if source != "" {
		fields["source"] = source
	}
	if summary.Mode != "" {
		fields["mode"] = summary.Mode
	}
	if summary.DryRun {
		fields["dry_run"] = true
	}
	if summary.Tracked {
		fields["tracked"] = true
	}
	if summary.Into != "" {
		fields["into"] = summary.Into
	}
	if summary.SkipAudit {
		fields["skip_audit"] = true
	}
	if summary.AuditVerbose {
		fields["audit_verbose"] = true
	}
	if summary.AuditThreshold != "" {
		fields["threshold"] = strings.ToUpper(summary.AuditThreshold)
	}
	if summary.SkillCount > 0 {
		fields["skill_count"] = summary.SkillCount
	}
	if len(summary.InstalledSkills) > 0 {
		fields["installed_skills"] = summary.InstalledSkills
		if _, ok := fields["skill_count"]; !ok {
			fields["skill_count"] = len(summary.InstalledSkills)
		}
	}
	if len(summary.FailedSkills) > 0 {
		fields["failed_skills"] = summary.FailedSkills
	}
	if len(fields) > 0 {
		e.Args = fields
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}

func printInstallHelp() {
	fmt.Println(`Usage: skillshare install [source|skill-name] [options]

Install skills from a local path, git repository, or global config.
When run with no arguments, installs all skills listed in config.yaml.
When using --update or --force with a skill name, skillshare uses stored metadata to resolve the source.

Sources:
  user/repo                  GitHub shorthand (expands to github.com/user/repo)
  user/repo/path/to/skill    GitHub shorthand with subdirectory
  github.com/user/repo       Full GitHub URL (discovers skills)
  github.com/user/repo/path  Subdirectory in GitHub repo (direct install)
  https://github.com/...     HTTPS git URL
  git@github.com:...         SSH git URL
  ~/path/to/skill            Local directory

Options:
  --name <name>       Override installed name when exactly one skill is installed
  --into <dir>        Install into subdirectory (e.g. "frontend" or "frontend/react")
  --force, -f         Overwrite existing skill; also continue if audit would block
  --update, -u        Update existing (git pull if possible, else reinstall)
  --track, -t         Install as tracked repo (preserves .git for updates)
  --skill, -s <names> Select specific skills from multi-skill repo (comma-separated;
                      supports glob patterns like "core-*", "test-?")
  --exclude <names>   Skip specific skills during install (comma-separated;
                      supports glob patterns like "test-*")
  --all               Install all discovered skills without prompting
  --yes, -y           Auto-accept all prompts (equivalent to --all for multi-skill repos)
  --dry-run, -n       Preview the installation without making changes
  --skip-audit        Skip security audit entirely for this install
  --audit-verbose     Show full audit finding lines (default: compact summary)
  --audit-threshold, --threshold, -T <t>
                      Block install by severity at/above: critical|high|medium|low|info
                      (also supports c|h|m|l|i)
  --project, -p       Use project-level config in current directory
  --global, -g        Use global config (~/.config/skillshare)
  --help, -h          Show this help

Examples:
  skillshare install anthropics/skills
  skillshare install anthropics/skills/skills/pdf
  skillshare install ComposioHQ/awesome-claude-skills
  skillshare install ~/my-skill
  skillshare install github.com/user/repo --force
  skillshare install ~/my-skill --skip-audit     # Bypass scan (no findings generated)
  skillshare install user/repo --all --audit-verbose
  skillshare install ~/my-skill -T high          # Override block threshold for this run

Selective install (non-interactive):
  skillshare install anthropics/skills -s pdf,commit     # Specific skills
  skillshare install anthropics/skills -s "core-*"       # Glob pattern
  skillshare install anthropics/skills --all             # All skills
  skillshare install anthropics/skills -y                # Auto-accept
  skillshare install anthropics/skills -s pdf --dry-run  # Preview selection
  skillshare install repo --all --exclude cli-sentry     # All except specific
  skillshare install repo --all --exclude "test-*"       # Exclude by pattern

Organize into subdirectories:
  skillshare install anthropics/skills -s pdf --into frontend
  skillshare install user/repo --track --into devops
  skillshare install ~/my-skill --into frontend/react

Tracked repositories (Team Edition):
  skillshare install team/shared-skills --track   # Clone as _shared-skills
  skillshare install _shared-skills --update      # Update tracked repo

Install from config (no arguments):
  skillshare install                         # Install all skills from config.yaml
  skillshare install --dry-run               # Preview config-based install

Update existing skills:
  skillshare install my-skill --update       # Update using stored source
  skillshare install my-skill --force        # Reinstall using stored source
  skillshare install my-skill --update -n    # Preview update`)
}
