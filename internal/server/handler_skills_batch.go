package server

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"skillshare/internal/install"
	"skillshare/internal/resource"
	"skillshare/internal/sourcefs"
	ssync "skillshare/internal/sync"
	"skillshare/internal/utils"
)

// matchesFolder reports whether relPath belongs to the given folder filter.
// folder="" → root-level only (dir == ".")
// folder="*" → all skills
// otherwise → dir == folder || dir has folder/ prefix
func matchesFolder(relPath, folder string) bool {
	if folder == "*" {
		return true
	}
	dir := filepath.ToSlash(filepath.Dir(relPath))
	if folder == "" {
		return dir == "."
	}
	return dir == folder || strings.HasPrefix(dir, folder+"/")
}

type batchSetTargetsRequest struct {
	Folder string `json:"folder"`
	Target string `json:"target"`
}

type batchSetTargetsResponse struct {
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}

// handleBatchSetTargets handles POST /api/skills/batch/targets.
// Sets or removes the target for all skills in a folder.
func (s *Server) handleBatchSetTargets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req batchSetTargetsRequest
	if err := decodeJSON(w, r, &req, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		}
		return
	}

	// Reject path traversal in folder
	if strings.Contains(req.Folder, "..") {
		writeError(w, http.StatusBadRequest, "invalid folder: path traversal not allowed")
		return
	}
	folder := filepath.ToSlash(filepath.Clean(req.Folder))
	if req.Folder == "" {
		folder = ""
	}

	// Snapshot config under read lock, then discover without holding the lock.
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	s.mu.RUnlock()

	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	var updated, skipped int
	var errors []string

	// Collect skills that need meta hash refresh (outside the lock).
	type updatedSkill struct {
		name string
		path string
	}
	var updatedSkills []updatedSkill
	overridden := false

	var values []string
	if req.Target != "" {
		values = []string{req.Target}
	}

	var src *sourcefs.Root
	if len(discovered) > 0 {
		if src, err = sourcefs.Open(source); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to open skills source: "+err.Error())
			return
		}
		defer src.Close()
	}

	// Acquire write lock only for the file-write loop.
	s.mu.Lock()
	for _, d := range discovered {
		relPath := filepath.ToSlash(d.RelPath)
		if !matchesFolder(relPath, folder) {
			continue
		}

		// Skip disabled skills
		if d.Disabled {
			skipped++
			continue
		}

		// Tracked-repo members keep their SKILL.md untouched so the clone
		// stays clean for update; the targets live in .metadata.json.
		if d.IsInRepo {
			s.skillsStore.SetTargetOverride(relPath, values)
			overridden = true
			updated++
			continue
		}

		skillMDPath := filepath.Join(d.SourcePath, "SKILL.md")
		if err := utils.SetFrontmatterListWith(src.Writer(), skillMDPath, "metadata.targets", values); err != nil {
			errors = append(errors, d.FlatName+": "+err.Error())
			continue
		}

		if name := filepath.Base(d.SourcePath); s.skillsStore.HasFileHashes(name) {
			updatedSkills = append(updatedSkills, updatedSkill{name: name, path: d.SourcePath})
		}
		updated++
	}
	// Save the overrides before unlocking: every API request reloads skillsStore
	// from disk, so an override left unsaved here can be dropped by the next one.
	if overridden {
		if err := s.skillsStore.Save(s.cfg.EffectiveSkillsSource()); err != nil {
			s.mu.Unlock()
			writeError(w, http.StatusInternalServerError, "failed to save target overrides: "+err.Error())
			return
		}
	}
	s.mu.Unlock()

	// Recompute file hashes outside the lock so reads aren't blocked; the
	// store itself is only touched under the lock.
	hashes := make(map[string]map[string]string, len(updatedSkills))
	for _, sk := range updatedSkills {
		if h, err := install.ComputeFileHashes(sk.path); err == nil {
			hashes[sk.name] = h
		}
	}
	if len(hashes) > 0 {
		s.mu.Lock()
		for name, h := range hashes {
			s.skillsStore.SetFileHashes(name, h)
		}
		err := s.skillsStore.Save(s.cfg.EffectiveSkillsSource())
		s.mu.Unlock()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save file hashes: "+err.Error())
			return
		}
	}

	s.writeOpsLog("batch-set-targets", "ok", start, map[string]any{
		"folder":  req.Folder,
		"target":  req.Target,
		"updated": updated,
		"skipped": skipped,
		"scope":   "ui",
	}, "")

	if errors == nil {
		errors = []string{}
	}
	writeJSON(w, batchSetTargetsResponse{
		Updated: updated,
		Skipped: skipped,
		Errors:  errors,
	})
}

