package server

import (
	"net/http"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/extraops"
)

// handleExtrasDelete delegates lifecycle changes to the same owner as CLI/TUI.
// The domain acquires the remote root lock; this boundary holds only s.mu.
func (s *Server) handleExtrasDelete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := r.PathValue("name")
	s.mu.Lock()
	defer s.mu.Unlock()

	extras := s.extrasConfig()
	var removed *config.ExtraConfig
	for i := range extras {
		if extras[i].Name == name {
			removed = &extras[i]
			break
		}
	}
	if removed == nil {
		writeCodedError(w, http.StatusNotFound, "instructions_shared_not_found", "extra not found: "+name, map[string]string{"name": name})
		return
	}
	remote := removed.Remote != ""
	baseline := *removed
	root := s.cfg.EffectiveExtrasSource()
	if s.IsProjectMode() {
		root = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	result, err := extraops.Remove(extraops.RemoveRequest{
		Extras: extras, Name: name, SourceDir: s.extrasSourceDir(*removed), ExtrasRoot: root, ProjectRoot: s.projectRoot,
		TargetPath: func(p string) string { return resolveExtrasTargetPath(s.projectRoot, p) },
		Load: func() ([]config.ExtraConfig, error) {
			if err := s.reloadExtrasReadOnly(); err != nil {
				return nil, err
			}
			fresh := s.extrasConfig()
			for _, e := range fresh {
				if e.Name == name {
					baseline = e
					break
				}
			}
			return fresh, nil
		},
		Save: func(remaining []config.ExtraConfig) error {
			if remote {
				if err := s.reloadExtrasReadOnly(); err != nil {
					return err
				}
				var err error
				remaining, err = extraops.WithoutUnchangedExtra(s.extrasConfig(), baseline)
				if err != nil {
					return err
				}
			}
			if s.IsProjectMode() {
				s.projectCfg.Extras = remaining
			} else {
				s.cfg.Extras = remaining
			}
			return s.saveAndReloadConfig()
		},
	})
	status, msg := "ok", ""
	if err != nil {
		status, msg = "partial", err.Error()
		// A rollback may have restored the on-disk config after Save reloaded it.
		_ = s.reloadExtrasReadOnly()
	}
	s.writeOpsLog("extras-remove", status, start, map[string]any{
		"name": name, "scope": "ui", "restored": result.Restored,
		"remote": result.Remote, "commit": result.Commit, "deleted_dir": result.DeletedDir,
		"missing_dir": result.MissingDir, "kept_edited_dir": result.KeptEditedDir, "kept_outputs": result.KeptOutputs,
	}, msg)
	if err != nil {
		writeCodedError(w, http.StatusInternalServerError, "instructions_restore_failed", err.Error(), map[string]string{"name": name, "detail": err.Error()})
		return
	}
	writeJSON(w, map[string]any{
		"success": true, "name": name, "restored": result.Restored, "remote": result.Remote,
		"commit": result.Commit, "deleted_dir": result.DeletedDir, "missing_dir": result.MissingDir, "kept_edited_dir": result.KeptEditedDir,
		"kept_outputs": result.KeptOutputs, "warnings": result.Warnings,
	})
}
