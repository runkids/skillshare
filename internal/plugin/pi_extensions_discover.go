package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Static discovery of a Pi package's extensions, mirroring package-manager.js in
// Pi 0.99.2 (collectPackageResources, collectManifestFiles, collectAutoExtensionEntries).
// It reads package.json files and directory listings only: no npm, no git, and
// no extension module is ever loaded. What it cannot reproduce exactly, such as
// ignore files or manifest globs, is reported as a problem and shown as unknown.

// Problem keys, translated by the dashboard as piExtensions.problem.<key>.
const (
	piProblemIgnoreFiles  = "ignoreFiles"
	piProblemManifestGlob = "manifestGlob"
	piProblemPathEscapes  = "pathEscapes"
	piProblemUnsafeLink   = "unsafeLink"
	piProblemManifestRule = "manifestRule"
	piProblemUnreadable   = "unreadable"
)

var piIgnoreFiles = []string{".gitignore", ".ignore", ".fdignore"}

// piLayout is what a package root declares, relative to the root, slash separated.
type piLayout struct {
	root *os.Root
	abs  string
	// follow reads through links that leave abs. Only Pi's own extension folders
	// use it, where Extras link their files in; packages stay inside their root.
	follow  bool
	problem string
	// manifest is true when package.json has a "pi" object; extensions holds its
	// "extensions" field when that is an array of strings.
	manifest      bool
	extensions    []string
	extensionsSet bool
	// declared holds the other resource fields the manifest sets (skills, prompts,
	// themes), each an array of strings as Pi's readPiManifest requires.
	declared map[string]bool
	// version is the root package.json "version", what Pi shows for the package.
	version string
}

func openPiLayout(abs string) (*piLayout, error) {
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	l := &piLayout{root: root, abs: abs}
	l.manifest, l.extensions, l.extensionsSet, l.declared = l.readManifest(".")
	return l, nil
}

func (l *piLayout) Close() { _ = l.root.Close() }

// piInstalledVersion is the version an installed package's package.json gives, "" when it can't be read.
func piInstalledVersion(install string) string {
	l, err := openPiLayout(install)
	if err != nil {
		return ""
	}
	defer l.Close()
	return l.version
}

func (l *piLayout) open(rel string) (*os.File, error) {
	if l.follow {
		return os.Open(filepath.Join(l.abs, filepath.FromSlash(rel)))
	}
	return l.root.Open(filepath.FromSlash(rel))
}

func (l *piLayout) fail(problem string) {
	if l.problem == "" {
		l.problem = problem
	}
}

// stat follows links inside the root; one that leaves it is unsafe.
func (l *piLayout) stat(rel string) (fs.FileInfo, bool) {
	var info fs.FileInfo
	var err error
	if l.follow {
		info, err = os.Stat(filepath.Join(l.abs, filepath.FromSlash(rel)))
	} else {
		info, err = l.root.Stat(filepath.FromSlash(rel))
	}
	if err == nil {
		return info, true
	}
	if !errors.Is(err, fs.ErrNotExist) {
		l.fail(piProblemUnsafeLink)
	}
	return nil, false
}

// readManifest is Pi's readPiManifest: anything unreadable or invalid is no manifest.
func (l *piLayout) readManifest(dir string) (bool, []string, bool, map[string]bool) {
	f, err := l.open(path.Join(dir, "package.json"))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			l.fail(piProblemUnsafeLink)
		}
		return false, nil, false, nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPreview+1))
	if err != nil || len(data) > maxPreview {
		l.fail(piProblemUnreadable)
		return false, nil, false, nil
	}
	// Version stays raw: one that is not a string must not cost the manifest Pi reads.
	var pkg struct {
		Pi      json.RawMessage `json:"pi"`
		Version json.RawMessage `json:"version"`
	}
	if json.Unmarshal(trimBOM(data), &pkg) != nil {
		return false, nil, false, nil
	}
	if dir == "." {
		_ = json.Unmarshal(pkg.Version, &l.version)
	}
	var pi map[string]json.RawMessage
	if json.Unmarshal(pkg.Pi, &pi) != nil || pi == nil {
		return false, nil, false, nil
	}
	declared := map[string]bool{}
	for _, kind := range []string{"skills", "prompts", "themes"} {
		var entries []string
		declared[kind] = json.Unmarshal(pi[kind], &entries) == nil && entries != nil
	}
	var ext []string
	if raw, ok := pi["extensions"]; ok && !piLossless(raw) {
		l.fail(piProblemManifestGlob) // Go would not read these paths as Pi does
	}
	if raw, ok := pi["extensions"]; ok && json.Unmarshal(raw, &ext) == nil && ext != nil {
		return true, ext, true, declared
	}
	return true, nil, false, declared
}

