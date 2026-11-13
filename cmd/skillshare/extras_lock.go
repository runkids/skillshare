package main

import (
	"fmt"

	"skillshare/internal/config"
	"skillshare/internal/install"
)

var lockRemoteExtraRoot = install.LockExtrasRoot

// lockRemoteExtraMutation runs before the writer's normal load, so a waiter
// never saves a config read before publication. Local extras take no root lock.
func lockRemoteExtraMutation(mode runMode, cwd, name string) (func(), error) {
	var entries []config.ExtraConfig
	var root string
	if mode == modeProject {
		cfg, err := config.LoadProjectReadOnly(cwd)
		if err != nil {
			return nil, err
		}
		entries, root = cfg.Extras, cfg.EffectiveExtrasSource(cwd)
	} else {
		cfg, err := config.LoadReadOnly()
		if err != nil {
			return nil, err
		}
		entries, root = cfg.Extras, cfg.EffectiveExtrasSource()
	}
	_, extra := findExtraByName(entries, name)
	if extra.Remote == "" {
		return func() {}, nil
	}
	unlock, err := lockRemoteExtraRoot(root)
	if err != nil {
		return nil, err
	}
	state := install.InspectRemoteExtra(root, name)
	if state.Err != nil {
		unlock()
		return nil, state.Err
	}
	if state.Interrupted {
		unlock()
		return nil, fmt.Errorf("extra %q is interrupted; reinstall it before editing or collecting", name)
	}
	return unlock, nil
}

func (m extrasListTUIModel) mutationMode() runMode {
	if m.projCfg != nil {
		return modeProject
	}
	return modeGlobal
}
