package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
	ssync "skillshare/internal/sync"
)

// Resolve the selected scope without ConfigPath's legacy migration side effects.
func (s *Server) profileScope() profileapply.Scope {
	return profileapply.Scope{ConfigPath: config.ReadOnlyConfigPath(), ProjectRoot: s.projectRoot}
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	v, err := profileapply.Load(s.profileScope())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	discovery, err := v.Discover()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	names := make([]string, 0, len(discovery.Skills))
	for _, skill := range discovery.Skills {
		if !skill.Disabled {
			names = append(names, skill.FlatName)
		}
	}
	slices.Sort(names)
	type row struct {
		Name        string                  `json:"name"`
		Description string                  `json:"description"`
		Skills      *profile.SkillSelection `json:"skills"`
		Selected    int                     `json:"selected"`
		Unmatched   []string                `json:"unmatched"`
		Usable      bool                    `json:"usable"`
		Reason      string                  `json:"reason,omitempty"`
	}
	rows := make([]row, 0, len(v.Catalog.Definitions))
	for _, name := range slices.Sorted(maps.Keys(v.Catalog.Definitions)) {
		d := v.Catalog.Definitions[name]
		preview, _ := profile.Preview(d, names)
		unmatched := preview.Unmatched
		if unmatched == nil {
			unmatched = []string{}
		}
		rows = append(rows, row{Name: name, Description: d.Description, Skills: d.Skills, Selected: len(preview.Selected), Unmatched: unmatched, Usable: d.Usable, Reason: d.Reason})
	}
	info := v.Info(discovery.Skills)
	if info == nil {
		info = &profileapply.Info{State: "none"}
	}
	writeJSON(w, struct {
		Active   string `json:"active"`
		State    string `json:"state"`
		Pending  bool   `json:"pending"`
		Profiles []row  `json:"profiles"`
	}{info.Name, info.State, info.Pending, rows})
}

func (s *Server) handleSetActiveProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         json.RawMessage `json:"name"`
		DryRun       bool            `json:"dryRun"`
		DiscardEdits bool            `json:"discardEdits"`
		AllowEmpty   bool            `json:"allowEmpty"`
	}
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	var name *string
	if len(body.Name) == 0 || json.Unmarshal(body.Name, &name) != nil {
		writeError(w, http.StatusBadRequest, "name is required and must be a string or null")
		return
	}
	req := profileapply.Request{Scope: s.profileScope(), None: name == nil, DryRun: body.DryRun, DiscardEdits: body.DiscardEdits, AllowEmpty: body.AllowEmpty}
	if name != nil {
		req.Name = *name
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out, err := profileapply.Apply(r.Context(), req)
	if err != nil && !errors.Is(err, profileapply.ErrIncomplete) {
		var params map[string]string
		if len(out.Blocked) > 0 {
			blocked, marshalErr := json.Marshal(out.Blocked)
			if marshalErr != nil {
				writeError(w, http.StatusInternalServerError, marshalErr.Error())
				return
			}
			params = map[string]string{"blocked": string(blocked)}
		}
		if writeProfileError(w, err, params) {
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, out)
}

type profileDraft struct {
	Name        json.RawMessage `json:"name"`
	Description *string         `json:"description"`
	Skills      json.RawMessage `json:"skills"`
}

func decodeProfileDraft(w http.ResponseWriter, r *http.Request) (profileDraft, *profile.SkillSelection, bool) {
	var input *profileDraft
	if err := decodeJSON(w, r, &input, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return profileDraft{}, nil, false
	}
	if input == nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return profileDraft{}, nil, false
	}
	body := *input
	if len(body.Skills) == 0 {
		return body, nil, true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body.Skills, &fields); err != nil {
		writeError(w, http.StatusBadRequest, "skills must be an object or null")
		return body, nil, false
	}
	selection := &profile.SkillSelection{}
	for key, raw := range fields {
		var values []string
		if (key != "include" && key != "exclude") || string(raw) == "null" || json.Unmarshal(raw, &values) != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("skills.%s must be a list of strings", key))
			return body, nil, false
		}
		if key == "include" {
			selection.Include, selection.IncludePresent = values, true
		} else {
			selection.Exclude, selection.ExcludePresent = values, true
		}
	}
	return body, selection, true
}

