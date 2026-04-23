package server

import (
	"encoding/json"
	"net/http"
	"os"

	"skillshare/internal/config"
	hookpkg "skillshare/internal/hooks"
	pluginpkg "skillshare/internal/plugins"
)

func (s *Server) pluginSourceDir() string {
	if s.IsProjectMode() {
		return config.PluginsSourceDirProject(s.projectRoot)
	}
	return s.cfg.EffectivePluginsSource()
}

func (s *Server) hookSourceDir() string {
	if s.IsProjectMode() {
		return config.HooksSourceDirProject(s.projectRoot)
	}
	return s.cfg.EffectiveHooksSource()
}

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	bundles, err := pluginpkg.Discover(s.pluginSourceDir())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"plugins": bundles})
}

func (s *Server) handlePluginsDiff(w http.ResponseWriter, r *http.Request) {
	type pluginDiffEntry struct {
		Name   string   `json:"name"`
		Target string   `json:"target"`
		Synced bool     `json:"synced"`
		Items  []string `json:"items,omitempty"`
	}
	bundles, _ := pluginpkg.Discover(s.pluginSourceDir())
	var out []pluginDiffEntry
	for _, bundle := range bundles {
		for _, target := range pluginpkg.SupportedTargets(bundle) {
			rendered := pluginpkg.RenderRoot(s.projectRoot, bundle.Name, target)
			_, err := os.Stat(rendered)
			entry := pluginDiffEntry{Name: bundle.Name, Target: target, Synced: err == nil}
			if err != nil {
				entry.Items = []string{"missing rendered state: " + rendered}
			}
			out = append(out, entry)
		}
	}
	writeJSON(w, map[string]any{"plugins": out})
}

func (s *Server) handlePluginsImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref  string `json:"ref"`
		From string `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	bundle, err := pluginpkg.Import(s.pluginSourceDir(), body.Ref, pluginpkg.ImportOptions{From: body.From})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"plugin": bundle})
}

func (s *Server) handlePluginsSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target  string `json:"target"`
		Install *bool  `json:"install"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	install := true
	if body.Install != nil {
		install = *body.Install
	}
	results, err := pluginpkg.SyncAll(s.pluginSourceDir(), s.projectRoot, body.Target, install)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"plugins": results})
}

func (s *Server) handleHooks(w http.ResponseWriter, r *http.Request) {
	bundles, err := hookpkg.Discover(s.hookSourceDir())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"hooks": bundles})
}

func (s *Server) handleHooksDiff(w http.ResponseWriter, r *http.Request) {
	type hookDiffEntry struct {
		Name   string   `json:"name"`
		Target string   `json:"target"`
		Synced bool     `json:"synced"`
		Items  []string `json:"items,omitempty"`
	}
	bundles, _ := hookpkg.Discover(s.hookSourceDir())
	var out []hookDiffEntry
	for _, bundle := range bundles {
		for _, target := range hookpkg.SupportedTargets(bundle) {
			root := hookpkg.RenderRoot(s.projectRoot, bundle.Name, target)
			_, err := os.Stat(root)
			entry := hookDiffEntry{Name: bundle.Name, Target: target, Synced: err == nil}
			if err != nil {
				entry.Items = []string{"missing rendered state: " + root}
			}
			out = append(out, entry)
		}
	}
	writeJSON(w, map[string]any{"hooks": out})
}

func (s *Server) handleHooksImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		From      string `json:"from"`
		All       bool   `json:"all"`
		OwnedOnly bool   `json:"owned_only"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	bundles, err := hookpkg.Import(s.hookSourceDir(), hookpkg.ImportOptions{
		From:      body.From,
		Project:   s.projectRoot,
		All:       body.All,
		OwnedOnly: body.OwnedOnly,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, map[string]any{"hooks": bundles})
}

func (s *Server) handleHooksSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Target string `json:"target"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	results, err := hookpkg.SyncAll(s.hookSourceDir(), s.projectRoot, body.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"hooks": results})
}
