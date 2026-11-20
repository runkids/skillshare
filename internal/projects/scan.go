package projects

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/projectdir"
	"skillshare/internal/utils"
)

// Results of a scanned folder.
const (
	ScanAdded   = "added"
	ScanKept    = "kept"
	ScanSkipped = "skipped"
)

// Why a scanned folder was skipped, for callers that word the reason themselves.
const (
	SkipDifferent = "different"
	SkipStorage   = "storage"
	SkipRemoved   = "removed"
	SkipOwnConfig = "own-config"
	SkipNameTaken = "name-taken"
	SkipBadName   = "bad-name"
)

// ScanRow is one folder a scan considered: the root or a checkout below it.
// Path is relative to the root ("." for the root itself). Item is the
// registered project, except for a skipped folder.
type ScanRow struct {
	Root   string `json:"root"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Result string `json:"result"`
	Skip   string `json:"skip,omitempty"`
	Reason string `json:"reason,omitempty"`
	Item   *Item  `json:"-"`
}

// Kinds of scan warnings.
const (
	ScanMissing    = "missing"
	ScanUnreadable = "unreadable"
)

// ScanWarning is a scan folder that does not exist, or a folder below one that
// could not be read. Folder is relative to the root.
type ScanWarning struct {
	Kind   string `json:"kind"`
	Folder string `json:"folder"`
	Detail string `json:"detail,omitempty"`
}

func (w ScanWarning) String() string {
	if w.Kind == ScanMissing {
		return fmt.Sprintf("scan folder %s does not exist", w.Folder)
	}
	return fmt.Sprintf("skipped %s, which could not be read: %s", w.Folder, w.Detail)
}

// ScanResult is a scan's registration. Item is the root.
type ScanResult struct {
	Result
	Rows     []ScanRow
	Warnings []ScanWarning
}

// Added counts the folders the scan registered, or would in a dry run.
func (r ScanResult) Added() int {
	n := 0
	for _, row := range r.Rows {
		if row.Result == ScanAdded {
			n++
		}
	}
	return n
}

// Scan registers root and each Git checkout below its scan folders with the
// same settings, in one validated write. It only adds: a registered folder
// stays as it is, so a rerun registers only new checkouts.
func (s *Service) Scan(root string, entry config.ManagedProject, dirs []string, opts PutOptions) (ScanResult, error) {
	cfg, err := s.load()
	if err != nil {
		return ScanResult{}, err
	}
	abs := config.ExpandPath(root)
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return ScanResult{}, fmt.Errorf("%w: %s", ErrFolderNotFound, root)
	}
	storage := opts.Storage
	if storage == "" {
		storage = cfg.NewProjectOrigin()
	}
	if err := checkStorage(storage); err != nil {
		return ScanResult{}, err
	}
	found, warnings, err := ScanCheckouts(abs, dirs)
	if err != nil {
		return ScanResult{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ScanResult{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ScanResult{}, err
	}
	rows, adds, err := planScan(cfg, root, entry, found, opts.Storage, cwd, home)
	if err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{Result: Result{Expanded: cfg, Registration: "kept"}, Rows: rows, Warnings: warnings}
	if len(adds) > 0 {
		// Entries share the settings' slices; each gets its own copy.
		c, err := layerEntries(s.path, cfg, cloneProjects(adds), storage)
		if err != nil {
			return ScanResult{}, err
		}
		result.Affected = c.Affected
		if err := c.Save.Check(); err != nil {
			return result, err
		}
		if err := commit(c, opts.DryRun, &result.Result, "updated"); err != nil {
			return result, err
		}
		result.Expanded = c.Save.Expanded
	}
	for i := range result.Rows {
		if result.Rows[i].Result != ScanSkipped {
			v := resultItem(result.Expanded, result.Rows[i].Root)
			result.Rows[i].Item = &v
		}
	}
	// The root is reported even when it was skipped.
	root = result.Rows[0].Root
	item := resultItem(result.Expanded, root)
	result.Item = &item
	return result, nil
}

// planScan decides each folder's result and the entries to add. storage is the
// layer the caller asked for, if any.
func planScan(cfg *config.Config, root string, entry config.ManagedProject, found []string, storage, cwd, home string) ([]ScanRow, map[string]config.ManagedProject, error) {
	taken := map[string]bool{}
	for key, p := range cfg.Projects {
		taken[p.ProjectName(key)] = true
	}
	adds := map[string]config.ManagedProject{}
	rootRow, err := registeredRow(cfg, root, entry, false, storage, cwd, home)
	if err != nil {
		return nil, nil, err
	}
	if rootRow == nil {
		rootRow = &ScanRow{Root: root, Name: entry.ProjectName(root), Result: ScanAdded}
	}
	rootRow.Path = "."
	if rootRow.Result == ScanAdded {
		adds[root] = entry
	}
	taken[rootRow.Name] = true
	rows := []ScanRow{*rootRow}

	var fresh []string
	bases := map[string]int{}
	for _, rel := range found {
		key := filepath.Join(root, filepath.FromSlash(rel))
		row, err := registeredRow(cfg, key, entry, true, storage, cwd, home)
		if err != nil {
			return nil, nil, err
		}
		if row == nil && ownsConfig(config.ExpandPath(key)) {
			row = &ScanRow{Root: key, Name: path.Base(rel), Result: ScanSkipped, Skip: SkipOwnConfig, Reason: "has its own .skillshare/ project config"}
		}
		if row != nil {
			row.Path = rel
			rows = append(rows, *row)
			continue
		}
		fresh = append(fresh, rel)
		bases[path.Base(rel)]++
	}

	// A checkout keeps its folder name unless another project or a sibling
	// checkout has it. Then the checkout directly under root keeps it and the
	// others are named from their path. Folder names are claimed first, so
	// the result does not depend on scan order.
	derived := map[string]bool{}
	for _, rel := range fresh {
		base := path.Base(rel)
		derived[rel] = taken[base] || (bases[base] > 1 && rel != base)
	}
	for _, pass := range []bool{false, true} {
		for _, rel := range fresh {
			if derived[rel] != pass {
				continue
			}
			key := filepath.Join(root, filepath.FromSlash(rel))
			child := entry
			child.Name = ""
			name := path.Base(rel)
			if pass {
				if name = scanName(rel); name != path.Base(rel) {
					child.Name = name
				}
			}
			row := ScanRow{Root: key, Path: rel, Name: name, Result: ScanAdded}
			switch {
			case taken[name]:
				row.Result, row.Skip, row.Reason = ScanSkipped, SkipNameTaken, fmt.Sprintf("name %q is already used; add it on its own with --name", name)
			case name == "" || strings.ContainsAny(name, config.ProjectTargetSep+`/\`) || ValidateName(name) != nil:
				row.Result, row.Skip, row.Reason = ScanSkipped, SkipBadName, fmt.Sprintf("%q cannot be a project name; add it on its own with --name", name)
			default:
				taken[name] = true
				adds[key] = child
			}
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows[1:], func(a, b ScanRow) int { return strings.Compare(a.Root, b.Root) })
	return rows, adds, nil
}

// registeredRow reports a folder that is already registered, nil otherwise.
// The root keeps its own name; a checkout's settings are compared without it.
func registeredRow(cfg *config.Config, key string, want config.ManagedProject, checkout bool, storage, cwd, home string) (*ScanRow, error) {
	match, err := FindRoot(cfg.Projects, key, cwd, home)
	if errors.Is(err, ErrNotFound) {
		if checkout && cfg.HasSharedProject(key) {
			return &ScanRow{Root: key, Name: want.ProjectName(key), Result: ScanSkipped, Skip: SkipRemoved, Reason: "was removed on this machine; add it on its own to restore it"}, nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	have := cfg.Projects[match]
	row := &ScanRow{Root: match, Name: have.ProjectName(match), Result: ScanKept}
	if checkout {
		want.Name = have.Name
	}
	origin, _ := cfg.ProjectOrigin(match)
	if !config.SameManagedProject(have, want) {
		row.Result, row.Skip, row.Reason = ScanSkipped, SkipDifferent, "has different settings; change it with: skillshare project edit "+row.Name
	} else if storage != "" && storage != origin {
		verb := map[string]string{OriginLocal: "localize", OriginShared: "share"}[storage]
		where := map[string]string{OriginLocal: "on this machine only", OriginShared: "in config.yaml"}[origin]
		row.Result, row.Skip, row.Reason = ScanSkipped, SkipStorage, fmt.Sprintf("is saved %s; move it with: skillshare project %s %s", where, verb, row.Name)
	}
	return row, nil
}

// ownsConfig includes a .skillshare/ whose config.yaml is gitignored, as a
// clone of a shared skills repository has it.
func ownsConfig(dir string) bool {
	_, full := projectdir.Find(dir)
	_, partial := projectdir.FindPartial(dir)
	return full || partial
}

// scanName names a checkout from its path under the root:
// .worktrees/app-a becomes worktrees-app-a.
func scanName(rel string) string {
	var parts []string
	for _, part := range strings.Split(rel, "/") {
		if part = strings.TrimLeft(part, "."); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "-")
}

// ScanCheckouts finds Git checkouts below the scan folders of root, an absolute
// project folder. A checkout has a .git folder, or a .git file for a linked
// worktree, and is never searched further. Hidden folders below a scan folder,
// node_modules and links are skipped. Found checkouts are slash-separated and
// relative to root. A scan folder that does not exist, or a folder that cannot
// be read, is skipped with a warning, so one such folder never stops the scan.
func ScanCheckouts(root string, dirs []string) (found []string, warnings []ScanWarning, err error) {
	// A root reached through a link is walked at its target, so the walk can
	// start there; links below the root are still skipped.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		rel := filepath.FromSlash(dir)
		if !filepath.IsLocal(rel) {
			return nil, nil, fmt.Errorf("--scan %q must be a folder inside %s", dir, root)
		}
		scanRoot := filepath.Join(root, rel)
		exists, err := localFolder(root, scanRoot)
		if errors.Is(err, fs.ErrPermission) {
			warnings = append(warnings, ScanWarning{Kind: ScanUnreadable, Folder: dir, Detail: fs.ErrPermission.Error()})
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("--scan %q: %w", dir, err)
		}
		if !exists {
			warnings = append(warnings, ScanWarning{Kind: ScanMissing, Folder: dir})
			continue
		}
		err = filepath.WalkDir(scanRoot, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				folder, _ := filepath.Rel(root, path)
				detail := err
				if pathErr, ok := err.(*fs.PathError); ok {
					detail = pathErr.Err
				}
				warnings = append(warnings, ScanWarning{Kind: ScanUnreadable, Folder: filepath.ToSlash(folder), Detail: detail.Error()})
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				return nil
			}
			if utils.IsLinkMode(path, entry.Type()) {
				return filepath.SkipDir
			}
			if path != scanRoot && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if path == root || !isCheckout(path) {
				return nil
			}
			checkout, _ := filepath.Rel(root, path)
			if checkout = filepath.ToSlash(checkout); !seen[checkout] {
				seen[checkout] = true
				found = append(found, checkout)
			}
			return filepath.SkipDir
		})
		if err != nil {
			return nil, nil, err
		}
	}
	slices.Sort(found)
	return found, warnings, nil
}

// localFolder reports whether path, inside root, is a folder reached without
// crossing a link. A missing folder is not an error.
func localFolder(root, path string) (bool, error) {
	rel, _ := filepath.Rel(root, path)
	if rel == "." {
		return true, nil
	}
	at := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		at = filepath.Join(at, part)
		info, err := os.Lstat(at)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if utils.IsLinkMode(at, info.Mode()) {
			return false, fmt.Errorf("%s is a link", at)
		}
		if !info.IsDir() {
			return false, fmt.Errorf("%s is not a folder", at)
		}
	}
	return true, nil
}

// isCheckout uses Lstat, unlike install.IsGitRepo, to reject a linked .git,
// which could point at another checkout.
func isCheckout(path string) bool {
	info, err := os.Lstat(filepath.Join(path, ".git"))
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}
