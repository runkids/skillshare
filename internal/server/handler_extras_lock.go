package server

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

var lockRemoteExtraRoot = install.LockExtrasRoot

// lockRemoteExtraMutation is called with s.mu held, before mutation. The
// root lock is acquired once here, followed by a fresh config read. Removal
// instead owns this lock in extraops.Remove and must not call this wrapper.
func (s *Server) lockRemoteExtraMutation(name string) (func(), error) {
	if err := s.reloadExtrasReadOnly(); err != nil {
		return nil, err
	}
	remote := false
	for _, extra := range s.extrasConfig() {
		if extra.Name == name {
			remote = extra.Remote != ""
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
	state := install.InspectRemoteExtra(root, name)
	if state.Err != nil {
		unlock()
		return nil, state.Err
	}
	if state.Interrupted {
		unlock()
		return nil, fmt.Errorf("extra %q is interrupted; reinstall it before editing", name)
	}
	return unlock, nil
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
		return nil
	}
	cfg, err := config.LoadReadOnly()
	if err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}
