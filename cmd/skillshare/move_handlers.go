package main

import (
	"fmt"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/oplog"
	"skillshare/internal/skillmove"
	"skillshare/internal/sourcewalk"
	ssync "skillshare/internal/sync"
	"skillshare/internal/ui"
)

// moveMode is what differs between global and project mode.
type moveMode struct {
	sourceDir string
	walk      sourcewalk.Options
	store     *install.MetadataStore
	targets   map[string]config.TargetConfig
	cfgPath   string // the config the oplog entry is written next to

	projectRoot                   string // "" in global mode
	gitignoreDir, gitignorePrefix string
	reconcile                     func() error
}

// options returns the core's view of the mode.
func (m *moveMode) options(o *moveOptions) skillmove.Options {
	return skillmove.Options{
		SourceDir:       m.sourceDir,
		Follow:          m.walk.Follow,
		Store:           m.store,
		Targets:         skillmove.EnabledTargets(m.targets),
		Force:           o.force,
		DryRun:          o.dryRun,
		ProjectRoot:     m.projectRoot,
		GitignoreDir:    m.gitignoreDir,
		GitignorePrefix: m.gitignorePrefix,
		Reconcile:       m.reconcile,
	}
}

// globalMoveMode loads what a move in the global source needs.
func globalMoveMode() (*moveMode, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	sourceDir := cfg.EffectiveSkillsSource()
	store, err := loadMoveStore(sourceDir)
	if err != nil {
		return nil, err
	}
	return &moveMode{
		sourceDir: sourceDir,
		walk:      cfg.SkillsWalk(),
		store:     store,
		targets:   cfg.Targets,
		cfgPath:   config.ConfigPath(),
		reconcile: func() error { return config.ReconcileGlobalSkills(cfg, store) },
	}, nil
}

// loadMoveStore reads the install records. An unreadable file stops the move:
// saving an empty store over it would lose every record the move must keep.
func loadMoveStore(sourceDir string) (*install.MetadataStore, error) {
	store, err := install.LoadMetadataWithMigration(sourceDir, "")
	if err != nil {
		return nil, fmt.Errorf("cannot read install records in %s: %w", install.MetadataPath(sourceDir), err)
	}
	return store, nil
}

// moveJSONOutput is the shape of `move --json`.
type moveJSONOutput struct {
	Moved    []moveJSONMoved  `json:"moved"`
	Failed   []moveJSONFailed `json:"failed"`
	Skipped  int              `json:"skipped"` // already in the destination
	Warnings []string         `json:"warnings"`
	DryRun   bool             `json:"dry_run"`
	Duration string           `json:"duration"`
}

type moveJSONMoved struct {
	Name   string `json:"name"`
	From   string `json:"from"`
	To     string `json:"to"`
	Record bool   `json:"record"` // an install record moved along
	Skills int    `json:"skills"` // 1 for a skill, the count below a folder
}

type moveJSONFailed struct {
	Name  string `json:"name"`
	Code  string `json:"code"`
	Error string `json:"error"`
}

// runMove resolves the names, applies the move and reports it.
func runMove(opts *moveOptions, m *moveMode, start time.Time) error {
	discovered, err := ssync.DiscoverSourceSkillsAll(m.sourceDir, m.walk)
	if err != nil {
		err = fmt.Errorf("failed to discover skills: %w", err)
		if opts.json {
			return writeJSONError(err)
		}
		return err
	}
	printSkippedSourceLinkWarnings(m.walk, opts.json)

	o := m.options(opts)
	out := skillmove.Run(skillmove.Plan(discovered, opts.names, opts.dest, o), o)

	report := moveJSONOutput{DryRun: opts.dryRun}
	skills := 0
	for _, item := range out.Items {
		switch {
		case item.Moved():
			report.Moved = append(report.Moved, moveJSONMoved{Name: item.Name, From: item.From, To: item.To, Record: item.Records > 0, Skills: len(item.Skills)})
			skills += len(item.Skills)
		case item.Skipped():
			report.Skipped++
		default:
			report.Failed = append(report.Failed, moveJSONFailed{Name: item.Name, Code: string(item.Err.Code), Error: item.Err.Error()})
		}
		report.Warnings = append(report.Warnings, item.Warnings...)
	}
	report.Duration = formatDuration(start)

	var cmdErr error
	switch {
	case out.Err != nil:
		cmdErr = fmt.Errorf("moved, but a follow-up step failed: %w", out.Err)
	case len(report.Failed) > 0:
		cmdErr = fmt.Errorf("%d of %d could not be moved", len(report.Failed), len(opts.names))
	}
	if !opts.dryRun {
		logMoveOp(m.cfgPath, opts, m.modeName(), len(report.Moved), start, cmdErr)
	}
	if opts.json {
		return writeJSONResult(&report, cmdErr)
	}
	renderMove(out.Items, &report, skills, start)
	return cmdErr
}

// modeName is the mode as the oplog records it.
func (m *moveMode) modeName() string {
	if m.projectRoot != "" {
		return "project"
	}
	return "global"
}

// renderMove prints one row per requested name, the warnings, and the closing
// line with the next step.
func renderMove(items []skillmove.Planned, report *moveJSONOutput, skills int, start time.Time) {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Name
	}
	width := ui.RowWidth(labels...)
	for _, item := range items {
		switch {
		case item.Moved() && report.DryRun:
			ui.Row(ui.MarkNone, item.Name, "would move to "+moveTarget(item), width)
		case item.Moved():
			ui.Row(ui.MarkOK, item.Name, "→ "+moveTarget(item), width)
		case item.Skipped():
			ui.Row(ui.MarkNone, item.Name, item.Err.Error(), width)
		default:
			ui.Row(ui.MarkFail, item.Name, fmt.Sprintf("%s (%s)", item.Err.Error(), item.Err.Code), width)
		}
	}
	for _, w := range report.Warnings {
		ui.Warning("%s", w)
	}

	fmt.Println()
	switch {
	case report.DryRun:
		ui.DryRun()
	case skills == 0 && len(report.Failed) == 0:
		ui.Done(ui.MarkNone, "Already in place", time.Since(start))
	case skills == 0:
		ui.Done(ui.MarkFail, "Nothing moved", time.Since(start))
	case len(report.Failed) > 0:
		ui.Done(ui.MarkWarn, "Moved "+plural(skills, "skill"), time.Since(start))
	default:
		ui.Done(ui.MarkOK, "Moved "+plural(skills, "skill"), time.Since(start))
	}
	if !report.DryRun && skills > 0 {
		ui.Next("skillshare sync", "rename the links in your targets")
	}
}

// moveTarget is the destination, with the number of skills of a folder or of a
// skill that carries nested ones.
func moveTarget(item skillmove.Planned) string {
	if len(item.Skills) > 1 {
		return fmt.Sprintf("%s (%s)", item.To, plural(len(item.Skills), "skill"))
	}
	return item.To
}

func logMoveOp(cfgPath string, opts *moveOptions, mode string, moved int, start time.Time, cmdErr error) {
	status := statusFromErr(cmdErr)
	if moved > 0 && cmdErr != nil {
		status = "partial"
	}
	e := oplog.NewEntry("move", status, time.Since(start))
	e.Args = map[string]any{"names": opts.names, "dest": opts.dest, "force": opts.force, "mode": mode}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}
