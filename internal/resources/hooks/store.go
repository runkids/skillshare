package hooks

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"skillshare/internal/config"
)

var hookWriteFile = os.WriteFile

// Store persists managed matcher-group hooks as YAML files under the managed hooks root.
type Store struct {
	root string
}

// NewStore creates a hook store for global mode (empty projectRoot) or project mode.
func NewStore(projectRoot string) *Store {
	return &Store{
		root: config.ManagedHooksDir(projectRoot),
	}
}

type hookFile struct {
	Tool       string    `yaml:"tool"`
	Event      string    `yaml:"event"`
	Matcher    string    `yaml:"matcher"`
	Sequential *bool     `yaml:"sequential,omitempty"`
	Handlers   []Handler `yaml:"handlers"`
	Targets    []string  `yaml:"targets,omitempty"`
	SourceType string    `yaml:"sourceType,omitempty"`
	Disabled   bool      `yaml:"disabled,omitempty"`
}

// Put writes one matcher-group hook file for the provided ID.
func (s *Store) Put(in Save) (Record, error) {
	fullPath, id, err := s.pathForID(in.ID)
	if err != nil {
		return Record{}, err
	}
	if err := validateSave(in, id); err != nil {
		return Record{}, err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return Record{}, fmt.Errorf("create hook directory: %w", err)
	}

	data, err := yaml.Marshal(hookFile{
		Tool:       strings.TrimSpace(in.Tool),
		Event:      strings.TrimSpace(in.Event),
		Matcher:    strings.TrimSpace(in.Matcher),
		Sequential: copyOptionalBool(in.Sequential),
		Handlers:   sanitizeHandlers(in.Handlers),
		Targets:    sanitizeTargets(in.Targets),
		SourceType: strings.TrimSpace(in.SourceType),
		Disabled:   in.Disabled,
	})
	if err != nil {
		return Record{}, fmt.Errorf("marshal hook: %w", err)
	}

	tempPath, err := s.writeTempHook(filepath.Dir(fullPath), data)
	if err != nil {
		return Record{}, fmt.Errorf("write hook: %w", err)
	}
	if err := s.replaceHookFile(tempPath, fullPath); err != nil {
		_ = os.Remove(tempPath)
		return Record{}, fmt.Errorf("write hook: rename temp file: %w", err)
	}

	return Record{
		ID:           id,
		Path:         fullPath,
		RelativePath: id,
		Tool:         strings.TrimSpace(in.Tool),
		Event:        strings.TrimSpace(in.Event),
		Matcher:      strings.TrimSpace(in.Matcher),
		Sequential:   copyOptionalBool(in.Sequential),
		Handlers:     sanitizeHandlers(in.Handlers),
		Targets:      sanitizeTargets(in.Targets),
		SourceType:   strings.TrimSpace(in.SourceType),
		Disabled:     in.Disabled,
	}, nil
}

// Get loads one managed matcher-group hook by ID.
func (s *Store) Get(id string) (Record, error) {
	fullPath, cleanedID, err := s.pathForID(id)
	if err != nil {
		return Record{}, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return Record{}, err
	}

	var file hookFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return Record{}, fmt.Errorf("parse hook %q: %w", cleanedID, err)
	}
	if err := validateFile(cleanedID, file); err != nil {
		return Record{}, err
	}

	return Record{
		ID:           cleanedID,
		Path:         fullPath,
		RelativePath: cleanedID,
		Tool:         strings.TrimSpace(file.Tool),
		Event:        strings.TrimSpace(file.Event),
		Matcher:      strings.TrimSpace(file.Matcher),
		Sequential:   copyOptionalBool(file.Sequential),
		Handlers:     sanitizeHandlers(file.Handlers),
		Targets:      sanitizeTargets(file.Targets),
		SourceType:   strings.TrimSpace(file.SourceType),
		Disabled:     file.Disabled,
	}, nil
}

