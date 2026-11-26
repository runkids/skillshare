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

func herdrIgnore(rel string, dir bool) bool {
	name := path.Base(rel)
	return (!dir && !strings.Contains(rel, "/") && (rel == "jsconfig.json" || strings.HasSuffix(rel, ".d.ts"))) ||
		strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__"
}
func herdrCandidate(name, path, component string) Candidate {
	return Candidate{Name: name, Path: path, Targets: []string{"herdr-app"}, Components: []string{component}, TargetInfo: map[string]TargetPackage{"herdr-app": {Components: []string{component}}}}
}
func herdrMods(root string) []Candidate {
	entries, _ := os.ReadDir(filepath.Join(root, "mods"))
	var out []Candidate
	for _, e := range entries {
		if !e.IsDir() || !namePattern.MatchString(e.Name()) {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, "mods", e.Name(), "main.js"))
		if err == nil && info.Mode().IsRegular() {
			out = append(out, herdrCandidate(e.Name(), filepath.ToSlash(filepath.Join("mods", e.Name())), "mods"))
		}
	}
	return out
}
func herdrLauncher(root, entry string) *Candidate {
	if entry == "" {
		return nil
	}
	clean := filepath.Clean(entry)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || !namePattern.MatchString(filepath.Base(clean)) {
		return nil
	}
	path := filepath.Join(root, clean)
	if noSymlink(root, path) != nil {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return nil
	}
	c := herdrCandidate(filepath.Base(clean), ".", "launcher")
	c.Entry = filepath.ToSlash(clean)
	infoForTarget := c.TargetInfo["herdr-app"]
	infoForTarget.Entry = c.Entry
	c.TargetInfo["herdr-app"] = infoForTarget

	return &c
}
func herdrCandidates(root, entry string) []Candidate {
	if entry != "" {
		if c := herdrLauncher(root, entry); c != nil {
			return []Candidate{*c}
		}
		return nil
	}
	return herdrMods(root)
}

func (s *Service) herdrHost(target string) Host {
	h := Host{Target: target, Status: HostReady, Installed: []Installed{}}
	info, err := os.Lstat(herdrHome())
	if os.IsNotExist(err) {
		h.Status = HostMissing
		h.Note = "Herdr app home " + herdrHome() + " is absent; Herdr bindings are skipped on this machine"
		return h
	}
	if err != nil {
		h.fail(err)
		return h
	}
	if !info.IsDir() {
		h.fail(fmt.Errorf("Herdr app home is not a directory: %s", herdrHome()))
		return h
	}
	var fingerprint []byte
	for _, layout := range []localFolderLayout{
		{filepath.Join(herdrHome(), "mods"), herdrHome(), herdrHome(), herdrIgnore},
		{filepath.Join(s.managedRoot(), "herdr-app"), s.managedRoot(), filepath.Join(s.managedRoot(), "herdr-app"), herdrIgnore},
	} {
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
	if c.Action == "remove" || c.Action == "uninstall" {
		if _, err := os.Lstat(herdrHome()); os.IsNotExist(err) {
			return nil
		}
	}
	home := herdrHome()
	layout := localFolderLayout{filepath.Join(home, "mods"), home, home, herdrIgnore}
	if b.Entry != "" {
		root := filepath.Join(s.managedRoot(), "herdr-app")
		layout = localFolderLayout{root, s.managedRoot(), root, herdrIgnore}
	}
	return s.applyLocalIn(layout, c, b, source)
}
