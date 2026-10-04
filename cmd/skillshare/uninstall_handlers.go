package main

import (
	"fmt"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/mattn/go-runewidth"

	"skillshare/internal/git"
	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/trash"
	"skillshare/internal/ui"
)

// uninstallMode holds what differs between global and project skill uninstall.
type uninstallMode struct {
	sourceDir   string
	sourceLabel string // names sourceDir in not-found errors
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

	dryRunGitignore  func(t *uninstallTarget) string // "" prints no line
	gitignoreEntries func(succeeded []*uninstallTarget) (dir string, entries []string)
	afterRemove      func(removed map[string]bool) // extra state cleanup, may be nil
	preflight        uninstallPreflightMessages
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

// performUninstallQuiet moves the skill to trash without printing output.
// Used by batch mode; returns the type label for StepDone display.
// Note: .gitignore cleanup is handled in batch by the caller.
func performUninstallQuiet(target *uninstallTarget, sourceDir, trashDir string) (typeLabel string, err error) {
	groupSkillCount := 0
	if !target.isTrackedRepo {
		groupSkillCount = len(countGroupSkills(target.path))
	}

	if err := sourcefs.CheckMoveOut(sourceDir, target.path); err != nil {
		return "", err
	}
	if _, err := trash.MoveToTrash(target.path, target.name, trashDir); err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}

	if target.isTrackedRepo {
		return "tracked repo", nil
	}
	if groupSkillCount > 0 {
		return fmt.Sprintf("group, %d skill%s", groupSkillCount, pluralS(groupSkillCount)), nil
	}
	return "skill", nil
}

