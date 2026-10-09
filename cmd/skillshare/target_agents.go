package main

import (
	"fmt"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sync"
	"skillshare/internal/targetsummary"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
)

func applyTargetListAgentSummary(item *targetListJSONItem, summary *targetsummary.AgentSummary) {
	if summary == nil {
		return
	}

	item.AgentPath = summary.Path
	item.AgentMode = summary.Mode
	item.AgentSync = formatTargetAgentSyncSummary(summary)
	item.AgentInclude = append([]string(nil), summary.Include...)
	item.AgentExclude = append([]string(nil), summary.Exclude...)
	item.AgentLinkedCount = intPtr(summary.ManagedCount)
	item.AgentLocalCount = intPtr(summary.LocalCount)
	item.AgentExpectedCount = intPtr(summary.ExpectedCount)
}

func newTargetListJSONItem(item targetTUIItem) targetListJSONItem {
	sc := item.target.SkillsConfig()

	jsonItem := targetListJSONItem{
		Name:         item.name,
		Path:         sc.Path,
		Mode:         sync.EffectiveMode(sc.Mode),
		TargetNaming: config.EffectiveTargetNaming(sc.TargetNaming),
		Sync:         item.skillSync,
		Include:      append([]string(nil), sc.Include...),
		Exclude:      append([]string(nil), sc.Exclude...),

		SkillsEnabled: sc.IsEnabled(),
	}
	if item.namingErr != nil {
		jsonItem.Warning = item.namingErr.Error()
	}
	applyTargetListAgentSummary(&jsonItem, item.agentSummary)
	return jsonItem
}

func printTargetAgentSection(summary *targetsummary.AgentSummary) {
	if summary == nil {
		return
	}

	displayPath := summary.DisplayPath
	if displayPath == "" {
		displayPath = summary.Path
	}

	ui.Section("Agents")
	width := ui.RowWidth()
	ui.Row(ui.MarkNone, "Path", shortenPath(displayPath), width)
	ui.Row(ui.MarkNone, "Mode", summary.Mode, width)
	ui.Row(ui.MarkNone, "Status", formatTargetAgentSyncSummary(summary), width)
	if summary.Mode == "symlink" {
		ui.Row(ui.MarkNone, "Filters", "ignored in symlink mode", width)
		return
	}
	printFilterRows(summary.Include, summary.Exclude, width)
}

// printTargetSkillsSection prints the Skills block of target info.
func printTargetSkillsSection(path, mode, naming, status string, include, exclude []string) {
	ui.Section("Skills")
	width := ui.RowWidth()
	ui.Row(ui.MarkNone, "Path", path, width)
	ui.Row(ui.MarkNone, "Mode", mode, width)
	ui.Row(ui.MarkNone, "Naming", naming, width)
	ui.Row(ui.MarkNone, "Status", status, width)
	printFilterRows(include, exclude, width)
}

// printFilterRows prints the include and exclude rows that are set.
func printFilterRows(include, exclude []string, width int) {
	if len(include) > 0 {
		ui.Row(ui.MarkNone, "Include", strings.Join(include, ", "), width)
	}
	if len(exclude) > 0 {
		ui.Row(ui.MarkNone, "Exclude", strings.Join(exclude, ", "), width)
	}
}

// filterSummary is the one-line form of a target's filters, or "" when
// none are set.
func filterSummary(include, exclude []string) string {
	var parts []string
	if len(include) > 0 {
		parts = append(parts, "include "+strings.Join(include, ", "))
	}
	if len(exclude) > 0 {
		parts = append(parts, "exclude "+strings.Join(exclude, ", "))
	}
	return strings.Join(parts, " · ")
}

// statusWithCounts follows a sync status with its non-zero counts, dimmed.
func statusWithCounts(status fmt.Stringer, synced int, label string, local int) string {
	counts := joinAgentCounts(synced, label, local)
	if counts == "" {
		return status.String()
	}
	return status.String() + ui.DimText(" · "+counts)
}

