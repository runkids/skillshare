package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
)

// treeNode represents a file or directory in the sidebar tree.
type treeNode struct {
	name     string // file or directory name
	relPath  string // path relative to skill directory (used for reading)
	isDir    bool
	expanded bool
	depth    int
}

const (
	maxTreeDepth = 3
	maxTreeFiles = 200
)

// skipDirName returns true for directories that should be hidden in the tree.
func skipDirName(name string) bool {
	switch {
	case strings.HasPrefix(name, "."):
		return true
	case name == "__pycache__":
		return true
	case name == "node_modules":
		return true
	}
	return false
}

// buildTreeNodes recursively scans skillDir and produces a flat list of treeNode.
// SKILL.md is placed first at depth 0. Directories default to collapsed.
func buildTreeNodes(skillDir string) []treeNode {
	var nodes []treeNode
	count := 0

	var walk func(dir, relPrefix string, depth int)
	walk = func(dir, relPrefix string, depth int) {
		if depth > maxTreeDepth || count >= maxTreeFiles {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		var dirs, files []os.DirEntry
		for _, e := range entries {
			if skipDirName(e.Name()) {
				continue
			}
			if e.IsDir() {
				dirs = append(dirs, e)
			} else {
				files = append(files, e)
			}
		}

		for _, e := range dirs {
			if count >= maxTreeFiles {
				return
			}
			rel := filepath.Join(relPrefix, e.Name())
			nodes = append(nodes, treeNode{
				name:    e.Name(),
				relPath: rel,
				isDir:   true,
				depth:   depth,
			})
			count++
			walk(filepath.Join(dir, e.Name()), rel, depth+1)
		}
		for _, e := range files {
			if count >= maxTreeFiles {
				return
			}
			rel := filepath.Join(relPrefix, e.Name())
			nodes = append(nodes, treeNode{
				name:    e.Name(),
				relPath: rel,
				depth:   depth,
			})
			count++
		}
	}

	walk(skillDir, "", 0)

	// Move SKILL.md to front at depth 0
	skillIdx := -1
	for i, n := range nodes {
		if n.depth == 0 && n.name == "SKILL.md" {
			skillIdx = i
			break
		}
	}
	if skillIdx > 0 {
		node := nodes[skillIdx]
		copy(nodes[1:skillIdx+1], nodes[:skillIdx])
		nodes[0] = node
	}

	return nodes
}

// buildVisibleNodes filters treeAllNodes to only include visible nodes.
func buildVisibleNodes(all []treeNode) []treeNode {
	var visible []treeNode
	skipUntilDepth := -1

	for _, n := range all {
		if skipUntilDepth >= 0 && n.depth > skipUntilDepth {
			continue
		}
		skipUntilDepth = -1
		visible = append(visible, n)
		if n.isDir && !n.expanded {
			skipUntilDepth = n.depth
		}
	}
	return visible
}

// loadContentForSkill populates the content viewer fields for the given skill.
func loadContentForSkill(m *listTUIModel, e skillEntry) {
	skillDir := filepath.Join(m.sourcePath, e.RelPath)
	m.contentSkillKey = e.RelPath
	m.contentScroll = 0
	m.treeCursor = 0
	m.treeScroll = 0
	m.sidebarFocused = false

	m.treeAllNodes = buildTreeNodes(skillDir)
	m.treeNodes = buildVisibleNodes(m.treeAllNodes)

	if len(m.treeNodes) == 0 {
		m.contentText = "(no files)"
		return
	}

	loadContentFile(m)
}

// loadContentFile reads the file at treeCursor and stores rendered content.
func loadContentFile(m *listTUIModel) {
	m.contentScroll = 0

	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		m.contentText = "(no files)"
		return
	}

	node := m.treeNodes[m.treeCursor]
	if node.isDir {
		m.contentText = fmt.Sprintf("(directory: %s)", node.name)
		return
	}

	skillDir := filepath.Join(m.sourcePath, m.contentSkillKey)
	filePath := filepath.Join(skillDir, node.relPath)

	var rawText string
	if node.name == "SKILL.md" {
		rawText = utils.ReadSkillBody(filePath)
	} else {
		data, err := os.ReadFile(filePath)
		if err != nil {
			m.contentText = fmt.Sprintf("(error reading file: %v)", err)
			return
		}
		rawText = strings.TrimSpace(string(data))
	}

	if rawText == "" {
		m.contentText = "(empty)"
		return
	}

	if strings.HasSuffix(strings.ToLower(node.name), ".md") {
		m.contentText = renderMarkdown(rawText, m.contentPanelWidth())
		return
	}

	m.contentText = rawText
}

// contentPanelWidth returns the available width for the right content panel.
func (m *listTUIModel) contentPanelWidth() int {
	sw := sidebarWidth(m.termWidth)
	// 2 (left margin) + sw (sidebar) + 1 (border) + 1 (gap) + content + 1 (right margin)
	w := m.termWidth - sw - 5
	if w < 40 {
		w = 40
	}
	return w
}

