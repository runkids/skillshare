package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/utils"
)

// ---------------------------------------------------------------------------
// Restore TUI — interactive backup restore: target → version → confirm → run
// Left-right split layout: list on left, detail panel on right.
// ---------------------------------------------------------------------------

// restorePhase tracks which screen is active.
type restorePhase int

const (
	phaseTargetList  restorePhase = iota // select target
	phaseVersionList                     // select backup version
	phaseConfirm                         // confirm restore
	phaseExecuting                       // restore in progress
	phaseDone                            // restore complete
)

// restoreMinSplitWidth is the minimum terminal width for horizontal split.
const restoreMinSplitWidth = tuiMinSplitWidth

// --- List items ---

type restoreTargetItem struct {
	summary backup.TargetBackupSummary
}

func (i restoreTargetItem) Title() string {
	return fmt.Sprintf("  %s", i.summary.TargetName)
}
func (i restoreTargetItem) Description() string {
	return fmt.Sprintf("%d backup(s), latest: %s",
		i.summary.BackupCount, i.summary.Latest.Format("2006-01-02"))
}
func (i restoreTargetItem) FilterValue() string { return i.summary.TargetName }

type restoreVersionItem struct {
	version backup.BackupVersion
}

func (i restoreVersionItem) Title() string {
	return fmt.Sprintf("  %s", i.version.Label)
}
func (i restoreVersionItem) Description() string {
	return fmt.Sprintf("%d skill(s), %s",
		i.version.SkillCount, formatBytes(i.version.TotalSize))
}
func (i restoreVersionItem) FilterValue() string { return i.version.Label }

// --- Messages ---

type restoreDoneMsg struct {
	err    error
	action string // "restore" or "delete"
}

// --- Model ---

type restoreTUIModel struct {
	phase      restorePhase
	quitting   bool
	termWidth  int
	termHeight int

	// Data
	backupDir string
	targets   map[string]config.TargetConfig
	cfgPath   string

	// Target list
	targetList     list.Model
	targetItems    []backup.TargetBackupSummary
	selectedTarget string

	// Version list
	versionList     list.Model
	versionItems    []backup.BackupVersion
	selectedVersion *backup.BackupVersion

	// Filter (shared between target + version lists)
	filterText  string
	filterInput textinput.Model
	filtering   bool
	matchCount  int

	// Detail scroll (right panel)
	detailScroll int

	// Cached detail content — recomputed only on selection change or mutation
	cachedDetailIdx   int
	cachedDetailPhase restorePhase
	cachedDetailStr   string

	// Confirm overlay
	confirmAction string // "restore" or "delete"

	// Execution
	opSpinner spinner.Model
	resultMsg string
}

