package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"

	"skillshare/internal/theme"
	"skillshare/internal/utils"
)

// fileBrowser is the file viewer with its own state, for screens that open
// one skill or agent: a folder shows a tree to browse, a file is shown
// alone. Numbered shows the raw text with line numbers, so a line can be
// marked and matches what a finding reports; otherwise Markdown is rendered.
type fileBrowser struct {
	command, name string
	root          string // the skill folder, or the one file shown
	single        bool
	all, nodes    []treeNode
	cursor        int
	scroll        int
	content       string
	contentScroll int
	numbered      bool
	markFile      string // relPath of the marked line's file
	markLine      int    // 1-based; 0 marks nothing
	lineRows      []int  // numbered: the screen row each file line starts on
	width, height int
}

// maxViewFileSize matches the audit's scan limit.
const maxViewFileSize = 1_000_000

func newFileBrowser(command, name, root string, numbered bool, width, height int) *fileBrowser {
	b := &fileBrowser{command: command, name: name, root: root, numbered: numbered, width: width, height: height}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		b.single = true
		b.all = []treeNode{{name: filepath.Base(root), relPath: filepath.Base(root)}}
	} else {
		b.all = buildTreeNodes(root)
	}
	b.nodes = buildVisibleNodes(b.all)
	b.load()
	return b
}

// openAt opens rel with line marked and in view. The tree skips dotfiles
// and stops at a depth and size the audit scan goes past, so a file that
// is not in it is added at the top under its full relative path.
func (b *fileBrowser) openAt(rel string, line int) {
	rel = filepath.Clean(rel)
	if b.single {
		rel = b.nodes[0].relPath
	} else if !b.hasNode(rel) && filepath.IsLocal(rel) {
		if info, err := os.Stat(filepath.Join(b.root, rel)); err == nil && !info.IsDir() {
			b.all = append([]treeNode{{name: rel, relPath: rel}}, b.all...)
		}
	}
	for i := range b.all {
		if b.all[i].isDir && strings.HasPrefix(rel, b.all[i].relPath+string(filepath.Separator)) {
			b.all[i].expanded = true
		}
	}
	b.nodes = buildVisibleNodes(b.all)
	for i, n := range b.nodes {
		if n.relPath == rel {
			b.cursor = i
		}
	}
	b.markFile, b.markLine = rel, line
	b.keepCursorVisible()
	b.load()
	if line > 0 && line <= len(b.lineRows) && b.nodes[b.cursor].relPath == rel {
		b.contentScroll = min(max(b.lineRows[line-1]-b.textHeight()/3, 0), b.maxScroll())
	}
}

func (b *fileBrowser) hasNode(rel string) bool {
	for _, n := range b.all {
		if n.relPath == rel {
			return true
		}
	}
	return false
}

func (b *fileBrowser) resize(width, height int) {
	b.width, b.height = width, height
	scroll := b.contentScroll
	b.load()
	b.contentScroll = min(scroll, b.maxScroll())
	b.keepCursorVisible()
}

func (b *fileBrowser) textHeight() int { return fileViewerHeight(b.height) }

func (b *fileBrowser) maxScroll() int {
	return max(strings.Count(b.content, "\n")+1-b.textHeight(), 0)
}

func (b *fileBrowser) keepCursorVisible() {
	h := b.textHeight()
	if b.cursor < b.scroll {
		b.scroll = b.cursor
	} else if b.cursor >= b.scroll+h {
		b.scroll = b.cursor - h + 1
	}
}

// load reads the file under the cursor into content.
func (b *fileBrowser) load() {
	b.contentScroll = 0
	b.lineRows = nil
	if len(b.nodes) == 0 {
		b.content = theme.Dim().Render("No files")
		return
	}
	n := b.nodes[b.cursor]
	if n.isDir {
		b.content = theme.Dim().Render(n.name + "/ is a folder")
		return
	}
	path := b.root
	if !b.single {
		path = filepath.Join(b.root, n.relPath)
	}
	w := fileViewerTextWidth(b.width, b.single)
	// The audit skips files this large too; reading one could stall the TUI.
	if info, err := os.Stat(path); err == nil && info.Size() > maxViewFileSize {
		b.content = theme.Dim().Render("This file is too large to show (" + formatBytes(info.Size()) + ").")
		return
	}
	if b.numbered {
		data, err := os.ReadFile(path)
		if err != nil {
			b.content = theme.Danger().Render(err.Error())
			return
		}
		mark := 0
		if n.relPath == b.markFile {
			mark = b.markLine
		}
		b.content, b.lineRows = numberLines(printableText(string(data)), mark, w)
		return
	}
	// A skill's or agent's front matter is shown elsewhere; other files
	// keep theirs.
	var text string
	if b.single || n.name == "SKILL.md" {
		text = utils.ReadSkillBody(path)
	} else if data, err := os.ReadFile(path); err == nil {
		text = strings.TrimSpace(string(data))
	} else {
		b.content = theme.Danger().Render(err.Error())
		return
	}
	text = printableText(text)
	switch {
	case text == "":
		b.content = theme.Dim().Render("Empty")
	case strings.HasSuffix(strings.ToLower(n.name), ".md"):
		b.content = hardWrapContent(renderMarkdown(text, w), w)
	default:
		b.content = hardWrapContent(text, w)
	}
}

