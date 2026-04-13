package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/inspect"
	managed "skillshare/internal/resources/managed"
	managedpi "skillshare/internal/resources/managed/pi"
	managedrules "skillshare/internal/resources/rules"
)

type managedRulePayload struct {
	ID           string   `json:"id"`
	Tool         string   `json:"tool"`
	Name         string   `json:"name"`
	RelativePath string   `json:"relativePath"`
	Content      string   `json:"content"`
	Targets      []string `json:"targets"`
	SourceType   string   `json:"sourceType"`
	Disabled     bool     `json:"disabled"`
}

type managedRulePreview struct {
	Target   string                      `json:"target"`
	Files    []managedrules.CompiledFile `json:"files"`
	Warnings []string                    `json:"warnings,omitempty"`
}

type managedRuleRequest struct {
	ID           string    `json:"id"`
	Tool         string    `json:"tool"`
	RelativePath string    `json:"relativePath"`
	Content      *string   `json:"content"`
	Targets      *[]string `json:"targets"`
	SourceType   *string   `json:"sourceType"`
	Disabled     *bool     `json:"disabled"`
}

var validateManagedRuleSave = managed.ValidateManagedRuleSave

func (s *Server) managedRulesProjectRoot() string {
	if s.IsProjectMode() {
		return s.projectRoot
	}
	return ""
}

func (s *Server) handleListManagedRules(w http.ResponseWriter, r *http.Request) {
	store := managedrules.NewStore(s.managedRulesProjectRoot())
	records, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list managed rules: "+err.Error())
		return
	}

	items := make([]managedRulePayload, 0, len(records))
	for _, record := range records {
		items = append(items, managedRulePayloadFromRecord(record))
	}

	writeJSON(w, map[string]any{"rules": items})
}

func (s *Server) handleCreateManagedRule(w http.ResponseWriter, r *http.Request) {
	var body managedRuleRequest
	if err := decodeManagedRuleRequest(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateManagedRuleSave(managed.RuleInput{
		Tool:         body.Tool,
		RelativePath: body.RelativePath,
		Content:      []byte(*body.Content),
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := managedrules.NewStore(s.managedRulesProjectRoot())

	s.mu.Lock()
	if _, err := store.Get(body.ID); err == nil {
		s.mu.Unlock()
		writeError(w, http.StatusConflict, "managed rule already exists: "+body.ID)
		return
	} else if !managedRuleNotFound(err) {
		s.mu.Unlock()
		writeError(w, managedRuleLoadStatus(err), "failed to check managed rule: "+err.Error())
		return
	}

	record, err := store.Put(managedRuleSave(body, nil))
	s.mu.Unlock()
	if err != nil {
		writeError(w, managedRuleSaveStatus(err), "failed to save managed rule: "+err.Error())
		return
	}

	s.writeManagedRuleDetail(w, http.StatusCreated, record)
}

func (s *Server) handleGetManagedRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	record, status, err := s.loadManagedRule(id)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}

	s.writeManagedRuleDetail(w, http.StatusOK, record)
}

func (s *Server) handleUpdateManagedRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	var body managedRuleRequest
	if err := decodeManagedRuleRequest(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.ID != id {
		writeError(w, http.StatusBadRequest, "rule id does not match request path")
		return
	}
	if err := validateManagedRuleSave(managed.RuleInput{
		Tool:         body.Tool,
		RelativePath: body.RelativePath,
		Content:      []byte(*body.Content),
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	store := managedrules.NewStore(s.managedRulesProjectRoot())
	s.mu.Lock()
	existing, err := store.Get(id)
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedRuleLoadStatus(err), err.Error())
		return
	}

	record, err := store.Put(managedRuleSave(body, &existing))
	s.mu.Unlock()
	if err != nil {
		writeError(w, managedRuleSaveStatus(err), "failed to save managed rule: "+err.Error())
		return
	}

	s.writeManagedRuleDetail(w, http.StatusOK, record)
}

func (s *Server) handleDeleteManagedRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	store := managedrules.NewStore(s.managedRulesProjectRoot())
	if _, status, err := s.loadManagedRule(id); err != nil {
		writeError(w, status, err.Error())
		return
	}

	s.mu.Lock()
	err := store.Delete(id)
	s.mu.Unlock()
	if err != nil {
		status := http.StatusInternalServerError
		if managedRuleNotFound(err) {
			status = http.StatusNotFound
		}
		writeError(w, status, "failed to delete managed rule: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"success": true})
}

func (s *Server) handleSetManagedRuleTargets(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	var body managedTargetsRequest
	if err := decodeStrictJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	store := managedrules.NewStore(s.managedRulesProjectRoot())
	s.mu.Lock()
	record, err := store.Get(id)
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedRuleLoadStatus(err), err.Error())
		return
	}
	if err := validateManagedRuleSave(managed.RuleInput{
		Tool:         record.Tool,
		RelativePath: record.RelativePath,
		Content:      record.Content,
	}); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err = store.Put(managedrules.Save{
		ID:         record.ID,
		Content:    record.Content,
		Targets:    normalizeManagedTargets([]string{body.Target}),
		SourceType: record.SourceType,
		Disabled:   record.Disabled,
	})
	s.mu.Unlock()
	if err != nil {
		writeError(w, managedRuleSaveStatus(err), "failed to save managed rule: "+err.Error())
		return
	}

	s.writeManagedRuleDetail(w, http.StatusOK, record)
}

