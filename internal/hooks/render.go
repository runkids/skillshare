package hooks

import (
	"fmt"
	"slices"
	"strings"
)

// RenderedFile is one native output of a single hook: its contribution to a shared
// hooks file in that file's native wrapper, or a whole file it owns.
type RenderedFile struct {
	Target  string `json:"target"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// RenderNative shows what one hook writes, per Agent, without the rest of any existing
// file. It reads no ownership state or native file contents, writes nothing and runs
// nothing. A disabled entry renders its stored bindings, which sync would not write.
// A binding whose path cannot be resolved or safely written reports its own error.
func (s *Service) RenderNative(m Mutation) ([]RenderedFile, error) {
	if m.Remove || m.Replace {
		return nil, fmt.Errorf("render takes a hook definition, not a remove or replace")
	}
	if m.Name == "" || m.Entry == nil {
		return nil, fmt.Errorf("hook name and entry are required")
	}
	root := ""
	scope := s
	if m.Project != "" {
		if s.ProjectRoot != "" {
			return nil, fmt.Errorf("hooks.projects belongs in the global config")
		}
		var err error
		if root, err = projectRoot(m.Project); err != nil {
			return nil, err
		}
		scope = s.scoped(root)
	}
	entry := *m.Entry
	if err := entry.normalize(); err != nil {
		return nil, err
	}
	if err := s.validateEntry(m.Name, entry, root); err != nil {
		return nil, err
	}
	entry.Enabled = nil
	out := []RenderedFile{}
	for _, target := range sortedKeys(entry.Bindings) {
		one := Entry{Bindings: map[string]Binding{target: entry.Bindings[target]}}
		d := &desired{elements: map[string][]wantElement{}, targets: map[string]string{}, roots: map[string]string{}, files: map[string]wantFile{}}
		if err := scope.renderScope(d, root, map[string]Entry{m.Name: one}); err != nil {
			out = append(out, RenderedFile{Target: target, Error: err.Error()})
			continue
		}
		// A skipped account home writes nothing; say why instead of rendering no file.
		if len(d.warnings) > 0 {
			out = append(out, RenderedFile{Target: target, Error: strings.Join(d.warnings, "; ")})
			continue
		}
		base, err := scope.base(target, root)
		if err != nil {
			out = append(out, RenderedFile{Target: target, Error: err.Error()})
			continue
		}
		var files []RenderedFile
		for path, want := range d.elements {
			f := RenderedFile{Target: target, Path: path}
			ops := make([]elementOp, len(want))
			for i, w := range want {
				ops[i] = elementOp{Event: w.event, Index: -1, Value: w.value}
			}
			doc, _ := parseNative(scope.agentOf(target), nil)
			if data, _, err := doc.edit(ops, nil, false); err != nil {
				f.Error = err.Error()
			} else {
				f.Content = string(data)
			}
			files = append(files, f)
		}
		for path, want := range d.files {
			files = append(files, RenderedFile{Target: target, Path: path, Content: string(want.content)})
		}
		for i := range files {
			if err := checkPath(base, files[i].Path); err != nil && files[i].Error == "" {
				files[i].Error = err.Error()
			}
		}
		slices.SortFunc(files, func(a, b RenderedFile) int { return strings.Compare(a.Path, b.Path) })
		out = append(out, files...)
	}
	return out, nil
}
