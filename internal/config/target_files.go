package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Reasons a target file path is refused, shared with the Web API.
const (
	TargetFileEmpty    = "empty"
	TargetFileAbsolute = "absolute"
	TargetFileOutside  = "outside"
	TargetFileIsDir    = "is_dir"
	TargetFileNoRoot   = "no_root"
)

// TargetFilePathError explains why a target file path is refused.
type TargetFilePathError struct {
	Reason string
	Path   string
}

func (e *TargetFilePathError) Error() string {
	return fmt.Sprintf("target file %q: %s", e.Path, e.Reason)
}

// TargetFile is a plain file a target's tool reads besides its instruction
// file. Path is slash-separated and relative to the target's file root.
type TargetFile struct {
	Path    string
	Builtin bool
}

// fileSpec is the built-in target whose files a configured target reads: the
// Agent of a target that is another config directory of it, else the target.
func fileSpec(name string, tc TargetConfig, project bool) (targetSpec, bool) {
	if !project && tc.Agent != "" && tc.ConfigDir != "" {
		return globalSpec(tc.Agent)
	}
	// Plain targets retain environment-selected homes; accounts use canonical templates.
	specs, _ := loadTargetSpecs()
	for _, spec := range specs {
		if spec.Name != "" && targetSpecMatchesName(spec, name) {
			return spec, true
		}
	}
	return targetSpec{}, false
}

// TargetFileRoot returns the directory a target's extra files are relative to.
// In global mode that is the tool's config directory (config_dir, else detect,
// else the parent of its skills directory); in a project, the parent of its
// project skills directory. It reports false when there is none or when it
// would be too broad (home, the project root, or the filesystem root).
func TargetFileRoot(name string, tc TargetConfig, project bool, projectRoot string) (string, bool) {
	if tc.ProjectRoot() != "" {
		return "", false
	}
	spec, builtin := fileSpec(name, tc, project)
	var root string
	switch {
	case project:
		p := tc.SkillsConfig().Path
		if builtin && spec.Skills.Project != "" {
			p = spec.Skills.Project
		}
		if p = normalizeTargetPath(strings.TrimSpace(p)); p != "" {
			if !filepath.IsAbs(p) {
				p = filepath.Join(projectRoot, p)
			}
			root = filepath.Dir(p)
		}
	case tc.Agent != "" && tc.ConfigDir != "":
		root = expandPath(tc.ConfigDir)
	case builtin && spec.ConfigDir != "":
		root = normalizeTargetPath(spec.ConfigDir)
	case builtin && spec.Detect != "":
		root = normalizeTargetPath(spec.Detect)
	case builtin && spec.Skills.Global != "":
		root = filepath.Dir(normalizeTargetPath(spec.Skills.Global))
	case !builtin:
		if p := expandPath(strings.TrimSpace(tc.SkillsConfig().Path)); p != "" {
			root = filepath.Dir(p)
		}
	}
	if root == "" || !filepath.IsAbs(root) {
		return "", false
	}
	root = filepath.Clean(root)
	home, _ := os.UserHomeDir()
	if (home != "" && root == filepath.Clean(home)) ||
		(projectRoot != "" && root == filepath.Clean(projectRoot)) ||
		root == filepath.Dir(root) {
		return "", false
	}
	return root, true
}

// TargetFiles returns a target's extra files: the built-in ones, then the
// user's entries that are not built-in, each once, in order.
func TargetFiles(name string, tc TargetConfig, project bool) []TargetFile {
	var out []TargetFile
	seen := map[string]bool{}
	add := func(p string, builtin bool) {
		p = CleanTargetFilePath(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, TargetFile{Path: p, Builtin: builtin})
	}
	if spec, ok := fileSpec(name, tc, project); ok && tc.ProjectRoot() == "" {
		for _, p := range spec.Files {
			add(p, true)
		}
	}
	for _, p := range tc.Files {
		add(p, false)
	}
	return out
}

// CleanTargetFilePath returns the slash-separated clean form of a target file
// path, used to store and compare entries. It returns "" for an empty path.
func CleanTargetFilePath(rel string) string {
	rel = strings.TrimSpace(rel)
	if filepath.Separator == '\\' {
		rel = strings.ReplaceAll(rel, `\`, "/")
	}
	if rel == "" {
		return ""
	}
	if c := path.Clean(rel); c != "." {
		return c
	}
	return ""
}

// ValidateTargetFilePath resolves rel (slash-separated) under root and returns
// the absolute file path. The file must stay inside root: no absolute paths,
// no ".." escapes, and no existing directory on the way that links outside
// root. The file itself may be a symlink (such as a shared file's link);
// reading and writing follow it.
func ValidateTargetFilePath(root, rel string) (string, error) {
	fail := func(reason string) (string, error) {
		return "", &TargetFilePathError{Reason: reason, Path: rel}
	}
	if root == "" {
		return fail(TargetFileNoRoot)
	}
	trimmed := strings.TrimSpace(rel)
	if trimmed == "" || strings.ContainsRune(trimmed, 0) {
		return fail(TargetFileEmpty)
	}
	slashed := trimmed
	if filepath.Separator == '\\' {
		slashed = strings.ReplaceAll(slashed, `\`, "/")
	}
	if strings.HasPrefix(slashed, "/") || slashed == "~" || strings.HasPrefix(slashed, "~/") ||
		filepath.IsAbs(trimmed) || filepath.VolumeName(filepath.FromSlash(slashed)) != "" {
		return fail(TargetFileAbsolute)
	}
	clean := CleanTargetFilePath(trimmed)
	if clean == "" {
		return fail(TargetFileEmpty)
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return fail(TargetFileOutside)
	}
	if strings.HasSuffix(slashed, "/") {
		return fail(TargetFileIsDir)
	}
	abs := filepath.Join(root, filepath.FromSlash(clean))

	// An existing directory between root and the file must not lead outside.
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		dir := root
		parts := strings.Split(clean, "/")
		for _, part := range parts[:len(parts)-1] {
			dir = filepath.Join(dir, part)
			if _, err := os.Lstat(dir); err != nil {
				break
			}
			resolved, err := filepath.EvalSymlinks(dir)
			if err != nil || !withinDir(realRoot, resolved) {
				return fail(TargetFileOutside)
			}
		}
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return fail(TargetFileIsDir)
	}
	return abs, nil
}

func withinDir(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