// contentGlamourStyle returns a modified dark style with no backgrounds or margins
// that would bleed or overflow in the constrained dual-pane layout.
func contentGlamourStyle() ansi.StyleConfig {
	s := styles.DarkStyleConfig
	zero := uint(0)

	// Document: remove margin — we handle our own padding in the layout
	s.Document.Margin = &zero

	// H1: remove background, use cyan foreground
	s.H1.StylePrimitive.BackgroundColor = nil
	s.H1.StylePrimitive.Color = stringPtr("6")
	s.H1.StylePrimitive.Bold = boolPtr(true)
	s.H1.StylePrimitive.Prefix = "# "
	s.H1.StylePrimitive.Suffix = ""

	// Inline code: remove background (causes colored bars)
	s.Code.StylePrimitive.BackgroundColor = nil

	// Code block: remove margin
	s.CodeBlock.Margin = &zero

	return s
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }

// renderMarkdown renders markdown text with glamour for terminal display.
func renderMarkdown(text string, width int) string {
	if width < 20 {
		width = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(contentGlamourStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return text
	}
	rendered, err := r.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimSpace(rendered)
}

// sidebarWidth returns the width for the left sidebar panel.
func sidebarWidth(termWidth int) int {
	quarter := termWidth / 4
	w := 30
	if quarter < w {
		w = quarter
	}
	if w < 15 {
		w = 15
	}
	return w
}

// renderContentOverlay renders the full-screen dual-pane content viewer.
func renderContentOverlay(m listTUIModel) string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	skillName := filepath.Base(m.contentSkillKey)

	fileName := ""
	if len(m.treeNodes) > 0 && m.treeCursor < len(m.treeNodes) {
		fileName = m.treeNodes[m.treeCursor].relPath
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render(fmt.Sprintf("  %s", skillName)))
	if fileName != "" {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ─  %s", fileName)))
	}
	b.WriteString("\n")

	sw := sidebarWidth(m.termWidth)
	rw := m.termWidth - sw - 5
	if rw < 20 {
		rw = 20
	}
	contentHeight := m.termHeight - 4 // title(2) + help(1) + margin(1)
	if contentHeight < 5 {
		contentHeight = 5
	}

	sidebarStr := renderSidebarStr(m, sw, contentHeight)
	contentStr, scrollInfo := renderContentStr(m, rw, contentHeight)

	// Constrain each panel with lipgloss (ANSI-aware width/height)
	leftPanel := lipgloss.NewStyle().
		Width(sw).MaxWidth(sw).
		Height(contentHeight).MaxHeight(contentHeight).
		PaddingLeft(1).
		Render(sidebarStr)

	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Height(contentHeight).MaxHeight(contentHeight)
	borderCol := strings.Repeat("│\n", contentHeight)
	borderPanel := borderStyle.Render(strings.TrimRight(borderCol, "\n"))

	rightPanel := lipgloss.NewStyle().
		Width(rw).MaxWidth(rw).
		Height(contentHeight).MaxHeight(contentHeight).
		PaddingLeft(1).
		Render(contentStr)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, borderPanel, rightPanel)
	b.WriteString(body)
	b.WriteString("\n")

	// Help line with scroll info
	help := "j/k browse  l expand  h collapse  Enter toggle dir  Esc back  q quit"
	if scrollInfo != "" {
		help += "  " + scrollInfo
	}
	b.WriteString(tuiHelpStyle.Render(help))
	b.WriteString("\n")

	return b.String()
}

