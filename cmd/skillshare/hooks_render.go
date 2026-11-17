package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"skillshare/internal/hooks"
	"skillshare/internal/ui"
)

func printHooksPlan(p *hooks.Plan) {
	printHooksSource(p.SourcePath)
	printHooksChanges(p)
	if len(p.Changes) == 0 {
		ui.Note("no hook changes")
	}
}

// printHooksChanges prints one row per change to an Agent file; conflicts
// are failures and inactive outputs are warnings.
func printHooksChanges(p *hooks.Plan) {
	home, _ := os.UserHomeDir()
	conflicts := []string{}
	labels := make([]string, len(p.Changes))
	for i, c := range p.Changes {
		labels[i] = hooksChangeLabel(c)
	}
	width := ui.RowWidth(labels...)
	for _, c := range p.Changes {
		detail := c.Target + "  " + hooksDisplayPath(c.Path, home)
		if c.Events != nil {
			detail += "  " + hooksEventDetail(c.Events)
		}
		if c.Message != "" {
			detail += " — " + c.Message
		}
		mark := ui.MarkNone
		switch c.Action {
		case "conflict":
			mark = ui.MarkFail
		case "inactive":
			mark = ui.MarkWarn
		}
		ui.Row(mark, hooksChangeLabel(c), c.Action+"  "+ui.DimText(detail), width)
		if c.Action == "conflict" && c.Name != "" && c.Root == "" && !slices.Contains(conflicts, c.Name) {
			conflicts = append(conflicts, c.Name)
		}
	}
	for _, w := range p.Warnings {
		ui.Warning("%s", w)
	}
	for _, name := range conflicts {
		ui.Note(fmt.Sprintf("To take over %s's conflicting Agent entries: skillshare hooks sync %s --replace", name, name))
	}
}

// hooksChangeLabel is the hook a change belongs to, or its Agent when the
// change concerns a whole file.
func hooksChangeLabel(c hooks.Change) string {
	if c.Name != "" {
		return c.Name
	}
	return c.Target
}

// hooksDisplayPath shortens a path under the home directory to ~/...
func hooksDisplayPath(path, home string) string {
	if home != "" && strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

// hooksEventDetail renders events added (+), updated (~) and removed (−).
func hooksEventDetail(e *hooks.EventChanges) string {
	var parts []string
	for _, group := range []struct {
		mark   string
		events []string
	}{{"+", e.Added}, {"~", e.Updated}, {"−", e.Removed}} {
		if len(group.events) > 0 {
			parts = append(parts, group.mark+" "+strings.Join(group.events, ", "))
		}
	}
	return strings.Join(parts, "  ")
}

func printHooksResult(result *hooks.Result, err error, o hooksOptions) error {
	if o.json {
		if result == nil {
			return err
		}
		return writeJSONResult(result, err)
	}
	if result != nil {
		if result.Plan != nil {
			printHooksPlan(result.Plan)
		}
		for _, id := range result.BackupIDs {
			ui.Note("backup " + id)
		}
		if err == nil && !o.sync {
			fmt.Println()
			ui.Done(ui.MarkOK, "Saved the hooks source", 0)
			ui.Next("skillshare hooks sync", "write it into Agent files")
		} else if err == nil {
			fmt.Println()
			ui.Done(ui.MarkOK, "Applied "+plural(len(result.Applied), "hook file"), 0)
			ui.Note("Restart or reload the Agent to load them")
		}
	}
	return err
}

// printHooksTrust prints one row per Codex hook sync --trust looked at, and the
// command of each one that needs trust.
func printHooksTrust(trust []hooks.CodexTrust) {
	if len(trust) == 0 {
		return
	}
	ui.Section("Codex trust")
	labels := make([]string, len(trust))
	for i, t := range trust {
		labels[i] = hooksTrustLabel(t)
	}
	width := ui.RowWidth(labels...)
	for _, t := range trust {
		mark, value := ui.MarkOK, t.Action
		if t.Action == "needed" {
			mark, value = ui.MarkNone, "needs trust"
		}
		if t.Disabled {
			mark, value = ui.MarkWarn, value+", disabled in Codex"
		}
		ui.Row(mark, hooksTrustLabel(t), value+"  "+ui.DimText(t.Target+"  "+t.Key), width)
		if t.Action == "needed" && t.Command != "" {
			ui.Note(t.Command)
		}
	}
	for _, t := range trust {
		if t.Disabled {
			ui.Note("Codex skips disabled hooks; enable them in /hooks")
			break
		}
	}
}

func hooksTrustLabel(t hooks.CodexTrust) string {
	if t.Entry != "" {
		return t.Entry
	}
	return t.Target
}

func printHooksCandidates(candidates []hooks.Candidate, asJSON bool) error {
	if asJSON {
		return writeJSON(candidates)
	}
	// A warning several candidates share is printed once, after the rows.
	seen := map[string]int{}
	for _, c := range candidates {
		for _, w := range c.Warnings {
			seen[w]++
		}
	}
	var shared []string
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Name
	}
	width := ui.RowWidth(names...)
	for _, c := range candidates {
		var own []string
		for _, w := range c.Warnings {
			if seen[w] > 1 {
				if !slices.Contains(shared, w) {
					shared = append(shared, w)
				}
				continue
			}
			own = append(own, w)
		}
		if len(c.Problems) > 0 {
			ui.Row(ui.MarkFail, c.Name, "blocked "+ui.DimText("· "+strings.Join(c.Problems, "; ")), width)
		} else if len(own) > 0 {
			ui.Row(ui.MarkNone, c.Name, "importable "+ui.DimText("· "+strings.Join(own, "; ")), width)
		} else {
			ui.Row(ui.MarkNone, c.Name, "importable", width)
		}
	}
	for _, w := range shared {
		ui.Note(strings.ToUpper(w[:1]) + w[1:] + ".")
	}
	if len(candidates) == 0 {
		ui.Done(ui.MarkNone, "No hooks found to import", 0)
	} else {
		ui.Next("skillshare hooks import --from <agent> <name>", "import one (add --dry-run to preview)")
	}
	return nil
}
