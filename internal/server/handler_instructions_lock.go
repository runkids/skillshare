package server

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	"skillshare/internal/memory"
	"skillshare/internal/utils"
)

// Resolve participants only after the root lock and its fresh config reload.
// A nil selection serializes a config-only instruction change.
func (s *Server) lockInstructionWrite(w http.ResponseWriter, selectNames func() []string) func() {
	unlock, root, err := s.lockExtraMutationRoot(nil)
	if err == nil && selectNames != nil {
		if names := selectNames(); len(names) > 0 {
			err = s.inspectExtraMutation(root, names)
		}
	}
	if err != nil {
		if unlock != nil {
			unlock()
		}
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return nil
	}
	return unlock
}

// Include both configured deliveries (also copies) and the sources a write
// reaches, including directory extras and existing parent aliases.
func (s *Server) instructionParticipants(paths []string, names ...string) []string {
	paths = slices.Clone(paths)
	for _, extra := range s.extrasConfig() {
		if slices.Contains(names, extra.Name) {
			paths = append(paths, filepath.Join(s.extrasSourceDir(extra), extra.File))
		}
	}
	selected := slices.Clone(names)
	res := s.instructionsResolver()
	for _, extra := range s.extrasConfig() {
		if extra.Remote == "" || slices.Contains(selected, extra.Name) {
			continue
		}
		owned := []string{filepath.Join(s.extrasSourceDir(extra), extra.File)}
		for j, target := range extra.Targets {
			path := res.TargetDir(target.Path)
			if extra.File != "" {
				path = instructions.ExtraFile(extra, j, res).Target
			}
			owned = append(owned, path)
		}
		for _, path := range paths {
			if slices.ContainsFunc(owned, func(p string) bool { return instructionTouches(path, p, extra.File == "") }) {
				selected = append(selected, extra.Name)
				break
			}
		}
	}
	return selected
}

func instructionTouches(path, owned string, directory bool) bool {
	for _, resolve := range []bool{false, true} {
		a, b := filepath.Clean(path), filepath.Clean(owned)
		if resolve {
			if real, err := utils.ProjectWritePath(a, ""); err == nil {
				a = real
			}
			if real, err := utils.ProjectWritePath(b, ""); err == nil {
				b = real
			}
		}
		if utils.PathsEqual(a, b) || directory && utils.PathHasPrefix(a, strings.TrimSuffix(b, string(filepath.Separator))+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (s *Server) targetInstructionMutation(targets []string, names ...string) func() []string {
	return func() []string {
		var paths []string
		for _, target := range targets {
			if it, _, ok := s.targetInstructions(target); ok {
				paths = append(paths, it.Path)
			}
		}
		return s.instructionParticipants(paths, names...)
	}
}

func (s *Server) sharedCopyPaths(name string) []string {
	var paths []string
	if extra, ok := s.sharedExtra(name); ok {
		for j := range extra.Targets {
			if f := instructions.ExtraFile(extra, j, s.instructionsResolver()); f.Mode == "copy" {
				paths = append(paths, f.Target)
			}
		}
	}
	return paths
}

func (s *Server) sharedInstructionMutation(name string) func() []string {
	return func() []string {
		return s.instructionParticipants(s.sharedCopyPaths(name), name)
	}
}

func (s *Server) locationInstructionMutation(name, path string) func() []string {
	return func() []string {
		var paths []string
		if i, j := s.findLocation(name, path); i != -1 && j != -1 {
			paths = append(paths, instructions.ExtraFile((*s.sharedExtras())[i], j, s.instructionsResolver()).Target)
		}
		return s.instructionParticipants(paths, name)
	}
}

func (s *Server) resolveInstructionMutation(name, target, path string) func() []string {
	if path != "" {
		return s.locationInstructionMutation(name, path)
	}
	return s.targetInstructionMutation([]string{target}, name)
}

func (s *Server) projectInstructionMutation(target string) func() []string {
	return func() []string {
		paths := []string{filepath.Join(s.projectRoot, instructions.AgentsFile)}
		if target != "" {
			paths = nil
			if tc, found := s.cfg.Targets[target]; found {
				if it, ok := config.TargetInstructions(target, tc, true); ok {
					paths = append(paths, filepath.Join(s.projectRoot, it.Path))
					if instructions.ProjectReach(s.projectRoot, target, it).Shim == instructions.ShimLink {
						paths = append(paths, filepath.Join(s.projectRoot, instructions.AgentsFile))
					}
				}
			}
		}
		return s.instructionParticipants(paths)
	}
}

func (s *Server) createInstructionMutation(name, from string) func() []string {
	return func() []string {
		extra := config.ExtraConfig{Name: name, File: instructions.AgentsFile}
		paths := []string{filepath.Join(s.extrasSourceDir(extra), extra.File)}
		if it, _, ok := s.targetInstructions(from); from != "" && ok {
			paths = append(paths, it.Path)
		}
		return s.instructionParticipants(paths)
	}
}

func (s *Server) addLocationInstructionMutation(name, stored, as string) func() []string {
	return func() []string {
		var paths []string
		if extra, ok := s.sharedExtra(name); ok {
			if as == "" {
				as = extra.File
			}
			paths = append(paths, filepath.Join(resolveExtrasTargetPath(s.projectRoot, stored), as))
		}
		return s.instructionParticipants(paths, name)
	}
}

func (s *Server) lockGuidanceMutation(w http.ResponseWriter, body guidanceRequest) (memory.GuidanceInput, error, func()) {
	var in memory.GuidanceInput
	var inputErr error
	unlock := s.lockInstructionWrite(w, func() []string {
		in, inputErr = s.guidanceInput()
		if inputErr != nil {
			return nil
		}
		plan, _ := memory.PlanGuidance(in, body.Targets, body.Modes)
		var paths, names []string
		for _, change := range plan.Changes {
			paths = append(paths, change.Path)
			if change.Shared != "" {
				names = append(names, change.Shared)
				paths = append(paths, s.sharedCopyPaths(change.Shared)...)
			}
		}
		return s.instructionParticipants(paths, names...)
	})
	return in, inputErr, unlock
}

// Conversion owns the root lock already. Inspect its actual planned writes
// before Apply; preview does not acquire a lock or inspect interruptions.
func (s *Server) checkInstructionChanges(w http.ResponseWriter, changes []instructions.Change) bool {
	var paths []string
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	names := s.instructionParticipants(paths)
	if len(names) == 0 {
		return true
	}
	root := s.cfg.EffectiveExtrasSource()
	if s.IsProjectMode() {
		root = s.projectCfg.EffectiveExtrasSource(s.projectRoot)
	}
	if err := s.inspectExtraMutation(root, names); err != nil {
		writeCodedError(w, http.StatusConflict, "instructions_write_failed", err.Error(), map[string]string{"detail": err.Error()})
		return false
	}
	return true
}
