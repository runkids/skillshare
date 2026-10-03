package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"skillshare/internal/backup"
	"skillshare/internal/config"
	"skillshare/internal/oplog"
	"skillshare/internal/theme"
	"skillshare/internal/utils"
)

// ---------------------------------------------------------------------------
// Restore TUI — interactive backup restore: target → version → confirm → run
// Left-right split layout: list on left, detail panel on right.
// ---------------------------------------------------------------------------

// isAgentBackupEntry returns true if the backup entry name represents an agent backup.
func isAgentBackupEntry(name string) bool {
	return strings.HasSuffix(name, "-agents")
}

// agentBaseTarget returns the base target name by stripping the "-agents" suffix.
func agentBaseTarget(name string) string {
	return strings.TrimSuffix(name, "-agents")
}

// resolveAgentBackupPath resolves the agent target path for a backup entry name,
// reusing the canonical resolveAgentTargetPath with builtin fallback.
func resolveAgentBackupPath(targets map[string]config.TargetConfig, entryName string) string {
	baseName := agentBaseTarget(entryName)
	tc := targets[baseName] // zero-value is safe — AgentsConfig returns empty, falls through to builtin
	return resolveAgentTargetPath(tc, config.DefaultAgentTargets(), baseName)
}

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
	name := i.summary.TargetName
	if isAgentBackupEntry(name) {
		return agentBaseTarget(name) + theme.Dim().Render(" agents")
	}
	return name
}
func (i restoreTargetItem) Description() string {
	return countNoun(i.summary.BackupCount, "backup") + " · " + timeAgo(i.summary.Latest)
}
func (i restoreTargetItem) FilterValue() string { return i.summary.TargetName }

type restoreVersionItem struct {
	version backup.BackupVersion
}

func (i restoreVersionItem) Title() string {
	return i.version.Label
}
func (i restoreVersionItem) Description() string {
	if i.version.TotalSize < 0 {
		return countNoun(i.version.SkillCount, "skill")
	}
	return countNoun(i.version.SkillCount, "skill") + " · " + formatBytes(i.version.TotalSize)
}

// restoreDelegate renders a target or backup row with its summary at the right.
type restoreDelegate struct{}

func (restoreDelegate) Height() int                             { return 1 }
func (restoreDelegate) Spacing() int                            { return 0 }
func (restoreDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (restoreDelegate) Render(w io.Writer, m list.Model, index int, li list.Item) {
	item, ok := li.(list.DefaultItem)
	if !ok {
		return
	}
	line := alignRow(item.Title(), theme.Dim().Render(item.Description()), m.Width()-rowIndent)
	renderPrefixRow(w, line, m.Width(), index == m.Index())
}

// newRestoreList builds a list with the shared row style and no chrome.
func newRestoreList(items []list.Item) list.Model {
	l := list.New(items, restoreDelegate{}, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	return l
}
func (i restoreVersionItem) FilterValue() string { return i.version.Label }

// --- Messages ---

type restoreDoneMsg struct {
	err    error
	action string // "restore" or "delete"
}

// versionSizeMsg delivers an asynchronously computed directory size.
type versionSizeMsg struct {
	dir  string
	size int64
}

func computeVersionSizeCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		return versionSizeMsg{dir: dir, size: backup.DirSize(dir)}
	}
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

	// Lazy size cache: version.Dir → computed size (populated on demand)
	versionSizeCache map[string]int64

	// Cached detail content — recomputed only on selection change or mutation
	cachedDetailIdx   int
	cachedDetailPhase restorePhase
	cachedDetailStr   string

	// Confirm overlay
	confirmAction string // "restore" or "delete"

	// Execution
	opSpinner spinner.Model
	resultMsg string

	showKeys bool // ? swaps the detail panel for the full key list
}

