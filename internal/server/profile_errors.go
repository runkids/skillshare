package server

import (
	"errors"
	"net/http"

	"skillshare/internal/profile"
	"skillshare/internal/profileapply"
)

// writeProfileError reports known profile errors and leaves unrelated errors to the caller.
func writeProfileError(w http.ResponseWriter, err error, params map[string]string) bool {
	var code string
	switch {
	case errors.Is(err, profile.ErrProfileUnavailable):
		code = "profile_unavailable"
	case errors.Is(err, profileapply.ErrRefused):
		code = string(profileapply.RefusalGeneric)
		var refused *profileapply.Refusal
		if errors.As(err, &refused) {
			switch refused.Code {
			case profileapply.RefusalLocked, profileapply.RefusalEmpty, profileapply.RefusalBlocked:
				code = string(refused.Code)
			}
		}
	default:
		return false
	}
	writeCodedError(w, http.StatusConflict, code, err.Error(), params)
	return true
}

// refuseUnavailableProfile requires s.mu and runs before discovery or SSE initialization.
func (s *Server) refuseUnavailableProfile(w http.ResponseWriter) bool {
	err := s.activeProfileError()
	if err == nil {
		return false
	}
	writeCodedError(w, http.StatusConflict, "profile_unavailable", err.Error(), nil)
	return true
}

// activeProfileError requires the server config lock to be held.
func (s *Server) activeProfileError() error {
	if s.IsProjectMode() && s.projectCfg != nil {
		return s.projectCfg.ActiveProfileError()
	}
	return s.cfg.ActiveProfileError()
}
