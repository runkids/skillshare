package memory

import (
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type IndexStatus struct {
	Version     string   `json:"version"`
	Unindexed   []string `json:"unindexed"`
	BrokenLinks []string `json:"broken_links"`
	Error       string   `json:"error,omitempty"`
}

// indexLinks resolves ordinary Markdown links, excluding examples in code blocks.
func indexLinks(content string) map[string]bool {
	links := map[string]bool{}
	doc := goldmark.DefaultParser().Parse(text.NewReader([]byte(content)))
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := node.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		u, err := url.Parse(string(link.Destination))
		if err == nil && u.Scheme == "" && u.Host == "" && u.Path != "" && !strings.HasPrefix(u.Path, "/") {
			p := path.Clean(u.Path)
			if !strings.HasPrefix(p, "../") && strings.EqualFold(path.Ext(p), ".md") {
				links[p] = true
			}
		}
		return ast.WalkContinue, nil
	})
	return links
}

func InspectIndex(root string, notes []Note) IndexStatus {
	status := IndexStatus{Unindexed: []string{}, BrokenLinks: []string{}}
	index, err := Read(root, "INDEX.md")
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Version = index.Version
	links := indexLinks(index.Content)
	present := map[string]bool{}
	for _, note := range notes {
		present[note.Path] = true
		if note.Path != "INDEX.md" && note.Invalid == "" && !links[note.Path] {
			status.Unindexed = append(status.Unindexed, note.Path)
		}
	}
	for link := range links {
		if !present[link] {
			status.BrokenLinks = append(status.BrokenLinks, link)
		}
	}
	sort.Strings(status.Unindexed)
	sort.Strings(status.BrokenLinks)
	return status
}

// LinkFromIndex appends one explicit link without interpreting user-owned sections.
func LinkFromIndex(root, rel, version string) (Note, error) {
	note, err := Read(root, rel)
	if err != nil {
		return Note{}, err
	}
	index, err := Read(root, "INDEX.md")
	if err != nil {
		return Note{}, err
	}
	if version == "" || index.Version != version {
		return Note{}, ErrConflict
	}
	if indexLinks(index.Content)[rel] {
		return index, nil
	}
	title := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "\n", " ", "\r", " ").Replace(note.Title)
	dest := (&url.URL{Path: rel}).EscapedPath()
	content := index.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n- [" + title + "](<" + dest + ">)\n"
	return Write(root, "INDEX.md", content, version)
}
