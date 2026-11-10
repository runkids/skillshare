package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
	"skillshare/internal/uninstall"
)

// uninstallMode holds what differs between global and project skill uninstall.
type uninstallMode struct {
	sourceDir   string
	sourceLabel string // names sourceDir in not-found errors
	walk        sourcewalk.Options
	trashDir    string
	configPath  string // oplog location
	store       *install.MetadataStore

	emptySourceErr string   // --all found no skills
	globs          bool     // expand glob patterns in skill names
	confirmSuffix  string   // appended to confirmation questions
	reinstallFlags string   // appended to reinstall and trash commands
	targetNames    []string // the targets sync will update, sorted
	// reportAfterFinalize reports a single-target trash failure after
	// metadata and lock updates.
	reportAfterFinalize bool

	dryRunGitignore func(t *uninstallTarget) string // "" prints no line
	gitignoreDir    string
	gitignoreEntry  func(t *uninstallTarget) string // "" leaves .gitignore alone
	afterRemove     func(removed map[string]bool)   // extra state cleanup, may be nil
	preflight       uninstallPreflightMessages
}

// items describes the targets to the shared uninstall operation.
func (m *uninstallMode) items(targets []*uninstallTarget) []uninstall.Item {
	items := make([]uninstall.Item, len(targets))
	for i, t := range targets {
		items[i] = uninstall.Item{
			Name:      t.name,
			Path:      t.path,
			TrashName: t.name,
			Repo:      t.isTrackedRepo,
			Gitignore: m.gitignoreEntry(t),
		}
	}
	return items
}

func (m *uninstallMode) options() uninstall.Options {
	return uninstall.Options{
		SourceDir:    m.sourceDir,
		Follow:       m.walk.Follow,
		TrashDir:     m.trashDir,
		Store:        m.store,
		GitignoreDir: m.gitignoreDir,
	}
}

// uninstallPreflightMessages reports tracked-repo git checks.
type uninstallPreflightMessages struct {
	forceStatus  func(name string, err error) // status unreadable, --force set
	forceDirty   func(name string)            // dirty, --force set
	batchDirty   func(name string, err error) // dirty repo skipped in a batch
	singleStatus func(err error)              // status unreadable, single target; may be nil
	noneLeft     func(skipped int) error      // every target skipped
}

var globalUninstallPreflight = uninstallPreflightMessages{
	forceStatus: func(name string, err error) {
		ui.Warning("Could not check git status for %s (proceeding with --force): %v", name, err)
	},
	forceDirty: func(name string) {
		ui.Warning("Repository %s has uncommitted changes (proceeding with --force)", name)
	},
	batchDirty: func(name string, _ error) {
		ui.StepSkip(name, "uncommitted changes, use --force")
	},
	singleStatus: func(err error) { ui.Error("%v", err) },
	noneLeft: func(skipped int) error {
		if skipped > 0 {
			return fmt.Errorf("%d tracked repo%s skipped due to uncommitted changes; use --force to override", skipped, pluralS(skipped))
		}
		return fmt.Errorf("no skills to uninstall after pre-flight checks")
	},
}

var projectUninstallPreflight = uninstallPreflightMessages{
	forceStatus: func(_ string, err error) {
		ui.Warning("Could not check git status (proceeding with --force): %v", err)
	},
	forceDirty: func(string) {
		ui.Warning("Repository has uncommitted changes (proceeding with --force)")
	},
	batchDirty: func(name string, err error) {
		ui.Error("Repository has uncommitted changes!")
		ui.Note("Use --force to uninstall anyway, or commit/stash your changes first")
		ui.Warning("Skipping %s: %v", name, err)
	},
	noneLeft: func(int) error {
		return fmt.Errorf("no skills to uninstall after pre-flight checks")
	},
}

// confirmUninstall prompts user for confirmation
func confirmUninstall(target *uninstallTarget, suffix string) (bool, error) {
	kind := ""
	if target.isTrackedRepo {
		kind = "tracked repository "
	} else if len(countGroupSkills(target.path)) > 0 {
		kind = "group "
	}

	return ui.ConfirmAction(fmt.Sprintf("Uninstall %s%s%s? %s", kind, target.name, suffix, uninstallTrashHint()), false)
}

func uninstallTrashHint() string {
	return theme.Dim().Render("moved to trash for 7 days")
}

