package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"skillshare/internal/config"
	"skillshare/internal/instructions"
	"skillshare/internal/memory"
	syncpkg "skillshare/internal/sync"
)

// Memory guidance is a marked block (see memory.Instructions) added to the
// instruction files targets already read. Connecting never changes the
// config, assignments, or links: the block goes into the file the target
// reads, and everything outside the block is kept. Status only reports what
// the files and links show; it never claims an agent read the notes.

type guidanceTarget struct {
	Name   string `json:"name"`
	State  string `json:"state"`            // unconfigured, configured, outdated or broken
	File   string `json:"file,omitempty"`   // where the block is, or would be written
	Detail string `json:"detail,omitempty"` // why broken: modified, malformed, mixed_modes, not_synced, unreadable or unsupported
	Mode   string `json:"mode,omitempty"`   // passive or active, when a block is found
}

type guidanceChange struct {
	Path    string   `json:"path"`
	Before  string   `json:"before"`
	After   string   `json:"after"`
	Targets []string `json:"targets"`
	Created bool     `json:"created"`
	shared  string
}

type guidanceSkip struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type guidanceWarning struct {
	Code    string   `json:"code"` // also_read_by or over_limit
	Path    string   `json:"path"`
	Targets []string `json:"targets,omitempty"`
	Target  string   `json:"target,omitempty"`
	Limit   int      `json:"limit,omitempty"`
	Chars   int      `json:"chars,omitempty"`
}

type guidancePlan struct {
	Token    string            `json:"token"`
	Changes  []guidanceChange  `json:"changes"`
	Skipped  []guidanceSkip    `json:"skipped"`
	Warnings []guidanceWarning `json:"warnings"`
}

// guidanceSource is one file in a target's read chain.
type guidanceSource struct {
	read   string // the path whose content the target gets
	write  string // the real file to change for it
	shared string // the shared instruction file it is, if any
	live   bool   // false when the shared file is attached but not synced
}

// guidanceSite is a target's chain and where a new block would go.
type guidanceSite struct {
	guidanceTarget
	sources  []guidanceSource
	dest     int    // the source that receives a new block
	blocks   int    // live sources holding an intact block
	shared   string // shared file of File, for syncing copies
	maxChars int
}

// memoryInstructions returns the guidance block of the current scope in the
// given update mode. Callers hold s.mu.
func (s *Server) memoryInstructions(root, mode string) string {
	if s.IsProjectMode() {
		return memory.Instructions(root, s.projectRoot, mode)
	}
	return memory.Instructions(root, "", mode)
}

// memoryInstructionsByMode returns the block of every update mode, for copying.
func (s *Server) memoryInstructionsByMode(root string) map[string]string {
	return map[string]string{memory.ModePassive: s.memoryInstructions(root, memory.ModePassive), memory.ModeActive: s.memoryInstructions(root, memory.ModeActive)}
}

// realFile returns the file a write to path changes: a link's destination.
func realFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}
	return filepath.EvalSymlinks(path)
}

// guidanceSites resolves every target's read chain. Callers hold s.mu.
func (s *Server) guidanceSites(root string) []guidanceSite {
	out := []guidanceSite{}
	if s.IsProjectMode() {
		agents := filepath.Join(s.projectRoot, instructions.AgentsFile)
		for _, reach := range s.projectReaches() {
			it, _ := config.TargetInstructions(reach.Target, s.cfg.Targets[reach.Target], true)
			own := filepath.Join(s.projectRoot, it.Path)
			site := guidanceSite{guidanceTarget: guidanceTarget{Name: reach.Target}, maxChars: it.MaxChars}
			if reach.Reads {
				// AGENTS.md may itself link elsewhere: change, preview and back up its real file.
				if write, err := realFile(agents); err == nil {
					site.sources = append(site.sources, guidanceSource{read: agents, write: write, live: true})
				} else {
					site.State, site.Detail, site.File = "broken", "unreadable", agents
				}
			}
			if reach.How != instructions.ReachDirect && reach.How != instructions.ReachLink {
				if write, err := realFile(own); err == nil {
					site.sources = append(site.sources, guidanceSource{read: own, write: write, live: true})
				} else {
					site.State, site.Detail, site.File = "broken", "unreadable", own
				}
			}
			out = append(out, s.resolveSite(site, root))
		}
		return out
	}
	for _, t := range s.instructionTargets() {
		site := guidanceSite{guidanceTarget: guidanceTarget{Name: t.Name}, maxChars: t.MaxChars}
		own := guidanceSource{read: t.Path, live: true}
		for _, a := range t.Assigned {
			extra, ok := s.sharedExtra(a.Name)
			if !ok {
				continue
			}
			src := guidanceSource{read: filepath.Join(s.extrasSourceDir(extra), extra.File), shared: a.Name, live: a.Status == "synced"}
			src.write = src.read
			if a.Mode != "import" {
				// A link or copy is the target's own file; edit the source.
				own.write, own.shared, own.live = src.write, a.Name, src.live
				if !src.live {
					own = src
				}
				continue
			}
			site.sources = append(site.sources, src)
		}
		if own.write == "" {
			write, err := realFile(t.Path)
			if err != nil {
				site.State, site.Detail, site.File = "broken", "unreadable", t.Path
			}
			own.write = write
		}
		site.sources = append([]guidanceSource{own}, site.sources...)
		// A tool that imports shared files gets the block in the first synced one.
		for i, src := range site.sources[1:] {
			if src.live {
				site.dest = i + 1
				break
			}
		}
		out = append(out, s.resolveSite(site, root))
	}
	return out
}

