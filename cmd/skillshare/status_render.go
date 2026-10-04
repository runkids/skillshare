package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/audit"
	"skillshare/internal/config"
	"skillshare/internal/git"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/sync"
	"skillshare/internal/theme"
	"skillshare/internal/ui"
	"skillshare/internal/utils"

	"github.com/mattn/go-runewidth"
)

// statusTarget is one target's line in the Targets table.
type statusTarget struct {
	name, path string
	mode       string      // skills mode; empty when skills are off
	skills     statusCell  // how the target's skills stand
	agents     *statusCell // nil when the target has no agents folder
}

// statusCell is one column of the Targets table: a mark, the state and a
// dim note such as "2 local".
type statusCell struct{ mark, text, note string }

func (c statusCell) width() int {
	w := 2 + runewidth.StringWidth(c.text)
	if c.note != "" {
		w += 3 + runewidth.StringWidth(c.note)
	}
	return w
}

func (c statusCell) render() string {
	s := ui.StyledMark(c.mark) + " " + c.text
	if c.note != "" {
		s += theme.Dim().Render(" · " + c.note)
	}
	return s
}

// skillsCell describes a target's skills. expected is how many skills a
// merge or copy target should hold.
func skillsCell(res targetStatusResult, path, mode string, expected int) statusCell {
	var c statusCell
	if res.localCount > 0 {
		c.note = fmt.Sprintf("%d local", res.localCount)
	}
	verb := "linked"
	if mode == "copy" {
		verb = "copied"
	}
	switch res.statusStr {
	case "skills off":
		return statusCell{mark: ui.MarkNone, text: theme.Dim().Render("skills off")}
	case "merged", "copied":
		if res.syncedCount < expected {
			c.mark, c.text = ui.MarkWarn, fmt.Sprintf("%d/%d %s", res.syncedCount, expected, verb)
		} else {
			c.mark, c.text = ui.MarkOK, fmt.Sprintf("%d %s", res.syncedCount, verb)
		}
	case "linked":
		if mode == "symlink" {
			c.mark, c.text = ui.MarkOK, "symlinked"
		} else {
			c.mark, c.text = ui.MarkWarn, "needs sync"
		}
	case "not exist":
		c.mark, c.text = ui.MarkWarn, "not synced yet"
	case "conflict":
		link, _ := os.Readlink(path)
		c.mark, c.text = ui.MarkFail, "links to "+utils.FoldHomePath(link)
	case "broken":
		c.mark, c.text = ui.MarkFail, "broken link"
	default:
		c.mark, c.text = ui.MarkWarn, res.statusStr
		if mode == "symlink" && res.statusStr == "merged" {
			c.text = "needs sync"
		}
	}
	return c
}

// agentsCell describes a target's agents: linked of expected, plus copies
// the user keeps there.
func agentsCell(linked, expected, preserved int) *statusCell {
	c := &statusCell{mark: ui.MarkOK, text: fmt.Sprintf("%d", linked-preserved)}
	if linked != expected && expected > 0 {
		c.mark, c.text = ui.MarkWarn, fmt.Sprintf("%d/%d", linked-preserved, expected)
	}
	if preserved > 0 {
		c.note = fmt.Sprintf("%d local", preserved)
	}
	return c
}