// performUninstall moves the skill to trash (verbose single-target output).
// Note: .gitignore cleanup is handled in batch by the caller.
func performUninstall(target *uninstallTarget, mode *uninstallMode) error {
	if err := sourcefs.CheckMoveOut(mode.sourceDir, target.path); err != nil {
		return err
	}
	if _, err := trash.MoveToTrash(target.path, target.name, mode.trashDir); err != nil {
		return err
	}
	fmt.Printf("%s Uninstall %s %s\n", ui.StyledMark(ui.MarkOK), target.name, ui.DimText("→ trash, kept 7 days"))
	return nil
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

// removeUninstalledFromGitignore batch-removes .gitignore entries for succeeded
// targets in one read/write pass.
func removeUninstalledFromGitignore(mode *uninstallMode, succeeded []*uninstallTarget) error {
	if len(succeeded) == 0 {
		return nil
	}
	dir, entries := mode.gitignoreEntries(succeeded)
	if dir == "" || len(entries) == 0 {
		return nil
	}
	_, err := install.RemoveFromGitIgnoreBatch(dir, entries)
	return err
}

// runUninstallSkills resolves, checks, confirms and removes skills for either
// mode. rawArgs feed the oplog entry.
func runUninstallSkills(opts *uninstallOptions, mode *uninstallMode, rawArgs []string, start time.Time) error {
	// --- Phase 1: RESOLVE ---
	var targets []*uninstallTarget
	seen := map[string]bool{} // dedup by path
	var resolveWarnings []string

	if opts.all {
		var sp *ui.Spinner
		if !opts.jsonOutput {
			sp = ui.StartSpinner("Discovering skills...")
		}
		discovered, _, err := sync.DiscoverSourceSkillsLite(mode.sourceDir)
		if err != nil {
			if sp != nil {
				sp.Fail("Discovery failed")
			}
			discoverErr := fmt.Errorf("failed to discover skills: %w", err)
			if opts.jsonOutput {
				return writeJSONError(discoverErr)
			}
			return discoverErr
		}
		if sp != nil {
			sp.Stop()
		}
		if len(discovered) == 0 {
			noSkillsErr := fmt.Errorf("%s", mode.emptySourceErr)
			if opts.jsonOutput {
				return writeJSONError(noSkillsErr)
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
				isTrackedRepo: install.IsGitRepo(skillPath),
			})
			seen[skillPath] = true
		}
	}

	for _, name := range opts.skillNames {
		// Glob pattern matching (e.g. "core-*", "_team-?")
		if mode.globs && isGlobPattern(name) {
			globMatches, globErr := resolveUninstallByGlob(name, mode.sourceDir)
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

		t, err := resolveUninstallTarget(name, mode.sourceDir, mode.sourceLabel)
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
		groupTargets, err := resolveGroupSkills(group, mode.sourceDir)
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
			return writeJSONError(globErr)
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
			return writeJSONError(noTargetsErr)
		}
		return noTargetsErr
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
		dirtyResults := checkUninstallTargetsDirty(targets)

		var preflight []*uninstallTarget
		for i, t := range targets {
			if !t.isTrackedRepo {
				preflight = append(preflight, t)
				continue
			}
			dr := dirtyResults[i]
			if dr.err != nil {
				if opts.force {
					if !opts.jsonOutput {
						mode.preflight.forceStatus(t.name, dr.err)
					}
					preflight = append(preflight, t)
					continue
				}
				statusErr := &gitStatusError{err: dr.err}
				if single {
					if opts.jsonOutput {
						return writeJSONError(statusErr)
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
			if !dr.dirty {
				preflight = append(preflight, t)
				continue
			}
			// Repo is dirty
			if !opts.force {
				dirtyErr := fmt.Errorf("uncommitted changes detected, use --force to override")
				if single {
					if opts.jsonOutput {
						return writeJSONError(dirtyErr)
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
				return writeJSONError(preflightErr)
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
			return uninstallOutputJSON(dryRunNames, nil, preflightSkipped, true, start, nil)
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
	var succeeded []*uninstallTarget
	var singleSource string // reinstall source of a single target
	failed := preflightFailed

	if opts.jsonOutput {
		// JSON mode: quiet execution, no UI output
		for _, t := range targets {
			if _, err := performUninstallQuiet(t, mode.sourceDir, mode.trashDir); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
			} else {
				succeeded = append(succeeded, t)
			}
		}
		removeUninstalledFromGitignore(mode, succeeded) //nolint:errcheck
	} else if batch {
		succeeded, failed = executeUninstallBatch(targets, summary, failed, preflightSkipped, mode, start)
	} else {
		for _, t := range targets {
			if entry := mode.store.Get(t.name); entry != nil {
				singleSource = entry.Source
			}
			if err := performUninstall(t, mode); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
				if mode.reportAfterFinalize {
					ui.Warning("Failed to uninstall %s: %v", t.name, err)
				}
			} else {
				succeeded = append(succeeded, t)
			}
		}

		// Batch-remove .gitignore entries after all targets processed.
		if err := removeUninstalledFromGitignore(mode, succeeded); err != nil {
			ui.Warning("Could not update .gitignore: %v", err)
		}
	}

	// --- Phase 7: FINALIZE ---
	// Batch-remove succeeded skills from metadata store
	if len(succeeded) > 0 {
		removedNames := map[string]bool{}
		for _, t := range succeeded {
			removedNames[t.name] = true
		}
		mode.store.RemoveByNames(removedNames)
		if saveErr := mode.store.Save(mode.sourceDir); saveErr != nil {
			ui.Warning("Failed to update metadata after uninstall: %v", saveErr)
		}
		if mode.afterRemove != nil {
			mode.afterRemove(removedNames)
		}
	}

	if !batch && !opts.jsonOutput && len(succeeded) == 1 {
		printUninstallNextSteps(mode, succeeded[0].name, singleSource)
	}

	var finalErr error
	if len(failed) > 0 && len(succeeded) == 0 {
		finalErr = fmt.Errorf("all uninstalls failed")
	}
	// Partial failure: report but exit success (skip & continue)

	logUninstallOp(mode.configPath, uninstallOpNames(rawArgs), len(succeeded), start, finalErr)

	if opts.jsonOutput {
		removedNames := make([]string, len(succeeded))
		for i, t := range succeeded {
			removedNames[i] = t.name
		}
		return uninstallOutputJSON(removedNames, failed, preflightSkipped, opts.dryRun, start, finalErr)
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

type uninstallDirtyResult struct {
	dirty bool
	err   error
}

// checkUninstallTargetsDirty runs git dirty checks for tracked repos in
// parallel, keyed by target index.
func checkUninstallTargetsDirty(targets []*uninstallTarget) map[int]uninstallDirtyResult {
	dirtyResults := make(map[int]uninstallDirtyResult)

	var trackedIndices []int
	for i, t := range targets {
		if t.isTrackedRepo {
			trackedIndices = append(trackedIndices, i)
		}
	}
	if len(trackedIndices) == 0 {
		return dirtyResults
	}

	const maxDirtyWorkers = 8
	results := make([]uninstallDirtyResult, len(trackedIndices))
	sem := make(chan struct{}, maxDirtyWorkers)
	var wg gosync.WaitGroup

	for j, idx := range trackedIndices {
		wg.Add(1)
		sem <- struct{}{}
		go func(slot int, t *uninstallTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			dirty, err := git.IsDirty(t.path)
			results[slot] = uninstallDirtyResult{dirty: dirty, err: err}
		}(j, targets[idx])
	}
	wg.Wait()

	for j, idx := range trackedIndices {
		dirtyResults[idx] = results[j]
	}
	return dirtyResults
}

// executeUninstallBatch removes several targets behind a spinner and prints
// the condensed result. It returns the succeeded targets and all failures.
func executeUninstallBatch(targets []*uninstallTarget, summary uninstallTypeSummary, failed []string, skipped int, mode *uninstallMode, start time.Time) ([]*uninstallTarget, []string) {
	type batchResult struct {
		target    *uninstallTarget
		typeLabel string
		errMsg    string
	}

	var succeeded []*uninstallTarget
	sp := ui.StartSpinner(fmt.Sprintf("Uninstalling %d %s", len(targets), summary.noun()))
	var results []batchResult

	for _, t := range targets {
		typeLabel, err := performUninstallQuiet(t, mode.sourceDir, mode.trashDir)
		if err != nil {
			results = append(results, batchResult{target: t, errMsg: err.Error()})
			failed = append(failed, fmt.Sprintf("%s: %v", t.name, err))
		} else {
			results = append(results, batchResult{target: t, typeLabel: typeLabel})
			succeeded = append(succeeded, t)
		}
	}

	removeUninstalledFromGitignore(mode, succeeded) //nolint:errcheck
	sp.Stop()

	// Failures always shown individually
	var successes []batchResult
	var failures []batchResult
	for _, r := range results {
		if r.errMsg != "" {
			failures = append(failures, r)
		} else {
			successes = append(successes, r)
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
	case len(succeeded) == 0:
		mark, parts = ui.MarkFail, append(parts, fmt.Sprintf("Failed to uninstall %d %s", len(failed), summary.noun()))
	case len(failed) > 0:
		mark, parts = ui.MarkWarn, append(parts, fmt.Sprintf("Uninstalled %d", len(succeeded)), fmt.Sprintf("%d failed", len(failed)))
	default:
		parts = append(parts, fmt.Sprintf("Uninstalled %d %s", len(succeeded), summary.noun()))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	fmt.Println()
	ui.Done(mark, strings.Join(parts, ", "), time.Since(start))

	if len(succeeded) > 0 {
		cleanupUninstallTrash(mode)
		ui.Next(
			"skillshare sync", mode.syncNote("them"),
			"skillshare trash list"+mode.reinstallFlags, "restore any of them within 7 days",
		)
	}
	return succeeded, failed
}