func (s *Server) handleSetManagedRuleDisabled(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
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

	store := managedrules.NewStore(s.managedRulesProjectRoot())
	s.mu.Lock()
	record, err := store.Get(id)
	if err != nil {
		s.mu.Unlock()
		writeError(w, managedRuleLoadStatus(err), err.Error())
		return
	}
	if err := validateManagedRuleSave(managed.RuleInput{
		Tool:         record.Tool,
		RelativePath: record.RelativePath,
		Content:      record.Content,
	}); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err = store.Put(managedrules.Save{
		ID:         record.ID,
		Content:    record.Content,
		Targets:    append([]string(nil), record.Targets...),
		SourceType: record.SourceType,
		Disabled:   *body.Disabled,
	})
	s.mu.Unlock()
	if err != nil {
		writeError(w, managedRuleSaveStatus(err), "failed to save managed rule: "+err.Error())
		return
	}

	s.writeManagedRuleDetail(w, http.StatusOK, record)
}

func (s *Server) handleCollectManagedRules(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs      []string              `json:"ids"`
		Strategy managedrules.Strategy `json:"strategy"`
	}
	if err := decodeStrictJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if len(body.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "at least one rule id is required")
		return
	}

	projectRoot := s.managedRulesProjectRoot()
	discovered, _, err := inspect.ScanRules(projectRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to scan rules: "+err.Error())
		return
	}

	discoveredByID := make(map[string]inspect.RuleItem, len(discovered))
	for _, item := range discovered {
		discoveredByID[item.ID] = item
	}

	selected := make([]inspect.RuleItem, 0, len(body.IDs))
	seenIDs := make(map[string]struct{}, len(body.IDs))
	for _, id := range body.IDs {
		if _, seen := seenIDs[id]; seen {
			continue
		}
		seenIDs[id] = struct{}{}

		item, ok := discoveredByID[id]
		if !ok {
			writeError(w, http.StatusBadRequest, "unknown discovered rule id: "+id)
			return
		}
		selected = append(selected, item)
	}

	s.mu.Lock()
	result, err := managed.CollectRules(projectRoot, selected, body.Strategy)
	s.mu.Unlock()
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, managedrules.ErrInvalidCollect) {
			status = http.StatusBadRequest
		}
		writeError(w, status, "failed to collect managed rules: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{
		"created":     result.Created,
		"overwritten": result.Overwritten,
		"skipped":     result.Skipped,
	})
}

func (s *Server) handleDiffManagedRules(w http.ResponseWriter, r *http.Request) {
	store := managedrules.NewStore(s.managedRulesProjectRoot())
	records, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list managed rules: "+err.Error())
		return
	}

	previews, err := s.compileManagedRulePreviews(records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compile managed rules diff: "+err.Error())
		return
	}

	writeJSON(w, map[string]any{"diffs": previews})
}

func (s *Server) loadManagedRule(id string) (managedrules.Record, int, error) {
	record, err := managedrules.NewStore(s.managedRulesProjectRoot()).Get(id)
	if err != nil {
		switch {
		case errors.Is(err, managedrules.ErrInvalidID):
			return managedrules.Record{}, http.StatusBadRequest, err
		case managedRuleNotFound(err):
			return managedrules.Record{}, http.StatusNotFound, err
		default:
			return managedrules.Record{}, http.StatusInternalServerError, err
		}
	}
	return record, http.StatusOK, nil
}

func (s *Server) writeManagedRuleDetail(w http.ResponseWriter, status int, record managedrules.Record) {
	store := managedrules.NewStore(s.managedRulesProjectRoot())
	records, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load managed rule previews: "+err.Error())
		return
	}

	previews, err := s.compileManagedRulePreviews(records)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compile managed rule previews: "+err.Error())
		return
	}

	writeJSONStatus(w, status, map[string]any{
		"rule":     managedRulePayloadFromRecord(record),
		"previews": previews,
	})
}

func (s *Server) compileManagedRulePreviews(records []managedrules.Record) ([]managedRulePreview, error) {
	targetNames := make([]string, 0, len(s.cfg.Targets))
	for name := range s.cfg.Targets {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)

	previews := make([]managedRulePreview, 0, len(targetNames))
	for _, name := range targetNames {
		target := s.cfg.Targets[name]
		compileTarget, compileRoot := s.resolveManagedRulePreviewTarget(name, target)
		files, warnings, err := managedrules.CompileTarget(records, compileTarget, name, compileRoot)
		if err != nil {
			if errors.Is(err, managedrules.ErrUnsupportedTarget) {
				previews = append(previews, managedRulePreview{
					Target:   name,
					Files:    []managedrules.CompiledFile{},
					Warnings: []string{err.Error()},
				})
				continue
			}
			return nil, err
		}
		if files == nil {
			files = []managedrules.CompiledFile{}
		}
		previews = append(previews, managedRulePreview{
			Target:   name,
			Files:    files,
			Warnings: warnings,
		})
	}
	return previews, nil
}

