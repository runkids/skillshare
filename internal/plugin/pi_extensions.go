package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The extensions of a Pi target (issue #342): which package extensions its settings
// select, read statically. Global, account and project targets can change the
// selection of one extension at a time when their own Pi is PiMinVersion or later. A
// project change is saved to the project's .pi/settings.json only; whether Pi
// trusts the project, and so reads that file, stays Pi's decision.

// PiMinVersion is the oldest Pi that scripts/pi/version-matrix.sh installed and passed
// (scripts/pi/version-evidence.json): the package contract against dist/core and the
// CLI's bundle, and the native lock test. Older or unparsable versions are read-only.
const PiMinVersion = "0.99.2"

// piVersionSupported reports a plain X.Y.Z version at or above PiMinVersion.
func piVersionSupported(version string) bool {
	parts := func(v string) []int {
		fields := strings.Split(v, ".")
		if len(fields) != 3 {
			return nil
		}
		out := make([]int, 3)
		for i, field := range fields {
			n, err := strconv.Atoi(field)
			if err != nil || n < 0 {
				return nil
			}
			out[i] = n
		}
		return out
	}
	got := parts(version)
	return got != nil && slices.Compare(got, parts(PiMinVersion)) >= 0
}

// Read-only reasons, translated by the dashboard as piExtensions.readOnly.<key>.
const (
	piReadOnlyFork        = "fork"
	piReadOnlyNoCLI       = "noCli"
	piReadOnlyUnsupported = "unsupportedVersion"
	piReadOnlySettings    = "settings"
)

type PiExtensionsView struct {
	Target             string               `json:"target"`
	Scope              string               `json:"scope"` // global, account or project
	SettingsPath       string               `json:"settingsPath"`
	GlobalSettingsPath string               `json:"globalSettingsPath,omitempty"`
	Version            string               `json:"version"`
	MinVersion         string               `json:"minVersion"`
	Editable           bool                 `json:"editable"`
	ReadOnly           string               `json:"readOnly,omitempty"`
	Problem            string               `json:"problem,omitempty"` // the settings file cannot be used
	Revision           string               `json:"revision"`
	Packages           []PiExtensionPackage `json:"packages"`
	Folders            []PiExtensionFolder  `json:"folders"`
	Trust              *PiTrustHints        `json:"trust,omitempty"`
}

// PiExtensionPackage is one entry of settings.json "packages". Rules is its
// "extensions" list as written, null when the entry has none.
type PiExtensionPackage struct {
	Index    int    `json:"index"`
	Source   string `json:"source"`
	Identity string `json:"identity"`
	Kind     string `json:"kind"`  // npm, git, local, unknown
	Form     string `json:"form"`  // string or object
	Scope    string `json:"scope"` // global or project: which file the entry is in
	Shape    string `json:"shape,omitempty"`
	Install  string `json:"install"` // present, missing or unknown
	Problem  string `json:"problem,omitempty"`
	// ReadOnly says why rows that Skillshare can read can't be switched here:
	// singleFile (Pi ignores filters) or otherResources (a first rule turns the
	// string entry into an object, and Skillshare can't show that this leaves the
	// package's skills, prompts and themes as they are).
	ReadOnly    string           `json:"readOnly,omitempty"`
	Rules       []string         `json:"rules"`
	GlobalRules []string         `json:"globalRules,omitempty"`
	OtherKeys   []string         `json:"otherKeys"`
	ManagedBy   string           `json:"managedBy,omitempty"`
	Rows        []PiExtensionRow `json:"rows"`
}

// PiExtensionRow is one extension. Selection is what the settings select (for a
// project: configured, if Pi trusts it); whether Pi loaded it is never known here.
type PiExtensionRow struct {
	Path      string   `json:"path"`
	File      string   `json:"file"`      // present or missing
	Selection string   `json:"selection"` // loads, skipped, unknown, or none for a missing file
	Origin    string   `json:"origin"`    // default, rule, glob, emptyList, file, inherited, project, unnamed
	Rule      string   `json:"rule,omitempty"`
	Globs     []string `json:"globs,omitempty"`
	Editable  bool     `json:"editable"`
}

// PiExtensionFolder is extensions Pi finds outside packages, always read-only.
type PiExtensionFolder struct {
	Path    string                 `json:"path"`
	Kind    string                 `json:"kind"`            // folder, or settings for paths listed in settings
	Scope   string                 `json:"scope,omitempty"` // project view only: global or project
	Problem string                 `json:"problem,omitempty"`
	Rows    []PiExtensionFolderRow `json:"rows"`
}

type PiExtensionFolderRow struct {
	Path       string `json:"path"`
	File       string `json:"file"`
	Selection  string `json:"selection"`
	Provenance string `json:"provenance"` // native or extras
	Extra      string `json:"extra,omitempty"`
}

// PiTrustHints are what Pi saved, shown as hints only: an extension handler or a
// session-only choice can decide trust differently.
type PiTrustHints struct {
	Saved   string `json:"saved"`   // trusted, untrusted, none, unknown
	Default string `json:"default"` // always, never, ask, unset, unknown
}

// piEntry is one parsed "packages" entry.
type piEntry struct {
	index         int
	source        string
	object        bool
	fields        map[string]json.RawMessage
	autoloadFalse bool
	rules         []string
	hasRules      bool
	problem       string
	// badSource: the source can't be read as Pi reads it, so the entry may be any package.
	badSource bool
}

type piSettings struct {
	path    string
	raw     []byte
	exists  bool
	problem string
	body    map[string]json.RawMessage
	entries []piEntry
}

// readPiSettings reads settings.json as Pi does, strictly: Pi refuses JSON with
// comments, so Skillshare never treats such a file as usable.
func readPiSettings(root, file string) *piSettings {
	st := &piSettings{path: file, body: map[string]json.RawMessage{}}
	if err := noSymlink(root, file); err != nil {
		st.problem = "symlink"
		return st
	}
	raw, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return st
	}
	if err != nil {
		st.problem = piProblemUnreadable
		return st
	}
	st.raw, st.exists = raw, true
	if len(raw) == 0 {
		return st
	}
	switch {
	case bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}):
		st.problem = "bom"
	case !json.Valid(raw):
		st.problem = "invalidJson"
	case jsonDuplicateKeys(raw):
		st.problem = "duplicateKey"
	case json.Unmarshal(raw, &st.body) != nil || st.body == nil:
		st.problem = "notObject"
	}
	if st.problem != "" {
		st.body = map[string]json.RawMessage{}
		return st
	}
	var list []json.RawMessage
	if field, ok := st.body["packages"]; ok && (json.Unmarshal(field, &list) != nil || list == nil) {
		st.problem = "packagesInvalid"
		return st
	}
	for i, raw := range list {
		st.entries = append(st.entries, parsePiEntry(i, raw))
	}
	return st
}

