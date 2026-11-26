package plugin

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func herdrHome() string {
	if p := os.Getenv("HERDR_APP_HOME"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "herdr-app")
}

// herdrCommonIgnore names what Herdr and the apps it runs write into an installed folder; the
// digest of an installed folder leaves them out, and an update does not keep them.
func herdrCommonIgnore(rel string) bool {
	name := path.Base(rel)
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__"
}

// herdrIgnore adds the type files Herdr writes at a mod's root.
func herdrIgnore(rel string) bool {
	return herdrCommonIgnore(rel) || (!strings.Contains(rel, "/") && (rel == "jsconfig.json" || strings.HasSuffix(rel, ".d.ts")))
}

// herdrLayouts are where mods and launchers live. A mod's lock, marker, copy and backup sit in
// the home, not in mods/, so Herdr never loads a half-built folder.
func (s *Service) herdrLayouts() (mod, launcher localFolderLayout) {
	home, launchers := herdrHome(), filepath.Join(s.managedRoot(), "herdr-app")
	return localFolderLayout{filepath.Join(home, "mods"), home, home, herdrIgnore}, localFolderLayout{launchers, s.managedRoot(), launchers, herdrCommonIgnore}
}

func herdrCandidate(name, path, component, entry string) Candidate {
	return Candidate{Name: name, Path: path, Targets: []string{"herdr-app"}, Components: []string{component}, TargetInfo: map[string]TargetPackage{"herdr-app": {Components: []string{component}, Entry: entry}}}
}

// herdrCandidates reads Herdr's own layout when no manifest names a package: with an entry, one
// launcher that is an executable file in the repository; without one, each root mods/<id>/main.js.
func herdrCandidates(root, entry string) []Candidate {
	var out []Candidate
	if entry != "" {
		path := filepath.Join(root, entry)
		if filepath.IsLocal(entry) && namePattern.MatchString(filepath.Base(entry)) && noSymlink(root, path) == nil {
			if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
				out = append(out, herdrCandidate(filepath.Base(entry), ".", "launcher", filepath.ToSlash(filepath.Clean(entry))))
			}
		}
		return out
	}
	entries, _ := os.ReadDir(filepath.Join(root, "mods"))
	for _, e := range entries {
		info, err := os.Lstat(filepath.Join(root, "mods", e.Name(), "main.js"))
		if e.IsDir() && namePattern.MatchString(e.Name()) && err == nil && info.Mode().IsRegular() {
			out = append(out, herdrCandidate(e.Name(), "mods/"+e.Name(), "mods", ""))
		}
	}
	return out
}

func (s *Service) herdrHost(target string) Host {
	h := Host{Target: target, Status: HostReady, Installed: []Installed{}}
	mod, launcher := s.herdrLayouts()
	layouts := []localFolderLayout{mod, launcher}
	if info, err := os.Stat(herdrHome()); os.IsNotExist(err) {
		// A machine that does not run the Herdr window skips its mods; launchers live elsewhere.
		h.Status, layouts = HostMissing, layouts[1:]
		h.Note = "Herdr app home " + herdrHome() + " is absent; Herdr bindings are skipped on this machine"
		h.NoteKey, h.NoteArgs = "plugins.note.herdrHomeMissing", map[string]string{"path": herdrHome()}
	} else if err != nil {
		h.fail(err)
		return h
	} else if !info.IsDir() {
		h.fail(fmt.Errorf("Herdr app home is not a directory: %s", herdrHome()))
		return h
	}
	// Folder names and markers only: digesting or inspecting every folder would let an unrelated
	// one fail the host; applyLocalIn digests the one folder it changes.
	var fingerprint []byte
	for _, layout := range layouts {
		if err := noSymlink(layout.base, layout.root); err != nil {
			h.fail(err)
			return h
		}
		entries, err := os.ReadDir(layout.root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			h.fail(err)
			return h
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			h.Installed = append(h.Installed, Installed{ID: e.Name(), Name: e.Name(), Scope: "user", Installed: true, Enabled: true})
			marker, _ := os.ReadFile(filepath.Join(layout.scratch, ".skillshare-"+e.Name()+".json"))
			fingerprint = append(fingerprint, []byte(layout.root+"\x00"+e.Name()+"\x00")...)
			fingerprint = append(fingerprint, marker...)
		}
	}
	h.Fingerprint = hash(fingerprint)
	return h
}

func (s *Service) applyHerdr(c Change, b Binding, source string) error {
	layout, launcher := s.herdrLayouts()
	if b.Entry != "" {
		layout = launcher
	} else if c.Action == "remove" || c.Action == "uninstall" {
		// Removing a mod where the home is absent has nothing to delete and must not create it.
		if _, err := os.Lstat(herdrHome()); os.IsNotExist(err) {
			return nil
		}
	}
	return s.applyLocalIn(layout, c, b, source)
}