func newRestoreTUIModel(summaries []backup.TargetBackupSummary, backupDir string, targets map[string]config.TargetConfig, cfgPath string) restoreTUIModel {
	listItems := make([]list.Item, len(summaries))
	for i, s := range summaries {
		listItems[i] = restoreTargetItem{summary: s}
	}

	tl := newRestoreList(listItems)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent()

	fi := newTUIFilterInput("type to match a name")

	return restoreTUIModel{
		phase:            phaseTargetList,
		backupDir:        backupDir,
		targets:          targets,
		cfgPath:          cfgPath,
		targetList:       tl,
		targetItems:      summaries,
		matchCount:       len(summaries),
		filterInput:      fi,
		opSpinner:        sp,
		versionSizeCache: make(map[string]int64),
	}
}

func (m restoreTUIModel) Init() tea.Cmd { return nil }

func (m restoreTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		lw := m.listWidth()
		h := m.restorePanelHeight()
		m.targetList.SetSize(lw, h)
		if m.phase == phaseVersionList {
			m.versionList.SetSize(lw, h)
		}
		sizeCmd := m.refreshDetailCache()
		return m, sizeCmd

	case spinner.TickMsg:
		if m.phase == phaseExecuting {
			var cmd tea.Cmd
			m.opSpinner, cmd = m.opSpinner.Update(msg)
			return m, cmd
		}

	case restoreDoneMsg:
		if msg.action == "delete" {
			if msg.err != nil {
				m.resultMsg = theme.Danger().Render("✗") + " Delete failed: " + msg.err.Error()
				m.phase = phaseDone
				return m, nil
			}
			// Show success, then reload version list
			label := ""
			if m.selectedVersion != nil {
				label = m.selectedVersion.Label
			}
			m.resultMsg = theme.Success().Render("✓") + " Deleted backup " + label
			m.confirmAction = ""
			m.selectedVersion = nil
			return m.enterVersionPhase()
		}
		m.phase = phaseDone
		if msg.err != nil {
			m.resultMsg = theme.Danger().Render("✗") + " " + msg.err.Error()
		} else {
			m.resultMsg = theme.Success().Render("✓") + fmt.Sprintf(" Restored %s from %s", m.selectedTarget, m.selectedVersion.Label)
		}
		return m, nil

	case versionSizeMsg:
		m.versionSizeCache[msg.dir] = msg.size
		// Re-render detail if still viewing the version whose size just arrived
		if m.phase == phaseVersionList {
			if item, ok := m.versionList.SelectedItem().(restoreVersionItem); ok {
				if item.version.Dir == msg.dir {
					m.invalidateDetailCache()
					m.refreshDetailCache() // size is cached now, won't dispatch another cmd
				}
			}
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
		case "n", "N", "esc", "q":
			m.phase = phaseVersionList
			m.confirmAction = ""
			return m, nil
		}
		return m, nil
	}

	// Filter mode
	if m.filtering {
		cmd := handleTUIFilterKey(msg, &m.filtering, &m.filterText, &m.filterInput, m.applyRestoreFilter)
		return m, cmd
	}

	// Normal keys
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showKeys = !m.showKeys
		return m, nil

	case "esc":
		if m.showKeys {
			m.showKeys = false
			return m, nil
		}
		if m.filterText != "" {
			m.filterText = ""
			m.filterInput.SetValue("")
			m.applyRestoreFilter()
			return m, m.refreshDetailCache()
		}
		if m.phase == phaseVersionList {
			m.resultMsg = ""
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
			return m, computeVersionSizeCmd(item.version.Dir)
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
			m.resultMsg = ""
			return m, computeVersionSizeCmd(item.version.Dir)
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
		if sizeCmd := m.refreshDetailCache(); sizeCmd != nil {
			return m, tea.Batch(cmd, sizeCmd)
		}
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
	versions, err := backup.ListBackupVersionsLite(m.backupDir, m.selectedTarget)
	if err != nil || len(versions) == 0 {
		// No versions left — refresh target list and go back
		m.refreshTargetList()
		m.phase = phaseTargetList
		m.selectedTarget = ""
		return m, nil
	}
	m.versionItems = versions
	m.versionSizeCache = make(map[string]int64) // reset for new target

	listItems := make([]list.Item, len(versions))
	for i, v := range versions {
		listItems[i] = restoreVersionItem{version: v}
	}

	lw := m.listWidth()
	vl := newRestoreList(listItems)
	if m.termWidth > 0 {
		vl.SetSize(lw, m.restorePanelHeight())
	}

	m.versionList = vl
	m.matchCount = len(versions)
	m.phase = phaseVersionList
	m.detailScroll = 0
	m.invalidateDetailCache()
	sizeCmd := m.refreshDetailCache()
	return m, sizeCmd
}