func parsePiEntry(i int, raw json.RawMessage) piEntry {
	e := piEntry{index: i}
	if json.Unmarshal(raw, &e.source) == nil {
		if !piSourceOK(raw, e.source) {
			e.problem, e.badSource = "unsupportedEntry", true
		}
		return e
	}
	e.object = true
	if json.Unmarshal(raw, &e.fields) != nil || e.fields == nil || json.Unmarshal(e.fields["source"], &e.source) != nil || !piSourceOK(e.fields["source"], e.source) {
		e.problem, e.badSource = "unsupportedEntry", true
		return e
	}
	// Native Pi uses strict === false; decoding null into bool also yields false.
	if a, ok := e.fields["autoload"]; ok && bytes.Equal(bytes.TrimSpace(a), []byte("false")) {
		e.autoloadFalse = true
	}
	if r, ok := e.fields["extensions"]; ok {
		e.hasRules = true
		if json.Unmarshal(r, &e.rules) != nil || e.rules == nil || !piLossless(r) {
			e.rules, e.problem = nil, "unsupportedEntry"
		}
	}
	return e
}

// piSourceOK is false for a source Pi can't use (encoding/json reads null as "")
// or one Go would not read as Pi does.
func piSourceOK(raw json.RawMessage, source string) bool {
	return strings.TrimSpace(source) != "" && piLossless(raw)
}