// printStatusTargets prints the Targets table, the modes in use and what
// needs attention.
func printStatusTargets(targets []statusTarget, withAgents bool, warnings []string, notSynced int) {
	nameW, pathW, skillsW := 0, 0, len("skills")-2
	for _, t := range targets {
		nameW = max(nameW, runewidth.StringWidth(t.name))
		pathW = max(pathW, runewidth.StringWidth(t.path))
		skillsW = max(skillsW, t.skills.width())
	}

	header := theme.Primary().Bold(true).Render("Targets")
	if len(targets) > 0 {
		cols := fmt.Sprintf("%-*s", skillsW+2, "skills")
		if withAgents {
			cols += "agents"
		}
		header += strings.Repeat(" ", max(1, 2+nameW+2+pathW+2-len("Targets"))) + theme.Dim().Render(strings.TrimRight(cols, " "))
	}
	fmt.Println()
	fmt.Println(header)

	for _, t := range targets {
		line := fmt.Sprintf("  %s  %s  %s", pad(t.name, nameW), pad(t.path, pathW), t.skills.render())
		if withAgents {
			line += strings.Repeat(" ", skillsW-t.skills.width()+2)
			if t.agents != nil {
				line += t.agents.render()
			} else {
				line += theme.Dim().Render("—")
			}
		}
		fmt.Println(strings.TrimRight(line, " "))
	}
	if len(targets) == 0 {
		ui.Note("none")
	}
	if note := modesNote(targets); note != "" {
		ui.Note(note)
	}

	for _, w := range warnings {
		ui.Warning("%s", w)
	}
	if notSynced > 0 {
		ui.Warning("%s not synced %s %s", plural(notSynced, "skill"), theme.Dim().Render("— run"), theme.Accent().Render("skillshare sync"))
	}
}

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-runewidth.StringWidth(s)))
}