func newRestoreTUIModel(summaries []backup.TargetBackupSummary, backupDir string, targets map[string]config.TargetConfig, cfgPath string) restoreTUIModel {
	listItems := make([]list.Item, len(summaries))
	for i, s := range summaries {
		listItems[i] = restoreTargetItem{summary: s}
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, true)

	tl := list.New(listItems, delegate, 0, 0)
	tl.Title = fmt.Sprintf("Backup Restore — %d target(s)", len(summaries))
	tl.Styles.Title = tc.ListTitle
	tl.SetShowStatusBar(false)
	tl.SetFilteringEnabled(false)
	tl.SetShowHelp(false)
	tl.SetShowPagination(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = tc.SpinnerStyle

	fi := textinput.New()
	fi.Prompt = "/ "
	fi.PromptStyle = tc.Filter
	fi.Cursor.Style = tc.Filter

	return restoreTUIModel{
		phase:       phaseTargetList,
		backupDir:   backupDir,
		targets:     targets,
		cfgPath:     cfgPath,
		targetList:  tl,
		targetItems: summaries,
		matchCount:  len(summaries),
		filterInput: fi,
		opSpinner:   sp,
	}
}

func (m restoreTUIModel) Init() tea.Cmd { return nil }

func (m restoreTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		lw := restoreListWidth(m.termWidth)
		h := m.restorePanelHeight()
		m.targetList.SetSize(lw, h)
		if m.phase == phaseVersionList {
			m.versionList.SetSize(lw, h)
		}
		m.refreshDetailCache()
		return m, nil

	case spinner.TickMsg:
		if m.phase == phaseExecuting {
			var cmd tea.Cmd
			m.opSpinner, cmd = m.opSpinner.Update(msg)
			return m, cmd
		}

	case restoreDoneMsg:
		if msg.action == "delete" {
			if msg.err != nil {
				m.resultMsg = tc.Red.Render(fmt.Sprintf("Delete failed: %s", msg.err))
				m.phase = phaseDone
				return m, nil
			}
			// Show success, then reload version list
			label := ""
			if m.selectedVersion != nil {
				label = m.selectedVersion.Label
			}
			m.resultMsg = tc.Green.Render(fmt.Sprintf("Deleted backup %s", label))
			m.confirmAction = ""
			m.selectedVersion = nil
			return m.enterVersionPhase()
		}
		m.phase = phaseDone
		if msg.err != nil {
			m.resultMsg = tc.Red.Render(fmt.Sprintf("Error: %s", msg.err))
		} else {
			m.resultMsg = tc.Green.Render(fmt.Sprintf("Restored %s from %s", m.selectedTarget, m.selectedVersion.Label))
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to active list
	switch m.phase {
	case phaseTargetList:
		var cmd tea.Cmd
		m.targetList, cmd = m.targetList.Update(msg)
		return m, cmd
	case phaseVersionList:
		var cmd tea.Cmd
		m.versionList, cmd = m.versionList.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m restoreTUIModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Executing — only quit
	if m.phase == phaseExecuting {
		if key == "q" || key == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// Done — any key quits
	if m.phase == phaseDone {
		m.quitting = true
		return m, tea.Quit
	}

	// Confirm overlay
	if m.phase == phaseConfirm {
		switch key {
		case "y", "Y", "enter":
			if m.confirmAction == "delete" {
				return m.startDelete()
			}
			return m.startRestore()
		case "n", "N", "esc":
			m.phase = phaseVersionList
			m.confirmAction = ""
			return m, nil
		}
		return m, nil
	}

	// Filter mode
	if m.filtering {
		switch key {
		case "esc":
			m.filtering = false
			m.filterText = ""
			m.filterInput.SetValue("")
			m.applyRestoreFilter()
			return m, nil
		case "enter":
			m.filtering = false
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		newVal := m.filterInput.Value()
		if newVal != m.filterText {
			m.filterText = newVal
			m.applyRestoreFilter()
		}
		return m, cmd
	}

	// Normal keys
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "esc":
		if m.phase == phaseVersionList {
			m.phase = phaseTargetList
			m.selectedTarget = ""
			m.filterText = ""
			m.filterInput.SetValue("")
			m.detailScroll = 0
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "/":
		m.filtering = true
		m.filterInput.Focus()
		return m, textinput.Blink

	// Detail scroll
	case "ctrl+d":
		m.detailScroll += 5
		return m, nil
	case "ctrl+u":
		m.detailScroll -= 5
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
		return m, nil

	case "d":
		if m.phase == phaseVersionList {
			item, ok := m.versionList.SelectedItem().(restoreVersionItem)
			if !ok {
				break
			}
			m.selectedVersion = &item.version
			m.confirmAction = "delete"
			m.phase = phaseConfirm
			m.resultMsg = ""
			return m, nil
		}

	case "enter":
		if m.phase == phaseTargetList {
			item, ok := m.targetList.SelectedItem().(restoreTargetItem)
			if !ok {
				break
			}
			m.selectedTarget = item.summary.TargetName
			m.filterText = ""
			m.filterInput.SetValue("")
			m.detailScroll = 0
			return m.enterVersionPhase()
		}
		if m.phase == phaseVersionList {
			item, ok := m.versionList.SelectedItem().(restoreVersionItem)
			if !ok {
				break
			}
			m.selectedVersion = &item.version
			m.confirmAction = "restore"
			m.phase = phaseConfirm
			return m, nil
		}
	}

	// Reset detail scroll on list navigation
	prevIdx := m.activeListIndex()

	// Delegate to active list
	var cmd tea.Cmd
	switch m.phase {
	case phaseTargetList:
		m.targetList, cmd = m.targetList.Update(msg)
	case phaseVersionList:
		m.versionList, cmd = m.versionList.Update(msg)
	}

	if m.activeListIndex() != prevIdx {
		m.detailScroll = 0
		m.invalidateDetailCache()
		m.refreshDetailCache()
	}

	return m, cmd
}

// activeListIndex returns the current cursor index for the active list.
func (m restoreTUIModel) activeListIndex() int {
	switch m.phase {
	case phaseTargetList:
		return m.targetList.Index()
	case phaseVersionList:
		return m.versionList.Index()
	}
	return -1
}

func (m restoreTUIModel) enterVersionPhase() (tea.Model, tea.Cmd) {
	versions, err := backup.ListBackupVersions(m.backupDir, m.selectedTarget)
	if err != nil || len(versions) == 0 {
		// No versions left — refresh target list and go back
		m.refreshTargetList()
		m.phase = phaseTargetList
		m.selectedTarget = ""
		return m, nil
	}
	m.versionItems = versions

	listItems := make([]list.Item, len(versions))
	for i, v := range versions {
		listItems[i] = restoreVersionItem{version: v}
	}

	delegate := list.NewDefaultDelegate()
	configureDelegate(&delegate, true)

	lw := restoreListWidth(m.termWidth)
	vl := list.New(listItems, delegate, 0, 0)
	vl.Title = fmt.Sprintf("%s — select version", m.selectedTarget)
	vl.Styles.Title = tc.ListTitle
	vl.SetShowStatusBar(false)
	vl.SetFilteringEnabled(false)
	vl.SetShowHelp(false)
	vl.SetShowPagination(false)
	if m.termWidth > 0 {
		vl.SetSize(lw, m.restorePanelHeight())
	}

	m.versionList = vl
	m.matchCount = len(versions)
	m.phase = phaseVersionList
	m.detailScroll = 0
	m.invalidateDetailCache()
	m.refreshDetailCache()
	return m, nil
}

func (m restoreTUIModel) startRestore() (tea.Model, tea.Cmd) {
	m.phase = phaseExecuting
	targetName := m.selectedTarget
	version := *m.selectedVersion
	targets := m.targets
	cfgPath := m.cfgPath

	cmd := func() tea.Msg {
		start := time.Now()
		targetCfg, ok := targets[targetName]
		if !ok {
			return restoreDoneMsg{err: fmt.Errorf("target '%s' not found in config", targetName)}
		}

		backupPath := filepath.Dir(version.Dir)
		opts := backup.RestoreOptions{Force: true}
		err := backup.RestoreToPath(backupPath, targetName, targetCfg.Path, opts)

		e := oplog.NewEntry("restore", statusFromErr(err), time.Since(start))
		e.Args = map[string]any{"target": targetName, "from": version.Label, "via": "tui"}
		if err != nil {
			e.Message = err.Error()
		}
		oplog.WriteWithLimit(cfgPath, oplog.OpsFile, e, logMaxEntries()) //nolint:errcheck

		return restoreDoneMsg{err: err}
	}

	return m, tea.Batch(m.opSpinner.Tick, cmd)
}

func (m *restoreTUIModel) refreshTargetList() {
	summaries, _ := backup.ListTargetsWithBackups(m.backupDir)
	m.targetItems = summaries
	items := make([]list.Item, len(summaries))
	for i, s := range summaries {
		items[i] = restoreTargetItem{summary: s}
	}
	m.targetList.SetItems(items)
	m.matchCount = len(summaries)
	m.targetList.Title = fmt.Sprintf("Backup Restore — %d target(s)", len(summaries))
	m.invalidateDetailCache()
}

func (m restoreTUIModel) startDelete() (tea.Model, tea.Cmd) {
	m.phase = phaseExecuting
	version := *m.selectedVersion

	cmd := func() tea.Msg {
		// version.Dir is e.g. .../backups/2024-01-15_14-04-05/claude
		// Parent is the timestamp dir: .../backups/2024-01-15_14-04-05
		tsDir := filepath.Dir(version.Dir)

		// Check if other targets exist in this timestamp dir
		entries, _ := os.ReadDir(tsDir)
		otherTargets := 0
		for _, e := range entries {
			if e.IsDir() {
				otherTargets++
			}
		}

		var err error
		if otherTargets <= 1 {
			// Only this target — remove entire timestamp dir
			err = os.RemoveAll(tsDir)
		} else {
			// Other targets exist — remove only this target's subdir
			err = os.RemoveAll(version.Dir)
		}

		return restoreDoneMsg{err: err, action: "delete"}
	}

	return m, tea.Batch(m.opSpinner.Tick, cmd)
}

// --- Filter ---

func (m *restoreTUIModel) applyRestoreFilter() {
	needle := strings.ToLower(m.filterText)
	switch m.phase {
	case phaseTargetList:
		if needle == "" {
			items := make([]list.Item, len(m.targetItems))
			for i, s := range m.targetItems {
				items[i] = restoreTargetItem{summary: s}
			}
			m.matchCount = len(m.targetItems)
			m.targetList.SetItems(items)
			m.targetList.ResetSelected()
			return
		}
		var matched []list.Item
		for _, s := range m.targetItems {
			if strings.Contains(strings.ToLower(s.TargetName), needle) {
				matched = append(matched, restoreTargetItem{summary: s})
			}
		}
		m.matchCount = len(matched)
		m.targetList.SetItems(matched)
		m.targetList.ResetSelected()

	case phaseVersionList:
		if needle == "" {
			items := make([]list.Item, len(m.versionItems))
			for i, v := range m.versionItems {
				items[i] = restoreVersionItem{version: v}
			}
			m.matchCount = len(m.versionItems)
			m.versionList.SetItems(items)
			m.versionList.ResetSelected()
			return
		}
		var matched []list.Item
		for _, v := range m.versionItems {
			if strings.Contains(v.Label, needle) {
				matched = append(matched, restoreVersionItem{version: v})
			}
		}
		m.matchCount = len(matched)
		m.versionList.SetItems(matched)
		m.versionList.ResetSelected()
	}
	m.invalidateDetailCache()
}

// --- Layout helpers ---

// restoreListWidth returns fixed left panel width.
func restoreListWidth(_ int) int {
	return 40
}

// restoreDetailWidth returns right panel width.
func restoreDetailWidth(termWidth int) int {
	w := termWidth - restoreListWidth(termWidth) - 3 // 3 = border column
	if w < 30 {
		w = 30
	}
	return w
}

// restorePanelHeight returns the panel height for the horizontal split.
// Footer: filter(1) + gap(1) + help(1) + trailing(1) = 4
func (m restoreTUIModel) restorePanelHeight() int {
	h := m.termHeight - 4
	if h < 10 {
		h = 10
	}
	return h
}

// --- Views ---

func (m restoreTUIModel) View() string {
	if m.quitting {
		return ""
	}

	switch m.phase {
	case phaseExecuting:
		verb := "Restoring"
		if m.confirmAction == "delete" {
			verb = "Deleting"
		}
		return fmt.Sprintf("\n  %s %s %s from %s...\n",
			m.opSpinner.View(), verb, m.selectedTarget, m.selectedVersion.Label)

	case phaseDone:
		return fmt.Sprintf("\n  %s\n\n  %s\n",
			m.resultMsg, tc.Help.Render("Press any key to exit"))

	case phaseConfirm:
		return m.viewRestoreConfirm()
	}

	// Horizontal split layout (list left, detail right)
	if m.termWidth >= restoreMinSplitWidth {
		return m.viewHorizontal()
	}
	return m.viewVertical()
}

// viewHorizontal renders the left-right split layout.
func (m restoreTUIModel) viewHorizontal() string {
	var b strings.Builder

	panelHeight := m.restorePanelHeight()
	leftWidth := restoreListWidth(m.termWidth)
	rightWidth := restoreDetailWidth(m.termWidth)

	// Left panel: active list
	var listView string
	switch m.phase {
	case phaseTargetList:
		listView = m.targetList.View()
	case phaseVersionList:
		listView = m.versionList.View()
	}

	// Right panel: detail (cached)
	detailStr := applyDetailScroll(m.buildDetailContent(), m.detailScroll, panelHeight)

	body := renderHorizontalSplit(listView, detailStr, leftWidth, rightWidth, panelHeight)
	b.WriteString(body)
	b.WriteString("\n")

	// Operation result message
	if m.resultMsg != "" {
		b.WriteString("  ")
		b.WriteString(m.resultMsg)
		b.WriteString("\n")
	}

	// Filter bar
	b.WriteString(m.renderRestoreFilterBar())

	// Help
	help := m.restoreHelpText()
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

// viewVertical renders the fallback vertical layout for narrow terminals.
func (m restoreTUIModel) viewVertical() string {
	var b strings.Builder

	switch m.phase {
	case phaseTargetList:
		b.WriteString(m.targetList.View())
	case phaseVersionList:
		b.WriteString(m.versionList.View())
	}
	b.WriteString("\n")

	if m.resultMsg != "" {
		b.WriteString("  ")
		b.WriteString(m.resultMsg)
		b.WriteString("\n")
	}

	b.WriteString(m.renderRestoreFilterBar())

	// Detail below list (limited height)
	detailContent := m.buildDetailContent()
	detailHeight := m.termHeight / 3
	if detailHeight < 6 {
		detailHeight = 6
	}
	b.WriteString(applyDetailScroll(detailContent, m.detailScroll, detailHeight))
	b.WriteString("\n")

	help := m.restoreHelpText()
	b.WriteString(tc.Help.Render(help))
	b.WriteString("\n")

	return b.String()
}

// refreshDetailCache recomputes the detail content only when the selection or phase changes.
func (m *restoreTUIModel) refreshDetailCache() {
	idx := m.activeListIndex()
	if idx == m.cachedDetailIdx && m.phase == m.cachedDetailPhase && m.cachedDetailStr != "" {
		return
	}
	m.cachedDetailIdx = idx
	m.cachedDetailPhase = m.phase
	switch m.phase {
	case phaseTargetList:
		if item, ok := m.targetList.SelectedItem().(restoreTargetItem); ok {
			m.cachedDetailStr = m.renderTargetDetail(item.summary)
			return
		}
	case phaseVersionList:
		if item, ok := m.versionList.SelectedItem().(restoreVersionItem); ok {
			m.cachedDetailStr = m.renderVersionDetail(item.version)
			return
		}
	}
	m.cachedDetailStr = ""
}

// invalidateDetailCache forces the next refreshDetailCache call to recompute.
func (m *restoreTUIModel) invalidateDetailCache() {
	m.cachedDetailStr = ""
	m.cachedDetailIdx = -1
}

// buildDetailContent returns the cached detail content string.
// Called from View() (value receiver), so it reads the cache populated by Update().
func (m restoreTUIModel) buildDetailContent() string {
	return m.cachedDetailStr
}

func (m restoreTUIModel) viewRestoreConfirm() string {
	var b strings.Builder
	b.WriteString("\n")

	if m.confirmAction == "delete" {
		fmt.Fprintf(&b, "  %s\n\n",
			tc.Red.Render(fmt.Sprintf("Delete backup %s for %s?", m.selectedVersion.Label, m.selectedTarget)))
	} else {
		fmt.Fprintf(&b, "  Restore %s from backup %s?\n\n", m.selectedTarget, m.selectedVersion.Label)
	}

	fmt.Fprintf(&b, "    Skills: %d\n", m.selectedVersion.SkillCount)
	fmt.Fprintf(&b, "    Size:   %s\n", formatBytes(m.selectedVersion.TotalSize))

	if len(m.selectedVersion.SkillNames) > 0 {
		b.WriteString("\n    Contents:\n")
		show := m.selectedVersion.SkillNames
		if len(show) > 10 {
			show = show[:10]
		}
		for _, name := range show {
			fmt.Fprintf(&b, "      %s\n", name)
		}
		if len(m.selectedVersion.SkillNames) > 10 {
			fmt.Fprintf(&b, "      ... and %d more\n", len(m.selectedVersion.SkillNames)-10)
		}
	}

	b.WriteString("\n  ")
	b.WriteString(tc.Help.Render("y confirm  n cancel"))
	b.WriteString("\n")
	return b.String()
}

func (m restoreTUIModel) renderRestoreFilterBar() string {
	totalCount := len(m.targetItems)
	noun := "targets"
	var pag string

	if m.phase == phaseVersionList {
		totalCount = len(m.versionItems)
		noun = "backups"
		pag = renderPageInfoFromPaginator(m.versionList.Paginator)
	} else {
		pag = renderPageInfoFromPaginator(m.targetList.Paginator)
	}

	return renderTUIFilterBar(
		m.filterInput.View(), m.filtering, m.filterText,
		m.matchCount, totalCount, 0, noun, pag,
	)
}

func (m restoreTUIModel) restoreHelpText() string {
	help := "↑↓ navigate  / filter"
	if m.phase == phaseTargetList {
		help += "  enter select  esc quit"
	} else {
		help += "  enter restore  d delete  Ctrl+d/u scroll  esc back  q quit"
	}
	return help
}

// --- Detail renderers ---

func (m restoreTUIModel) renderTargetDetail(s backup.TargetBackupSummary) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(tc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
	}

	row("Target:  ", s.TargetName)

	// Target path and current state
	if t, ok := m.targets[s.TargetName]; ok {
		row("Path:    ", t.Path)
		if t.Mode != "" {
			row("Mode:    ", t.Mode)
		}
		row("Status:  ", describeTargetState(t.Path))
	}

	b.WriteString("\n")
	row("Backups: ", fmt.Sprintf("%d", s.BackupCount))
	row("Latest:  ", fmt.Sprintf("%s (%s)", s.Latest.Format("2006-01-02 15:04:05"), timeAgo(s.Latest)))
	row("Oldest:  ", fmt.Sprintf("%s (%s)", s.Oldest.Format("2006-01-02 15:04:05"), timeAgo(s.Oldest)))

	// Preview skills from latest backup
	latestVersions, _ := backup.ListBackupVersions(m.backupDir, s.TargetName)
	if len(latestVersions) > 0 {
		latest := latestVersions[0]
		b.WriteString("\n")
		b.WriteString(tc.Separator.Render("── Latest backup skills ──────────────"))
		b.WriteString("\n")
		for _, name := range latest.SkillNames {
			desc := readSkillDescription(filepath.Join(latest.Dir, name))
			if desc != "" {
				b.WriteString(tc.Value.Render("  " + name))
				b.WriteString("\n")
				b.WriteString(tc.Dim.Render("    " + truncateStr(desc, 60)))
				b.WriteString("\n")
			} else {
				b.WriteString(tc.Value.Render("  " + name))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func (m restoreTUIModel) renderVersionDetail(v backup.BackupVersion) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(tc.Label.Render(label))
		b.WriteString(tc.Value.Render(value))
		b.WriteString("\n")
	}

	row("Date:    ", fmt.Sprintf("%s (%s)", v.Label, timeAgo(v.Timestamp)))
	row("Skills:  ", fmt.Sprintf("%d", v.SkillCount))
	row("Size:    ", formatBytes(v.TotalSize))

	// Diff with current target
	if t, ok := m.targets[m.selectedTarget]; ok {
		added, removed, common := diffSkillSets(v.SkillNames, listDirNames(t.Path))
		if len(added) > 0 || len(removed) > 0 {
			b.WriteString("\n")
			b.WriteString(tc.Separator.Render("── Diff vs current target ────────────"))
			b.WriteString("\n")
			if len(common) > 0 {
				row("Same:    ", fmt.Sprintf("%d skill(s)", len(common)))
			}
			if len(added) > 0 {
				b.WriteString(tc.Label.Render("Restore: "))
				b.WriteString(tc.Green.Render(fmt.Sprintf("+%d (in backup, not in target)", len(added))))
				b.WriteString("\n")
				for _, name := range added {
					b.WriteString(tc.Green.Render("  + " + name))
					b.WriteString("\n")
				}
			}
			if len(removed) > 0 {
				b.WriteString(tc.Label.Render("Remove:  "))
				b.WriteString(tc.Red.Render(fmt.Sprintf("-%d (in target, not in backup)", len(removed))))
				b.WriteString("\n")
				for _, name := range removed {
					b.WriteString(tc.Red.Render("  - " + name))
					b.WriteString("\n")
				}
			}
		} else if len(common) > 0 {
			b.WriteString("\n")
			b.WriteString(tc.Dim.Render("  Backup matches current target"))
			b.WriteString("\n")
		}
	}

	// Skill list with descriptions
	if len(v.SkillNames) > 0 {
		b.WriteString("\n")
		b.WriteString(tc.Separator.Render("── Contents ──────────────────────────"))
		b.WriteString("\n")
		for _, name := range v.SkillNames {
			desc := readSkillDescription(filepath.Join(v.Dir, name))
			files := listSkillFiles(filepath.Join(v.Dir, name))
			b.WriteString(tc.Value.Render("  " + name))
			b.WriteString("\n")
			if desc != "" {
				b.WriteString(tc.Dim.Render("    " + truncateStr(desc, 60)))
				b.WriteString("\n")
			}
			if len(files) > 0 {
				b.WriteString(tc.Dim.Render("    " + strings.Join(files, "  ")))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// --- Helpers ---

// timeAgo returns a human-readable relative time string like "5m ago".
func timeAgo(t time.Time) string {
	s := formatDurationShort(time.Since(t))
	if s == "just now" {
		return s
	}
	return s + " ago"
}

// describeTargetState returns a human-readable description of the target path.
func describeTargetState(path string) string {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return tc.Yellow.Render("not found")
		}
		return tc.Red.Render("error")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		dest, _ := os.Readlink(path)
		return tc.Cyan.Render("symlink → " + dest)
	}
	entries, _ := os.ReadDir(path)
	return fmt.Sprintf("directory (%d items)", len(entries))
}

// readSkillDescription reads the description field from a skill's SKILL.md frontmatter.
func readSkillDescription(skillDir string) string {
	return utils.ParseFrontmatterField(filepath.Join(skillDir, "SKILL.md"), "description")
}

// listDirNames returns sorted subdirectory names in a directory.
func listDirNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// diffSkillSets compares backup skills vs current target skills.
// Returns: onlyInBackup, onlyInTarget, inBoth.
func diffSkillSets(backupSkills, currentSkills []string) (added, removed, common []string) {
	bSet := make(map[string]bool, len(backupSkills))
	for _, s := range backupSkills {
		bSet[s] = true
	}
	cSet := make(map[string]bool, len(currentSkills))
	for _, s := range currentSkills {
		cSet[s] = true
	}
	for _, s := range backupSkills {
		if cSet[s] {
			common = append(common, s)
		} else {
			added = append(added, s)
		}
	}
	for _, s := range currentSkills {
		if !bSet[s] {
			removed = append(removed, s)
		}
	}
	return
}

// runRestoreTUI starts the backup restore TUI.
func runRestoreTUI(summaries []backup.TargetBackupSummary, backupDir string, targets map[string]config.TargetConfig, cfgPath string) error {
	model := newRestoreTUIModel(summaries, backupDir, targets, cfgPath)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
