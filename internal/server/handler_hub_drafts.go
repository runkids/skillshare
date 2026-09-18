package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/hub"
)

func (s *Server) hubDraftStore() hub.DraftStore {
	return hub.DraftStore{Dir: filepath.Join(filepath.Dir(s.configPath()), "hub-drafts")}
}

func draftError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, hub.ErrDraftConflict) {
		status = http.StatusConflict
	}
	if os.IsNotExist(err) {
		status = http.StatusNotFound
	}
	if os.IsPermission(err) {
		status = http.StatusInternalServerError
	}
	writeError(w, status, err.Error())
}

func writeDraft(w http.ResponseWriter, d hub.Draft) {
	writeJSON(w, map[string]any{"draft": d, "problems": hub.DraftProblems(d)})
}

func decodeDraftBody(w http.ResponseWriter, r *http.Request, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("expected a single JSON document")
	}
	return nil
}

func (s *Server) handleHubDraftCandidates(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	source := s.skillsSource()
	s.mu.RUnlock()
	entries, err := hub.DraftCandidates(source)
	if err != nil {
		draftError(w, err)
		return
	}
	writeJSON(w, entries)
}

func (s *Server) handleHubDrafts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		drafts, err := s.hubDraftStore().List()
		if err != nil {
			draftError(w, err)
			return
		}
		writeJSON(w, drafts)
		return
	}
	start := time.Now()
	var d hub.Draft
	if err := decodeDraftBody(w, r, &d); err != nil {
		draftError(w, err)
		return
	}
	created, err := s.hubDraftStore().Create(d)
	if err != nil {
		draftError(w, err)
		return
	}
	s.writeOpsLog("hub-draft-create", "ok", start, map[string]any{"id": created.ID}, "")
	writeDraft(w, created)
}

func (s *Server) handleHubDraft(w http.ResponseWriter, r *http.Request) {
	store, id := s.hubDraftStore(), r.PathValue("id")
	start := time.Now()
	switch r.Method {
	case http.MethodGet:
		d, err := store.Get(id)
		if err != nil {
			draftError(w, err)
			return
		}
		writeDraft(w, d)
	case http.MethodPut:
		var d hub.Draft
		if err := decodeDraftBody(w, r, &d); err != nil {
			draftError(w, err)
			return
		}
		d.ID = id
		saved, err := store.Save(d)
		if err != nil {
			draftError(w, err)
			return
		}
		s.writeOpsLog("hub-draft-save", "ok", start, map[string]any{"id": id}, "")
		writeDraft(w, saved)
	case http.MethodDelete:
		if err := store.Delete(id, r.URL.Query().Get("revision")); err != nil {
			draftError(w, err)
			return
		}
		s.writeOpsLog("hub-draft-delete", "ok", start, map[string]any{"id": id}, "")
		writeJSON(w, map[string]bool{"success": true})
	}
}

func (s *Server) handleHubDraftImport(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4<<20))
	if err != nil {
		draftError(w, err)
		return
	}
	d, err := hub.ImportDraft(raw)
	if err != nil {
		draftError(w, err)
		return
	}
	d, err = s.hubDraftStore().Create(d)
	if err != nil {
		draftError(w, err)
		return
	}
	s.writeOpsLog("hub-draft-import", "ok", start, map[string]any{"id": d.ID}, "")
	writeDraft(w, d)
}

func (s *Server) handleHubDraftExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision string `json:"revision"`
	}
	if err := decodeDraftBody(w, r, &req); err != nil {
		draftError(w, err)
		return
	}
	d, err := s.hubDraftStore().Get(r.PathValue("id"))
	if err != nil {
		draftError(w, err)
		return
	}
	if req.Revision != d.Revision {
		draftError(w, hub.ErrDraftConflict)
		return
	}
	data, err := hub.ExportDraft(d)
	if err != nil {
		draftError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="skillshare-hub.json"`)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