// piLossless reports whether Go decodes the strings of a JSON value as Pi
// (JavaScript) does. Go turns an unpaired UTF-16 surrogate escape, and invalid
// UTF-8, into U+FFFD, while JavaScript keeps the surrogate, so a rule could read
// as naming a different file. raw must be valid JSON.
func piLossless(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	hex := func(i int) uint64 {
		if i+4 > len(raw) {
			return 0
		}
		n, err := strconv.ParseUint(string(raw[i:i+4]), 16, 32)
		if err != nil {
			return 0
		}
		return n
	}
	in := false
	for i := 0; i < len(raw); i++ {
		switch {
		case raw[i] == '"':
			in = !in
		case in && raw[i] == '\\' && i+1 < len(raw) && raw[i+1] == 'u':
			switch r := hex(i + 2); {
			case r >= 0xD800 && r <= 0xDBFF:
				if i+12 > len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
					return false
				}
				if lo := hex(i + 8); lo < 0xDC00 || lo > 0xDFFF {
					return false
				}
				i += 11
			case r >= 0xDC00 && r <= 0xDFFF:
				return false
			default:
				i += 5
			}
		case in && raw[i] == '\\':
			i++ // the escaped character, which may be a backslash
		}
	}
	return true
}

// otherKeys names the entry's keys besides source and extensions, never their values.
func (e piEntry) otherKeys() []string {
	keys := []string{}
	for k := range e.fields {
		if k != "source" && k != "extensions" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

func jsonDuplicateKeys(data []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func() bool
	walk = func() bool {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch tok {
		case json.Delim('{'):
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return false
				}
				k, _ := key.(string)
				if seen[k] {
					return true
				}
				seen[k] = true
				if walk() {
					return true
				}
			}
			_, _ = dec.Token()
		case json.Delim('['):
			for dec.More() {
				if walk() {
					return true
				}
			}
			_, _ = dec.Token()
		}
		return false
	}
	return walk()
}

// piPackage locates one entry's files and evaluates it against any rule list.
type piPackage struct {
	src     piSource
	install string
	problem string
	single  bool // a local source that is one file: Pi ignores its filters
	// convertLoadsOthers: a string entry can't be shown to take a rule without
	// changing other resources (see piLayout.conversionLoadsOthers).
	convertLoadsOthers bool
	abs                string // package root, slash separated, for Pi's absolute-path matching
	// The file sets an evaluation draws from (see piLayout.defaults and base).
	defaultsString, defaultsObject, base []string
}

func openPiPackage(source, agentDir, projectDir, scope string) *piPackage {
	p := &piPackage{src: resolvePiSource(source, agentDir, projectDir, scope), install: "unknown"}
	if p.src.kind == "unknown" {
		p.problem = "sourceUnknown"
		return p
	}
	info, err := os.Stat(p.src.install)
	if err != nil {
		if p.src.kind == "npm" && scope == "user" {
			// Native Pi may fall back to a legacy global npm/pnpm root. Without
			// resolving that root, absence here does not establish non-installation.
			p.problem = "sourceUnknown"
			return p
		}
		p.install, p.problem = "missing", "notInstalled"
		return p
	}
	p.install, p.abs = "present", filepath.ToSlash(p.src.install)
	if info.Mode().IsRegular() && p.src.kind == "local" {
		p.single = true
		return p
	}
	if !info.IsDir() {
		p.install, p.problem = "unknown", piProblemUnreadable
		return p
	}
	l, err := openPiLayout(p.src.install)
	if err != nil {
		p.problem = piProblemUnreadable
		return p
	}
	defer l.Close()
	p.defaultsString, p.defaultsObject, p.base = l.defaults(false), l.defaults(true), l.base()
	p.convertLoadsOthers = l.conversionLoadsOthers()
	p.problem = l.problem
	return p
}