// resolveSite sets the state from the chain: a current block anywhere the
// target reads wins; then a block it cannot be trusted with; then an
// outdated one. Without a block, site.dest receives it. Each block is checked
// against the text of the mode it records; blocks of different modes in one
// chain contradict each other, and rewriting one file would not fix that.
func (s *Server) resolveSite(site guidanceSite, root string) guidanceSite {
	if site.State != "" {
		return site
	}
	rank := map[string]int{memory.StateConfigured: 4, memory.StateModified: 3, memory.StateMalformed: 3, memory.StateOutdated: 2}
	best, bestRank := -1, 0
	modes := map[string]bool{}
	for i, src := range site.sources {
		data, err := os.ReadFile(src.read)
		if err != nil && !os.IsNotExist(err) {
			site.State, site.Detail, site.File = "broken", "unreadable", src.write
			return site
		}
		// JSON previews cannot show other encodings byte for byte, so such a
		// file is never inspected or rewritten.
		if !utf8.Valid(data) {
			site.State, site.Detail, site.File = "broken", "unsupported", src.write
			return site
		}
		mode := memory.Mode(string(data), guidanceScope(s))
		state := memory.Inspect(string(data), s.memoryInstructions(root, mode))
		if !src.live {
			if state != memory.StateUnconfigured {
				site.State, site.Detail, site.File, site.shared = "broken", "not_synced", src.write, src.shared
				return site
			}
			continue
		}
		if state != memory.StateUnconfigured && state != memory.StateMalformed {
			modes[mode] = true
			site.blocks++
		}
		if rank[state] > bestRank {
			best, bestRank = i, rank[state]
			site.Detail, site.Mode = state, mode
		}
	}
	if best >= 0 {
		src := site.sources[best]
		site.File, site.shared = src.write, src.shared
		if len(modes) > 1 {
			site.State, site.Detail, site.Mode = "broken", "mixed_modes", ""
			return site
		}
		switch site.Detail {
		case memory.StateConfigured:
			site.State, site.Detail = "configured", ""
		case memory.StateOutdated:
			site.State, site.Detail = "outdated", ""
		default:
			site.State = "broken"
		}
		return site
	}
	site.State = "unconfigured"
	dest := site.sources[site.dest]
	if !dest.live {
		site.State, site.Detail = "broken", "not_synced"
	}
	site.File, site.shared = dest.write, dest.shared
	return site
}

