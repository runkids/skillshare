package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// An npm source (npm:<package>[@version], as on pi.dev) is installed by Pi itself. Skillshare
// can't fetch or review the package first, and Pi's npm install runs the package's install
// scripts, so the preview says so. The binding has no Source: like an imported package, it
// is reinstalled, updated and removed through Pi.

func isNpmSource(source string) bool { return strings.HasPrefix(source, "npm:") }

// npmSpec splits a source the way Pi does: the name keeps a leading @scope.
func npmSpec(source string) (name, version string) {
	spec := strings.TrimSpace(strings.TrimPrefix(source, "npm:"))
	start := 0
	if strings.HasPrefix(spec, "@") {
		start = 1
	}
	if i := strings.Index(spec[start:], "@"); i >= 0 {
		return spec[:start+i], spec[start+i+1:]
	}
	return spec, ""
}

// exactNpmVersion is what Pi pins: an exact semver version, not a range or tag.
var exactNpmVersion = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

// validNpmSource accepts what Pi itself resolves as an npm package (see resolvePiSource), so
// a source Pi can't install is refused before a binding is recorded.
func validNpmSource(source string) bool {
	m := piNpmSpec.FindStringSubmatch(strings.TrimPrefix(source, "npm:"))
	return m != nil && piNpmName.MatchString(m[1]) && !strings.Contains(m[1], "..") && validTargetID("pi", source) && !strings.ContainsAny(source, " \t")
}

// sameNpmPackage reports whether a binding ID is an npm source of the same package, which Pi
// keeps in one entry whatever the version.
func sameNpmPackage(id, source string) bool {
	a, _ := npmSpec(id)
	b, _ := npmSpec(source)
	return isNpmSource(id) && a == b
}

// otherPiCLI is the executable a Pi account runs instead of pi, such as a fork; Skillshare
// does not know how a fork installs npm packages.
func (s *Service) otherPiCLI(target string) string {
	if a, ok := s.account(target); ok && a.CLI != "" && !piCLIIsNative(a.CLI, runtime.GOOS) {
		return a.CLI
	}
	return ""
}

func (s *Service) npmTarget(target string) bool {
	return s.agentOf(target) == "pi" && s.otherPiCLI(target) == ""
}

// npmChange plans adding an npm package to one target. installed is that target's native
// inventory, to adopt the package when Pi already has it; owner is another Skillshare package
// bound to the same Pi package there.
func (s *Service) npmChange(name, target, source, owner string, old Binding, bound bool, installed []Installed) Change {
	c := Change{Name: name, Target: target, ID: source, Binding: Binding{ID: source}, Action: "install"}
	if s.agentOf(target) != "pi" {
		c.Action, c.Message, c.MessageKey = "blocked", "npm packages are installed by Pi; choose a Pi target.", "plugins.error.npmPiOnly"
		return c
	}
	if cli := s.otherPiCLI(target); cli != "" {
		c.Action, c.Message, c.MessageKey = "blocked", fmt.Sprintf("This account runs %s; Skillshare installs npm packages only through pi. Install it in %s, then import it.", cli, cli), "plugins.error.npmOtherCli"
		c.MessageArgs = map[string]string{"cli": cli}
		return c
	}
	if owner != "" {
		c.Action, c.Message = "blocked", fmt.Sprintf("Skillshare package %s already manages this Pi package; update or remove that one instead.", owner)
		return c
	}
	// Pi keeps one entry per package name, and installing another version replaces its source.
	existing := ""
	for _, item := range installed {
		if sameNpmPackage(item.ID, source) {
			existing = item.ID
			break
		}
	}
	if bound {
		if !sameNpmPackage(old.ID, source) {
			c.Action, c.Message = "blocked", "Package already has a different binding for this target; choose another name or remove it first."
			return c
		}
		c.Binding = old
		c.Binding.ID = source
		// Pi keeps the entry's keys when it replaces the source, so the old version's record is stale.
		if old.ID != source {
			c.Binding.PiRegistration = ""
		}
		if old.ID == source && existing == source {
			c.Action = "noop"
			return c
		}
	} else if existing == source {
		c.Action, c.Message = "import", "Already installed natively; adopt without reinstalling."
		return c
	}
	c.Message, c.MessageKey = "Pi installs this package from npm and runs its install scripts; Skillshare can't review its content first.", "plugins.note.npmInstall"
	if existing != "" && existing != source {
		c.Message = fmt.Sprintf("Pi replaces %s with this version in its settings, installs it from npm and runs its install scripts; Skillshare can't review its content first.", existing)
		c.MessageKey, c.MessageArgs = "plugins.note.npmReplace", map[string]string{"from": existing}
	}
	return c
}

