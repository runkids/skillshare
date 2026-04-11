package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/inspect"
	managedhooks "skillshare/internal/resources/hooks"
	managed "skillshare/internal/resources/managed"
)

type managedHookHandlerPayload struct {
	Type           string `json:"type"`
	Command        string `json:"command,omitempty"`
	URL            string `json:"url,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
	Timeout        string `json:"timeout,omitempty"`
	TimeoutSeconds *int   `json:"timeoutSec,omitempty"`
	StatusMessage  string `json:"statusMessage,omitempty"`
}

type managedHookPayload struct {
	ID         string                      `json:"id"`
	Tool       string                      `json:"tool"`
	Event      string                      `json:"event"`
	Matcher    string                      `json:"matcher"`
	Handlers   []managedHookHandlerPayload `json:"handlers"`
	Targets    []string                    `json:"targets"`
	SourceType string                      `json:"sourceType"`
	Disabled   bool                        `json:"disabled"`
}

type managedHookPreview struct {
	Target   string                      `json:"target"`
	Files    []managedhooks.CompiledFile `json:"files"`
	Warnings []string                    `json:"warnings,omitempty"`
}

type managedHookRequest struct {
	ID         string                      `json:"id"`
	Tool       string                      `json:"tool"`
	Event      string                      `json:"event"`
	Matcher    *string                     `json:"matcher"`
	Handlers   []managedHookHandlerPayload `json:"handlers"`
	Targets    *[]string                   `json:"targets"`
	SourceType *string                     `json:"sourceType"`
	Disabled   *bool                       `json:"disabled"`
}

func (s *Server) managedHooksProjectRoot() string {
	if s.IsProjectMode() {
		return s.projectRoot
	}
	return ""
}

func (s *Server) handleListManagedHooks(w http.ResponseWriter, r *http.Request) {
	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	records, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list managed hooks: "+err.Error())
		return
	}

	items := make([]managedHookPayload, 0, len(records))
	for _, record := range records {
		items = append(items, managedHookRecordPayload(record))
	}
	writeJSON(w, map[string]any{"hooks": items})
}

func (s *Server) handleCreateManagedHook(w http.ResponseWriter, r *http.Request) {
	var body managedHookRequest
	if err := decodeManagedHookRequest(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	canonicalID, err := managedHookCanonicalID(body.Tool, body.Event, body.matcher())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.mu.Lock()
	if _, err := store.Get(canonicalID); err == nil {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "managed hook already exists: "+canonicalID)
		return
	} else if !managedHookNotFound(err) {
		s.mu.Unlock()
		writeError(w, managedHookLoadStatus(err), "failed to check managed hook: "+err.Error())
		return
	}

	record, err := store.Put(managedHookSave(body, canonicalID, nil))
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookSaveStatus(err), "failed to save managed hook: "+err.Error())
		return
	}

	previews, err := s.loadManagedHookPreviews(store)
	if err != nil {
		rollbackErr := store.Delete(record.ID)
		s.mu.Unlock()
		writeManagedHookMutationPreviewError(w, err, rollbackErr, "failed to rollback created managed hook")
		return
	}
	s.mu.Unlock()

	writeManagedHookDetailResponse(w, http.StatusCreated, record, previews)
}

func (s *Server) handleGetManagedHook(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "hook id is required")
		return
	}

	record, status, err := s.loadManagedHook(id)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}

	s.writeManagedHookDetail(w, http.StatusOK, record)
}

func (s *Server) handleUpdateManagedHook(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "hook id is required")
		return
	}

	var body managedHookRequest
	if err := decodeManagedHookRequest(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	s.mu.Lock()
	existing, err := store.Get(id)
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookLoadStatus(err), managedHookLoadError(id, err).Error())
		return
	}

	canonicalID, err := managedHookCanonicalID(body.Tool, body.Event, body.matcher())
	if err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	moved := canonicalID != existing.ID
	if moved {
		if _, err := store.Get(canonicalID); err == nil {
			s.mu.Unlock()
			writeError(w, http.StatusConflict, "managed hook already exists: "+canonicalID)
			return
		} else if !managedHookNotFound(err) {
			s.mu.Unlock()
			writeError(w, managedHookLoadStatus(err), "failed to check managed hook: "+err.Error())
			return
		}
	}

	record, err := store.Put(managedHookSave(body, canonicalID, &existing))
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookSaveStatus(err), "failed to save managed hook: "+err.Error())
		return
	}

	if moved {
		if err := store.Delete(existing.ID); err != nil && !managedHookNotFound(err) {
			_ = store.Delete(record.ID)
			s.mu.Unlock()
			writeError(w, http.StatusInternalServerError, "failed to rename managed hook: "+err.Error())
			return
		}
	}

	previews, err := s.loadManagedHookPreviews(store)
	if err != nil {
		if moved {
			_ = store.Delete(record.ID)
		}
		rollbackErr := restoreManagedHookRecord(store, existing)
		s.mu.Unlock()
		writeManagedHookMutationPreviewError(w, err, rollbackErr, "failed to restore previous managed hook")
		return
	}
	s.mu.Unlock()

	writeManagedHookDetailResponse(w, http.StatusOK, record, previews)
}

func (s *Server) handleSetManagedHookTargets(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "hook id is required")
		return
	}

	var body managedTargetsRequest
	if err := decodeStrictJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	s.mu.Lock()
	record, err := store.Get(id)
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookLoadStatus(err), managedHookLoadError(id, err).Error())
		return
	}

	record, err = store.Put(managedhooks.Save{
		ID:         record.ID,
		Tool:       record.Tool,
		Event:      record.Event,
		Matcher:    record.Matcher,
		Handlers:   append([]managedhooks.Handler(nil), record.Handlers...),
		Targets:    normalizeManagedTargets([]string{body.Target}),
		SourceType: record.SourceType,
		Disabled:   record.Disabled,
	})
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookSaveStatus(err), "failed to save managed hook: "+err.Error())
		return
	}

	previews, err := s.loadManagedHookPreviews(store)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeManagedHookDetailResponse(w, http.StatusOK, record, previews)
}

func (s *Server) handleSetManagedHookDisabled(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "hook id is required")
		return
	}

	var body managedDisabledRequest
	if err := decodeStrictJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if body.Disabled == nil {
		writeError(w, http.StatusBadRequest, "disabled is required")
		return
	}

	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	s.mu.Lock()
	record, err := store.Get(id)
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookLoadStatus(err), managedHookLoadError(id, err).Error())
		return
	}

	record, err = store.Put(managedhooks.Save{
		ID:         record.ID,
		Tool:       record.Tool,
		Event:      record.Event,
		Matcher:    record.Matcher,
		Handlers:   append([]managedhooks.Handler(nil), record.Handlers...),
		Targets:    append([]string(nil), record.Targets...),
		SourceType: record.SourceType,
		Disabled:   *body.Disabled,
	})
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedHookSaveStatus(err), "failed to save managed hook: "+err.Error())
		return
	}

	previews, err := s.loadManagedHookPreviews(store)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeManagedHookDetailResponse(w, http.StatusOK, record, previews)
}

func (s *Server) handleDeleteManagedHook(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "hook id is required")
		return
	}

	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	if _, status, err := s.loadManagedHook(id); err != nil {
		writeError(w, status, err.Error())
		return
	}

	s.mu.Lock()
	err := store.Delete(id)
	s.mu.Unlock()
	if err != nil {
		status := http.StatusInternalServerError
		if managedHookNotFound(err) {
			status = http.StatusNotFound
		}
		writeError(w, status, "failed to delete managed hook: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleCollectManagedHooks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		GroupIDs []string              `json:"groupIds"`
		Strategy managedhooks.Strategy `json:"strategy"`
	}
	if err := decodeStrictJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(body.GroupIDs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one hook group id is required")
		return
	}

	projectRoot := s.managedHooksProjectRoot()
	discovered, _, err := inspect.ScanHooks(projectRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan hooks: "+err.Error())
		return
	}

	discoveredByGroup := make(map[string][]inspect.HookItem)
	for _, item := range discovered {
		groupID := strings.TrimSpace(item.GroupID)
		if groupID == "" {
			continue
		}
		discoveredByGroup[groupID] = append(discoveredByGroup[groupID], item)
	}

	selected := make([]inspect.HookItem, 0, len(discovered))
	seenGroupIDs := make(map[string]struct{}, len(body.GroupIDs))
	for _, rawGroupID := range body.GroupIDs {
		groupID := strings.TrimSpace(rawGroupID)
		if groupID == "" {
			writeError(w, http.StatusBadRequest, "unknown discovered hook group id: "+rawGroupID)
			return
		}
		if _, seen := seenGroupIDs[groupID]; seen {
			continue
		}
		seenGroupIDs[groupID] = struct{}{}

		groupItems, ok := discoveredByGroup[groupID]
		if !ok || len(groupItems) == 0 {
			writeError(w, http.StatusBadRequest, "unknown discovered hook group id: "+groupID)
			return
		}
		selected = append(selected, groupItems...)
	}

	s.mu.Lock()
	result, err := managed.CollectHooks(projectRoot, selected, body.Strategy)
	s.mu.Unlock()
	if err != nil {
		status := http.StatusInternalServerError
		if managedHookCollectInputError(err) {
			status = http.StatusBadRequest
		}
		writeError(w, status, "failed to collect managed hooks: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"created":     result.Created,
		"overwritten": result.Overwritten,
		"skipped":     result.Skipped,
	})
}

func (s *Server) handleDiffManagedHooks(w http.ResponseWriter, r *http.Request) {
	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	records, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list managed hooks: "+err.Error())
		return
	}

	previews, err := s.compileManagedHookPreviews(records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compile managed hooks diff: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"diffs": previews})
}

func (s *Server) loadManagedHook(id string) (managedhooks.Record, int, error) {
	record, err := managedhooks.NewStore(s.managedHooksProjectRoot()).Get(id)
	if err != nil {
		return managedhooks.Record{}, managedHookLoadStatus(err), managedHookLoadError(id, err)
	}
	return record, http.StatusOK, nil
}

func (s *Server) writeManagedHookDetail(w http.ResponseWriter, status int, record managedhooks.Record) {
	store := managedhooks.NewStore(s.managedHooksProjectRoot())
	previews, err := s.loadManagedHookPreviews(store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeManagedHookDetailResponse(w, status, record, previews)
}

func (s *Server) loadManagedHookPreviews(store *managedhooks.Store) ([]managedHookPreview, error) {
	records, err := store.List()
	if err != nil {
		return nil, errors.New("failed to load managed hook previews: " + err.Error())
	}

	previews, err := s.compileManagedHookPreviews(records)
	if err != nil {
		return nil, errors.New("failed to compile managed hook previews: " + err.Error())
	}
	return previews, nil
}

func writeManagedHookDetailResponse(w http.ResponseWriter, status int, record managedhooks.Record, previews []managedHookPreview) {
	writeJSONStatus(w, status, map[string]any{
		"hook":     managedHookRecordPayload(record),
		"previews": previews,
	})
}

func restoreManagedHookRecord(store *managedhooks.Store, record managedhooks.Record) error {
	_, err := store.Put(managedhooks.Save{
		ID:         record.ID,
		Tool:       record.Tool,
		Event:      record.Event,
		Matcher:    record.Matcher,
		Handlers:   record.Handlers,
		Targets:    append([]string(nil), record.Targets...),
		SourceType: record.SourceType,
		Disabled:   record.Disabled,
	})
	return err
}

func writeManagedHookMutationPreviewError(w http.ResponseWriter, previewErr, rollbackErr error, rollbackPrefix string) {
	if rollbackErr != nil {
		writeError(w, http.StatusInternalServerError, previewErr.Error()+"; "+rollbackPrefix+": "+rollbackErr.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, previewErr.Error())
}

func (s *Server) compileManagedHookPreviews(records []managedhooks.Record) ([]managedHookPreview, error) {
	targetNames := make([]string, 0, len(s.cfg.Targets))
	for name := range s.cfg.Targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	previews := make([]managedHookPreview, 0, len(targetNames))
	for _, name := range targetNames {
		target := s.cfg.Targets[name]
		compileTarget, compileRoot, ok := s.resolveManagedHookPreviewTarget(name, target)
		if !ok {
			previews = append(previews, managedHookPreview{
				Target:   name,
				Files:    []managedhooks.CompiledFile{},
				Warnings: []string{"unsupported target \"" + name + "\""},
			})
			continue
		}

		rawConfig, err := loadManagedHookRawConfig(compileTarget, compileRoot)
		if err != nil {
			return nil, err
		}
		files, warnings, err := managedhooks.CompileTarget(records, compileTarget, compileRoot, rawConfig)
		if err != nil {
			return nil, err
		}
		if files == nil {
			files = []managedhooks.CompiledFile{}
		}
		previews = append(previews, managedHookPreview{
			Target:   name,
			Files:    files,
			Warnings: warnings,
		})
	}
	return previews, nil
}

func (s *Server) resolveManagedHookPreviewTarget(name string, target config.TargetConfig) (string, string, bool) {
	sc := target.SkillsConfig()
	compileTarget, ok := resolveManagedHookPreviewTool(name, sc.Path)
	if !ok {
		return "", "", false
	}
	if s.IsProjectMode() {
		return compileTarget, s.projectRoot, true
	}
	return compileTarget, managedHookGlobalPreviewRoot(sc.Path), true
}

func resolveManagedHookPreviewTool(name, targetPath string) (string, bool) {
	for _, supported := range []string{"claude", "codex"} {
		if config.MatchesTargetName(supported, name) {
			return supported, true
		}
	}

	switch managedHookPathFamily(targetPath) {
	case "claude", "codex":
		return managedHookPathFamily(targetPath), true
	default:
		return "", false
	}
}

func managedHookPathFamily(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return ""
	}

	base := strings.ToLower(filepath.Base(cleaned))
	if base == "skills" {
		base = strings.ToLower(filepath.Base(filepath.Dir(cleaned)))
	}

	switch base {
	case ".claude", "claude":
		return "claude"
	case ".codex", "codex", ".agents", "agents":
		return "codex"
	default:
		return ""
	}
}

func managedHookGlobalPreviewRoot(targetPath string) string {
	cleaned := filepath.Clean(strings.TrimSpace(targetPath))
	if cleaned == "" || cleaned == "." {
		return targetPath
	}
	if strings.EqualFold(filepath.Base(cleaned), "skills") {
		cleaned = filepath.Dir(cleaned)
	}

	switch strings.ToLower(filepath.Base(cleaned)) {
	case ".claude", "claude", ".codex", "codex", ".agents", "agents":
		return filepath.Dir(cleaned)
	default:
		return cleaned
	}
}

func loadManagedHookRawConfig(target, root string) (string, error) {
	path, ok := managedHookConfigPath(target, root)
	if !ok {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func managedHookConfigPath(target, root string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "claude":
		return filepath.Join(root, ".claude", "settings.json"), true
	case "codex":
		return filepath.Join(root, ".codex", "config.toml"), true
	default:
		return "", false
	}
}

func decodeManagedHookRequest(r *http.Request, body *managedHookRequest) error {
	if err := decodeStrictJSON(r, body); err != nil {
		return errors.New("invalid request body: " + err.Error())
	}

	body.ID = strings.TrimSpace(body.ID)
	body.Tool = strings.TrimSpace(body.Tool)
	body.Event = strings.TrimSpace(body.Event)
	if body.Tool == "" {
		return errors.New("tool is required")
	}
	if body.Event == "" {
		return errors.New("event is required")
	}
	if body.Matcher == nil && !managedHookAllowsEmptyMatcher(body.Tool, body.Event) {
		return errors.New("matcher is required")
	}
	if len(body.Handlers) == 0 {
		return errors.New("handlers are required")
	}
	if body.Matcher != nil {
		m := strings.TrimSpace(*body.Matcher)
		body.Matcher = &m
	}
	if body.SourceType != nil {
		sourceType := strings.TrimSpace(*body.SourceType)
		body.SourceType = &sourceType
	}
	if !managedHookAllowsEmptyMatcher(body.Tool, body.Event) {
		if body.matcher() == "" {
			return errors.New("matcher is required")
		}
	}
	return nil
}

func managedHookAllowsEmptyMatcher(tool, event string) bool {
	normalizedTool := strings.ToLower(strings.TrimSpace(tool))
	normalizedEvent := strings.TrimSpace(event)
	return normalizedTool == "codex" && (normalizedEvent == "UserPromptSubmit" || normalizedEvent == "Stop")
}

func (r managedHookRequest) matcher() string {
	if r.Matcher == nil {
		return ""
	}
	return strings.TrimSpace(*r.Matcher)
}

func (r managedHookRequest) toHandlers() []managedhooks.Handler {
	if len(r.Handlers) == 0 {
		return nil
	}

	out := make([]managedhooks.Handler, len(r.Handlers))
	for i, handler := range r.Handlers {
		out[i] = managedhooks.Handler{
			Type:           strings.TrimSpace(handler.Type),
			Command:        strings.TrimSpace(handler.Command),
			URL:            strings.TrimSpace(handler.URL),
			Prompt:         strings.TrimSpace(handler.Prompt),
			Timeout:        strings.TrimSpace(handler.Timeout),
			TimeoutSeconds: handler.TimeoutSeconds,
			StatusMessage:  strings.TrimSpace(handler.StatusMessage),
		}
	}
	return out
}

func managedHookCanonicalID(tool, event, matcher string) (string, error) {
	return managedhooks.CanonicalRelativePath(tool, event, matcher)
}

func managedHookRecordPayload(record managedhooks.Record) managedHookPayload {
	handlers := make([]managedHookHandlerPayload, len(record.Handlers))
	for i, handler := range record.Handlers {
		handlers[i] = managedHookHandlerPayload{
			Type:           handler.Type,
			Command:        handler.Command,
			URL:            handler.URL,
			Prompt:         handler.Prompt,
			Timeout:        handler.Timeout,
			TimeoutSeconds: handler.TimeoutSeconds,
			StatusMessage:  handler.StatusMessage,
		}
	}
	return managedHookPayload{
		ID:         record.ID,
		Tool:       record.Tool,
		Event:      record.Event,
		Matcher:    record.Matcher,
		Handlers:   handlers,
		Targets:    append([]string(nil), record.Targets...),
		SourceType: record.SourceType,
		Disabled:   record.Disabled,
	}
}

func managedHookSave(body managedHookRequest, id string, existing *managedhooks.Record) managedhooks.Save {
	sourceType := "local"
	disabled := false
	var targets []string
	if existing != nil {
		sourceType = existing.SourceType
		disabled = existing.Disabled
		targets = append([]string(nil), existing.Targets...)
	}
	if body.Targets != nil {
		targets = normalizeManagedTargets(*body.Targets)
	}
	if body.SourceType != nil {
		sourceType = strings.TrimSpace(*body.SourceType)
	}
	if sourceType == "" {
		sourceType = "local"
	}
	if body.Disabled != nil {
		disabled = *body.Disabled
	}
	return managedhooks.Save{
		ID:         id,
		Tool:       body.Tool,
		Event:      body.Event,
		Matcher:    body.matcher(),
		Handlers:   body.toHandlers(),
		Targets:    targets,
		SourceType: sourceType,
		Disabled:   disabled,
	}
}

func managedHookNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func managedHookInvalidID(err error) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "invalid hook id")
}

func managedHookLoadStatus(err error) int {
	switch {
	case managedHookInvalidID(err):
		return http.StatusBadRequest
	case managedHookNotFound(err):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func managedHookSaveStatus(err error) int {
	if managedHookValidationError(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func managedHookValidationError(err error) bool {
	if managedHookInvalidID(err) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(err.Error())), "hook \"")
}

func managedHookLoadError(id string, err error) error {
	if managedHookNotFound(err) {
		return errors.New("managed hook not found: " + id)
	}
	return err
}

func managedHookCollectInputError(err error) bool {
	msg := strings.TrimSpace(strings.ToLower(err.Error()))
	return strings.Contains(msg, "invalid collect strategy") ||
		strings.Contains(msg, "cannot collect ")
}
