package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
)

var (
	ruleWriteFile    = os.WriteFile
	ruleSaveMetadata = saveRuleMetadata
)

// Store persists managed rules as files under the managed rules root.
type Store struct {
	root string
}

// NewStore creates a rule store for global mode (empty projectRoot) or project mode.
func NewStore(projectRoot string) *Store {
	return &Store{
		root: config.ManagedRulesDir(projectRoot),
	}
}

// Put writes the rule file for the provided ID.
func (s *Store) Put(in Save) (Record, error) {
	fullPath, id, err := s.pathForID(in.ID)
	if err != nil {
		return Record{}, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return Record{}, fmt.Errorf("create rule directory: %w", err)
	}
	previousContent, hadPreviousContent, err := readExistingRuleContent(fullPath)
	if err != nil {
		return Record{}, err
	}

	tempPath, err := s.writeTempRule(filepath.Dir(fullPath), in.Content)
	if err != nil {
		return Record{}, fmt.Errorf("write rule: %w", err)
	}
	if err := s.replaceRuleFile(tempPath, fullPath); err != nil {
		_ = os.Remove(tempPath)
		return Record{}, fmt.Errorf("write rule: rename temp file: %w", err)
	}
	if err := ruleSaveMetadata(fullPath, ruleMetadata{
		Targets:    in.Targets,
		SourceType: in.SourceType,
		Disabled:   in.Disabled,
	}); err != nil {
		rollbackErr := s.restoreRuleContent(fullPath, previousContent, hadPreviousContent)
		if rollbackErr != nil {
			return Record{}, errors.Join(
				fmt.Errorf("write rule metadata: %w", err),
				fmt.Errorf("rollback rule content: %w", rollbackErr),
			)
		}
		return Record{}, fmt.Errorf("write rule metadata: %w", err)
	}

	tool, name := splitRuleID(id)
	return Record{
		ID:           id,
		Path:         fullPath,
		Tool:         tool,
		RelativePath: id,
		Name:         name,
		Content:      append([]byte(nil), in.Content...),
		Targets:      append([]string(nil), sanitizeRuleMetadata(ruleMetadata{Targets: in.Targets}).Targets...),
		SourceType:   strings.TrimSpace(in.SourceType),
		Disabled:     in.Disabled,
	}, nil
}

// Get loads one managed rule by ID.
func (s *Store) Get(id string) (Record, error) {
	fullPath, cleanedID, err := s.pathForID(id)
	if err != nil {
		return Record{}, err
	}
	if err := ensureRegularRuleFile(fullPath, cleanedID); err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return Record{}, err
	}
	metadata, err := loadRuleMetadata(fullPath)
	if err != nil {
		return Record{}, err
	}
	tool, name := splitRuleID(cleanedID)
	return Record{
		ID:           cleanedID,
		Path:         fullPath,
		Tool:         tool,
		RelativePath: cleanedID,
		Name:         name,
		Content:      data,
		Targets:      metadata.Targets,
		SourceType:   metadata.SourceType,
		Disabled:     metadata.Disabled,
	}, nil
}

