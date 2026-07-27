package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"skillshare/internal/oplog"
	"skillshare/internal/sync"
	"skillshare/internal/ui"
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
		header := "Collect"
		if plan.kind == kindAgents {
			header = "Collect agents"
		}
		ui.Header(ui.WithModeLabel(header))
		sp = ui.StartSpinner(fmt.Sprintf("Scanning for local %s...", label))
	}

	res := plan.scan(!opts.jsonOutput)

	if len(res.items) == 0 {
		if sp != nil {
			sp.Success(fmt.Sprintf("No local %s found", label))
		}
		if opts.jsonOutput {
			return summary, collectOutputJSON(nil, opts.dryRun, start, nil)
		}
		return summary, nil
	}

	if sp != nil {
		sp.Success(fmt.Sprintf("Found %d local %s", len(res.items), label))
		displayLocalCollectItems(fmt.Sprintf("Local %s found", label), res.items)
	}

	if opts.dryRun {
		result := &sync.PullResult{Pulled: res.names}
		summary = updateCollectLogSummary(summary, result)
		if opts.jsonOutput {
			return summary, collectOutputJSON(result, true, start, nil)
		}
		ui.Info("Dry run - no changes made")
		return summary, nil
	}

	if !opts.force && !opts.jsonOutput {
		if !confirmCollect(label) {
			ui.Info("Cancelled")
			return summary, nil
		}
	}

	result, collectErr := res.pull(sync.PullOptions{
		DryRun: opts.dryRun,
		Force:  opts.force,
	})
	summary = updateCollectLogSummary(summary, result)
	if opts.jsonOutput {
		return summary, collectOutputJSON(result, opts.dryRun, start, collectErr)
	}
	if collectErr != nil {
		return summary, collectErr
	}
	return summary, renderCollectResult(label, result, plan.source)
}

func displayLocalCollectItems(title string, items []collectDisplayItem) {
	ui.Header(ui.WithModeLabel(title))
	for _, item := range items {
		ui.ListItem("info", item.Name, fmt.Sprintf("[%s] %s", item.TargetName, item.Path))
	}
}

func confirmCollect(resourceLabel string) bool {
	fmt.Println()
	fmt.Printf("Collect these %s to source? [y/N]: ", resourceLabel)
	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

func renderCollectResult(resourceLabel string, result *sync.PullResult, source string) error {
	ui.Header(ui.WithModeLabel("Collecting " + resourceLabel))

	for _, name := range result.Pulled {
		ui.StepDone(name, "copied to source")
	}
	for _, name := range result.Skipped {
		ui.StepSkip(name, "already exists in source, use --force to overwrite")
	}
	for name, err := range result.Failed {
		ui.StepFail(name, err.Error())
	}

	ui.OperationSummary("Collect", 0,
		ui.Metric{Label: "collected", Count: len(result.Pulled), HighlightColor: pterm.Green},
		ui.Metric{Label: "skipped", Count: len(result.Skipped), HighlightColor: pterm.Yellow},
		ui.Metric{Label: "failed", Count: len(result.Failed), HighlightColor: pterm.Red},
	)

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
	fmt.Println()
	if resourceLabel == "agents" {
		if ui.ModeLabel == "project" {
			ui.Info("Run 'skillshare sync -p agents' to distribute to all agent targets")
		} else {
			ui.Info("Run 'skillshare sync agents' to distribute to all agent targets")
		}
	} else if ui.ModeLabel == "project" {
		ui.Info("Run 'skillshare sync -p' to distribute to all targets")
	} else {
		ui.Info("Run 'skillshare sync' to distribute to all targets")
	}

	gitDir := filepath.Join(source, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		ui.Info("Commit changes: cd %s && git add . && git commit", source)
	}
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
