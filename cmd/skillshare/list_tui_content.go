package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/theme"
	"skillshare/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	xansi "github.com/charmbracelet/x/ansi"
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
	m.contentSkillKey = e.RelPath
	m.contentKind = e.Kind
	m.contentScroll = 0
	m.treeCursor = 0
	m.treeScroll = 0

	if e.Kind == "agent" {
		// Agents are single .md files — render directly, minimal tree
		agentFile := filepath.Join(m.agentsSourcePath, e.RelPath)
		if _, err := os.Stat(agentFile); err != nil {
			m.contentText = fmt.Sprintf("(error reading agent: %v)", err)
			m.treeAllNodes = nil
			m.treeNodes = nil
			return
		}
		// Front matter is in the details; rendering it turns "---" into rules.
		raw := printableText(utils.ReadSkillBody(agentFile))
		if raw == "" {
			m.contentText = "(empty)"
		} else {
			w := m.contentPanelWidth()
			m.contentText = hardWrapContent(renderMarkdown(raw, w), w)
		}
		// Single-file tree: just the agent .md file
		m.treeAllNodes = []treeNode{{name: filepath.Base(e.RelPath), relPath: e.RelPath}}
		m.treeNodes = m.treeAllNodes
		return
	}

	// Existing skill directory logic
	skillDir := filepath.Join(m.sourcePath, e.RelPath)
	m.treeAllNodes = buildTreeNodes(skillDir)
	m.treeNodes = buildVisibleNodes(m.treeAllNodes)

	if len(m.treeNodes) == 0 {
		m.contentText = "(no files)"
		return
	}

	// Auto-preview the first file (SKILL.md if present)
	autoPreviewFile(m)
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

	var filePath string
	if m.contentKind == "agent" {
		filePath = filepath.Join(m.agentsSourcePath, node.relPath)
	} else {
		skillDir := filepath.Join(m.sourcePath, m.contentSkillKey)
		filePath = filepath.Join(skillDir, node.relPath)
	}

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
	rawText = printableText(rawText)

	if rawText == "" {
		m.contentText = "(empty)"
		return
	}

	w := m.contentPanelWidth()

	if strings.HasSuffix(strings.ToLower(node.name), ".md") {
		m.contentText = hardWrapContent(renderMarkdown(rawText, w), w)
		return
	}

	m.contentText = hardWrapContent(rawText, w)
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

func (m *listTUIModel) contentPanelWidth() int {
	return fileViewerTextWidth(m.termWidth, m.contentKind == "agent")
}

// hardWrapContent hard-wraps content so every logical line fits within width.
// This ensures scroll calculations (based on line count) match the visual
// line count in the panel, preventing bottom content from being clipped.
func hardWrapContent(text string, width int) string {
	if width <= 0 {
		return text
	}
	return xansi.Hardwrap(text, width, false)
}

// ─── Glamour Markdown Rendering ──────────────────────────────────────