// planGuidance builds the changes for the named targets, each in the mode
// modes gives it; a target left out keeps its block's mode, or gets passive.
// Targets reading one file share its block, so they must share a mode. The
// token hashes everything the plan depends on, so apply can reject a plan
// whose files or config changed after review. Callers hold s.mu.
func (s *Server) planGuidance(names []string, modes map[string]string) (guidancePlan, error) {
	root, err := s.memoryRoot()
	if err != nil {
		return guidancePlan{}, err
	}
	sites := s.guidanceSites(root)
	byName := map[string]guidanceSite{}
	for _, site := range sites {
		byName[site.Name] = site
	}
	names = slices.Compact(slices.Sorted(slices.Values(names)))
	if len(names) == 0 {
		return guidancePlan{}, errors.New("choose at least one target")
	}
	plan := guidancePlan{Changes: []guidanceChange{}, Skipped: []guidanceSkip{}, Warnings: []guidanceWarning{}}
	index := map[string]int{}
	chosen, fileModes := map[string]string{}, map[string]string{}
	for _, name := range names {
		site, ok := byName[name]
		if !ok {
			return guidancePlan{}, errors.New("target has no instruction file: " + name)
		}
		mode, err := memory.ParseMode(modes[name])
		if err != nil {
			return guidancePlan{}, err
		}
		if modes[name] == "" && site.Mode != "" {
			mode = site.Mode
		}
		if other, ok := fileModes[site.File]; ok && other != mode && site.File != "" {
			return guidancePlan{}, errors.New("targets reading " + site.File + " share its guidance and must use the same mode")
		}
		fileModes[site.File], chosen[name] = mode, mode
		switch {
		case site.State == "configured" && site.Mode == mode:
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, "configured"})
			continue
		case site.State == "broken":
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, site.Detail})
			continue
		case site.Mode != mode && site.blocks > 1:
			// Rewriting only site.File would leave the other blocks in the old mode.
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, "multiple_blocks"})
			continue
		}
		if i, ok := index[site.File]; ok {
			plan.Changes[i].Targets = append(plan.Changes[i].Targets, name)
			continue
		}
		want := s.memoryInstructions(root, mode)
		data, readErr := os.ReadFile(site.File)
		if readErr != nil && !os.IsNotExist(readErr) {
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, "unreadable"})
			continue
		}
		if !utf8.Valid(data) {
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, "unsupported"})
			continue
		}
		after, err := memory.Apply(string(data), want)
		if err != nil {
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, memory.Inspect(string(data), want)})
			continue
		}
		if after == string(data) {
			plan.Skipped = append(plan.Skipped, guidanceSkip{name, "configured"})
			continue
		}
		index[site.File] = len(plan.Changes)
		plan.Changes = append(plan.Changes, guidanceChange{Path: site.File, Before: string(data), After: after, Targets: []string{name}, Created: os.IsNotExist(readErr), shared: site.shared})
	}
	for _, c := range plan.Changes {
		// Every reader of the changed file is affected, selected or not.
		var others []string
		chars := utf8.RuneCountInString(c.After)
		for _, site := range sites {
			if !slices.ContainsFunc(site.sources, func(src guidanceSource) bool { return src.write == c.Path }) {
				continue
			}
			if !slices.Contains(c.Targets, site.Name) {
				others = append(others, site.Name)
			}
			if site.maxChars > 0 && chars > site.maxChars {
				plan.Warnings = append(plan.Warnings, guidanceWarning{Code: "over_limit", Path: c.Path, Target: site.Name, Limit: site.maxChars, Chars: chars})
			}
		}
		if len(others) > 0 {
			sort.Strings(others)
			plan.Warnings = append(plan.Warnings, guidanceWarning{Code: "also_read_by", Path: c.Path, Targets: others})
		}
	}
	// Bind the routing too: a mode or assignment change can leave the
	// reviewed text equal while changing which files the block reaches.
	type route struct {
		Name, State, Detail, File, Shared string
		Sources                           [][4]string
	}
	routes := make([]route, 0, len(sites))
	for _, site := range sites {
		r := route{site.Name, site.State, site.Detail, site.File, site.shared, nil}
		for _, src := range site.sources {
			r.Sources = append(r.Sources, [4]string{src.read, src.write, src.shared, strconv.FormatBool(src.live)})
		}
		routes = append(routes, r)
	}
	data, err := json.Marshal(struct {
		Root    string
		Names   []string
		Modes   map[string]string
		Plan    guidancePlan
		Routes  []route
		Extras  []config.ExtraConfig
		Targets any
	}{root, names, chosen, plan, routes, s.extrasConfig(), s.cfg.Targets})
	if err != nil {
		return guidancePlan{}, err
	}
	sum := sha256.Sum256(data)
	plan.Token = hex.EncodeToString(sum[:])
	return plan, nil
}

// handleMemoryGuidance — GET /api/extras/memory/guidance
func (s *Server) handleMemoryGuidance(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	root, err := s.memoryRoot()
	if err != nil {
		writeMemoryError(w, err)
		return
	}
	writeJSON(w, map[string]any{"scope": guidanceScope(s), "instructions": s.memoryInstructionsByMode(root), "targets": guidanceTargets(s.guidanceSites(root))})
}

func guidanceScope(s *Server) string {
	if s.IsProjectMode() {
		return memory.ScopeProject
	}
	return memory.ScopeGlobal
}

