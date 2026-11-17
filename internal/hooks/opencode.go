package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"skillshare/internal/opencode"
)

const openCodeDependencyWarning = "OpenCode v2: package files and registration only; provision the declared dependencies yourself before loading. Skillshare does not execute plugins or install dependencies."

func (s *Service) openCode2() (bool, error) {
	bin := "opencode"
	if s.scopedAccount != "" && s.Accounts[s.scopedAccount].CLI != "" {
		bin = s.Accounts[s.scopedAccount].CLI
	}
	version, err := s.versionProbe()(bin)
	if err != nil {
		return false, fmt.Errorf("detect OpenCode API with %s --version: %w", bin, err)
	}
	key, err := opencode.ConfigKey(version)
	return key == "plugins", err
}

func (s *Service) versionProbe() func(string) (string, error) {
	if s.OpenCodeVersion != nil {
		return s.OpenCodeVersion
	}
	return probeOpenCodeVersion
}

func probeOpenCodeVersion(bin string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(path), ".cmd") {
		path, err = openCodeNPMExecutable(path)
		if err != nil {
			return "", err
		}
	}
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.WaitDelay = 100 * time.Millisecond
	output, err := cmd.Output()
	return string(output), err
}

// memoProbe asks each OpenCode binary once per plan, however many bindings and
// owned outputs need the answer. The answer is not kept between plans.
func (s *Service) memoProbe() *Service {
	probe := s.versionProbe()
	type answer struct {
		version string
		err     error
	}
	seen := map[string]answer{}
	c := *s
	c.OpenCodeVersion = func(bin string) (string, error) {
		a, ok := seen[bin]
		if !ok {
			a.version, a.err = probe(bin)
			seen[bin] = a
		}
		return a.version, a.err
	}
	return &c
}

func validateOpenCodePackage(files map[string]string) error {
	for name, content := range files {
		if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || !utf8.ValidString(content) {
			return fmt.Errorf("OpenCode package file %q must be a relative UTF-8 file without traversal", name)
		}
		for parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(name))); parent != "."; parent = filepath.ToSlash(filepath.Dir(filepath.FromSlash(parent))) {
			if _, exists := files[parent]; exists {
				return fmt.Errorf("OpenCode package file %q is also a parent directory", parent)
			}
		}
		for _, part := range strings.Split(name, "/") {
			if part == "node_modules" || part == ".git" {
				return fmt.Errorf("OpenCode package must not contain %s", part)
			}
		}
	}
	return opencode.Package(func(name string) ([]byte, error) {
		data, ok := files[name]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return []byte(data), nil
	})
}

func (s *Service) openCodeConfig() (string, error) {
	// Config overrides can redirect the native loader away from these scoped files.
	if os.Getenv("OPENCODE_CONFIG") != "" || os.Getenv("OPENCODE_CONFIG_CONTENT") != "" {
		return "", fmt.Errorf("unset OPENCODE_CONFIG and OPENCODE_CONFIG_CONTENT before managing OpenCode hook registrations")
	}
	dir, err := s.configDir("opencode")
	if err != nil {
		return "", err
	}
	var found string
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		p := filepath.Join(dir, name)
		if _, err := os.Lstat(p); err == nil {
			if found != "" {
				return "", fmt.Errorf("both opencode.json and opencode.jsonc exist; consolidate before hooks sync")
			}
			found = p
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if found != "" {
		return found, nil
	}
	return filepath.Join(dir, "opencode.json"), nil
}

func (s *Service) renderOpenCode(d *desired, root, target, name string, b Binding, addFile func(wantFile) error) error {
	config, err := s.openCodeConfig()
	if err != nil {
		return err
	}
	dir, err := s.nativePath("opencode")
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "skillshare-"+name)
	files := b.Package
	if files == nil {
		manifest, _ := json.MarshalIndent(map[string]any{"name": "skillshare-" + name, "type": "module", "exports": "./index.ts", "dependencies": map[string]string{"@opencode/plugin": "^2.0.0"}}, "", "  ")
		files = map[string]string{"index.ts": b.Code, "package.json": string(manifest) + "\n"}
	}
	if err := validateOpenCodePackage(files); err != nil {
		return err
	}
	if d.packageFiles == nil {
		d.packageFiles = map[string]bool{}
	}
	for _, file := range sortedKeys(files) {
		path := filepath.Join(dir, filepath.FromSlash(file))
		if err := addFile(wantFile{target: target, path: path, root: root, entry: name, content: []byte(files[file]), mode: 0644}); err != nil {
			return err
		}
		d.packageFiles[path] = true
	}
	d.targets[config], d.roots[config] = target, root
	d.elements[config] = append(d.elements[config], wantElement{target: target, path: config, root: root, entry: name, event: "plugins", value: opencode.FileURL(dir)})
	return nil
}

// importOpenCodePackages copies source files only. Installed dependencies and Git
// metadata are not portable hook declarations, and symlinks are never followed.
func (s *Service) importOpenCodePackages(state ledger) ([]Candidate, error) {
	dir, err := s.nativePath("opencode")
	if err != nil {
		return nil, err
	}
	var out []Candidate
	for _, name := range s.unownedFiles("opencode", dir, state) {
		root := filepath.Join(dir, name)
		info, err := os.Lstat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		if err := checkPath(s.baseOrDir("opencode"), root); err != nil {
			return nil, err
		}
		files := map[string]string{}
		// A file that cannot be read, such as a symlink, is this package's problem
		// only; the other packages and flat files stay importable.
		var problems []string
		_ = fs.WalkDir(os.DirFS(root), ".", func(name string, item fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				problems = append(problems, fmt.Sprintf("OpenCode package file %q cannot be imported: %v", name, walkErr))
				return nil
			}
			if item.IsDir() {
				if item.Name() == "node_modules" || item.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			data, _, _, err := safeRead(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				problems = append(problems, fmt.Sprintf("OpenCode package file %q cannot be imported: %v", name, err))
				return nil
			}
			files[name] = string(data)
			return nil
		})
		// Import reports the package's own validation problems once, for every candidate.
		c := Candidate{Name: candidateName(strings.TrimPrefix(name, "skillshare-")), Problems: problems, Entry: Entry{Bindings: map[string]Binding{s.targetKey("opencode"): {Package: files}}}}
		c.Warnings = []string{fmt.Sprintf("%s keeps loading; remove its original files and directory registration after syncing so the plugin does not run twice", root)}
		out = append(out, c)
	}
	return out, nil
}