// readOnly is why an entry of this form can't be switched here, "" if it can.
func (p *piPackage) readOnly(object bool) string {
	switch {
	case p.single:
		return "singleFile"
	case p.convertLoadsOthers && !object:
		return "otherResources"
	}
	return ""
}

type piRowState struct {
	rel     string
	present bool
	eval    piEvaluation
	origin  string
	// inherited: a project override leaves this path to the global entry.
	inherited bool
}

// evaluate is the selection of an entry with this form and rule list, as Pi
// applies it in global scope.
func (p *piPackage) evaluate(object, hasRules bool, rules []string) []piRowState {
	if p.single {
		return []piRowState{{rel: path.Base(p.abs), present: true, eval: piEvaluation{state: piOn}, origin: "file"}}
	}
	rows := []piRowState{}
	switch {
	case !hasRules:
		files := p.defaultsString
		if object {
			files = p.defaultsObject
		}
		for _, f := range files {
			rows = append(rows, piRowState{rel: f, present: true, eval: piEvaluation{state: piOn}, origin: "default"})
		}
	case len(rules) == 0:
		for _, f := range p.base {
			rows = append(rows, piRowState{rel: f, present: true, eval: piEvaluation{state: piOff}, origin: "emptyList"})
		}
	default:
		for _, f := range p.base {
			e := piEvaluate(rules, f, path.Join(p.abs, f))
			origin := "default"
			if e.exact != "" {
				origin = "rule"
			} else if len(e.globs) > 0 {
				origin = "glob"
			}
			rows = append(rows, piRowState{rel: f, present: true, eval: e, origin: origin})
		}
	}
	// An exact rule for a file the package no longer has still sits in the list.
	for _, rule := range rules {
		if !strings.HasPrefix(rule, "+") && !strings.HasPrefix(rule, "-") {
			continue
		}
		target := strings.TrimPrefix(rule[1:], "./")
		if slices.ContainsFunc(rows, func(r piRowState) bool { return r.rel == target || path.Join(p.abs, r.rel) == target }) {
			continue
		}
		rows = append(rows, piRowState{rel: target, eval: piEvaluation{state: piUnknown, exact: rule}, origin: "rule"})
	}
	return rows
}

func (r piRowState) row(editable bool) PiExtensionRow {
	row := PiExtensionRow{Path: r.rel, File: "present", Selection: r.eval.state.String(), Origin: r.origin, Rule: r.eval.exact, Globs: r.eval.globs}
	if !r.present {
		row.File, row.Selection = "missing", "none"
	}
	// A switch needs a known state; a missing file can only lose its rule. Under []
	// any added rule would change every extension of the package.
	row.Editable = editable && r.origin != "emptyList" && (r.present && r.eval.state != piUnknown || !r.present && r.eval.exact != "")
	return row
}

// piTargetState is everything a preview or apply of a global target needs.
type piTargetState struct {
	view     *PiExtensionsView
	settings *piSettings
	agentDir string
	packages map[int]*piPackage
}

