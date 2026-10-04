package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/oplog"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"
)

type collectOptions struct {
	dryRun     bool
	force      bool
	collectAll bool
	jsonOutput bool
	targetName string
}

type collectLogSummary struct {
	Kind    string
	Scope   string
	Pulled  int
	Skipped int
	Failed  int
	DryRun  bool
	Force   bool
}

type collectDisplayItem struct {
	Name       string
	TargetName string
	Path       string
}

// collectJSONOutput is the JSON representation for collect --json output.
type collectJSONOutput struct {
	Pulled   []string          `json:"pulled"`
	Skipped  []string          `json:"skipped"`
	Failed   map[string]string `json:"failed"`
	DryRun   bool              `json:"dry_run"`
	Duration string            `json:"duration"`
}

func parseCollectOptions(args []string) collectOptions {
	opts := collectOptions{}
	for _, arg := range args {
		switch arg {
		case "--dry-run", "-n":
			opts.dryRun = true
		case "--force", "-f":
			opts.force = true
		case "--all", "-a":
			opts.collectAll = true
		case "--json":
			opts.jsonOutput = true
		default:
			if opts.targetName == "" && !strings.HasPrefix(arg, "-") {
				opts.targetName = arg
			}
		}
	}
	return opts
}

func newCollectLogSummary(kind resourceKindFilter, scope string, opts collectOptions) collectLogSummary {
	return collectLogSummary{
		Kind:   kind.String(),
		Scope:  scope,
		DryRun: opts.dryRun,
		Force:  opts.force,
	}
}

func updateCollectLogSummary(summary collectLogSummary, result *sync.PullResult) collectLogSummary {
	if result == nil {
		return summary
	}
	summary.Pulled = len(result.Pulled)
	summary.Skipped = len(result.Skipped)
	summary.Failed = len(result.Failed)
	return summary
}

func collectCommandError(err error, jsonOutput bool) error {
	if err == nil {
		return nil
	}
	if jsonOutput {
		return writeJSONError(err)
	}
	return err
}

// collectPlan describes what to collect (kind + source) and how to scan for it.
// The scan callback is called lazily so the spinner wraps the actual I/O.
type collectPlan struct {
	kind   resourceKindFilter
	source string
	follow *sourcewalk.FollowSet
	scan   func(warn bool) collectResources
}

// collectResources holds the results of scanning a target for local resources.
type collectResources struct {
	items []collectDisplayItem
	names []string
	pull  func(sync.PullOptions) (*sync.PullResult, error)
}

// toCollectResources converts a typed slice into collectResources.
// toDisplay maps each item to a collectDisplayItem; pull is the batch pull function.
func toCollectResources[T any](
	items []T,
	source string,
	toDisplay func(T) collectDisplayItem,
	pull func([]T, string, sync.PullOptions) (*sync.PullResult, error),
) collectResources {
	display := make([]collectDisplayItem, len(items))
	names := make([]string, len(items))
	for i, item := range items {
		d := toDisplay(item)
		display[i] = d
		names[i] = d.Name
	}
	return collectResources{
		items: display,
		names: names,
		pull: func(opts sync.PullOptions) (*sync.PullResult, error) {
			return pull(items, source, opts)
		},
	}
}

// runCollectPlan is the unified collection flow for both skills and agents.
func runCollectPlan(plan collectPlan, opts collectOptions, start time.Time, scope string) (collectLogSummary, error) {
	label := plan.kind.String()
	summary := newCollectLogSummary(plan.kind, scope, opts)

	var sp *ui.Spinner
	if !opts.jsonOutput {
		sp = ui.StartSpinner(fmt.Sprintf("Scanning for local %s...", label))
	}

	res := plan.scan(!opts.jsonOutput)
	if sp != nil {
		sp.Stop()
	}

	if len(res.items) == 0 {
		if opts.jsonOutput {
			return summary, collectOutputJSON(nil, opts.dryRun, start, nil)
		}
		ui.Done(ui.MarkNone, fmt.Sprintf("No local %s found", label), time.Since(start))
		return summary, nil
	}

	if !opts.jsonOutput {
		displayLocalCollectItems(fmt.Sprintf("Local %s in targets", label), res.items)
	}

	if opts.dryRun {
		result := &sync.PullResult{Pulled: res.names}
		summary = updateCollectLogSummary(summary, result)
		if opts.jsonOutput {
			return summary, collectOutputJSON(result, true, start, nil)
		}
		fmt.Println()
		ui.Done(ui.MarkNone, "Would collect "+plural(len(res.items), strings.TrimSuffix(label, "s")), 0)
		ui.DryRun()
		return summary, nil
	}

	if !opts.force && !opts.jsonOutput {
		ok, err := confirmCollect(label)
		if err != nil {
			return summary, err
		}
		if !ok {
			ui.Cancelled("collected")
			return summary, nil
		}
	}

	result, collectErr := res.pull(sync.PullOptions{
		DryRun: opts.dryRun,
		Force:  opts.force,
		Follow: plan.follow,
	})
	summary = updateCollectLogSummary(summary, result)
	if opts.jsonOutput {
		return summary, collectOutputJSON(result, opts.dryRun, start, collectErr)
	}
	if collectErr != nil {
		return summary, collectErr
	}
	return summary, renderCollectResult(label, result, plan.source, start)
}

