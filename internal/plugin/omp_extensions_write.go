package plugin

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

var (
	ErrOMPExtensionsStale    = errors.New("OMP extension inputs changed; refresh and preview again")
	ErrOMPExtensionsBusy     = errors.New("OMP native settings are locked by another writer")
	ErrOMPExtensionsReadOnly = errors.New("OMP extension selection is read-only")
)

type OMPExtensionChange struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

type OMPExtensionRowChange struct {
	Key       string `json:"key"`
	DerivedID string `json:"derivedId"`
	Before    bool   `json:"before"`
	After     bool   `json:"after"`
}

type OMPExtensionsPlan struct {
	Revision     string                  `json:"revision"`
	SettingsPath string                  `json:"settingsPath"`
	Rows         []OMPExtensionRowChange `json:"rows"`
	Warnings     []string                `json:"warnings"`
	BackupID     string                  `json:"backupId,omitempty"`
}

// ompYAML retains scalar styles, comments, ordering and unknown fields. Refuse
// aliases, duplicate keys and multi-documents rather than guessing Bun's meaning.
func ompYAML(path string) (*yaml.Node, []byte, error) {
	raw, err := ompRead(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("unreadable OMP settings")
	}
	doc := &yaml.Node{}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	err = dec.Decode(doc)
	if err == io.EOF {
		doc = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	} else if err != nil {
		return nil, nil, fmt.Errorf("malformed OMP YAML settings")
	}
	var extra yaml.Node
	if dec.Decode(&extra) != io.EOF {
		return nil, nil, fmt.Errorf("multiple OMP YAML documents are unsupported")
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode || doc.Content[0].Tag != "!!map" {
		return nil, nil, fmt.Errorf("OMP settings must be a YAML mapping")
	}
	if doc.Content[0].Style&yaml.FlowStyle != 0 {
		return nil, nil, fmt.Errorf("flow-style root settings are read-only; use a block YAML mapping")
	}
	var validate func(*yaml.Node) error
	validate = func(n *yaml.Node) error {
		if n.Kind == yaml.AliasNode || n.Anchor != "" {
			return fmt.Errorf("OMP YAML aliases and anchors are unsupported")
		}
		if n.Kind == yaml.MappingNode {
			seen := map[string]bool{}
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
					return fmt.Errorf("ambiguous OMP YAML mapping keys")
				}
				seen[key.Value] = true
			}
		}
		for _, child := range n.Content {
			if err := validate(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(doc); err != nil {
		return nil, nil, err
	}
	for i := 0; i < len(doc.Content[0].Content); i += 2 {
		if doc.Content[0].Content[i].Column != 1 {
			return nil, nil, fmt.Errorf("indented root YAML settings are read-only")
		}
		if doc.Content[0].Content[i].Value == "disabledExtensions" {
			if _, err := ompDisabledNode(doc.Content[0].Content[i+1]); err != nil {
				return nil, nil, err
			}
		}
	}
	return doc, raw, nil
}

func ompDisabledNode(n *yaml.Node) ([]string, error) {
	if n.Tag == "!!null" {
		return nil, nil
	}
	if n.Kind != yaml.SequenceNode || n.Tag != "!!seq" {
		return nil, fmt.Errorf("disabledExtensions must be a string list")
	}
	values := []string{}
	for _, node := range n.Content {
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return nil, fmt.Errorf("disabledExtensions must contain only strings")
		}
		values = append(values, node.Value)
	}
	return values, nil
}

func (s *Service) ompPlan(ctx context.Context, target string, changes []OMPExtensionChange) (*OMPExtensionsPlan, *OMPExtensionsView, []byte, []byte, error) {
	v, err := s.OMPExtensions(ctx, target)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(changes) == 0 || len(changes) > 1000 {
		return nil, nil, nil, nil, fmt.Errorf("provide 1 to 1000 extension changes")
	}
	doc, before, err := ompYAML(v.SettingsPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("%w: %s", ErrOMPExtensionsReadOnly, err)
	}
	rows := map[string]OMPExtensionRow{}
	for _, r := range v.Rows {
		rows[r.Key] = r
	}
	sorted := slices.Clone(changes)
	slices.SortFunc(sorted, func(a, b OMPExtensionChange) int { return bytes.Compare([]byte(a.Key), []byte(b.Key)) })
	plan := &OMPExtensionsPlan{SettingsPath: v.SettingsPath, Rows: []OMPExtensionRowChange{}, Warnings: slices.Clone(v.Warnings)}
	root := doc.Content[0]
	var disabled *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "disabledExtensions" {
			disabled = root.Content[i+1]
			break
		}
	}
	if disabled == nil || disabled.Tag == "!!null" {
		replacement := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, id := range v.disabled {
			replacement.Content = append(replacement.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id})
		}
		if disabled == nil {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "disabledExtensions"}, replacement)
			disabled = replacement
		} else {
			replacement.HeadComment, replacement.LineComment, replacement.FootComment = disabled.HeadComment, disabled.LineComment, disabled.FootComment
			*disabled = *replacement
		}
	}
	for i, c := range sorted {
		if c.Key == "" || i > 0 && c.Key == sorted[i-1].Key {
			return nil, nil, nil, nil, fmt.Errorf("invalid or duplicate extension row key")
		}
		r, ok := rows[c.Key]
		if !ok {
			return nil, nil, nil, nil, fmt.Errorf("unknown extension row key")
		}
		if !r.Selectable || r.Enabled == nil {
			return nil, nil, nil, nil, fmt.Errorf("%w: %s", ErrOMPExtensionsReadOnly, r.ReadOnlyReason)
		}
		plan.Rows = append(plan.Rows, OMPExtensionRowChange{Key: r.Key, DerivedID: r.DerivedID, Before: *r.Enabled, After: c.Enabled})
		if c.Enabled {
			kept := disabled.Content[:0]
			for _, n := range disabled.Content {
				if n.Value != r.DerivedID {
					kept = append(kept, n)
				} else {
					// Keep comments even when their disabled ID is removed.
					for _, comment := range []string{n.HeadComment, n.LineComment, n.FootComment} {
						if comment != "" {
							disabled.HeadComment += "\n" + comment
						}
					}
				}
			}
			disabled.Content = kept
		} else {
			exists := false
			for _, n := range disabled.Content {
				exists = exists || n.Value == r.DerivedID
			}
			if !exists {
				disabled.Content = append(disabled.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: r.DerivedID})
			}
		}
	}
	after, err := ompRenderDisabled(before, root, disabled)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	plan.Revision = ompPlanRevision(v.Revision, sorted)
	return plan, v, before, after, nil
}