// conversionLoadsOthers reports whether turning a string entry into an object,
// which a first rule needs, can't be shown to leave skills, prompts and themes as
// they are. With a manifest, a string entry takes each of them only from its
// manifest field, while an object entry falls back to the convention folder for a
// field the manifest leaves out (Pi 0.99.2 collectDefaultResources). Any entry at
// that name, a link, or one that can't be checked is enough; which files Pi would
// load from it is not evaluated.
func (l *piLayout) conversionLoadsOthers() bool {
	if !l.manifest {
		return false
	}
	for _, kind := range []string{"skills", "prompts", "themes"} {
		if l.declared[kind] {
			continue
		}
		if _, err := l.root.Lstat(kind); !errors.Is(err, fs.ErrNotExist) {
			return true
		}
	}
	return false
}

// inside joins a declared path to dir, refusing absolute paths and escapes.
func (l *piLayout) inside(dir, entry string) (string, bool) {
	if path.IsAbs(entry) || filepath.IsAbs(entry) {
		l.fail(piProblemPathEscapes)
		return "", false
	}
	rel := path.Clean(path.Join(dir, filepath.ToSlash(entry)))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		l.fail(piProblemPathEscapes)
		return "", false
	}
	return rel, true
}

// entries is Pi's resolveExtensionEntries: a folder whose package.json lists
// existing extensions, or that has index.ts / index.js, is one extension.
func (l *piLayout) entries(dir string) ([]string, bool) {
	if _, ext, set, _ := l.readManifest(dir); set && len(ext) > 0 {
		found := []string{}
		for _, e := range ext {
			if rel, ok := l.inside(dir, e); ok {
				if _, exists := l.stat(rel); exists {
					found = append(found, rel)
				}
			}
		}
		if len(found) > 0 {
			return found, true
		}
	}
	for _, index := range []string{"index.ts", "index.js"} {
		if _, ok := l.stat(path.Join(dir, index)); ok {
			return []string{path.Join(dir, index)}, true
		}
	}
	return nil, false
}

// autoEntries is Pi's collectAutoExtensionEntries for one folder.
func (l *piLayout) autoEntries(dir string) []string {
	if info, ok := l.stat(dir); !ok || !info.IsDir() {
		return nil
	}
	if found, ok := l.entries(dir); ok {
		return found
	}
	for _, name := range piIgnoreFiles {
		if _, ok := l.stat(path.Join(dir, name)); ok {
			// Pi applies gitignore rules here; Skillshare does not reproduce them.
			l.fail(piProblemIgnoreFiles)
			return nil
		}
	}
	f, err := l.open(dir)
	if err != nil {
		l.fail(piProblemUnreadable)
		return nil
	}
	defer f.Close()
	list, err := f.ReadDir(-1)
	if err != nil {
		l.fail(piProblemUnreadable)
		return nil
	}
	found := []string{}
	for _, entry := range list {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		rel := path.Join(dir, name)
		info, ok := l.stat(rel)
		if !ok {
			continue
		}
		if info.Mode().IsRegular() && (strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".js")) {
			found = append(found, rel)
		} else if info.IsDir() {
			sub, _ := l.entries(rel)
			found = append(found, sub...)
		}
	}
	slices.Sort(found)
	return found
}

// manifestFiles is Pi's collectManifestFiles over the manifest's extensions:
// the declared paths, narrowed by the manifest's own override patterns.
func (l *piLayout) manifestFiles() []string {
	files := []string{}
	patterns := []string{}
	for _, e := range l.extensions {
		if strings.HasPrefix(e, "+") || strings.HasPrefix(e, "-") || strings.HasPrefix(e, "!") {
			patterns = append(patterns, e)
			continue
		}
		if strings.ContainsAny(e, "*?") {
			l.fail(piProblemManifestGlob)
			return nil
		}
		rel, ok := l.inside(".", e)
		if !ok {
			return nil
		}
		info, exists := l.stat(rel)
		if !exists {
			continue
		}
		if info.Mode().IsRegular() {
			files = append(files, rel)
		} else if info.IsDir() {
			files = append(files, l.autoEntries(rel)...)
		}
	}
	if len(patterns) == 0 {
		return files
	}
	kept := []string{}
	for _, f := range files {
		switch piEvaluate(patterns, f, path.Join(filepath.ToSlash(l.abs), f)).state {
		case piOn:
			kept = append(kept, f)
		case piUnknown:
			l.fail(piProblemManifestRule)
			return nil
		}
	}
	return kept
}

// base is the extension set a filter narrows (collectManifestFiles).
func (l *piLayout) base() []string {
	if len(l.extensions) > 0 {
		return l.manifestFiles()
	}
	return l.autoEntries("extensions")
}

// defaults is what an entry without an extensions filter loads. A string entry
// and an object entry differ here: with a manifest that has no extensions field,
// Pi loads none for the string but the extensions folder for the object.
func (l *piLayout) defaults(object bool) []string {
	if l.extensionsSet {
		if len(l.extensions) == 0 {
			return nil
		}
		return l.manifestFiles()
	}
	if l.manifest && !object {
		return nil
	}
	return l.autoEntries("extensions")
}

