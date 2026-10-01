package hooks

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
)

var gitSectionHeader = regexp.MustCompile(`(?m)^[\t ]*\[[^\r\n]+\][\t ]*(?:[#;][^\r\n]*)?\r?$`)

type gitRawSection struct {
	header     string
	start, end int
}

func gitRawSections(data []byte) []gitRawSection {
	indices := gitSectionHeader.FindAllIndex(data, -1)
	sections := make([]gitRawSection, len(indices))
	for i, loc := range indices {
		end := len(data)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}
		sections[i] = gitRawSection{header: string(data[loc[0]:loc[1]]), start: loc[0], end: end}
	}
	return sections
}

func isGitHookHeader(header, name string) bool {
	pattern := `(?i:^\s*\[hook)(?:\s+"` + regexp.QuoteMeta(name) + `"|\.` + regexp.QuoteMeta(name) + `)\][\t ]*(?:[#;].*)?\r?$`
	return regexp.MustCompile(pattern).MatchString(header)
}

func (s *Service) gitParseBytes(data []byte) ([]gitConfigValue, error) {
	out, err := s.gitRunner().Run("", data, "config", "--file", "-", "--no-includes", "--null", "--show-origin", "--show-scope", "--list")
	if err != nil {
		return nil, fmt.Errorf("Git could not parse config section safely: %w", err)
	}
	return parseGitConfig(out)
}

func sectionRows(rows []gitConfigValue, name string, keep bool) []gitConfigValue {
	out := []gitConfigValue{}
	for _, row := range rows {
		section, _, ok := gitHookKey(row.Key)
		if (ok && section == name) == keep {
			row.Origin = ""
			row.Scope = ""
			out = append(out, row)
		}
	}
	return out
}

// Git parses both versions independently; unsupported syntax fails closed if the
// narrow text edit changes another section or cannot remove every desired key.
func (s *Service) removeGitSection(data []byte, name string) ([]byte, []gitSectionFragment, error) {
	before, err := s.gitParseBytes(data)
	if err != nil {
		return nil, nil, err
	}
	sections := gitRawSections(data)
	var fragments []gitSectionFragment
	var out bytes.Buffer
	pos := 0
	countHeader := func(header string) int {
		n := 0
		for _, sec := range sections {
			if sec.header == header && !isGitHookHeader(sec.header, name) {
				n++
			}
		}
		return n
	}
	for i, sec := range sections {
		if !isGitHookHeader(sec.header, name) {
			continue
		}
		fragment := gitSectionFragment{Text: string(data[sec.start:sec.end])}
		for j := i + 1; j < len(sections); j++ {
			if !isGitHookHeader(sections[j].header, name) {
				fragment.Next = sections[j].header
				break
			}
		}
		for j := i - 1; j >= 0; j-- {
			if !isGitHookHeader(sections[j].header, name) {
				fragment.Previous = sections[j].header
				break
			}
		}
		if fragment.Next != "" && countHeader(fragment.Next) != 1 || fragment.Next == "" && fragment.Previous != "" && countHeader(fragment.Previous) != 1 {
			return nil, nil, fmt.Errorf("hook.%s section anchors are ambiguous; remove it manually", name)
		}
		out.Write(data[pos:sec.start])
		pos = sec.end
		fragments = append(fragments, fragment)
	}
	out.Write(data[pos:])
	after, err := s.gitParseBytes(out.Bytes())
	if err != nil {
		return nil, nil, err
	}
	if len(fragments) == 0 || len(sectionRows(after, name, true)) != 0 || !reflect.DeepEqual(sectionRows(before, name, false), sectionRows(after, name, false)) {
		return nil, nil, fmt.Errorf("unsupported hook.%s section syntax; remove it manually", name)
	}
	return out.Bytes(), fragments, nil
}

func (s *Service) gitSectionPlan(d gitDestination, name string) (*filePlan, error) {
	data, exists, mode, err := safeRead(d.includeTarget)
	if err != nil {
		return nil, err
	}
	_, fragments, err := s.removeGitSection(data, name)
	if err != nil {
		return nil, err
	}
	section := gitSectionDigest(data, name)
	var before []byte
	for _, fragment := range fragments {
		before = append(before, []byte(fragment.Text)...)
	}
	return &filePlan{path: d.includeTarget, target: "git", root: d.root, base: filepath.Dir(d.includeTarget), kind: kindGitSection, exists: exists, mode: mode, section: section, before: before, gitService: s, gitSection: &gitSectionOp{Name: name, Fragments: fragments}}, nil
}

func (f *filePlan) refreshGitSection() ([]byte, bool, os.FileMode, *nativeDoc, error) {
	if err := checkPath(f.base, f.path); err != nil {
		return nil, false, 0, nil, err
	}
	data, exists, mode, err := safeRead(f.path)
	if err != nil {
		return nil, false, 0, nil, err
	}
	rows, err := f.gitService.gitParseBytes(data)
	if err != nil {
		return nil, false, 0, nil, err
	}
	_ = rows
	if gitSectionDigest(data, f.gitSection.Name) != f.section {
		return nil, false, 0, nil, fmt.Errorf("hook.%s changed since preview: %w", f.gitSection.Name, ErrStaleRevision)
	}
	return f.before, exists, mode, nil, nil
}

func gitSectionDigest(data []byte, name string) string {
	var section []byte
	for _, s := range gitRawSections(data) {
		if isGitHookHeader(s.header, name) {
			section = append(section, data[s.start:s.end]...)
		}
	}
	return digest(section)
}

func (s *Service) restoreGitSection(data []byte, op gitSectionOp) ([]byte, error) {
	before, err := s.gitParseBytes(data)
	if err != nil {
		return nil, err
	}
	if len(sectionRows(before, op.Name, true)) > 0 {
		return nil, fmt.Errorf("hook.%s exists after backup: %w", op.Name, ErrStaleRevision)
	}
	out := append([]byte(nil), data...)
	// Adjacent repeated sections share anchors. Insert them as one fragment so
	// placing them after a previous header does not reverse their native order.
	var fragments []gitSectionFragment
	for _, fragment := range op.Fragments {
		if len(fragments) > 0 && fragments[len(fragments)-1].Next == fragment.Next && fragments[len(fragments)-1].Previous == fragment.Previous {
			fragments[len(fragments)-1].Text += fragment.Text
		} else {
			fragments = append(fragments, fragment)
		}
	}
	for _, fragment := range fragments {
		sections := gitRawSections(out)
		anchor := fragment.Next
		after := false
		if anchor == "" {
			anchor = fragment.Previous
			after = true
		}
		position, count := 0, 0
		for _, sec := range sections {
			if sec.header == anchor {
				count++
				position = sec.start
				if after {
					position = sec.end
				}
			}
		}
		if anchor != "" && count != 1 {
			return nil, fmt.Errorf("hook.%s section anchor changed or is ambiguous", op.Name)
		}
		if anchor == "" {
			position = 0
		}
		insert := []byte(fragment.Text)
		out = append(append(append([]byte(nil), out[:position]...), insert...), out[position:]...)
	}
	parsed, err := s.gitParseBytes(out)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(sectionRows(before, op.Name, false), sectionRows(parsed, op.Name, false)) {
		return nil, fmt.Errorf("section restore would modify other Git settings")
	}
	return out, nil
}