func guidanceTargets(sites []guidanceSite) []guidanceTarget {
	out := make([]guidanceTarget, 0, len(sites))
	for _, site := range sites {
		out = append(out, site.guidanceTarget)
	}
	return out
}

type guidanceRequest struct {
	Targets []string          `json:"targets"`
	Modes   map[string]string `json:"modes"` // target name to passive or active
	Token   string            `json:"token"`
}

func decodeGuidanceRequest(w http.ResponseWriter, r *http.Request) (guidanceRequest, bool) {
	var body guidanceRequest
	if err := decodeJSON(w, r, &body, defaultJSONBodyLimit); err != nil {
		if !errors.Is(err, errBodyTooLarge) {
			writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", "invalid JSON body", map[string]string{})
		}
		return body, false
	}
	return body, true
}

// handleMemoryGuidancePlan — POST /api/extras/memory/guidance/plan
func (s *Server) handleMemoryGuidancePlan(w http.ResponseWriter, r *http.Request) {
	body, ok := decodeGuidanceRequest(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	plan, err := s.planGuidance(body.Targets, body.Modes)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", err.Error(), map[string]string{})
		return
	}
	writeJSON(w, plan)
}

// handleMemoryGuidanceApply — POST /api/extras/memory/guidance/apply
// Applies a reviewed plan only if recomputing it gives the same token. Each
// existing file is backed up before it is changed; copies of a changed shared
// file are synced.
func (s *Server) handleMemoryGuidanceApply(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	body, ok := decodeGuidanceRequest(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.planGuidance(body.Targets, body.Modes)
	if err != nil {
		writeCodedError(w, http.StatusBadRequest, "memory_guidance_invalid", err.Error(), map[string]string{})
		return
	}
	if body.Token == "" || body.Token != plan.Token {
		writeCodedError(w, http.StatusConflict, "memory_guidance_stale", errGuidanceStale.Error(), map[string]string{})
		return
	}
	type failure struct {
		Path  string `json:"path"`
		Error string `json:"error"`
		Code  string `json:"code,omitempty"`
	}
	applied, failures := []string{}, []failure{}
	for _, c := range plan.Changes {
		if err := writeGuidanceChange(c); err != nil {
			code := ""
			if errors.Is(err, errGuidanceStale) {
				code = "memory_guidance_stale"
			}
			failures = append(failures, failure{Path: c.Path, Error: err.Error(), Code: code})
			continue
		}
		applied = append(applied, c.Path)
		if extra, ok := s.sharedExtra(c.shared); ok && c.shared != "" {
			for _, result := range s.syncSharedCopies(extra) {
				if result.Error != "" {
					failures = append(failures, failure{Path: result.Target, Error: result.Error})
				}
			}
		}
	}
	status, msg := "ok", ""
	if len(failures) > 0 {
		status, msg = "partial", failures[0].Path+": "+failures[0].Error
	}
	s.writeOpsLog("memory-guidance", status, start, map[string]any{"targets": body.Targets, "modes": body.Modes, "files": applied, "scope": "ui"}, msg)
	root, _ := s.memoryRoot()
	writeJSON(w, map[string]any{"success": len(failures) == 0, "applied": applied, "errors": failures, "targets": guidanceTargets(s.guidanceSites(root))})
}

var errGuidanceStale = errors.New("instruction files changed since the preview; review the changes again")

// writeGuidanceChange rechecks each file as earlier writes and syncs may take time.
func writeGuidanceChange(c guidanceChange) error {
	if err := checkGuidanceChange(c); err != nil {
		return err
	}
	if !c.Created {
		if err := syncpkg.BackupFile(c.Path, syncpkg.BackupReasonEdit); err != nil {
			return err
		}
	}
	if err := checkGuidanceChange(c); err != nil {
		return err
	}
	return commitGuidanceChange(c)
}

func checkGuidanceChange(c guidanceChange) error {
	data, err := os.ReadFile(c.Path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if c.Created != os.IsNotExist(err) || string(data) != c.Before {
		return errGuidanceStale
	}
	return nil
}

// commitGuidanceChange writes a file after its final review check.
func commitGuidanceChange(c guidanceChange) error {
	if !c.Created {
		return instructions.WriteFile(c.Path, c.After)
	}
	if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(c.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if os.IsExist(err) {
		return errGuidanceStale
	}
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			os.Remove(c.Path)
		}
	}()
	_, writeErr := f.WriteString(c.After)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	committed = true
	return nil
}