var piNpmSpec = regexp.MustCompile(`^(@?[^@]+(?:/[^@]+)?)(?:@(.+))?$`)
var piNpmName = regexp.MustCompile(`^(@[A-Za-z0-9._~-]+/)?[A-Za-z0-9._~-]+$`)
var piGitScheme = regexp.MustCompile(`(?i)^(https?|ssh|git)://`)

// piSource is a package source resolved the way Pi does for one scope.
type piSource struct {
	kind     string // npm, git, local, unknown
	identity string
	install  string // absolute install or local path; "" when unknown
}

func trimBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

// resolvePiSource locates a source for scope "user" (agentDir) or "project" (projectDir, the .pi folder).
func resolvePiSource(source, agentDir, projectDir, scope string) piSource {
	trimmed := strings.TrimSpace(source)
	base := agentDir
	if scope == "project" {
		base = projectDir
	}
	switch {
	case strings.HasPrefix(trimmed, "npm:"):
		m := piNpmSpec.FindStringSubmatch(strings.TrimSpace(trimmed[4:]))
		if m == nil || !piNpmName.MatchString(m[1]) || strings.Contains(m[1], "..") {
			return piSource{kind: "unknown"}
		}
		return piSource{kind: "npm", identity: "npm:" + m[1], install: filepath.Join(base, "npm", "node_modules", filepath.FromSlash(m[1]))}
	case strings.HasPrefix(trimmed, "git:") || piGitScheme.MatchString(trimmed):
		host, repo, ok := piGitLocation(trimmed)
		if !ok {
			return piSource{kind: "unknown"}
		}
		return piSource{kind: "git", identity: "git:" + host + "/" + repo, install: filepath.Join(base, "git", host, filepath.FromSlash(repo))}
	}
	for _, prefix := range []string{"github:", "http:", "https:", "ssh:", "builtin:", "file:"} {
		if strings.HasPrefix(trimmed, prefix) {
			return piSource{kind: "unknown"}
		}
	}
	p := trimmed
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return piSource{kind: "unknown"}
		}
		p = filepath.Join(home, p[1:])
	} else if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	p = filepath.Clean(p)
	return piSource{kind: "local", identity: "local:" + p, install: p}
}

// piGitLocation reduces the git source forms Skillshare can read without
// hosted-git-info to host and repository path. Anything else is unknown.
func piGitLocation(source string) (string, string, bool) {
	s := strings.TrimSpace(strings.TrimPrefix(source, "git:"))
	if strings.Contains(s, "?") {
		return "", "", false // how Pi resolves a query is unverified, and it may hold a credential
	}
	if i := strings.Index(s, "#"); i >= 0 {
		s = s[:i]
	}
	if loc := piGitScheme.FindStringIndex(s); loc != nil {
		s = s[loc[1]:]
		if at := strings.LastIndex(strings.SplitN(s, "/", 2)[0], "@"); at >= 0 {
			s = s[at+1:]
		}
	} else if at, colon := strings.Index(s, "@"), strings.Index(s, ":"); at >= 0 && colon > at && !strings.Contains(s[:colon], "/") {
		s = s[at+1:colon] + "/" + s[colon+1:] // git@host:owner/repo
	}
	host, repo, ok := strings.Cut(s, "/")
	if !ok || host == "" || strings.Contains(host, ":") || (!strings.Contains(host, ".") && host != "localhost") {
		return "", "", false
	}
	if at := strings.LastIndex(repo, "@"); at >= 0 {
		repo = repo[:at]
	}
	repo = strings.TrimSuffix(strings.TrimSuffix(repo, "/"), ".git")
	clean := path.Clean(repo)
	if repo == "" || clean != repo || strings.HasPrefix(clean, "..") || strings.Contains(repo, "\\") {
		return "", "", false
	}
	return strings.ToLower(host), repo, true
}

// redactSource hides credentials a URL or git source may carry, in its userinfo
// or its query; the whole query is hidden. It is for display only: the settings
// file always keeps the source as written. Local paths and npm names are unchanged.
func redactSource(source string) string {
	trimmed := strings.TrimSpace(source)
	at, colon := strings.Index(trimmed, "@"), strings.Index(trimmed, ":")
	scp := at >= 0 && colon > at && !strings.Contains(trimmed[:colon], "/") // git@host:owner/repo
	if !strings.Contains(source, "://") && !strings.HasPrefix(trimmed, "git:") && !scp {
		return source
	}
	if q := strings.Index(source, "?"); q >= 0 {
		source = source[:q] + "?***"
	}
	if i := strings.Index(source, "://"); i >= 0 {
		rest := source[i+3:]
		hostPart := strings.SplitN(rest, "/", 2)[0]
		if at := strings.LastIndex(hostPart, "@"); at >= 0 {
			return source[:i+3] + "***@" + rest[at+1:]
		}
	}
	return source
}
