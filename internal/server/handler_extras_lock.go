package server

import (
	"fmt"
	"slices"

	"skillshare/internal/config"
	"skillshare/internal/install"
	"skillshare/internal/memory"
)

var lockRemoteExtraRoot = install.LockExtrasRoot

// lockRemoteExtraMutation is called with s.mu held, before mutation. The
// root lock is acquired once here, followed by a fresh config read. Removal
// instead owns this lock in extraops.Remove and must not call this wrapper.
func (s *Server) lockRemoteExtraMutation(names ...string) (func(), error) {
	if err := s.reloadExtrasReadOnly(); err != nil {
		return nil, err
	}
	remote := false
	for _, extra := range s.extrasConfig() {
		if extra.Remote != "" && (len(names) == 0 || slices.Contains(names, extra.Name)) {
			remote = true
			break
		}
	}
	if !remote {
		return func() {}, nil
	}
	root := s.cfg.EffectiveExtrasSource()
	if s.IsProjectMode() {
		root = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	unlock, err := lockRemoteExtraRoot(root)
	if err != nil {
		return nil, err
	}
	if err := s.reloadExtrasReadOnly(); err != nil {
		unlock()
		return nil, err
	}
	for _, extra := range s.extrasConfig() {
		if extra.Remote == "" || len(names) != 0 && !slices.Contains(names, extra.Name) {
			continue
		}
		state := install.InspectRemoteExtra(root, extra.Name)
		if state.Err != nil {
			unlock()
			return nil, state.Err
		}
		if state.Interrupted {
			unlock()
			return nil, fmt.Errorf("extra %q is interrupted; reinstall it before editing", extra.Name)
		}
	}
	return unlock, nil
}

// Memory's existing declaration lookup is case-insensitive; lock its actual
// name, then let the writer resolve the freshly loaded source under that lock.
func (s *Server) lockRemoteMemoryMutation() (func(), error) {
	cached, _, cachedErr := memory.Extra(s.extrasConfig())
	if err := s.reloadExtrasReadOnly(); err != nil {
		// Local handlers historically retain their loaded source when a later
		// config save fails (including partially initialized starter notes).
		// A cached remote declaration must still fail closed.
		if cachedErr == nil && cached.Remote == "" {
			return func() {}, nil
		}
		return nil, err
	}
	extra, _, err := memory.Extra(s.extrasConfig())
	if err != nil {
		return nil, err
	}
	return s.lockRemoteExtraMutation(extra.Name)
}

// reloadExtrasReadOnly refreshes the owning config without migration writes
// before the root lock or while resolving a removal's baseline.
func (s *Server) reloadExtrasReadOnly() error {
	if s.IsProjectMode() {
		cfg, err := config.LoadProjectReadOnly(s.projectRoot)
		if err != nil {
			return err
		}
		s.projectCfg = cfg
		s.cfg.Targets, s.unresolvedTargets = config.ResolveValidProjectTargets(s.projectRoot, cfg)
		return nil
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}