func (s *Service) PreviewOMPExtensions(ctx context.Context, target string, changes []OMPExtensionChange, revision string) (*OMPExtensionsPlan, error) {
	current, err := s.OMPExtensions(ctx, target)
	if err != nil {
		return nil, err
	}
	if revision == "" || current.Revision != revision {
		return nil, ErrOMPExtensionsStale
	}
	plan, v, _, _, err := s.ompPlan(ctx, target, changes)
	if err != nil {
		return nil, err
	}
	if revision == "" || v.Revision != revision {
		return nil, ErrOMPExtensionsStale
	}
	return plan, nil
}

// ApplyOMPExtensions never invokes OMP or loads extension/hook code. Its sole
// native setting is disabledExtensions; inherited arrays are explicitly copied
// into a project override so unrelated disabled IDs stay disabled.
func (s *Service) ApplyOMPExtensions(ctx context.Context, target string, changes []OMPExtensionChange, revision string) (*OMPExtensionsPlan, error) {
	current, err := s.OMPExtensions(ctx, target)
	if err != nil {
		return nil, err
	}
	if revision == "" || ompPlanRevision(current.Revision, changes) != revision {
		return nil, ErrOMPExtensionsStale
	}
	plan, _, _, _, err := s.ompPlan(ctx, target, changes)
	if err != nil {
		return nil, err
	}
	if revision == "" || plan.Revision != revision {
		return nil, ErrOMPExtensionsStale
	}
	unlock, err := ompAcquireLock(plan.SettingsPath)
	if err != nil {
		return nil, err
	}
	defer unlock()
	plan, _, before, after, err := s.ompPlan(ctx, target, changes)
	if err != nil {
		return nil, err
	}
	if plan.Revision != revision {
		return nil, ErrOMPExtensionsStale
	}
	changed := false
	for _, r := range plan.Rows {
		changed = changed || r.Before != r.After
	}
	if !changed {
		return plan, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent, base := filepath.Dir(plan.SettingsPath), filepath.Base(plan.SettingsPath)
	if err := ompSafePath(plan.SettingsPath); err != nil {
		return nil, err
	}
	// Only create the already-proven config root, never extension directories.
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	suffix := fmt.Sprintf("%x", nonce)
	temp := ".skillshare-omp-" + suffix + ".tmp"
	write := func(name string, raw []byte) error {
		f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(raw)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
	// Selection-only records do not duplicate unrelated credentials into projects.
	backup, err := s.ompBackup(plan.SettingsPath, before, after, suffix)
	if err != nil {
		return nil, err
	}
	if err := write(temp, after); err != nil {
		return nil, err
	}
	defer root.Remove(temp)
	final, _, _, _, err := s.ompPlan(ctx, target, changes)
	if err != nil {
		return nil, err
	}
	if final.Revision != revision {
		return nil, ErrOMPExtensionsStale
	}
	if err := ompSafePath(plan.SettingsPath); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	anchored, err := root.Stat(".")
	if err != nil {
		return nil, err
	}
	currentParent, err := os.Stat(parent)
	if err != nil || !os.SameFile(anchored, currentParent) {
		return nil, ErrOMPExtensionsStale
	}
	if err := root.Rename(temp, base); err != nil {
		return nil, err
	}
	plan.BackupID = backup
	return plan, nil
}