func (m restoreTUIModel) startRestore() (tea.Model, tea.Cmd) {
	m.phase = phaseExecuting
	targetName := m.selectedTarget
	version := *m.selectedVersion
	targets := m.targets
	cfgPath := m.cfgPath

	cmd := func() tea.Msg {
		start := time.Now()

		var destPath string
		if isAgentBackupEntry(targetName) {
			destPath = resolveAgentBackupPath(targets, targetName)
		} else {
			if tc, ok := targets[targetName]; ok {
				destPath = tc.SkillsConfig().Path
			}
		}
		if destPath == "" {
			return restoreDoneMsg{err: fmt.Errorf("target '%s' not found in config", targetName)}
		}

		backupPath := filepath.Dir(version.Dir)
		opts := backup.RestoreOptions{Force: true}
		err := backup.RestoreToPath(backupPath, targetName, destPath, opts)

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
		entries, err := os.ReadDir(tsDir)
		if err != nil {
			return restoreDoneMsg{err: fmt.Errorf("failed to inspect backup directory %s: %w", tsDir, err), action: "delete"}
		}
		otherTargets := 0
		for _, e := range entries {
			if e.IsDir() {
				otherTargets++
			}
		}

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
	return max(termWidth-restoreListWidth(termWidth), 30)
}

// listWidth is the list's width: the left panel, or the full width when
// the details sit below it.
func (m restoreTUIModel) listWidth() int {
	if m.termWidth < restoreMinSplitWidth {
		return m.termWidth
	}
	return restoreListWidth(m.termWidth)
}

// restorePanelHeight returns the list height.
func (m restoreTUIModel) restorePanelHeight() int {
	bodyHeight := max(m.termHeight-frameChrome, 6)
	if m.termWidth < restoreMinSplitWidth {
		return max(bodyHeight/2, 4) // narrow: the details sit below the list
	}
	return bodyHeight
}

// --- Views ---

func (m restoreTUIModel) View() string {
	if m.quitting {
		return ""
	}
	listView := m.targetList.View()
	if m.phase != phaseTargetList && m.selectedTarget != "" {
		listView = m.versionList.View()
	}
	bodyHeight := max(m.termHeight-frameChrome, 6)
	title := m.renderTitleLine()
	if m.termWidth < restoreMinSplitWidth {
		detailHeight := max(bodyHeight-m.restorePanelHeight()-1, 4)
		detail := lipgloss.NewStyle().Height(detailHeight).MaxHeight(detailHeight).PaddingLeft(1).
			Render(m.renderRight(m.termWidth-2, detailHeight))
		return title + "\n\n" + listView + "\n\n" + detail + "\n" + m.renderBottom()
	}
	leftWidth := restoreListWidth(m.termWidth)
	rightWidth := restoreDetailWidth(m.termWidth)
	return title + "\n\n" +
		renderFrameSplit(listView, m.renderRight(rightWidth-2, bodyHeight), leftWidth, rightWidth, bodyHeight) + "\n" +
		m.renderBottom()
}

// renderTitleLine renders the target count, or the chosen target and its
// backup count.
func (m restoreTUIModel) renderTitleLine() string {
	if m.selectedTarget == "" {
		return renderFrameTitle(m.termWidth, "restore", []string{countNoun(len(m.targetItems), "target")}, nil)
	}
	name := m.selectedTarget
	if isAgentBackupEntry(name) {
		name = agentBaseTarget(name) + " agents"
	}
	return renderFrameTitle(m.termWidth, "restore", []string{name, countNoun(len(m.versionItems), "backup")}, nil)
}

// renderRight renders the detail panel, or the key list while ? is on.
func (m restoreTUIModel) renderRight(width, height int) string {
	if m.showKeys {
		return renderKeysPanel(restoreKeyGroups)
	}
	detail, _ := wrapAndScroll(m.buildDetailContent(), width, m.detailScroll, height)
	return detail
}

// renderBottom renders the note line and the key line. Confirmations, the
// running operation, the filter input and the final result take over the
// key line in place.
func (m restoreTUIModel) renderBottom() string {
	note := ""
	if m.resultMsg != "" {
		note = "  " + m.resultMsg
	}
	var line string
	switch {
	case m.phase == phaseExecuting:
		verb := "Restoring " + m.selectedTarget + " from "
		if m.confirmAction == "delete" {
			verb = "Deleting backup "
		}
		line = renderBusyLine(m.termWidth, m.opSpinner.View(), verb+m.selectedVersion.Label+"…")
	case m.phase == phaseDone:
		line = renderKeyLine(m.termWidth, []keyHint{{"any key", "quit"}}, "")
	case m.phase == phaseConfirm:
		note = theme.Dim().Render("  " + m.confirmNote())
		if m.confirmAction == "delete" {
			line = renderConfirmLine(m.termWidth, "Delete backup "+m.selectedVersion.Label+"?", true, "")
		} else {
			line = renderConfirmLine(m.termWidth, "Restore "+m.selectedTarget+" from "+m.selectedVersion.Label+"?", false, "")
		}
	case m.filtering:
		line = renderFilterLine(m.termWidth, m.filterInput.View(), m.matchCount)
	case m.showKeys:
		line = renderKeyLine(m.termWidth, []keyHint{{"?/esc", "close"}}, "")
	default:
		filter := keyHint{"/", "filter"}
		if m.filterText != "" {
			filter = keyHint{"esc", "clear filter"}
		}
		hints := []keyHint{{"↑↓", "move"}, filter, {"enter", "backups"}, {"?", "keys"}}
		if m.phase == phaseVersionList {
			hints = []keyHint{{"↑↓", "move"}, filter, {"enter", "restore"}, {"d", "delete"}, {"esc", "back"}, {"?", "keys"}}
		}
		line = renderKeyLine(m.termWidth, hints, framePosition(m.activeListIndex()+1, m.matchCount))
	}
	return note + "\n" + line
}

// confirmNote says what the pending restore or delete covers.
func (m restoreTUIModel) confirmNote() string {
	v := m.selectedVersion
	size := "size unknown"
	if sz, ok := m.versionSizeCache[v.Dir]; ok {
		size = formatBytes(sz)
	} else if v.TotalSize >= 0 {
		size = formatBytes(v.TotalSize)
	}
	what := countNoun(v.SkillCount, "skill") + " · " + size
	if m.confirmAction == "delete" {
		return what + " · cannot be undone"
	}
	return what + " · replaces what is in the target now"
}

// restoreKeyGroups lists every key for the ? panel.
var restoreKeyGroups = []keyGroup{
	{"Move", []keyHint{
		{"↑↓", "move"},
		{"←→", "page"},
		{"/", "filter"},
		{"enter", "open a target's backups, or restore a backup"},
		{"ctrl+d/u", "scroll the details"},
		{"esc", "clear the filter, go back, then quit"},
		{"q", "quit"},
	}},
	{"Backups", []keyHint{
		{"d", "delete the backup"},
	}},
}

// refreshDetailCache recomputes the detail content only when the selection or phase changes.
// Returns a tea.Cmd if an async size computation is needed (nil otherwise).
func (m *restoreTUIModel) refreshDetailCache() tea.Cmd {
	idx := m.activeListIndex()
	if idx == m.cachedDetailIdx && m.phase == m.cachedDetailPhase && m.cachedDetailStr != "" {
		return nil
	}
	m.cachedDetailIdx = idx
	m.cachedDetailPhase = m.phase
	switch m.phase {
	case phaseTargetList:
		if item, ok := m.targetList.SelectedItem().(restoreTargetItem); ok {
			m.cachedDetailStr = m.renderTargetDetail(item.summary)
			return nil
		}
	case phaseVersionList:
		if item, ok := m.versionList.SelectedItem().(restoreVersionItem); ok {
			v := item.version
			if v.TotalSize < 0 {
				if cached, ok := m.versionSizeCache[v.Dir]; ok {
					v.TotalSize = cached
				} else {
					// Render now without size; dispatch async computation
					m.cachedDetailStr = m.renderVersionDetail(v)
					return computeVersionSizeCmd(v.Dir)
				}
			}
			m.cachedDetailStr = m.renderVersionDetail(v)
			return nil
		}
	}
	m.cachedDetailStr = ""
	return nil
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

// --- Detail renderers ---

func (m restoreTUIModel) renderTargetDetail(s backup.TargetBackupSummary) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(10).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(s.TargetName))
	b.WriteString("\n\n")

	if isAgentBackupEntry(s.TargetName) {
		agentPath := resolveAgentBackupPath(m.targets, s.TargetName)
		if agentPath != "" {
			row("Path", shortenPath(agentPath))
			row("Status", describeTargetState(agentPath))
		}
	} else if t, ok := m.targets[s.TargetName]; ok {
		sc := t.SkillsConfig()
		row("Path", shortenPath(sc.Path))
		if sc.Mode != "" {
			row("Mode", sc.Mode)
		}
		row("Status", describeTargetState(sc.Path))
	}

	b.WriteString("\n")
	row("Backups", fmt.Sprintf("%d", s.BackupCount))
	row("Latest", s.Latest.Format("2006-01-02 15:04")+theme.Dim().Render(" · "+timeAgo(s.Latest)))
	row("Oldest", s.Oldest.Format("2006-01-02 15:04")+theme.Dim().Render(" · "+timeAgo(s.Oldest)))

	// Preview skills from latest backup — read directory directly instead of
	// calling ListBackupVersions (which would walk all versions + dirSize).
	latestDir := filepath.Join(m.backupDir, s.Latest.Format("2006-01-02_15-04-05"), s.TargetName)
	if skillEntries, err := os.ReadDir(latestDir); err == nil {
		var skillNames []string
		for _, se := range skillEntries {
			if se.IsDir() {
				skillNames = append(skillNames, se.Name())
			}
		}
		sort.Strings(skillNames)

		if len(skillNames) > 0 {
			b.WriteString("\n")
			b.WriteString(theme.Primary().Bold(true).Render("In the latest backup"))
			b.WriteString("\n")
			const maxPreview = 20
			show := skillNames
			if len(show) > maxPreview {
				show = show[:maxPreview]
			}
			for _, name := range show {
				desc := readSkillDescription(filepath.Join(latestDir, name))
				if desc != "" {
					b.WriteString(lipgloss.NewStyle().Render("  " + name))
					b.WriteString("\n")
					b.WriteString(theme.Dim().Render("    " + truncateStr(desc, 60)))
					b.WriteString("\n")
				} else {
					b.WriteString(lipgloss.NewStyle().Render("  " + name))
					b.WriteString("\n")
				}
			}
			if len(skillNames) > maxPreview {
				b.WriteString(theme.Dim().Render(fmt.Sprintf("  ... and %d more", len(skillNames)-maxPreview)))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func (m restoreTUIModel) renderVersionDetail(v backup.BackupVersion) string {
	var b strings.Builder

	row := func(label, value string) {
		b.WriteString(theme.Dim().Width(10).Render(label))
		b.WriteString(value)
		b.WriteString("\n")
	}

	b.WriteString(theme.Primary().Bold(true).Render(v.Label))
	b.WriteString("\n\n")
	row("Taken", timeAgo(v.Timestamp))
	row("Skills", fmt.Sprintf("%d", v.SkillCount))
	if v.TotalSize >= 0 {
		row("Size", formatBytes(v.TotalSize))
	} else {
		row("Size", theme.Dim().Render("calculating…"))
	}

	var diffPath string
	if isAgentBackupEntry(m.selectedTarget) {
		diffPath = resolveAgentBackupPath(m.targets, m.selectedTarget)
	} else if t, ok := m.targets[m.selectedTarget]; ok {
		diffPath = t.SkillsConfig().Path
	}
	if diffPath != "" {
		added, removed, common := diffSkillSets(v.SkillNames, listDirNames(diffPath))
		if len(added) > 0 || len(removed) > 0 {
			b.WriteString("\n")
			b.WriteString(theme.Primary().Bold(true).Render("Compared with the target now"))
			b.WriteString("\n")
			if len(common) > 0 {
				row("Same", countNoun(len(common), "skill"))
			}
			if len(added) > 0 {
				b.WriteString(theme.Dim().Width(10).Render("Brings"))
				b.WriteString(theme.Success().Render(fmt.Sprintf("+%d", len(added))) + theme.Dim().Render(" in the backup, not in the target"))
				b.WriteString("\n")
				for _, name := range added {
					b.WriteString(theme.Success().Render("  + " + name))
					b.WriteString("\n")
				}
			}
			if len(removed) > 0 {
				b.WriteString(theme.Dim().Width(10).Render("Removes"))
				b.WriteString(theme.Danger().Render(fmt.Sprintf("-%d", len(removed))) + theme.Dim().Render(" in the target, not in the backup"))
				b.WriteString("\n")
				for _, name := range removed {
					b.WriteString(theme.Danger().Render("  - " + name))
					b.WriteString("\n")
				}
			}
		} else if len(common) > 0 {
			b.WriteString("\n")
			b.WriteString(theme.Success().Render("✓") + " Same skills as the target now")
			b.WriteString("\n")
		}
	}

	// Skill list with descriptions (cap I/O at 20 skills)
	if len(v.SkillNames) > 0 {
		b.WriteString("\n")
		b.WriteString(theme.Primary().Bold(true).Render("Contents"))
		b.WriteString("\n")
		const maxDetail = 20
		for i, name := range v.SkillNames {
			if i < maxDetail {
				desc := readSkillDescription(filepath.Join(v.Dir, name))
				files := listSkillFiles(filepath.Join(v.Dir, name))
				b.WriteString(lipgloss.NewStyle().Render("  " + name))
				b.WriteString("\n")
				if desc != "" {
					b.WriteString(theme.Dim().Render("    " + truncateStr(desc, 60)))
					b.WriteString("\n")
				}
				if len(files) > 0 {
					b.WriteString(theme.Dim().Render("    " + strings.Join(files, "  ")))
					b.WriteString("\n")
				}
			} else {
				b.WriteString(theme.Dim().Render("  " + name))
				b.WriteString("\n")
			}
		}
		if len(v.SkillNames) > maxDetail {
			b.WriteString(theme.Dim().Render(fmt.Sprintf("  ... %d skill(s) above shown without details", len(v.SkillNames)-maxDetail)))
			b.WriteString("\n")
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
			return theme.Warning().Render("not found")
		}
		return theme.Danger().Render("error")
	}
	if utils.IsLinkMode(path, info.Mode()) {
		dest, _ := os.Readlink(path)
		return theme.Accent().Render("symlink → " + dest)
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
