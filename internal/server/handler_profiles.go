package server

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"

	"skillshare/internal/config"
	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
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