// List returns all managed matcher-group hooks under the store root.
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
		if strings.HasPrefix(filepath.Base(p), ".hook-tmp-") {
			return nil
		}

		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return err
		}
		id := filepath.ToSlash(rel)

		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}

		var file hookFile
		if err := yaml.Unmarshal(data, &file); err != nil {
			return fmt.Errorf("parse hook %q: %w", id, err)
		}
		if err := validateFile(id, file); err != nil {
			return err
		}

		out = append(out, Record{
			ID:           id,
			Path:         p,
			RelativePath: id,
			Tool:         strings.TrimSpace(file.Tool),
			Event:        strings.TrimSpace(file.Event),
			Matcher:      strings.TrimSpace(file.Matcher),
			Sequential:   copyOptionalBool(file.Sequential),
			Handlers:     sanitizeHandlers(file.Handlers),
			Targets:      sanitizeTargets(file.Targets),
			SourceType:   strings.TrimSpace(file.SourceType),
			Disabled:     file.Disabled,
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

// Delete removes one managed matcher-group hook by ID.
func (s *Store) Delete(id string) error {
	fullPath, _, err := s.pathForID(id)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}

func (s *Store) pathForID(id string) (fullPath string, cleanedID string, err error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(id), "\\", "/")
	cleanedID = path.Clean(normalized)

	if cleanedID == "" || cleanedID == "." || cleanedID == ".." {
		return "", "", fmt.Errorf("invalid hook id %q", id)
	}
	if strings.HasPrefix(cleanedID, "/") || strings.HasPrefix(cleanedID, "../") {
		return "", "", fmt.Errorf("invalid hook id %q", id)
	}
	if len(cleanedID) >= 2 && cleanedID[1] == ':' {
		return "", "", fmt.Errorf("invalid hook id %q", id)
	}
	for _, part := range strings.Split(cleanedID, "/") {
		if strings.HasPrefix(part, ".hook-tmp-") {
			return "", "", fmt.Errorf("invalid hook id %q", id)
		}
	}

	fullPath = filepath.Join(s.root, filepath.FromSlash(cleanedID))
	return fullPath, cleanedID, nil
}

func validateSave(in Save, id string) error {
	file := hookFile{
		Tool:       in.Tool,
		Event:      in.Event,
		Matcher:    in.Matcher,
		Sequential: in.Sequential,
		Handlers:   in.Handlers,
	}
	return validateFile(id, file)
}

func validateFile(id string, in hookFile) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("hook id is required")
	}
	if strings.TrimSpace(in.Tool) == "" {
		return fmt.Errorf("hook %q: tool is required", id)
	}
	tool := strings.ToLower(strings.TrimSpace(in.Tool))
	if !isSupportedManagedHookTool(tool) {
		return fmt.Errorf("hook %q: tool %q is not supported", id, in.Tool)
	}
	idTool, ok := managedHookToolFromID(id)
	if !ok {
		return fmt.Errorf("hook %q: managed hook id must start with a supported tool prefix", id)
	}
	if tool != idTool {
		return fmt.Errorf("hook %q: tool %q does not match managed id prefix %q", id, in.Tool, idTool)
	}
	if tool == "codex" {
		if err := validateCodexManagedHook(id, in); err != nil {
			return err
		}
	}
	if tool == "gemini" {
		if err := validateGeminiManagedHook(id, in); err != nil {
			return err
		}
	}
	if strings.TrimSpace(in.Event) == "" {
		return fmt.Errorf("hook %q: event is required", id)
	}
	if strings.TrimSpace(in.Matcher) == "" && !managedHookAllowsEmptyMatcher(tool, strings.TrimSpace(in.Event)) {
		return fmt.Errorf("hook %q: matcher is required", id)
	}
	if len(in.Handlers) == 0 {
		return fmt.Errorf("hook %q: handlers must not be empty", id)
	}
	for i, h := range in.Handlers {
		actionType := strings.TrimSpace(h.Type)
		if actionType == "" {
			return fmt.Errorf("hook %q: handlers[%d].type is required", id, i)
		}
		switch actionType {
		case "command":
			if strings.TrimSpace(h.Command) == "" {
				return fmt.Errorf("hook %q: handlers[%d].command is required for type command", id, i)
			}
		case "http":
			if strings.TrimSpace(h.URL) == "" {
				return fmt.Errorf("hook %q: handlers[%d].url is required for type http", id, i)
			}
		case "prompt", "agent":
			if strings.TrimSpace(h.Prompt) == "" {
				return fmt.Errorf("hook %q: handlers[%d].prompt is required for type %s", id, i, actionType)
			}
		default:
			return fmt.Errorf("hook %q: handlers[%d].type %q is not supported", id, i, actionType)
		}
	}
	return nil
}