func piCLIIsNative(cli, goos string) bool {
	if goos == "windows" {
		// Windows paths and launcher names are case-insensitive. Only accept the
		// official executable/shim names, not arbitrary or stacked suffixes.
		name := strings.ToLower(path.Base(strings.ReplaceAll(cli, `\`, "/")))
		return name == "pi" || name == "pi.cmd" || name == "pi.exe"
	}
	return filepath.Base(cli) == "pi"
}

func (s *Service) piGate(ctx context.Context, target string) (version, readOnly string) {
	if a, ok := s.account(target); ok && a.CLI != "" && !piCLIIsNative(a.CLI, runtime.GOOS) {
		// Known unsupported by name: never run it just to read its version.
		return "", piReadOnlyFork
	}
	out, err := s.run(ctx, target, "--version")
	version = strings.TrimSpace(string(out))
	switch {
	case err != nil:
		return "", piReadOnlyNoCLI
	case !piVersionSupported(version):
		return version, piReadOnlyUnsupported
	}
	return version, ""
}

// PiExtensions reads the extensions of a Pi target: "pi" or a Pi account, in the
// project when ProjectRoot is set.
func (s *Service) PiExtensions(ctx context.Context, target string) (*PiExtensionsView, error) {
	if s.agentOf(target) != "pi" {
		return nil, errors.New("not a Pi target")
	}
	if s.ProjectRoot != "" {
		st, err := s.piProjectState(ctx, target)
		if err != nil {
			return nil, err
		}
		return st.view, nil
	}
	st, err := s.piGlobalState(ctx, target)
	if err != nil {
		return nil, err
	}
	return st.view, nil
}

func (s *Service) piGlobalState(ctx context.Context, target string) (*piTargetState, error) {
	agentDir, err := s.piAgentDir(target)
	if err != nil {
		return nil, err
	}
	settings := readPiSettings(agentDir, filepath.Join(agentDir, "settings.json"))
	version, readOnly := s.piGate(ctx, target)
	if readOnly == "" && settings.problem != "" {
		readOnly = piReadOnlySettings
	}
	scope := "global"
	if _, ok := s.account(target); ok {
		scope = "account"
	}
	v := &PiExtensionsView{
		Target: target, Scope: scope, SettingsPath: settings.path, Version: version, MinVersion: PiMinVersion,
		Editable: readOnly == "", ReadOnly: readOnly, Problem: settings.problem, Revision: hash(settings.raw),
		Packages: []PiExtensionPackage{}, Folders: []PiExtensionFolder{},
	}
	st := &piTargetState{view: v, settings: settings, agentDir: agentDir, packages: map[int]*piPackage{}}
	managed := s.piManaged(target)
	seen := map[string]bool{}
	unresolved := false
	for _, e := range settings.entries {
		pkg := PiExtensionPackage{Index: e.index, Source: redactSource(e.source), Form: "string", Scope: "global", Install: "unknown", Rules: nil, OtherKeys: e.otherKeys(), Rows: []PiExtensionRow{}}
		if e.object {
			pkg.Form = "object"
		}
		if e.hasRules {
			pkg.Rules = e.rules
		}
		if e.problem != "" {
			pkg.Problem = e.problem
			// Pi still reads the entry, so it is the one that counts for its package.
			if !e.badSource {
				id := resolvePiSource(e.source, agentDir, "", "user").identity
				seen[id] = true
				unresolved = unresolved || id == ""
			} else {
				unresolved = true
			}
			v.Packages = append(v.Packages, pkg)
			continue
		}
		p := openPiPackage(e.source, agentDir, "", "user")
		st.packages[e.index] = p
		pkg.Kind, pkg.Identity, pkg.Install, pkg.Problem = p.src.kind, redactSource(p.src.identity), p.install, p.problem
		pkg.ManagedBy = managed[strings.TrimSpace(e.source)]
		if pkg.ManagedBy == "" && p.src.kind == "local" {
			pkg.ManagedBy = managed[p.src.install]
		}
		switch {
		case p.src.identity != "" && seen[p.src.identity]:
			// Pi keeps the first entry of a package and ignores the rest.
			pkg.Problem = "duplicate"
		case unresolved:
			// An earlier unknown identity may be the first entry of this package.
			pkg.Problem = "sourceUnknown"
		case e.autoloadFalse:
			pkg.Problem = "autoloadGlobal"
		}
		unresolved = unresolved || p.src.identity == ""
		seen[p.src.identity] = true
		pkg.ReadOnly = p.readOnly(e.object)
		if pkg.Problem == "" || pkg.Problem == "notInstalled" && len(e.rules) > 0 {
			editable := v.Editable && pkg.Problem == "" && pkg.ReadOnly == ""
			for _, r := range p.evaluate(e.object, e.hasRules, e.rules) {
				// Rules for an uninstalled package are listed so they can be read, not changed.
				if pkg.Problem != "" && r.present {
					continue
				}
				pkg.Rows = append(pkg.Rows, r.row(editable))
			}
		}
		v.Packages = append(v.Packages, pkg)
	}
	v.Folders = append(v.Folders, s.piFolders(agentDir, settings)...)
	return st, nil
}

// piManaged maps the source of each plugin Skillshare installs into target to its name.
func (s *Service) piManaged(target string) map[string]string {
	result := map[string]string{}
	d, err := s.load()
	if err != nil {
		return result
	}
	for name, p := range d.packages {
		if b, ok := p.Bindings[target]; ok && b.ID != "" {
			result[b.ID] = name
		}
	}
	return result
}

// piFolders lists the extensions Pi discovers in baseDir/extensions and the paths
// listed in the settings' own "extensions", which apply to them as overrides.
func (s *Service) piFolders(baseDir string, settings *piSettings) []PiExtensionFolder {
	var top []string
	if raw, ok := settings.body["extensions"]; ok && (json.Unmarshal(raw, &top) != nil || !piLossless(raw)) {
		return []PiExtensionFolder{{Path: filepath.Join(baseDir, "extensions"), Kind: "folder", Problem: "unsupportedEntry", Rows: []PiExtensionFolderRow{}}}
	}
	overrides, plain := []string{}, []string{}
	for _, r := range top {
		switch {
		case strings.HasPrefix(r, "+") || strings.HasPrefix(r, "-") || strings.HasPrefix(r, "!"):
			overrides = append(overrides, r)
		case !strings.ContainsAny(r, "*?"):
			plain = append(plain, r)
		}
	}
	selection := func(abs string, patterns []string) string {
		rel, err := filepath.Rel(baseDir, abs)
		if err != nil || len(patterns) == 0 {
			return piOn.String()
		}
		return piEvaluate(patterns, filepath.ToSlash(rel), filepath.ToSlash(abs)).state.String()
	}
	folder := PiExtensionFolder{Path: filepath.Join(baseDir, "extensions"), Kind: "folder", Rows: []PiExtensionFolderRow{}}
	if l, err := openPiLayout(baseDir); err == nil {
		l.follow = true
		for _, rel := range l.autoEntries("extensions") {
			abs := filepath.Join(baseDir, filepath.FromSlash(rel))
			row := PiExtensionFolderRow{Path: rel, File: "present", Selection: selection(abs, overrides), Provenance: "native"}
			if name := s.extraOf(baseDir, rel); name != "" {
				row.Provenance, row.Extra = "extras", name
			}
			folder.Rows = append(folder.Rows, row)
		}
		folder.Problem = l.problem
		l.Close()
	}
	folders := []PiExtensionFolder{folder}
	if len(plain) > 0 {
		listed := PiExtensionFolder{Path: settings.path, Kind: "settings", Rows: []PiExtensionFolderRow{}}
		patterns := []string{}
		for _, r := range top {
			if strings.HasPrefix(r, "+") || strings.HasPrefix(r, "-") || strings.HasPrefix(r, "!") || strings.ContainsAny(r, "*?") {
				patterns = append(patterns, r)
			}
		}
		for _, p := range plain {
			src := resolvePiSource(p, baseDir, baseDir, "user")
			row := PiExtensionFolderRow{Path: p, File: "missing", Selection: "none", Provenance: "native"}
			if src.kind == "local" {
				if _, err := os.Stat(src.install); err == nil {
					row.File = "present"
					row.Selection = selection(src.install, patterns)
					if info, _ := os.Stat(src.install); info != nil && info.IsDir() {
						row.Selection = piUnknown.String()
					}
				}
			}
			listed.Rows = append(listed.Rows, row)
		}
		folders = append(folders, listed)
	}
	return folders
}

// extraOf names the extra whose source a folder entry links into, "" for a native file.
func (s *Service) extraOf(baseDir, rel string) string {
	first, _, _ := strings.Cut(rel, "/")
	link := filepath.Join(baseDir, first)
	if first == "extensions" {
		next, _, _ := strings.Cut(strings.TrimPrefix(rel, "extensions/"), "/")
		link = filepath.Join(baseDir, "extensions", next)
	}
	dest, err := os.Readlink(link)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(link), dest)
	}
	for _, name := range slices.Sorted(maps.Keys(s.ExtrasSources)) {
		if r, err := filepath.Rel(s.ExtrasSources[name], dest); err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			return name
		}
	}
	return ""
}

// piDeltaStates is a project autoload:false entry: the paths its rules name are
// decided here, every other path keeps the global selection when there is one,
// and without one Pi doesn't load it at all.
func piDeltaStates(rules []string, p *piPackage, inherited []piRowState, hasBase bool) []piRowState {
	rows := []piRowState{}
	named := map[string]bool{}
	for _, f := range p.base {
		state, ok, rule := piEvaluateDelta(rules, f, path.Join(p.abs, f))
		if !ok {
			continue
		}
		named[f] = true
		rows = append(rows, piRowState{rel: f, present: true, eval: piEvaluation{state: state, exact: rule}, origin: "project"})
	}
	// Missing exact paths still belong to the project and can lose their rules.
	for _, rule := range rules {
		if !strings.HasPrefix(rule, "+") && !strings.HasPrefix(rule, "-") {
			continue
		}
		target := strings.TrimPrefix(rule[1:], "./")
		if slices.ContainsFunc(rows, func(r piRowState) bool { return r.rel == target || path.Join(p.abs, r.rel) == target }) {
			continue
		}
		named[target] = true
		rows = append(rows, piRowState{rel: target, eval: piEvaluation{state: piUnknown, exact: rule}, origin: "project"})
	}
	if hasBase {
		for _, r := range inherited {
			if !named[r.rel] {
				r.inherited = true
				rows = append(rows, r)
			}
		}
		return rows
	}
	for _, f := range p.base {
		if !named[f] {
			rows = append(rows, piRowState{rel: f, present: true, eval: piEvaluation{state: piOff}, origin: "unnamed"})
		}
	}
	return rows
}

func piDeltaRows(states []piRowState, editable bool) []PiExtensionRow {
	rows := []PiExtensionRow{}
	for _, r := range states {
		row := r.row(editable)
		if r.inherited {
			row.Origin = "inherited"
		}
		rows = append(rows, row)
	}
	return rows
}

// piTrust reads the decision Pi saved for root and its default, as hints.
func piTrust(agentDir, root string, global *piSettings) *PiTrustHints {
	hints := &PiTrustHints{Saved: "none", Default: "unset"}
	if raw, ok := global.body["defaultProjectTrust"]; ok {
		var d string
		hints.Default = "unknown"
		if json.Unmarshal(raw, &d) == nil && slices.Contains([]string{"always", "never", "ask"}, d) {
			hints.Default = d
		}
	}
	if global.problem != "" {
		hints.Default = "unknown"
	}
	file := filepath.Join(agentDir, "trust.json")
	f, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		return hints
	}
	if err != nil {
		hints.Saved = "unknown"
		return hints
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPreview))
	var decisions map[string]*bool
	if err != nil || json.Unmarshal(trimBOM(data), &decisions) != nil {
		hints.Saved = "unknown"
		return hints
	}
	dir := root
	if real, err := filepath.EvalSymlinks(root); err == nil {
		dir = real
	}
	for dir = filepath.Clean(dir); ; dir = filepath.Dir(dir) {
		if d := decisions[dir]; d != nil {
			hints.Saved = map[bool]string{true: "trusted", false: "untrusted"}[*d]
			return hints
		}
		if filepath.Dir(dir) == dir {
			return hints
		}
	}
}