// printTargetFilterChanges reports applied filter changes; nothing when
// there are none.
func printTargetFilterChanges(name string, changes []string) {
	if len(changes) == 0 {
		return
	}
	width := ui.RowWidth(name)
	for _, change := range changes {
		ui.Row(ui.MarkOK, name, change, width)
	}
	ui.Next("skillshare sync", "apply the filter changes")
}

func printTargetListPlain(items []targetTUIItem) {
	if len(items) == 0 {
		ui.Done(ui.MarkNone, "No targets configured", 0)
		ui.Next("skillshare target add <name> <path>", "add one")
		return
	}
	width := ui.RowWidth("Skills", "Agents")
	for idx, item := range items {
		if idx > 0 {
			fmt.Println()
		}

		sc := item.target.SkillsConfig()
		displayPath := item.displayPath
		if displayPath == "" {
			displayPath = sc.Path
		}

		fmt.Println(theme.Primary().Bold(true).Render(item.name))
		detail := item.skillSyncText
		if sc.IsEnabled() {
			detail = sync.EffectiveMode(sc.Mode) + " · " + config.EffectiveTargetNaming(sc.TargetNaming) + " · " + item.skillSyncText
		}
		ui.Row(ui.MarkNone, "Skills", shortenPath(displayPath)+"  "+ui.DimText(detail), width)
		if item.namingErr != nil {
			ui.Row(ui.MarkWarn, "", item.namingErr.Error(), width)
		}
		if f := filterSummary(sc.Include, sc.Exclude); sc.IsEnabled() && f != "" {
			ui.Row(ui.MarkNone, "", ui.DimText(f), width)
		}

		if item.agentSummary == nil {
			continue
		}

		agentPath := item.agentSummary.DisplayPath
		if agentPath == "" {
			agentPath = item.agentSummary.Path
		}
		ui.Row(ui.MarkNone, "Agents", shortenPath(agentPath)+"  "+ui.DimText(item.agentSummary.Mode+" · "+formatTargetAgentSyncSummary(item.agentSummary)), width)
		if item.agentSummary.Mode == "symlink" {
			ui.Row(ui.MarkNone, "", ui.DimText("filters ignored in symlink mode"), width)
		} else if f := filterSummary(item.agentSummary.Include, item.agentSummary.Exclude); f != "" {
			ui.Row(ui.MarkNone, "", ui.DimText(f), width)
		}
	}
	fmt.Println()
	ui.Done(ui.MarkNone, plural(len(items), "target"), 0)
}

func formatTargetAgentSyncSummary(summary *targetsummary.AgentSummary) string {
	if summary == nil {
		return ""
	}

	if summary.ExpectedCount == 0 {
		counts := joinAgentCounts(summary.ManagedCount, targetAgentCountLabel(summary.Mode), summary.LocalCount)
		if counts != "" {
			return fmt.Sprintf("no source agents yet (%s)", counts)
		}
		return "no source agents yet"
	}

	summaryText := fmt.Sprintf("%d/%d %s", summary.ManagedCount, summary.ExpectedCount, targetAgentCountLabel(summary.Mode))
	if summary.LocalCount > 0 {
		summaryText += fmt.Sprintf(", %d local", summary.LocalCount)
	}
	if summary.Mode == "symlink" {
		return summaryText + " (directory symlink)"
	}
	return summaryText
}

func joinAgentCounts(managed int, managedLabel string, local int) string {
	var parts []string
	if managed > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", managed, managedLabel))
	}
	if local > 0 {
		parts = append(parts, fmt.Sprintf("%d local", local))
	}
	return strings.Join(parts, ", ")
}

func targetAgentCountLabel(mode string) string {
	if mode == "copy" {
		return "managed"
	}
	return "linked"
}

func intPtr(v int) *int {
	return &v
}