// List returns all managed rules under the store root.
func (s *Store) List() ([]Record, error) {
	if _, err := os.Stat(s.root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []Record
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if isTransientRuleFile(d.Name()) || isRuleMetadataFile(d.Name()) {
			return nil
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return err
		}
		id := filepath.ToSlash(rel)
		tool, name := splitRuleID(id)
		metadata, err := loadRuleMetadata(p)
		if err != nil {
			return err
		}
		out = append(out, Record{
			ID:           id,
			Path:         p,
			Tool:         tool,
			RelativePath: id,
			Name:         name,
			Content:      data,
			Targets:      metadata.Targets,
			SourceType:   metadata.SourceType,
			Disabled:     metadata.Disabled,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Delete removes one managed rule by ID.
func (s *Store) Delete(id string) error {
	fullPath, _, err := s.pathForID(id)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil {
		return err
	}
	if err := deleteRuleMetadata(fullPath); err != nil {
		return err
	}
	return nil
}

func (s *Store) pathForID(id string) (fullPath string, cleanedID string, err error) {
	cleanedID, err = NormalizeRuleID(id)
	if err != nil {
		return "", "", err
	}

	fullPath = filepath.Join(s.root, filepath.FromSlash(cleanedID))
	return fullPath, cleanedID, nil
}

// NormalizeRuleID validates and canonicalizes a managed rule ID into slash form.
func NormalizeRuleID(id string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(id), "\\", "/")
	cleanedID := path.Clean(normalized)

	if cleanedID == "" || cleanedID == "." || cleanedID == ".." {
		return "", fmt.Errorf("%w %q", ErrInvalidID, id)
	}
	if strings.HasPrefix(cleanedID, "/") || strings.HasPrefix(cleanedID, "../") {
		return "", fmt.Errorf("%w %q", ErrInvalidID, id)
	}

	parts := strings.Split(cleanedID, "/")
	for _, part := range parts {
		if len(part) >= 2 && part[1] == ':' {
			return "", fmt.Errorf("%w %q", ErrInvalidID, id)
		}
		if isTransientRuleFile(part) || isRuleMetadataFile(part) {
			return "", fmt.Errorf("%w %q", ErrInvalidID, id)
		}
	}
	if !isSupportedRuleToolPrefix(parts[0]) {
		return "", fmt.Errorf("%w %q", ErrInvalidID, id)
	}
	if len(parts) < 2 {
		return "", fmt.Errorf("%w %q", ErrInvalidID, id)
	}
	if parts[0] == "pi" && !isSupportedPiRuleID(cleanedID) {
		return "", fmt.Errorf("%w %q", ErrInvalidID, id)
	}

	return cleanedID, nil
}

func (s *Store) writeTempRule(dir string, content []byte) (string, error) {
	tempFile, err := os.CreateTemp(dir, ".rule-tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	tempPath := tempFile.Name()
	closeWithCleanup := func(writeErr error) (string, error) {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return "", writeErr
	}

	if err := tempFile.Close(); err != nil {
		return closeWithCleanup(fmt.Errorf("close temp file: %w", err))
	}

	if err := ruleWriteFile(tempPath, content, 0644); err != nil {
		return closeWithCleanup(err)
	}

	return tempPath, nil
}

func (s *Store) restoreRuleContent(fullPath string, content []byte, hadPreviousContent bool) error {
	if !hadPreviousContent {
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	tempPath, err := s.writeTempRule(filepath.Dir(fullPath), content)
	if err != nil {
		return err
	}
	if err := s.replaceRuleFile(tempPath, fullPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func (s *Store) replaceRuleFile(tempPath, fullPath string) error {
	return replaceRuleFile(tempPath, fullPath)
}

func readExistingRuleContent(fullPath string) ([]byte, bool, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func splitRuleID(id string) (tool string, name string) {
	cleaned := path.Clean(strings.ReplaceAll(strings.TrimSpace(id), "\\", "/"))
	parts := strings.Split(cleaned, "/")
	if len(parts) > 0 {
		tool = parts[0]
	}
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}
	return tool, name
}

func isSupportedRuleToolPrefix(tool string) bool {
	switch tool {
	case "claude", "codex", "gemini", "pi":
		return true
	default:
		return false
	}
}

func isSupportedPiRuleID(id string) bool {
	switch id {
	case "pi/AGENTS.md", "pi/SYSTEM.md", "pi/APPEND_SYSTEM.md":
		return true
	default:
		return false
	}
}

func isTransientRuleFile(name string) bool {
	return strings.HasPrefix(name, ".rule-tmp-") || strings.HasPrefix(name, ruleMetadataTempPrefix)
}

func ensureRegularRuleFile(fullPath, id string) error {
	info, err := os.Lstat(fullPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("rule %q is not a regular file", id)
	}
	return nil
}
