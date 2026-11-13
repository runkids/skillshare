package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/sourcelink"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/uninstall"
	"skillshare/internal/utils"
)

type sourceLinkRequest struct {
	Path   string `json:"path"`
	Name   string `json:"name,omitempty"`
	Enable bool   `json:"enable,omitempty"`
}

type sourceLinkItem struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	Available bool   `json:"available"`
	Warning   string `json:"warning,omitempty"`
}

func listSourceLinks(source string, walk sourcewalk.Options) ([]sourceLinkItem, error) {
	entries, err := sourcewalk.ReadDir(source, sourcewalk.Options{})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// Skips recorded so far, including traversal failures discovery met inside
	// a target that was readable at its root; call this after discovery.
	skipped := map[string]string{}
	for _, sk := range walk.Follow.Skipped() {
		skipped[sk.Name] = sk.Reason
	}
	links := []sourceLinkItem{}
	for _, entry := range entries {
		path := filepath.Join(source, entry.Name())
		if !utils.IsLinkMode(path, entry.Type()) {
			continue
		}
		item := sourceLinkItem{Name: entry.Name()}
		item.Target, err = utils.ResolveLinkTarget(path)
		resolved, followed := walk.Follow.Resolve(path)
		switch {
		case err != nil:
			item.Warning = err.Error()
		case followed && skipped[item.Name] == "":
			item.Target, item.Available = resolved, true
		case walk.Follow == nil:
			item.Warning = "follow_source_links is off"
		case skipped[item.Name] != "":
			item.Warning = skipped[item.Name]
		default:
			item.Warning = "target is not a directory"
		}
		links = append(links, item)
	}
	return links, nil
}

func (s *Server) handleCreateSourceLink(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	var body sourceLinkRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return
	}
	if body.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := sourcelink.Create(s.skillsSource(), config.SkillsTargetPaths(s.cfg.Targets), body.Path, body.Name)
	args := map[string]any{"name": body.Name, "target": body.Path, "enable": body.Enable, "scope": "ui"}
	if err != nil {
		s.writeOpsLog("link", "error", start, args, err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer s.snapshotFollow()
	following := s.cfg.FollowSourceLinks
	if s.IsProjectMode() {
		following = s.projectCfg.FollowSourceLinks
	}
	if body.Enable && !following {
		if s.IsProjectMode() {
			s.projectCfg.FollowSourceLinks = true
		} else {
			s.cfg.FollowSourceLinks = true
		}
		if err := s.saveConfig(); err != nil {
			if s.IsProjectMode() {
				s.projectCfg.FollowSourceLinks = false
			} else {
				s.cfg.FollowSourceLinks = false
			}
			// Enabling is part of the request: do not leave a link behind a failure.
			_ = sourcelink.Discard(s.skillsSource(), filepath.Base(res.Path))
			s.writeOpsLog("link", "error", start, args, err.Error())
			writeError(w, http.StatusInternalServerError, "failed to enable follow_source_links: "+err.Error())
			return
		}
	}
	s.writeOpsLog("link", "ok", start, args, "")
	writeJSON(w, map[string]any{"path": res.Path, "target": res.Target, "kind": res.Kind, "warning": res.Warning})
}

func (s *Server) handleRemoveSourceLink(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	name := r.PathValue("name")
	args := map[string]any{"name": name, "scope": "ui"}
	trashPath, err := sourcelink.Remove(s.skillsSource(), s.trashBase(), name)
	if err != nil {
		s.writeOpsLog("unlink", "error", start, args, err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var warning string
	if !s.IsProjectMode() {
		err := s.reloadSkillsStore()
		if err == nil {
			err = s.forgetUnlinkedRecords(name, trashPath)
		}
		if err != nil {
			warning = fmt.Sprintf("Unlinked, but could not update metadata: %v", err)
			log.Printf("warning: %s", warning)
		}
	}
	s.snapshotFollow()
	status := "ok"
	if warning != "" {
		status = "partial"
	}
	s.writeOpsLog("unlink", status, start, args, warning)
	writeJSON(w, map[string]any{"success": true, "name": name, "warning": warning})
}

// forgetUnlinkedRecords persists cleanup against the store just reloaded
// by the handler. Refresh the cache if persistence fails, as for uninstall.
func (s *Server) forgetUnlinkedRecords(name, trashPath string) error {
	if err := uninstall.ForgetUnlinkedName(s.skillsSource(), trashPath, name, s.skillsStore); err != nil {
		if reloadErr := s.reloadSkillsStore(); reloadErr != nil {
			log.Printf("warning: failed to reload skills metadata: %v", reloadErr)
		}
		return err
	}
	return nil
}