func validateCodexManagedHook(id string, in hookFile) error {
	if !isSupportedCodexManagedEvent(strings.TrimSpace(in.Event)) {
		return fmt.Errorf("hook %q: event %q is not supported for codex", id, in.Event)
	}
	if event := strings.TrimSpace(in.Event); event == "UserPromptSubmit" || event == "Stop" {
		if strings.TrimSpace(in.Matcher) != "" {
			return fmt.Errorf("hook %q: matcher must be empty for codex %s", id, event)
		}
	}
	for i, h := range in.Handlers {
		actionType := strings.TrimSpace(h.Type)
		if actionType != "command" {
			return fmt.Errorf("hook %q: handlers[%d].type %q is not supported for codex", id, i, actionType)
		}
		if strings.TrimSpace(h.Command) == "" {
			return fmt.Errorf("hook %q: handlers[%d].command is required for codex", id, i)
		}
		if strings.TrimSpace(h.Timeout) != "" && h.TimeoutSeconds == nil {
			if _, err := strconv.Atoi(strings.TrimSpace(h.Timeout)); err != nil {
				return fmt.Errorf("hook %q: handlers[%d].timeout must be numeric seconds for codex", id, i)
			}
		}
	}
	return nil
}

func isSupportedCodexManagedEvent(event string) bool {
	switch strings.TrimSpace(event) {
	case "SessionStart", "PreToolUse", "PostToolUse", "UserPromptSubmit", "Stop":
		return true
	default:
		return false
	}
}

func isSupportedManagedHookTool(tool string) bool {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "claude", "codex", "gemini":
		return true
	default:
		return false
	}
}

func managedHookToolFromID(id string) (string, bool) {
	normalized := strings.ReplaceAll(strings.TrimSpace(id), "\\", "/")
	cleaned := path.Clean(normalized)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "", false
	}
	parts := strings.SplitN(cleaned, "/", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(parts[0])), true
}

func sanitizeHandlers(in []Handler) []Handler {
	if len(in) == 0 {
		return nil
	}
	out := make([]Handler, len(in))
	for i, h := range in {
		out[i] = Handler{
			Type:           strings.TrimSpace(h.Type),
			Name:           strings.TrimSpace(h.Name),
			Description:    strings.TrimSpace(h.Description),
			Command:        strings.TrimSpace(h.Command),
			URL:            strings.TrimSpace(h.URL),
			Prompt:         strings.TrimSpace(h.Prompt),
			Timeout:        strings.TrimSpace(h.Timeout),
			TimeoutSeconds: h.TimeoutSeconds,
			StatusMessage:  strings.TrimSpace(h.StatusMessage),
		}
	}
	return out
}

func validateGeminiManagedHook(id string, in hookFile) error {
	if !isSupportedGeminiManagedEvent(strings.TrimSpace(in.Event)) {
		return fmt.Errorf("hook %q: event %q is not supported for gemini", id, in.Event)
	}
	for i, h := range in.Handlers {
		actionType := strings.TrimSpace(h.Type)
		if actionType != "command" {
			return fmt.Errorf("hook %q: handlers[%d].type %q is not supported for gemini", id, i, actionType)
		}
		if strings.TrimSpace(h.Command) == "" {
			return fmt.Errorf("hook %q: handlers[%d].command is required for gemini", id, i)
		}
		if strings.TrimSpace(h.Timeout) != "" {
			if _, err := strconv.Atoi(strings.TrimSpace(h.Timeout)); err != nil {
				return fmt.Errorf("hook %q: handlers[%d].timeout must be numeric milliseconds for gemini", id, i)
			}
		}
	}
	return nil
}

func isSupportedGeminiManagedEvent(event string) bool {
	switch strings.TrimSpace(event) {
	case "SessionStart", "SessionEnd", "BeforeAgent", "AfterAgent", "BeforeModel", "AfterModel", "BeforeToolSelection", "BeforeTool", "AfterTool", "PreCompress", "Notification":
		return true
	default:
		return false
	}
}

func managedHookAllowsEmptyMatcher(tool, event string) bool {
	tool = strings.ToLower(strings.TrimSpace(tool))
	event = strings.TrimSpace(event)
	if tool == "codex" && (event == "UserPromptSubmit" || event == "Stop") {
		return true
	}
	return tool == "gemini"
}

func copyOptionalBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func sanitizeTargets(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, target := range in {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		out = append(out, target)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Store) writeTempHook(dir string, data []byte) (string, error) {
	tempFile, err := os.CreateTemp(dir, ".hook-tmp-*")
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

	if err := hookWriteFile(tempPath, data, 0644); err != nil {
		return closeWithCleanup(err)
	}

	return tempPath, nil
}