// renderSidebarStr renders the file tree as a single string for the left panel.
func renderSidebarStr(m listTUIModel, width, height int) string {
	if len(m.treeNodes) == 0 {
		return "(no files)"
	}

	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(tuiBrandYellow)
	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	fileStyle := lipgloss.NewStyle()
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	total := len(m.treeNodes)
	start := m.treeScroll
	if start > total-height {
		start = total - height
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > total {
		end = total
	}

	var lines []string
	for i := start; i < end; i++ {
		n := m.treeNodes[i]
		indent := strings.Repeat("  ", n.depth)

		var prefix string
		if n.isDir {
			if n.expanded {
				prefix = "▾ "
			} else {
				prefix = "▸ "
			}
		} else {
			prefix = "  "
		}

		name := n.name
		if n.isDir {
			name += "/"
		}

		label := indent + prefix + name

		// Truncate to fit width (reserve 2 for cursor mark + space)
		maxLabel := width - 2
		if maxLabel < 5 {
			maxLabel = 5
		}
		if len(label) > maxLabel {
			label = label[:maxLabel-3] + "..."
		}

		var line string
		if i == m.treeCursor {
			line = selectedStyle.Render(label)
		} else if n.isDir {
			line = dirStyle.Render(label)
		} else {
			line = fileStyle.Render(label)
		}
		lines = append(lines, line)
	}

	if total > height {
		lines = append(lines, dimStyle.Render(fmt.Sprintf(" (%d/%d)", m.treeCursor+1, total)))
	}

	return strings.Join(lines, "\n")
}

// renderContentStr renders the right content panel as a single string.
// Returns the rendered text and a scroll info string (empty if content fits).
func renderContentStr(m listTUIModel, width, height int) (string, string) {
	lines := strings.Split(m.contentText, "\n")
	totalLines := len(lines)

	if totalLines <= height {
		return strings.Join(lines, "\n"), ""
	}

	maxScroll := totalLines - height
	offset := m.contentScroll
	if offset > maxScroll {
		offset = maxScroll
	}

	visible := lines[offset : offset+height]
	result := make([]string, height)
	copy(result, visible)

	scrollInfo := fmt.Sprintf("(%d/%d)", offset+maxScroll-maxScroll+1, maxScroll+1)
	_ = width // reserved for future truncation

	return strings.Join(result, "\n"), scrollInfo
}

// contentMaxScroll returns the maximum scroll offset for the current content.
func (m *listTUIModel) contentMaxScroll() int {
	lines := strings.Split(m.contentText, "\n")
	height := m.termHeight - 4
	if height < 5 {
		height = 5
	}
	maxScroll := len(lines) - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// handleContentMouse handles mouse events in the content viewer overlay.
func (m listTUIModel) handleContentMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	sw := sidebarWidth(m.termWidth)
	inSidebar := msg.X < sw+3 // left panel width + border

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		if inSidebar {
			if m.treeCursor > 0 {
				m.treeCursor--
				m.ensureTreeCursorVisible()
			}
		} else {
			if m.contentScroll > 0 {
				m.contentScroll--
			}
		}
	case msg.Button == tea.MouseButtonWheelDown:
		if inSidebar {
			if m.treeCursor < len(m.treeNodes)-1 {
				m.treeCursor++
				m.ensureTreeCursorVisible()
			}
		} else {
			max := m.contentMaxScroll()
			if m.contentScroll < max {
				m.contentScroll++
			}
		}
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		if inSidebar {
			row := msg.Y - 2
			idx := m.treeScroll + row
			if idx >= 0 && idx < len(m.treeNodes) {
				m.treeCursor = idx
				m.sidebarFocused = true
				node := m.treeNodes[idx]
				if node.isDir {
					toggleTreeDir(&m)
				} else {
					loadContentFile(&m)
					m.sidebarFocused = false
				}
			}
		} else {
			m.sidebarFocused = false
		}
	}
	return m, nil
}

// autoPreviewFile loads the file under treeCursor if it's not a directory.
func autoPreviewFile(m *listTUIModel) {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	if !m.treeNodes[m.treeCursor].isDir {
		loadContentFile(m)
	}
}

// expandDir expands the directory under treeCursor (no-op if already expanded).
func expandDir(m *listTUIModel) {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	node := m.treeNodes[m.treeCursor]
	if !node.isDir || node.expanded {
		return
	}
	for i := range m.treeAllNodes {
		if m.treeAllNodes[i].relPath == node.relPath {
			m.treeAllNodes[i].expanded = true
			break
		}
	}
	m.treeNodes = buildVisibleNodes(m.treeAllNodes)
}

// collapseOrParent collapses the current directory, or jumps to the parent directory.
// If cursor is on an expanded directory → collapse it.
// If cursor is on a file or collapsed directory → jump to parent directory.
func collapseOrParent(m *listTUIModel) {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	node := m.treeNodes[m.treeCursor]

	// Expanded directory → collapse
	if node.isDir && node.expanded {
		toggleTreeDir(m)
		return
	}

	// Find parent directory (first node above with depth-1)
	if node.depth > 0 {
		for i := m.treeCursor - 1; i >= 0; i-- {
			if m.treeNodes[i].isDir && m.treeNodes[i].depth == node.depth-1 {
				m.treeCursor = i
				m.ensureTreeCursorVisible()
				return
			}
		}
	}
}

// ensureTreeCursorVisible adjusts treeScroll so the cursor is within the visible range.
func (m *listTUIModel) ensureTreeCursorVisible() {
	contentHeight := m.termHeight - 4
	if contentHeight < 5 {
		contentHeight = 5
	}
	if m.treeCursor < m.treeScroll {
		m.treeScroll = m.treeCursor
	} else if m.treeCursor >= m.treeScroll+contentHeight {
		m.treeScroll = m.treeCursor - contentHeight + 1
	}
}

// toggleTreeDir toggles expand/collapse of a directory node at treeCursor.
func toggleTreeDir(m *listTUIModel) {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	node := m.treeNodes[m.treeCursor]
	if !node.isDir {
		return
	}

	for i := range m.treeAllNodes {
		if m.treeAllNodes[i].relPath == node.relPath {
			m.treeAllNodes[i].expanded = !m.treeAllNodes[i].expanded
			break
		}
	}

	m.treeNodes = buildVisibleNodes(m.treeAllNodes)
	if m.treeCursor >= len(m.treeNodes) {
		m.treeCursor = len(m.treeNodes) - 1
	}
}

// sortTreeNodes sorts nodes: directories first, then files, alphabetically.
func sortTreeNodes(nodes []treeNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if a.depth == b.depth {
			if a.isDir != b.isDir {
				return a.isDir
			}
			return strings.ToLower(a.name) < strings.ToLower(b.name)
		}
		return false
	})
}