// numberLines renders text with dim line numbers and mark (1-based)
// highlighted. A long line wraps under its text without a number; rows
// holds the screen row each file line starts on.
func numberLines(text string, mark, width int) (string, []int) {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	digits := len(fmt.Sprint(len(lines)))
	gutter := strings.Repeat(" ", digits+2)
	var out []string
	rows := make([]int, len(lines))
	for i, line := range lines {
		rows[i] = len(out)
		parts := strings.Split(xansi.Hardwrap(strings.ReplaceAll(line, "\t", "    "), max(width-digits-2, 10), false), "\n")
		num := fmt.Sprintf("%*d ", digits, i+1)
		for j, part := range parts {
			switch {
			case i+1 == mark && j == 0:
				out = append(out, theme.Warning().Bold(true).Render(num+"›")+theme.Warning().Render(part))
			case i+1 == mark:
				out = append(out, gutter+theme.Warning().Render(part))
			case j == 0:
				out = append(out, theme.Dim().Render(num+" ")+part)
			default:
				out = append(out, gutter+part)
			}
		}
	}
	return strings.Join(out, "\n"), rows
}

// key handles the viewer's own keys and reports whether it used k.
func (b *fileBrowser) key(k string) bool {
	half := b.textHeight() / 2
	switch k {
	case "up", "k":
		if !b.single && b.cursor > 0 {
			b.cursor--
			b.keepCursorVisible()
			b.load()
		}
	case "down", "j":
		if !b.single && b.cursor < len(b.nodes)-1 {
			b.cursor++
			b.keepCursorVisible()
			b.load()
		}
	case "right", "l", "enter":
		if !b.single && len(b.nodes) > 0 && b.nodes[b.cursor].isDir && !b.nodes[b.cursor].expanded {
			b.toggle()
		}
	case "left", "h":
		b.collapse()
	case "ctrl+d", "pgdown":
		b.contentScroll = min(b.contentScroll+half, b.maxScroll())
	case "ctrl+u", "pgup":
		b.contentScroll = max(b.contentScroll-half, 0)
	case "G", "end":
		b.contentScroll = b.maxScroll()
	case "g", "home":
		b.contentScroll = 0
	default:
		return false
	}
	return true
}

// wheel scrolls the open file by delta lines.
func (b *fileBrowser) wheel(delta int) {
	b.contentScroll = min(max(b.contentScroll+delta, 0), b.maxScroll())
}

func (b *fileBrowser) toggle() {
	rel := b.nodes[b.cursor].relPath
	for i := range b.all {
		if b.all[i].relPath == rel {
			b.all[i].expanded = !b.all[i].expanded
		}
	}
	b.nodes = buildVisibleNodes(b.all)
	b.cursor = min(b.cursor, len(b.nodes)-1)
}

// collapse closes the open folder, or moves to the folder of a file.
func (b *fileBrowser) collapse() {
	if b.single || len(b.nodes) == 0 {
		return
	}
	n := b.nodes[b.cursor]
	if n.isDir && n.expanded {
		b.toggle()
		return
	}
	for i := b.cursor - 1; i >= 0; i-- {
		if b.nodes[i].isDir && b.nodes[i].depth == n.depth-1 {
			b.cursor = i
			b.keepCursorVisible()
			b.load()
			return
		}
	}
}

func (b *fileBrowser) view(note string, extraHints []keyHint) string {
	return renderFileViewer(b.width, b.height, fileViewer{
		command: b.command, name: b.name,
		nodes: b.nodes, cursor: b.cursor, scroll: b.scroll,
		content: b.content, contentScroll: b.contentScroll,
		noTree: b.single, note: note, extraHints: extraHints,
	})
}