// contentGlamourStyle returns a glamour style tuned for the dual-pane list
// detail view. The base config tracks the resolved terminal theme so body
// text stays legible on light backgrounds (glamour's dark base uses light
// gray prose which disappears on white). Backgrounds and margins are
// stripped to avoid bleeding in the constrained layout.
func contentGlamourStyle() ansi.StyleConfig {
	var s ansi.StyleConfig
	if theme.Get().Mode == theme.ModeLight {
		s = styles.LightStyleConfig
	} else {
		s = styles.DarkStyleConfig
	}
	zero := uint(0)

	s.Document.Margin = &zero

	s.H1.StylePrimitive.BackgroundColor = nil
	s.H1.StylePrimitive.Color = stringPtr("6")
	s.H1.StylePrimitive.Bold = boolPtr(true)
	s.H1.StylePrimitive.Prefix = "# "
	s.H1.StylePrimitive.Suffix = ""

	s.Code.StylePrimitive.BackgroundColor = nil
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

// ─── Keyboard Handling ───────────────────────────────────────────────

// handleContentKey handles keyboard input in the content viewer.
// Agents (single file) only support scroll; skills support tree navigation + scroll.
func (m listTUIModel) handleContentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.showContent = false
		return m, nil

	// Left tree: navigate (skills only — agents have no sidebar)
	case "j", "down":
		if m.contentKind != "agent" && m.treeCursor < len(m.treeNodes)-1 {
			m.treeCursor++
			m.ensureTreeCursorVisible()
			autoPreviewFile(&m)
		}
		return m, nil
	case "k", "up":
		if m.contentKind != "agent" && m.treeCursor > 0 {
			m.treeCursor--
			m.ensureTreeCursorVisible()
			autoPreviewFile(&m)
		}
		return m, nil
	case "l", "right", "enter":
		if m.contentKind != "agent" && len(m.treeNodes) > 0 && m.treeCursor < len(m.treeNodes) {
			node := m.treeNodes[m.treeCursor]
			if node.isDir {
				toggleTreeDir(&m)
			}
		}
		return m, nil
	case "h", "left":
		if m.contentKind != "agent" {
			collapseOrParent(&m)
		}
		return m, nil

	// Content scroll
	case "ctrl+d":
		half := m.contentViewHeight() / 2
		max := m.contentMaxScroll()
		m.contentScroll += half
		if m.contentScroll > max {
			m.contentScroll = max
		}
		return m, nil
	case "ctrl+u":
		half := m.contentViewHeight() / 2
		m.contentScroll -= half
		if m.contentScroll < 0 {
			m.contentScroll = 0
		}
		return m, nil
	case "G":
		m.contentScroll = m.contentMaxScroll()
		return m, nil
	case "g":
		m.contentScroll = 0
		return m, nil
	}
	return m, nil
}

// ─── Rendering ───────────────────────────────────────────────────────

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

// renderContentOverlay renders the file viewer. An agent is a single .md
// file, so it has no file tree.
func renderContentOverlay(m listTUIModel) string {
	return renderFileViewer(m.termWidth, m.termHeight, fileViewer{
		command: "list", name: filepath.Base(m.contentSkillKey),
		nodes: m.treeNodes, cursor: m.treeCursor, scroll: m.treeScroll,
		content: m.contentText, contentScroll: m.contentScroll,
		noTree: m.contentKind == "agent",
	})
}

// ─── Mouse Handling ──────────────────────────────────────────────────

// handleContentMouse handles mouse events in the dual-pane content viewer.
// Left side = tree navigation, right side = content scrolling.
func (m listTUIModel) handleContentMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	inSidebar := m.contentKind != "agent" && msg.X < sidebarWidth(m.termWidth) // an agent has no tree

	switch {
	case msg.Button == tea.MouseButtonWheelUp:
		if inSidebar {
			if m.treeCursor > 0 {
				m.treeCursor--
				m.ensureTreeCursorVisible()
				autoPreviewFile(&m)
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
				autoPreviewFile(&m)
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
				node := m.treeNodes[idx]
				if node.isDir {
					toggleTreeDir(&m)
				} else {
					loadContentFile(&m)
				}
			}
		}
	}
	return m, nil
}

// ─── Tree Navigation Helpers ─────────────────────────────────────────

// contentViewHeight returns the usable height for the content area.
func (m *listTUIModel) contentViewHeight() int {
	return fileViewerHeight(m.termHeight)
}

// contentMaxScroll returns the maximum scroll offset for the current content.
func (m *listTUIModel) contentMaxScroll() int {
	lines := strings.Split(m.contentText, "\n")
	maxScroll := len(lines) - m.contentViewHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// ensureTreeCursorVisible adjusts treeScroll so the cursor is within the visible range.
func (m *listTUIModel) ensureTreeCursorVisible() {
	contentHeight := m.contentViewHeight()
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
func collapseOrParent(m *listTUIModel) {
	if len(m.treeNodes) == 0 || m.treeCursor >= len(m.treeNodes) {
		return
	}
	node := m.treeNodes[m.treeCursor]

	if node.isDir && node.expanded {
		toggleTreeDir(m)
		return
	}

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