// pinnedNpm reports the exact version an npm binding is pinned to, which pi update keeps.
func pinnedNpm(id string) string {
	if !isNpmSource(id) {
		return ""
	}
	if _, version := npmSpec(id); exactNpmVersion.MatchString(version) {
		return version
	}
	return ""
}

// recordPiEntry records the entry Pi wrote for b, as an import does, so a reinstall restores
// its filters.
func (s *Service) recordPiEntry(ctx context.Context, target string, b *Binding) error {
	c := Change{Target: target, ID: b.ID, Action: "import", Binding: *b}
	if err := s.preparePiRegistration(ctx, &c); err != nil {
		return err
	}
	if err := s.savePiRegistration(c); err != nil {
		return err
	}
	b.PiRegistration = c.Binding.PiRegistration
	return nil
}

// npmRegistry is where an update check asks for a package's latest version; tests point it elsewhere.
var npmRegistry = "https://registry.npmjs.org"

// npmVersions is the installed and latest plain X.Y.Z versions of an npm package added without a version, nil
// where either is unknown. latests holds the registry's answers for one preview.
// ponytail: asks the public registry only; a package from a private registry or an .npmrc
// scope reads as unknown, and the check falls back to the native client.
func (s *Service) npmVersions(ctx context.Context, target, id string, h Host, latests map[string][]int) (installed, latest []int) {
	// Pi keeps a package added with a version, range or tag to that spec; only one added
	// without a version follows npm's latest.
	name, spec := npmSpec(id)
	if !isNpmSource(id) || spec != "" {
		return nil, nil
	}
	settings, err := s.piSettingsPath(target)
	if err != nil || !npmUsesPublicRegistry(ctx, filepath.Join(filepath.Dir(settings), "npm"), name) {
		return nil, nil
	}
	for _, item := range h.Installed {
		if item.ID == id {
			installed = plainVersion(item.Version)
		}
	}
	if installed == nil {
		return nil, nil
	}
	latest, ok := latests[name]
	if !ok {
		latest = plainVersion(npmLatest(ctx, name))
		latests[name] = latest
	}
	return installed, latest
}

// npmConfig runs `npm config get` with keys in dir, where Pi runs npm; tests stand in for it.
var npmConfig = func(ctx context.Context, dir string, keys ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "npm", append([]string{"config", "get"}, keys...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

// npmUsesPublicRegistry asks npm, in the folder Pi runs it in, which registry it fetches name
// from, for everything and for the package's scope. npm resolves its environment, every .npmrc
// and the overrides they make itself. Another registry, or npm not answering, is a no, so a
// private name never goes to npmjs.
func npmUsesPublicRegistry(ctx context.Context, dir, name string) bool {
	keys := []string{"registry"}
	if scope, _, ok := strings.Cut(name, "/"); ok && strings.HasPrefix(scope, "@") {
		keys = append(keys, scope+":registry")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := npmConfig(ctx, dir, keys...)
	if err != nil {
		return false
	}
	// One key prints its value; several print key=value lines, "undefined" for one not set.
	values := map[string]string{"registry": strings.TrimSpace(out)}
	if len(keys) > 1 {
		for line := range strings.SplitSeq(out, "\n") {
			if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
				values[key] = value
			}
		}
	}
	for _, key := range keys {
		value := strings.TrimSuffix(strings.TrimSpace(values[key]), "/")
		if key != "registry" && (value == "" || value == "undefined") {
			continue
		}
		if value != "https://registry.npmjs.org" {
			return false
		}
	}
	return true
}

// npmLatest is the version npm's "latest" tag names, "" when the registry does not say.
func npmLatest(ctx context.Context, name string) string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// A scoped name keeps its slash encoded, as the registry addresses package documents.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, npmRegistry+"/"+strings.Replace(name, "/", "%2f", 1)+"/latest", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var doc struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&doc) != nil {
		return ""
	}
	return doc.Version
}