func (s *Server) resolveManagedRulePreviewTarget(name string, target config.TargetConfig) (string, string) {
	sc := target.SkillsConfig()
	compileTarget, ok := managed.ResolveManagedFamily(managed.ResourceKindRules, name, sc.Path)
	if !ok {
		return name, sc.Path
	}

	if s.IsProjectMode() {
		return compileTarget, s.projectRoot
	}

	return compileTarget, managed.RuleGlobalPreviewRoot(sc.Path)
}

func decodeManagedRuleRequest(r *http.Request, body *managedRuleRequest) error {
	if err := decodeStrictJSON(r, body); err != nil {
		return errors.New("invalid request body: " + err.Error())
	}

	normalizedID := strings.TrimSpace(body.ID)
	if normalizedID != "" {
		var err error
		normalizedID, err = managedrules.NormalizeRuleID(normalizedID)
		if err != nil {
			return err
		}
	}

	hasDerivedFields := strings.TrimSpace(body.Tool) != "" || strings.TrimSpace(body.RelativePath) != ""
	if hasDerivedFields {
		derivedID, err := managedRuleDerivedID(*body)
		if err != nil {
			return err
		}
		if normalizedID != "" && normalizedID != derivedID {
			return errors.New("rule id does not match tool and relativePath")
		}
		normalizedID = derivedID
	}

	if normalizedID == "" {
		return errors.New("rule id is required")
	}
	if body.Content == nil {
		return errors.New("content is required")
	}
	body.ID = normalizedID
	if body.SourceType != nil {
		sourceType := strings.TrimSpace(*body.SourceType)
		body.SourceType = &sourceType
	}
	return nil
}

type managedTargetsRequest struct {
	Target string `json:"target"`
}

type managedDisabledRequest struct {
	Disabled *bool `json:"disabled"`
}

func managedRulePayloadFromRecord(record managedrules.Record) managedRulePayload {
	return managedRulePayload{
		ID:           record.ID,
		Tool:         record.Tool,
		Name:         record.Name,
		RelativePath: record.RelativePath,
		Content:      string(record.Content),
		Targets:      append([]string(nil), record.Targets...),
		SourceType:   record.SourceType,
		Disabled:     record.Disabled,
	}
}

func managedRuleSave(body managedRuleRequest, existing *managedrules.Record) managedrules.Save {
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
	return managedrules.Save{
		ID:         body.ID,
		Content:    []byte(*body.Content),
		Targets:    targets,
		SourceType: sourceType,
		Disabled:   disabled,
	}
}

func normalizeManagedTargets(targets []string) []string {
	if len(targets) == 0 {
		return nil
	}
	out := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func managedRuleDerivedID(body managedRuleRequest) (string, error) {
	tool, err := normalizeManagedRuleTool(body.Tool)
	if err != nil {
		return "", err
	}
	rawRel := strings.ReplaceAll(strings.TrimSpace(body.RelativePath), "\\", "/")
	if tool == "" || rawRel == "" {
		return "", errors.New("tool and relativePath are required together")
	}

	if strings.HasPrefix(rawRel, "/") {
		return "", errors.New("invalid rule relativePath")
	}
	if len(rawRel) >= 2 && rawRel[1] == ':' {
		return "", errors.New("invalid rule relativePath")
	}
	for _, part := range strings.Split(rawRel, "/") {
		if part == ".." {
			return "", errors.New("invalid rule relativePath")
		}
	}

	rel := path.Clean(rawRel)
	if rel == "." || rel == "/" {
		return "", errors.New("invalid rule relativePath")
	}
	if tool == "pi" {
		normalized, ok := managedpi.NormalizeManagedRuleID(rel)
		if !ok {
			return "", errors.New("invalid rule relativePath")
		}
		return normalized, nil
	}
	if strings.HasPrefix(rel, tool+"/") {
		normalized, err := managedrules.NormalizeRuleID(rel)
		if err != nil {
			return "", errors.New("invalid rule relativePath")
		}
		return normalized, nil
	}
	normalized, err := managedrules.NormalizeRuleID(tool + "/" + strings.TrimPrefix(rel, "/"))
	if err != nil {
		return "", errors.New("invalid rule relativePath")
	}
	return normalized, nil
}

func normalizeManagedRuleTool(raw string) (string, error) {
	tool := strings.ToLower(strings.TrimSpace(raw))
	switch tool {
	case "claude", "codex", "gemini", "pi":
		return tool, nil
	case "":
		return "", nil
	default:
		return "", errors.New("unsupported rule tool")
	}
}

func decodeStrictJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected extra JSON value")
		}
		return err
	}
	return nil
}

func managedRuleNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func managedRuleSaveStatus(err error) int {
	if errors.Is(err, managedrules.ErrInvalidID) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func managedRuleLoadStatus(err error) int {
	switch {
	case errors.Is(err, managedrules.ErrInvalidID):
		return http.StatusBadRequest
	case managedRuleNotFound(err):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeJSONStatus(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