type setSkillTargetsRequest struct {
	Target string `json:"target"`
}

// handleSetSkillTargets handles PATCH /api/skills/{name}/targets.
// Sets or removes the target for a single skill.
func (s *Server) handleSetSkillTargets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	name := r.PathValue("name")

	var req setSkillTargetsRequest
	if err := decodeJSON(w, r, &req, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		}
		return
	}

	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	s.mu.RUnlock()

	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to discover skills: "+err.Error())
		return
	}

	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName != name && baseName != name {
			continue
		}

		var values []string
		if req.Target != "" {
			values = []string{req.Target}
		}

		if d.IsInRepo {
			// Tracked-repo members keep their SKILL.md untouched so the clone
			// stays clean for update; the targets live in .metadata.json.
			s.mu.Lock()
			s.skillsStore.SetTargetOverride(d.RelPath, values)
			err := s.skillsStore.Save(s.cfg.EffectiveSkillsSource())
			s.mu.Unlock()

			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update skill: "+err.Error())
				return
			}
		} else {
			skillMDPath := filepath.Join(d.SourcePath, "SKILL.md")

			s.mu.Lock()
			err := setSourceFrontmatterList(source, skillMDPath, values)
			s.mu.Unlock()

			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update skill: "+err.Error())
				return
			}

			s.mu.RLock()
			refresh := s.skillsStore.HasFileHashes(d.RelPath)
			s.mu.RUnlock()
			if refresh {
				if hashes, err := install.ComputeFileHashes(d.SourcePath); err == nil {
					s.mu.Lock()
					s.skillsStore.SetFileHashes(d.RelPath, hashes)
					err := s.skillsStore.Save(s.cfg.EffectiveSkillsSource())
					s.mu.Unlock()

					if err != nil {
						writeError(w, http.StatusInternalServerError, "failed to update skill: "+err.Error())
						return
					}
				}
			}
		}

		s.writeOpsLog("set-skill-targets", "ok", start, map[string]any{
			"name":   name,
			"target": req.Target,
			"scope":  "ui",
		}, "")

		writeJSON(w, map[string]any{"success": true})
		return
	}

	// Try agents if skill not found
	s.mu.RLock()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	if agentsSource != "" {
		agents, _ := resource.AgentKind{}.Discover(agentsSource)
		for _, d := range agents {
			if d.FlatName != name {
				continue
			}

			var values []string
			if req.Target != "" {
				values = []string{req.Target}
			}

			s.mu.Lock()
			err := utils.SetFrontmatterList(d.SourcePath, "targets", values)
			s.mu.Unlock()

			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update agent: "+err.Error())
				return
			}

			s.writeOpsLog("set-agent-targets", "ok", start, map[string]any{
				"name":   name,
				"target": req.Target,
				"scope":  "ui",
			}, "")

			writeJSON(w, map[string]any{"success": true})
			return
		}
	}

	writeError(w, http.StatusNotFound, "resource not found: "+name)
}

// setSourceFrontmatterList sets metadata.targets in a SKILL.md in the skills
// source through its handle, so a linked SKILL.md is refused instead of
// written through.
func setSourceFrontmatterList(source, skillMDPath string, values []string) error {
	src, err := sourcefs.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()
	return utils.SetFrontmatterListWith(src.Writer(), skillMDPath, "metadata.targets", values)
}