func (s *Server) handleMutateProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	mutations := []profile.Mutation{{Op: "delete", Name: name}}
	if r.Method != http.MethodDelete {
		body, skills, ok := decodeProfileDraft(w, r)
		if !ok {
			return
		}
		previous := name
		if len(body.Name) > 0 {
			var provided *string
			if json.Unmarshal(body.Name, &provided) != nil || provided == nil {
				writeError(w, http.StatusBadRequest, "name must be a string")
				return
			}
			name = *provided
		}
		mutations = nil
		if r.Method == http.MethodPost {
			mutations = append(mutations, profile.Mutation{Op: "create", Name: name})
		} else if len(body.Name) > 0 {
			mutations = append(mutations, profile.Mutation{Op: "rename", Name: previous, NewName: name})
		}
		// A name-only request is a raw rename, including for definitions from
		// newer versions. Actual semantic edits still validate the definition.
		if r.Method == http.MethodPost || len(body.Name) == 0 || body.Description != nil || len(body.Skills) > 0 {
			mutations = append(mutations, profile.Mutation{Op: "replace", Name: name, Description: body.Description, Replacement: skills})
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mutateProfile(w, r, mutations)
}

// The shared transaction owns the file lock, validation, saving and oplog.
func (s *Server) mutateProfile(w http.ResponseWriter, r *http.Request, mutations []profile.Mutation) {
	change, err := profileapply.Mutate(r.Context(), s.profileScope(), mutations...)
	if err != nil {
		if writeProfileError(w, err, nil) {
			return
		}
		status := http.StatusBadRequest
		if change.Changed {
			status = http.StatusInternalServerError
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, struct {
		Changed   bool                  `json:"changed"`
		NeedsSync bool                  `json:"needsSync"`
		Skills    []profile.SkillResult `json:"skills"`
	}{change.Changed, change.NeedsSync, change.Skills})
}

func (s *Server) handleProfileSkills(w http.ResponseWriter, r *http.Request) {
	var body struct{ Add, Remove []string }
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if len(body.Add)+len(body.Remove) == 0 {
		writeError(w, http.StatusBadRequest, "add or remove requires at least one skill")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := profileapply.Load(s.profileScope())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	catalog, err := ssync.DiscoverSourceSkillsAll(v.Config.EffectiveSkillsSource(), v.Config.SkillsWalk())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, name := range append(slices.Clone(body.Add), body.Remove...) {
		if !slices.ContainsFunc(catalog, func(skill ssync.DiscoveredSkill) bool { return skill.FlatName == name }) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("skill %q does not exist in the source", name))
			return
		}
	}
	name := r.PathValue("name")
	s.mutateProfile(w, r, []profile.Mutation{{Op: "add-skills", Name: name, Skills: body.Add}, {Op: "remove-skills", Name: name, Skills: body.Remove}})
}

func (s *Server) handlePreviewProfile(w http.ResponseWriter, r *http.Request) {
	_, skills, ok := decodeProfileDraft(w, r)
	if !ok {
		return
	}
	draft := profile.Definition{Name: "preview", Skills: skills, Usable: true}
	if err := profile.ValidateDefinition(draft); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	v, err := profileapply.Load(s.profileScope())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	discovery, err := v.Discover()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	names := make([]string, 0, len(discovery.Skills))
	for _, skill := range discovery.Skills {
		if !skill.Disabled {
			names = append(names, skill.FlatName)
		}
	}
	slices.Sort(names)
	preview, err := profile.Preview(draft, names)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, struct {
		Selected  []string `json:"selected"`
		Unmatched []string `json:"unmatched"`
	}{append([]string{}, preview.Selected...), append([]string{}, preview.Unmatched...)})
}
