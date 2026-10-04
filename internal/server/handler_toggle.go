package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"skillshare/internal/resource"
	"skillshare/internal/skillignore"
	"skillshare/internal/sourcefs"
	ssync "skillshare/internal/sync"
)

func (s *Server) handleDisableSkill(w http.ResponseWriter, r *http.Request) {
	s.handleToggleSkill(w, r, false)
}

func (s *Server) handleEnableSkill(w http.ResponseWriter, r *http.Request) {
	s.handleToggleSkill(w, r, true)
}

func (s *Server) handleToggleSkill(w http.ResponseWriter, r *http.Request, enable bool) {
	start := time.Now()

	name := r.PathValue("name")
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "agent" && kind != "skill" {
		writeError(w, http.StatusBadRequest, "invalid kind: "+kind)
		return
	}

	// Resolve under RLock — discovery is I/O-heavy, don't hold write lock
	s.mu.RLock()
	source := s.cfg.EffectiveSkillsSource()
	agentsSource := s.agentsSource()
	s.mu.RUnlock()

	relPath := ""
	isDisabled := false
	ignorePath := ""
	if kind == "agent" {
		if agentsSource == "" {
			writeError(w, http.StatusNotFound, "agent not found: "+name)
			return
		}
		var err error
		relPath, isDisabled, err = s.resolveAgentRelPathWithStatus(agentsSource, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		ignorePath = filepath.Join(agentsSource, ".agentignore")
	} else {
		var err error
		relPath, isDisabled, err = s.resolveSkillRelPathWithStatus(source, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := sourcefs.CheckMoveOut(source, filepath.Join(source, relPath)); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		ignorePath = filepath.Join(source, ".skillignore")
	}
	// Write lock only for the file mutation
	s.mu.Lock()
	defer s.mu.Unlock()

	iw, closeIgnore, err := skillignore.OpenWriter(ignorePath, kind != "agent")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
		return
	}
	defer closeIgnore()

	action := "disable"
	disabled := true

	if enable {
		action = "enable"
		disabled = false
		if !isDisabled {
			writeJSON(w, map[string]any{"success": true, "name": name, "disabled": false, "message": "not disabled"})
			return
		}
		removed, err := skillignore.RemovePatternWith(iw, ignorePath, relPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
			return
		}
		if !removed {
			writeError(w, http.StatusConflict, "skill is disabled by a glob or directory pattern in .skillignore or .skillignore.local — edit the file manually to remove the matching rule")
			return
		}
	} else {
		added, err := skillignore.AddPatternWith(iw, ignorePath, relPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update .skillignore: "+err.Error())
			return
		}
		if !added && isDisabled {
			writeJSON(w, map[string]any{"success": true, "name": name, "disabled": true, "message": "already disabled"})
			return
		}
	}

	// The file write alone does not decide the outcome: a later rule, such as a
	// "!" line in the .local file, can still override it.
	states, err := s.disabledByRelPath(kind, source, agentsSource)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	logArgs := map[string]any{
		"name":  name,
		"kind":  kind,
		"scope": "ui",
	}
	if msg := toggleOverrideError(kind, enable, states[relPath]); msg != "" {
		// The ignore file may already have changed, so the attempt is still logged.
		s.writeOpsLog(action, "error", start, logArgs, msg)
		writeError(w, http.StatusConflict, msg)
		return
	}

	s.writeOpsLog(action, "ok", start, logArgs, "")

	writeJSON(w, map[string]any{"success": true, "name": name, "disabled": disabled})
}

// disabledByRelPath re-discovers skills or agents and reports, by relPath,
// whether each is disabled under the merged ignore and .local rules.
func (s *Server) disabledByRelPath(kind, source, agentsSource string) (map[string]bool, error) {
	states := map[string]bool{}
	if kind == "agent" {
		discovered, err := resource.AgentKind{}.Discover(agentsSource)
		if err != nil {
			return nil, fmt.Errorf("failed to discover agents: %w", err)
		}
		for _, d := range discovered {
			states[d.RelPath] = d.Disabled
		}
		return states, nil
	}
	discovered, err := ssync.DiscoverSourceSkillsAll(source)
	if err != nil {
		return nil, fmt.Errorf("failed to discover skills: %w", err)
	}
	for _, d := range discovered {
		states[d.RelPath] = d.Disabled
	}
	return states, nil
}

// toggleOverrideError explains why a toggle did not take effect, or returns ""
// when the resource ended up in the requested state.
func toggleOverrideError(kind string, enable, disabled bool) string {
	file := ".skillignore"
	if kind == "agent" {
		file = ".agentignore"
	}
	switch {
	case !enable && !disabled:
		return fmt.Sprintf("still enabled: a \"!\" rule in %[1]s.local or %[1]s re-includes it — remove that rule to disable it", file)
	case enable && disabled:
		return fmt.Sprintf("still disabled: another rule in %[1]s or %[1]s.local matches it — remove that rule to enable it", file)
	}
	return ""
}

// resolveSkillRelPath finds a skill's relPath by flatName or baseName.
// Searches all skills including disabled ones. Caller must provide source path.
func (s *Server) resolveSkillRelPath(source, name string) (string, error) {
	relPath, _, err := s.resolveSkillRelPathWithStatus(source, name)
	return relPath, err
}

// resolveSkillRelPathWithStatus is like resolveSkillRelPath but also returns
// whether the skill is currently disabled (matched by .skillignore).
func (s *Server) resolveSkillRelPathWithStatus(source, name string) (string, bool, error) {
	s.mu.RLock()
	follow := s.skillFollowSet()
	s.mu.RUnlock()
	discovered, err := ssync.DiscoverSourceSkillsAllWithOptions(source, ssync.DiscoveryOptions{Follow: follow})
	if err != nil {
		return "", false, fmt.Errorf("failed to discover skills: %w", err)
	}
	for _, d := range discovered {
		baseName := filepath.Base(d.SourcePath)
		if d.FlatName == name || baseName == name || d.RelPath == name {
			return d.RelPath, d.Disabled, nil
		}
	}
	return "", false, fmt.Errorf("skill not found: %s", name)
}