// printUninstallNextSteps suggests syncing, undoing the uninstall and, when
// the skill came from a remote source, reinstalling it later.
func printUninstallNextSteps(mode *uninstallMode, name, source string) {
	cleanupUninstallTrash(mode)
	next := []string{
		"skillshare sync", mode.syncNote("it"),
		"skillshare trash restore " + name + mode.reinstallFlags, "undo",
	}
	if source != "" {
		next = append(next, "skillshare install "+source+mode.reinstallFlags, "reinstall it later")
	}
	ui.Next(next...)
}

// syncNote says what sync does after an uninstall; pronoun is "it" or "them".
func (m *uninstallMode) syncNote(pronoun string) string {
	switch {
	case len(m.targetNames) == 0:
		return "update your targets"
	case len(m.targetNames) > 3:
		return "remove " + pronoun + " from " + plural(len(m.targetNames), "target")
	}
	return "remove " + pronoun + " from " + strings.Join(m.targetNames, ", ")
}

// cleanupUninstallTrash opportunistically removes expired trash items.
func cleanupUninstallTrash(mode *uninstallMode) {
	if n, _ := trash.Cleanup(mode.trashDir, 0); n > 0 {
		ui.Note(fmt.Sprintf("Cleaned up %d expired trash item%s", n, pluralS(n)))
	}
}