// modesNote names the sync modes in use: "all use merge", or which targets
// use which mode when they differ.
func modesNote(targets []statusTarget) string {
	byMode := map[string][]string{}
	for _, t := range targets {
		if t.mode != "" {
			byMode[t.mode] = append(byMode[t.mode], t.name)
		}
	}
	switch len(byMode) {
	case 0:
		return ""
	case 1:
		for mode := range byMode {
			return "all use " + mode
		}
	}
	modes := make([]string, 0, len(byMode))
	for mode := range byMode {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	parts := make([]string, len(modes))
	for i, mode := range modes {
		parts[i] = mode + ": " + strings.Join(byMode[mode], ", ")
	}
	return strings.Join(parts, " · ")
}

// printSourceStatus prints where skills and agents come from, with paths
// shown by show. agentCount is negative when there is no agents folder.
func printSourceStatus(skillsPath, agentsPath string, show func(string) string, skillCount, agentCount int, stats *skillignore.IgnoreStats) {
	ui.Section("Source")
	width := ui.RowWidth("skills", "agents")
	if _, err := os.Stat(skillsPath); err != nil {
		ui.Row(ui.MarkFail, "skills", show(skillsPath)+" "+theme.Dim().Render("not found"), width)
		return
	}
	skills, agents := show(skillsPath), show(agentsPath)
	pathW := runewidth.StringWidth(skills)
	if agentCount >= 0 {
		pathW = max(pathW, runewidth.StringWidth(agents))
	}
	ui.Row(ui.MarkNone, "skills", pad(skills, pathW)+"  "+theme.Dim().Render(plural(skillCount, "skill")), width)
	if agentCount >= 0 {
		ui.Row(ui.MarkNone, "agents", pad(agents, pathW)+"  "+theme.Dim().Render(plural(agentCount, "agent")), width)
	}
	printSkillignoreLine(stats)
}

func printSkillignoreLine(stats *skillignore.IgnoreStats) {
	if stats == nil || !stats.Active() {
		return
	}
	hint := ".skillignore"
	if stats.HasLocal() {
		hint += " (.local active)"
	}
	ui.Note(fmt.Sprintf("%s: %d patterns, %d skills ignored", hint, stats.PatternCount(), stats.IgnoredCount()))
}

func printSkillfollowLine(source string, follow *sourcewalk.FollowSet) {
	if follow == nil {
		return
	}
	hint := ".skillfollow"
	if follow.HasLocal() {
		hint += " (.local active)"
	}
	ui.Note(fmt.Sprintf("%s: %d entries, %d skipped", hint, len(follow.ParsedEntries()), len(follow.Unavailable())))
	for _, message := range skillfollowPauses(source, follow) {
		ui.Warning("%s", message)
	}
}

// printTrackedReposStatus prints each tracked repository with its skill
// count and whether it has uncommitted changes.
func printTrackedReposStatus(sourcePath string, discovered []sync.DiscoveredSkill, trackedRepos []string) {
	if len(trackedRepos) == 0 {
		return
	}

	ui.Section("Tracked repositories")
	width := ui.RowWidth(trackedRepos...)
	for _, repoName := range trackedRepos {
		skillCount := 0
		for _, d := range discovered {
			if d.IsInRepo && strings.HasPrefix(d.RelPath, repoName+"/") {
				skillCount++
			}
		}
		count := plural(skillCount, "skill")

		isDirty, err := git.IsDirty(filepath.Join(sourcePath, repoName))
		switch {
		case err != nil:
			ui.Row(ui.MarkWarn, repoName, count+" "+theme.Dim().Render("· "+(&gitStatusError{err: err}).Error()), width)
		case isDirty:
			ui.Row(ui.MarkWarn, repoName, count+" "+theme.Dim().Render("· uncommitted changes"), width)
		default:
			ui.Row(ui.MarkOK, repoName, count, width)
		}
	}
}

// printExtrasStatus prints each extra's targets and how many files they get.
func printExtrasStatus(extras []config.ExtraConfig, sourceDirFn func(config.ExtraConfig) string) {
	ui.Section("Extras")
	width := extrasRowWidth(extras)
	pathW := 0
	for _, extra := range extras {
		for _, t := range extra.Targets {
			pathW = max(pathW, runewidth.StringWidth(shortenPath(t.Path)))
		}
	}
	for _, extra := range extras {
		files, err := sync.DiscoverExtraSource(sourceDirFn(extra), extra.File)
		if err != nil {
			ui.Row(ui.MarkWarn, extra.Name, "source not found", width)
			continue
		}
		if len(extra.Targets) == 0 {
			ui.Row(ui.MarkWarn, extra.Name, "no targets configured", width)
			continue
		}
		for _, t := range extra.Targets {
			if err := config.ValidateExtraMode(t.Mode); err != nil {
				ui.Row(ui.MarkWarn, extra.Name, fmt.Sprintf("%s %s", shortenPath(t.Path), err), width)
				continue
			}
			mode := sync.ExtraTargetMode(t.Mode, extra.File != "")
			ui.Row(ui.MarkNone, extra.Name, pad(shortenPath(t.Path), pathW)+"  "+theme.Dim().Render(plural(len(files), "file")+" · "+mode), width)
		}
	}
}

// printAuditStatus prints the audit policy on one line, naming the dedupe
// mode and analyzers only when they differ from the defaults.
func printAuditStatus(ac config.AuditConfig) {
	policy := audit.ResolvePolicy(audit.PolicyInputs{
		ConfigProfile:   ac.Profile,
		ConfigThreshold: ac.BlockThreshold,
		ConfigDedupe:    ac.DedupeMode,
		ConfigAnalyzers: ac.EnabledAnalyzers,
	})

	parts := []string{strings.ToLower(policyProfileLabel(string(policy.Profile)))}
	threshold := strings.ToLower(policy.Threshold)
	if threshold == "critical" {
		parts = append(parts, "blocks critical")
	} else {
		parts = append(parts, "blocks "+threshold+" and above")
	}
	if policy.DedupeMode != audit.DedupeGlobal {
		parts = append(parts, string(policy.DedupeMode)+" dedupe")
	}
	if len(policy.EnabledAnalyzers) > 0 {
		parts = append(parts, "analyzers: "+strings.Join(policy.EnabledAnalyzers, ", "))
	}
	fmt.Println()
	fmt.Println(statusLine("Audit", strings.Join(parts, theme.Dim().Render(" · "))))
}

// statusLine is a closing line of status: a bold name and its value.
func statusLine(name, value string) string {
	return theme.Primary().Bold(true).Render(pad(name, len("Version"))) + "  " + value
}