func displayLocalCollectItems(title string, items []collectDisplayItem) {
	fmt.Println(theme.Primary().Bold(true).Render(title))
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	width := ui.RowWidth(names...)
	for _, item := range items {
		ui.Row(ui.MarkNone, item.Name, item.TargetName+ui.DimText(" · "+utils.FoldHomePath(item.Path)), width)
	}
}

func confirmCollect(resourceLabel string) (bool, error) {
	return ui.ConfirmAction(fmt.Sprintf("Collect these %s to source?", resourceLabel), false)
}

func renderCollectResult(resourceLabel string, result *sync.PullResult, source string, start time.Time) error {
	var names []string
	names = append(names, result.Pulled...)
	names = append(names, result.Skipped...)
	for name := range result.Failed {
		names = append(names, name)
	}
	width := ui.RowWidth(names...)

	fmt.Println()
	for _, name := range result.Pulled {
		ui.Row(ui.MarkOK, name, "copied to source", width)
	}
	for _, name := range result.Skipped {
		ui.Row(ui.MarkWarn, name, "already exists in source "+ui.DimText("· use --force to overwrite"), width)
	}
	for name, err := range result.Failed {
		ui.Row(ui.MarkFail, name, err.Error(), width)
	}

	mark, text := ui.MarkOK, "Collected "+plural(len(result.Pulled), strings.TrimSuffix(resourceLabel, "s"))
	if len(result.Skipped) > 0 {
		mark, text = ui.MarkWarn, fmt.Sprintf("%s, %d skipped", text, len(result.Skipped))
	}
	if len(result.Failed) > 0 {
		mark, text = ui.MarkWarn, fmt.Sprintf("%s, %d failed", text, len(result.Failed))
		if len(result.Pulled) == 0 {
			mark = ui.MarkFail
		}
	}
	fmt.Println()
	ui.Done(mark, text, time.Since(start))

	if len(result.Pulled) > 0 {
		showCollectNextSteps(resourceLabel, source)
	}

	return nil
}

// collectOutputJSON converts a collect result to JSON and writes to stdout.
func collectOutputJSON(result *sync.PullResult, dryRun bool, start time.Time, collectErr error) error {
	output := collectJSONOutput{
		DryRun:   dryRun,
		Duration: formatDuration(start),
		Failed:   make(map[string]string),
	}
	if result != nil {
		output.Pulled = result.Pulled
		output.Skipped = result.Skipped
		for k, v := range result.Failed {
			output.Failed[k] = v.Error()
		}
	}
	return writeJSONResult(&output, collectErr)
}

func showCollectNextSteps(resourceLabel, source string) {
	syncCmd := "skillshare sync"
	if ui.ModeLabel == "project" {
		syncCmd += " -p"
	}
	if resourceLabel == "agents" {
		syncCmd += " agents"
	}
	next := []string{syncCmd, "link them into every target"}

	// commit works on the global source repository only
	gitDir := filepath.Join(source, ".git")
	if _, err := os.Stat(gitDir); err == nil && ui.ModeLabel != "project" {
		next = append(next, "skillshare commit", "save them in git")
	}
	ui.Next(next...)
}

func logCollectOp(cfgPath string, start time.Time, cmdErr error, summary collectLogSummary) {
	status := statusFromErr(cmdErr)
	if cmdErr == nil && summary.Failed > 0 {
		status = "partial"
	}

	e := oplog.NewEntry("collect", status, time.Since(start))
	e.Args = map[string]any{
		"kind":    summary.Kind,
		"scope":   summary.Scope,
		"pulled":  summary.Pulled,
		"skipped": summary.Skipped,
		"failed":  summary.Failed,
		"dry_run": summary.DryRun,
		"force":   summary.Force,
	}
	if cmdErr != nil {
		e.Message = cmdErr.Error()
	}
	oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck
}