// runUninstallSkills resolves, checks, confirms and removes skills for either
// mode. rawArgs feed the oplog entry.
func runUninstallSkills(opts *uninstallOptions, mode *uninstallMode, rawArgs []string, start time.Time) error {
	// In JSON mode the UI goes to /dev/null; each JSON write flushes it first.
	jsonUI := newJSONUISuppressor(opts.jsonOutput)
	defer jsonUI.Flush()
	jsonError := func(err error) error {
		jsonUI.Flush()
		return writeJSONError(err)
	}
	jsonResult := func(removed, failed []string, skipped int, dryRun bool, err error) error {
		jsonUI.Flush()
		return uninstallOutputJSON(removed, failed, skipped, dryRun, start, err)
	}
	// --- Phase 1: RESOLVE ---
	var targets []*uninstallTarget
	seen := map[string]bool{} // dedup by path
	var resolveWarnings []string

	if opts.all {
		var sp *ui.Spinner
		if !opts.jsonOutput {
			sp = ui.StartSpinner("Discovering skills...")
		}
		discovered, _, err := sync.DiscoverSourceSkillsLite(mode.sourceDir, mode.walk)
		if err != nil {
			if sp != nil {
				sp.Fail("Discovery failed")
			}
			discoverErr := fmt.Errorf("failed to discover skills: %w", err)
			if opts.jsonOutput {
				return jsonError(discoverErr)
			}
			return discoverErr
		}
		if sp != nil {
			sp.Stop()
		}
		if len(discovered) == 0 {
			noSkillsErr := fmt.Errorf("%s", mode.emptySourceErr)
			if opts.jsonOutput {
				return jsonError(noSkillsErr)
			}
			return noSkillsErr
		}
		// Collect unique top-level directories to avoid nested skill duplication
		topDirs := map[string]bool{}
		for _, d := range discovered {
			topDirs[topLevelDir(d.RelPath)] = true
		}
		for dir := range topDirs {
			skillPath := filepath.Join(mode.sourceDir, dir)
			targets = append(targets, &uninstallTarget{
				name:          dir,
				path:          skillPath,
				isTrackedRepo: install.IsTrackedCheckout(skillPath),
			})
			seen[skillPath] = true
		}
	}

	for _, name := range opts.skillNames {
		// Glob pattern matching (e.g. "core-*", "_team-?")
		if mode.globs && isGlobPattern(name) {
			globMatches, globErr := resolveUninstallByGlob(name, mode.sourceDir, mode.walk)
			if globErr != nil {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, globErr))
				continue
			}
			if len(globMatches) == 0 {
				resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: no skills match pattern", name))
				continue
			}
			if !opts.jsonOutput {
				ui.Note(fmt.Sprintf("Pattern '%s' matched %s", name, plural(len(globMatches), "item")))
			}
			for _, t := range globMatches {
				if !seen[t.path] {
					seen[t.path] = true
					targets = append(targets, t)
				}
			}
			continue
		}

		t, err := resolveUninstallTarget(name, mode.sourceDir, mode.sourceLabel, mode.walk)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !seen[t.path] {
			seen[t.path] = true
			targets = append(targets, t)
		}
	}

	for _, group := range opts.groups {
		groupTargets, err := resolveGroupSkills(group, mode.sourceDir, mode.walk)
		if err != nil {
			resolveWarnings = append(resolveWarnings, fmt.Sprintf("--group %s: %v", group, err))
			continue
		}
		for _, t := range groupTargets {
			if !seen[t.path] {
				seen[t.path] = true
				targets = append(targets, t)
			}
		}
	}

	if !opts.jsonOutput {
		for _, w := range resolveWarnings {
			ui.Warning("%s", w)
		}
	}

	// Shell glob detection: if positional args look like shell-expanded filenames,
	// intercept early and suggest --all instead
	if !opts.all && looksLikeShellGlob(opts.skillNames, resolveWarnings) {
		globErr := fmt.Errorf("shell glob expansion detected")
		if opts.jsonOutput {
			return jsonError(globErr)
		}
		ui.Warning("It looks like '*' was expanded by your shell into file names.")
		ui.Note("To uninstall all skills, use: skillshare uninstall --all")
		return globErr
	}

	// --- Phase 2: VALIDATE ---
	if len(targets) == 0 {
		var noTargetsErr error
		if len(resolveWarnings) > 0 {
			noTargetsErr = fmt.Errorf("no valid skills to uninstall")
		} else {
			noTargetsErr = fmt.Errorf("no skills found")
		}
		if opts.jsonOutput {
			return jsonError(noTargetsErr)
		}
		return noTargetsErr
	}
	// Preflight refuses a linked folder that is itself a skill before it looks
	// at git state, so a dirty checkout is never told to retry with --force.
	checkItems := mode.items(targets)
	if opts.dryRun {
		for i := range checkItems {
			checkItems[i].Repo = false // a dry run reports no git state
		}
	}
	checks := uninstall.Preflight(checkItems, mode.options())
	for _, err := range checks {
		var gitErr *uninstall.StatusError
		if err == nil || errors.Is(err, uninstall.ErrDirty) || errors.As(err, &gitErr) {
			continue // tracked-repo state, reported in the pre-flight phase
		}
		if opts.jsonOutput {
			return jsonError(err)
		}
		return err
	}

	// --- Phase 3: DISPLAY ---
	single := len(targets) == 1
	summary := summarizeUninstallTargets(targets)
	if opts.jsonOutput {
		// Skip display in JSON mode
	} else if single {
		displayUninstallInfo(targets[0])
	} else if !opts.force && !opts.dryRun {
		// The list is for the confirmation; the result rows name each target.
		displayUninstallBatch(targets, summary)
	}

	// --- Phase 4: PRE-FLIGHT ---
	var preflightSkipped int
	var preflightFailed []string
	if !opts.dryRun {
		var preflight []*uninstallTarget
		for i, t := range targets {
			if !t.isTrackedRepo {
				preflight = append(preflight, t)
				continue
			}
			var gitErr *uninstall.StatusError
			if errors.As(checks[i], &gitErr) {
				if opts.force {
					if !opts.jsonOutput {
						mode.preflight.forceStatus(t.name, gitErr.Err)
					}
					preflight = append(preflight, t)
					continue
				}
				statusErr := gitErr
				if single {
					if opts.jsonOutput {
						return jsonError(statusErr)
					}
					if mode.preflight.singleStatus != nil {
						mode.preflight.singleStatus(statusErr)
					}
					return statusErr
				}
				if !opts.jsonOutput {
					ui.StepFail(t.name, statusErr.Error())
				}
				preflightFailed = append(preflightFailed, fmt.Sprintf("%s: %v", t.name, statusErr))
				continue
			}
			if checks[i] == nil {
				preflight = append(preflight, t)
				continue
			}
			// Repo is dirty
			if !opts.force {
				dirtyErr := fmt.Errorf("uncommitted changes detected, use --force to override")
				if single {
					if opts.jsonOutput {
						return jsonError(dirtyErr)
					}
					ui.Error("Repository has uncommitted changes!")
					ui.Note("Use --force to uninstall anyway, or commit/stash your changes first")
					return dirtyErr
				}
				if !opts.jsonOutput {
					mode.preflight.batchDirty(t.name, dirtyErr)
				}
				continue
			}
			if !opts.jsonOutput {
				mode.preflight.forceDirty(t.name)
			}
			preflight = append(preflight, t)
		}
		preflightSkipped = len(targets) - len(preflight) - len(preflightFailed)
		targets = preflight
		summary = summarizeUninstallTargets(targets)

		if preflightSkipped > 0 && !opts.jsonOutput {
			ui.Note(fmt.Sprintf("%d tracked repo%s skipped, %d remaining", preflightSkipped, pluralS(preflightSkipped), len(targets)))
			fmt.Println()
		}

		if len(targets) == 0 {
			preflightErr := mode.preflight.noneLeft(preflightSkipped)
			if len(preflightFailed) > 0 {
				preflightErr = fmt.Errorf("%s", strings.Join(preflightFailed, "; "))
			}
			if opts.jsonOutput {
				return jsonError(preflightErr)
			}
			return preflightErr
		}
	}

	// --- Phase 5: DRY-RUN or CONFIRM ---
	if opts.dryRun {
		if opts.jsonOutput {
			dryRunNames := make([]string, len(targets))
			for i, t := range targets {
				dryRunNames[i] = t.name
			}
			logUninstallOp(mode.configPath, uninstallOpNames(rawArgs), 0, start, nil)
			return jsonResult(dryRunNames, nil, preflightSkipped, true, nil)
		}
		names := make([]string, len(targets))
		for i, t := range targets {
			names[i] = t.name
		}
		width := ui.RowWidth(names...)
		for _, t := range targets {
			value := "would move to trash"
			if !single {
				value += " " + ui.DimText("· "+batchTargetKind(t, summary))
			}
			ui.Row(ui.MarkNone, t.name, value, width)
			if line := mode.dryRunGitignore(t); line != "" {
				ui.Note(line)
			}
			if entry := mode.store.Get(t.name); entry != nil && entry.Source != "" {
				ui.Note("reinstall with skillshare install " + entry.Source + mode.reinstallFlags)
			}
		}
		fmt.Println()
		ui.DryRun()
		return nil
	}

	if !opts.force && !opts.jsonOutput {
		var confirmed bool
		var err error
		if single {
			confirmed, err = confirmUninstall(targets[0], mode.confirmSuffix)
		} else {
			confirmed, err = ui.ConfirmAction(fmt.Sprintf("Uninstall %d %s%s? %s", len(targets), summarizeUninstallTargets(targets).noun(), mode.confirmSuffix, uninstallTrashHint()), false)
		}
		if err != nil {
			return err
		}
		if !confirmed {
			ui.Cancelled("removed")
			return nil
		}
	}

	// --- Phase 6: EXECUTE ---
	batch := len(targets) > 1
	var singleSource string // reinstall source of a single target
	if entry := mode.store.Get(targets[0].name); !batch && entry != nil {
		singleSource = entry.Source
	}
	var sp *ui.Spinner
	if batch && !opts.jsonOutput {
		sp = ui.StartSpinner(fmt.Sprintf("Uninstalling %d %s", len(targets), summary.noun()))
	}
	// The pre-flight phase already refused or waived every dirty repo.
	runOpts := mode.options()
	runOpts.Force = true
	out := uninstall.Run(mode.items(targets), runOpts)
	if sp != nil {
		sp.Stop()
	}

	var succeeded []*uninstallTarget
	failed := preflightFailed
	for i, r := range out.Results {
		if r.Err == nil {
			succeeded = append(succeeded, targets[i])
		} else {
			failed = append(failed, fmt.Sprintf("%s: %v", targets[i].name, r.Err))
		}
	}

	switch {
	case opts.jsonOutput:
	case batch:
		printUninstallBatch(targets, out.Results, summary, len(failed), preflightSkipped, mode, start)
	case out.Results[0].Err == nil:
		fmt.Printf("%s Uninstall %s %s\n", ui.StyledMark(ui.MarkOK), targets[0].name, ui.DimText("→ trash, kept 7 days"))
		if out.GitignoreErr != nil {
			ui.Warning("Could not update .gitignore: %v", out.GitignoreErr)
		}
	default:
		err := out.Results[0].Err
		var trashErr *uninstall.TrashError
		if errors.As(err, &trashErr) {
			err = trashErr.Err // a single target reports the bare cause
		}
		if mode.reportAfterFinalize || errors.Is(err, sourcefs.ErrLinkedSkillRoot) {
			ui.Warning("Failed to uninstall %s: %v", targets[0].name, err)
		}
	}

	// --- Phase 7: FINALIZE ---
	if out.SaveErr != nil {
		ui.Warning("Failed to update metadata after uninstall: %v", out.SaveErr)
	}
	if len(succeeded) > 0 && mode.afterRemove != nil {
		removedNames := map[string]bool{}
		for _, t := range succeeded {
			removedNames[t.name] = true
		}
		mode.afterRemove(removedNames)
	}

	if !batch && !opts.jsonOutput && len(succeeded) == 1 {
		printUninstallNextSteps(mode, succeeded[0].name, singleSource)
	}

	var finalErr error
	if len(failed) > 0 && len(succeeded) == 0 {
		finalErr = fmt.Errorf("all uninstalls failed: %s", strings.Join(failed, "; "))
	}
	// Partial failure: report but exit success (skip & continue)

	logUninstallOp(mode.configPath, uninstallOpNames(rawArgs), len(succeeded), start, finalErr)

	if opts.jsonOutput {
		removedNames := make([]string, len(succeeded))
		for i, t := range succeeded {
			removedNames[i] = t.name
		}
		return jsonResult(removedNames, failed, preflightSkipped, opts.dryRun, finalErr)
	}
	return finalErr
}

// displayUninstallBatch lists the targets with what each holds. A long list
// names only groups and tracked repos and counts the plain skills.
func displayUninstallBatch(targets []*uninstallTarget, summary uninstallTypeSummary) {
	long := len(targets) > 20
	var shown []*uninstallTarget
	for _, t := range targets {
		if !long || t.isTrackedRepo || summary.groupSkillCount[t.path] > 0 {
			shown = append(shown, t)
		}
	}
	nameW := 0
	for _, t := range shown {
		nameW = max(nameW, runewidth.StringWidth(t.name))
	}
	for _, t := range shown {
		fmt.Printf("  %s  %s\n", pad(t.name, nameW), ui.DimText(batchTargetKind(t, summary)))
	}
	if long && summary.skills > 0 {
		ui.Note(fmt.Sprintf("… and %s", plural(summary.skills, "skill")))
	}
}

func batchTargetKind(t *uninstallTarget, summary uninstallTypeSummary) string {
	if t.isTrackedRepo {
		return "tracked repository"
	}
	if c := summary.groupSkillCount[t.path]; c > 0 {
		return "group, " + plural(c, "skill")
	}
	return "skill"
}

// printUninstallBatch prints the condensed result of a batch: failures one by
// one, successes condensed when many, then the closing line. failed counts
// pre-flight failures too.
func printUninstallBatch(targets []*uninstallTarget, results []uninstall.Result, summary uninstallTypeSummary, failed, skipped int, mode *uninstallMode, start time.Time) {
	type batchResult struct {
		target    *uninstallTarget
		typeLabel string
		errMsg    string
	}

	// Failures always shown individually
	var successes []batchResult
	var failures []batchResult
	for i, r := range results {
		t := targets[i]
		switch c := summary.groupSkillCount[t.path]; {
		case r.Err != nil:
			failures = append(failures, batchResult{target: t, errMsg: r.Err.Error()})
		case t.isTrackedRepo:
			successes = append(successes, batchResult{target: t, typeLabel: "tracked repo"})
		case c > 0:
			successes = append(successes, batchResult{target: t, typeLabel: fmt.Sprintf("group, %d skill%s", c, pluralS(c))})
		default:
			successes = append(successes, batchResult{target: t, typeLabel: "skill"})
		}
	}

	if len(failures) > 0 {
		ui.SectionLabel("Failed")
		for _, r := range failures {
			ui.StepFail(r.target.name, r.errMsg)
		}
	}

	// Successes: condensed when many
	if len(successes) > 0 {
		ui.SectionLabel("Removed")
		switch {
		case len(successes) > 50:
			ui.StepDone(fmt.Sprintf("%d uninstalled", len(successes)), "")
		case len(successes) > 10:
			const maxShown = 10
			names := make([]string, 0, maxShown)
			for i := 0; i < maxShown && i < len(successes); i++ {
				names = append(names, successes[i].target.name)
			}
			detail := strings.Join(names, ", ")
			if len(successes) > maxShown {
				detail = fmt.Sprintf("%s ... +%d more", detail, len(successes)-maxShown)
			}
			ui.StepDone(fmt.Sprintf("%d uninstalled", len(successes)), detail)
		default:
			for _, r := range successes {
				ui.StepDone(r.target.name, ui.DimText(r.typeLabel))
			}
		}
	}

	mark, parts := ui.MarkOK, []string{}
	switch {
	case len(successes) == 0:
		mark, parts = ui.MarkFail, append(parts, fmt.Sprintf("Failed to uninstall %d %s", failed, summary.noun()))
	case failed > 0:
		mark, parts = ui.MarkWarn, append(parts, fmt.Sprintf("Uninstalled %d", len(successes)), fmt.Sprintf("%d failed", failed))
	default:
		parts = append(parts, fmt.Sprintf("Uninstalled %d %s", len(successes), summary.noun()))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	fmt.Println()
	ui.Done(mark, strings.Join(parts, ", "), time.Since(start))

	if len(successes) > 0 {
		cleanupUninstallTrash(mode)
		ui.Next(
			"skillshare sync", mode.syncNote("them"),
			"skillshare trash list"+mode.reinstallFlags, "restore any of them within 7 days",
		)
	}
}
