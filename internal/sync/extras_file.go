package sync

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/config"
	"skillshare/internal/sourcefs"
	"skillshare/internal/utils"
)

// Managed import block markers. Claude strips block-level HTML comments before
// loading context, so the markers cost nothing there.
const (
	importBlockBegin = "<!-- skillshare:instructions:begin -->"
	importBlockEnd   = "<!-- skillshare:instructions:end -->"
)

// FileWarning preserves the CLI message alongside its translation code and values.
type FileWarning struct {
	Code    string            `json:"code"`
	Params  map[string]string `json:"params"`
	Message string            `json:"message"`
}

func (r *ExtraResult) addFileWarning(code, message string, params map[string]string) {
	r.Warnings = append(r.Warnings, message)
	r.FileWarnings = append(r.FileWarnings, FileWarning{Code: code, Params: params, Message: message})
}

// ExtraFile is one target of a single-file extra (an extra with file: set).
type ExtraFile struct {
	Source string // absolute path of <source dir>/<file>
	Target string // <target path>/<as or file>
	Mode   string // merge (default), symlink, copy, or import

	projectRoot  string
	linkFallback bool // Mode is copy because file links are unavailable
}

// NewExtraFile resolves the source and target files of a single-file extra.
// merge and symlink both link the one file, or copy it when file links are
// unavailable (see ExtraTargetMode).
func NewExtraFile(sourceDir, file, targetDir, as, mode string) ExtraFile {
	if as == "" {
		as = file
	}
	src := filepath.Join(sourceDir, file)
	if abs, err := filepath.Abs(src); err == nil {
		src = abs
	}
	m := ExtraTargetMode(mode, true)
	return ExtraFile{Source: src, Target: filepath.Join(targetDir, as), Mode: m, linkFallback: m != EffectiveMode(mode)}
}

// DiscoverExtraSource returns the source files of an extra relative to
// sourceDir: the single file when file is set, otherwise every file under the
// directory (DiscoverExtraFiles).
func DiscoverExtraSource(sourceDir, file string) ([]string, error) {
	if file == "" {
		return DiscoverExtraFiles(sourceDir)
	}
	path := filepath.Join(sourceDir, file)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("extras source file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to stat extras source: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("extras source file is a directory: %s", path)
	}
	return []string{file}, nil
}

func (f ExtraFile) sourceExists() bool {
	info, err := os.Stat(f.Source)
	return err == nil && !info.IsDir()
}

// isOurLink reports whether target is a symlink pointing at the source file.
func (f ExtraFile) isOurLink() bool {
	dest, err := os.Readlink(f.Target)
	return err == nil && filepath.Clean(resolveReadlink(dest, f.Target)) == filepath.Clean(f.Source)
}

// SyncExtraFile writes one single-file extra target. A real target file that
// differs from the source is backed up and replaced without --force: on first
// attach it becomes the restore point, afterwards it is an edit and goes to
// the drift backups. A directory in the way is skipped. import mode maintains a managed @<source>
// line instead of replacing the file.
func SyncExtraFile(f ExtraFile, dryRun bool, projectRoot string) (*ExtraResult, error) {
	f.projectRoot = projectRoot
	if !f.sourceExists() {
		return nil, fmt.Errorf("extras source file does not exist: %s", f.Source)
	}
	switch f.Mode {
	case "import":
		return syncExtraImport(f, dryRun)
	case "merge", "symlink", "copy":
		result, err := syncExtraFileReplace(f, dryRun, projectRoot)
		if err == nil && f.linkFallback {
			result.addFileWarning("file_link_fallback", FileLinkFallbackWarning, map[string]string{})
		}
		return result, err
	default:
		return nil, fmt.Errorf("unsupported extras sync mode: %q", f.Mode)
	}
}

func syncExtraFileReplace(f ExtraFile, dryRun bool, projectRoot string) (*ExtraResult, error) {
	result := &ExtraResult{}
	relative := shouldUseRelative(projectRoot, f.Source, f.Target)
	copyMode := f.Mode == "copy"
	attached := extraAttached(f.Target)

	info, err := os.Lstat(f.Target)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, fmt.Errorf("failed to inspect target: %w", err)
	case utils.IsSymlinkOrJunction(f.Target):
		if !copyMode && f.isOurLink() && fileLinkUsable(f.Target) {
			dest, _ := os.Readlink(f.Target)
			if linkNeedsReformat(dest, relative) && !dryRun {
				if err := reformatLink(f.Target, f.Source, relative); err != nil {
					return nil, fmt.Errorf("failed to reformat symlink: %w", err)
				}
			}
			result.Synced = 1
			return result, nil
		}
		// Before replacing a user's import link, remove our line from its
		// destination and retain the user's base for a later mode switch.
		data, _ := os.ReadFile(f.Target)
		base, leavingImport := f.removeImport(string(data))
		drift := attached && !f.isOurLink() && !leavingImport
		if drift || !attached && !f.isOurLink() {
			warning := replacementWarning(f.Target, dryRun)
			result.addFileWarning(warning.Code, warning.Message, warning.Params)
		}
		if !dryRun && leavingImport {
			if !attached {
				if err := recordExtraRestoreLink(f.Target); err != nil {
					return nil, err
				}
				attached = true
			}
			if err := recordExtraImportBase(f.Target, base); err != nil {
				return nil, err
			}
			info, err := os.Stat(f.Target)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(f.Target, []byte(base), info.Mode().Perm()); err != nil {
				return nil, err
			}
		}
		if !dryRun && drift {
			if err := backupExtraDrift(f.Target, ""); err != nil {
				return nil, err
			}
		}
		// Any other symlink is left over from a mode change, or, on first
		// attach, the user's own link: record it so restore can put it back.
		if !dryRun {
			if !attached && !f.isOurLink() {
				if err := recordExtraRestoreLink(f.Target); err != nil {
					return nil, err
				}
				attached = true
			}
			if err := os.Remove(f.Target); err != nil {
				return nil, fmt.Errorf("failed to remove conflicting symlink: %w", err)
			}
		}
	case info.IsDir():
		result.Skipped = 1
		result.addFileWarning("target_directory", fmt.Sprintf("%s is a directory; not replaced", f.Target), map[string]string{"path": f.Target})
		return result, nil
	default:
		// A file leaving import mode: keep its own lines for switching back.
		leavingImport := ""
		hadImport := false
		if data, err := os.ReadFile(f.Target); err == nil && f.hasImport(string(data)) {
			leavingImport, hadImport = f.removeImport(string(data))
		}
		same := contentEqual(f.Source, f.Target)
		if copyMode && same {
			if !dryRun && attached && !isOurExtraCopy(f.Target) {
				_ = recordExtraWritten(f.Target) // best effort: only saves a later drift backup
			}
			result.Synced = 1
			return result, nil
		}
		// skillshare's own earlier copy, left unedited, is replaced silently.
		edited := !same && !isOurExtraCopy(f.Target)
		if edited {
			warning := replacementWarning(f.Target, dryRun)
			result.addFileWarning(warning.Code, warning.Message, warning.Params)
		}
		if !dryRun {
			// On first attach, back up even an identical file: restoring the
			// target later has nothing else to put back once the link is
			// removed. Once attached, a differing file is an edit; keep it as
			// a drift backup so the restore point stays the pre-attach state.
			if !attached {
				if err := recordExtraRestorePoint(f.Target, f.importLine()); err != nil {
					return nil, err
				}
				attached = true
			} else if edited {
				// A copy left by copy mode, now replaced by a link, is a mode switch.
				reason := ""
				if !copyMode && hasExtraWritten(f.Target) {
					reason = DriftReasonMode
				}
				if err := backupExtraDrift(f.Target, reason); err != nil {
					return nil, err
				}
			}
			if hadImport {
				if err := recordExtraImportBase(f.Target, leavingImport); err != nil {
					return nil, fmt.Errorf("back up %s: %w", f.Target, err)
				}
			}
			if err := os.Remove(f.Target); err != nil {
				return nil, fmt.Errorf("failed to remove existing file: %w", err)
			}
		}
	}

	if dryRun {
		result.Synced = 1
		return result, nil
	}
	if err := os.MkdirAll(filepath.Dir(f.Target), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent dir: %w", err)
	}
	if copyMode {
		if err := copyFile(sourcefs.OS, f.Source, f.Target); err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
	} else if err := createLink(f.Target, f.Source, relative); err != nil {
		return nil, fmt.Errorf("failed to create symlink: %w", err)
	}
	if !attached {
		if err := markExtraCreated(f.Target); err != nil {
			return nil, fmt.Errorf("failed to record created file: %w", err)
		}
	}
	if copyMode {
		if err := recordExtraWritten(f.Target); err != nil {
			return nil, fmt.Errorf("failed to record copied file: %w", err)
		}
	} else {
		clearExtraWritten(f.Target)
	}
	result.Synced = 1
	return result, nil
}

func replacementWarning(path string, dryRun bool) FileWarning {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink == 0 && utils.IsLinkMode(path, info.Mode()) {
		if dest, err := os.Readlink(path); err == nil {
			code, verb := "junction_replaced", "replaced"
			if dryRun {
				code, verb = "would_replace_junction", "would replace"
			}
			return FileWarning{Code: code, Params: map[string]string{"path": path, "destination": dest}, Message: fmt.Sprintf("%s junction %s pointing to %s; restore will recreate it", verb, path, dest)}
		}
	}
	code, message := "backed_up", fmt.Sprintf("backed up %s before replacing it", path)
	if dryRun {
		code, message = "would_back_up", fmt.Sprintf("would back up %s before replacing it", path)
	}
	return FileWarning{Code: code, Params: map[string]string{"path": path}, Message: message}
}

func (f ExtraFile) relativeImportLine() string {
	rel, err := filepath.Rel(filepath.Dir(f.Target), f.Source)
	if err != nil {
		return "@" + f.Source
	}
	return "@" + filepath.ToSlash(rel)
}

func (f ExtraFile) importLine() string {
	if shouldUseRelative(f.projectRoot, f.Source, f.Target) {
		return f.relativeImportLine()
	}
	return "@" + f.Source
}

func (f ExtraFile) hasImport(content string) bool {
	return hasImportLine(content, "@"+f.Source) || hasImportLine(content, f.relativeImportLine())
}

func (f ExtraFile) removeImport(content string) (string, bool) {
	content, a := removeImportLine(content, "@"+f.Source)
	content, b := removeImportLine(content, f.relativeImportLine())
	return content, a || b
}

// ImportLine is the @path line import mode keeps in the target file.
func (f ExtraFile) ImportLine() string { return f.importLine() }

// AddManagedImport adds line to content's managed import block, as import mode
// does, and reports whether content changed.
func AddManagedImport(content, line string) (string, bool) { return addImportLine(content, line) }

// ManagedImportLines returns the 0-based indexes of the lines of content that
// belong to the managed import block, markers included.
func ManagedImportLines(content string) []int {
	lines := strings.Split(content, "\n")
	begin, end := findImportBlock(lines)
	var out []int
	for i := begin; begin != -1 && i <= end; i++ {
		out = append(out, i)
	}
	return out
}

// BackupFile saves the current content of path to the extras backup history
// before skillshare rewrites or removes it.
// reason is one of the BackupReason constants.
func BackupFile(path, reason string) error { return backupExtraFile(path, reason) }

// StoreBackup is BackupFile returning the backup that holds the current content: the new
// copy, or the latest one when it already held the same content.
func StoreBackup(path, reason string) (string, error) {
	name, err := storeExtraBackup(path, reason)
	if err != nil {
		return "", err
	}
	dir := extraBackupDir(path)
	if name == "" {
		names := extraBackupNames(dir)
		name = names[len(names)-1]
	}
	return filepath.Join(dir, name), nil
}

func syncExtraImport(f ExtraFile, dryRun bool) (*ExtraResult, error) {
	result := &ExtraResult{Synced: 1}
	attached := extraAttached(f.Target)

	// A link to the source is left over from symlink mode; writing through it
	// would edit the source, so it is replaced by a new file.
	ourLink := f.isOurLink()
	var data []byte
	var err error
	if !ourLink {
		data, err = os.ReadFile(f.Target)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read target: %w", err)
		}
	}
	if malformedImportBlock(string(data)) {
		return nil, fmt.Errorf("%s has a damaged managed import block; restore or repair it before syncing", f.Target)
	}
	exists := ourLink || err == nil
	rest, others := splitImportBlock(string(data))

	// A whole-file copy is left over from copy mode: unedited when it is what
	// skillshare wrote or, apart from other extras' import lines, the source.
	// An edited one still came from copy mode when a written record remains.
	var leftoverCopy, editedCopy bool
	if !ourLink && exists && attached {
		src, _ := os.ReadFile(f.Source)
		leftoverCopy = isOurExtraCopy(f.Target) || rest == string(src)
		editedCopy = !leftoverCopy && hasExtraWritten(f.Target)
	}

	var updated string
	changed := true
	if ourLink || leftoverCopy || editedCopy {
		// Rebuild the attach-time file plus the managed import block, so the
		// target reads as it did right after the original import attach.
		updated = extraRestoreBase(f.Target)
		for _, line := range others {
			updated, _ = addImportLine(updated, line)
		}
		updated, _ = addImportLine(updated, f.importLine())
	} else {
		if f.hasImport(string(data)) {
			updated, changed = string(data), false
		} else {
			updated, changed = addImportLine(string(data), f.importLine())
		}
	}
	if editedCopy {
		warning := replacementWarning(f.Target, dryRun)
		result.addFileWarning(warning.Code, warning.Message, warning.Params)
	}
	if dryRun {
		return result, nil
	}
	if !changed {
		clearExtraImportBase(f.Target)
		return result, nil
	}

	if editedCopy {
		if err := backupExtraDrift(f.Target, DriftReasonMode); err != nil {
			return nil, err
		}
	}
	if ourLink {
		if err := os.Remove(f.Target); err != nil {
			return nil, fmt.Errorf("failed to remove leftover symlink: %w", err)
		}
	}
	perm := os.FileMode(0644)
	if info, statErr := os.Stat(f.Target); statErr == nil {
		perm = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(f.Target), 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent dir: %w", err)
	}
	if err := os.WriteFile(f.Target, []byte(updated), perm); err != nil {
		return nil, fmt.Errorf("failed to write target: %w", err)
	}
	clearExtraWritten(f.Target)
	clearExtraImportBase(f.Target)
	// A switch from another mode keeps the restore point recorded then.
	if (!exists || ourLink) && !attached {
		if err := markExtraCreated(f.Target); err != nil {
			return nil, fmt.Errorf("failed to record created file: %w", err)
		}
	}
	return result, nil
}

// splitImportBlock returns content without its managed import block (and the
// blank line addImportLine puts after it), plus the block's import lines.
func splitImportBlock(content string) (rest string, lines []string) {
	all := strings.Split(content, "\n")
	begin, end := findImportBlock(all)
	if begin == -1 {
		return content, nil
	}
	for _, l := range all[begin+1 : end] {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	tail := all[end+1:]
	if len(tail) > 1 && strings.TrimSpace(tail[0]) == "" {
		tail = tail[1:]
	}
	return strings.Join(append(append([]string{}, all[:begin]...), tail...), "\n"), lines
}

// findImportBlock returns the line indexes of the managed block markers, or
// (-1, -1) when the block is absent.
func findImportBlock(lines []string) (begin, end int) {
	begin = -1
	for i, l := range lines {
		switch strings.TrimSpace(l) {
		case importBlockBegin:
			if begin == -1 {
				begin = i
			}
		case importBlockEnd:
			if begin != -1 {
				return begin, i
			}
		}
	}
	return -1, -1
}

func hasImportLine(content, line string) bool {
	lines := strings.Split(content, "\n")
	begin, end := findImportBlock(lines)
	for i := begin + 1; begin != -1 && i < end; i++ {
		if strings.TrimSpace(lines[i]) == line {
			return true
		}
	}
	return false
}

// addImportLine appends line to the managed block, creating the block at the
// top of content when absent. Content outside the block is never changed.
func addImportLine(content, line string) (string, bool) {
	if hasImportLine(content, line) {
		return content, false
	}
	newline := "\n"
	if i := strings.IndexByte(content, '\n'); i > 0 && content[i-1] == '\r' {
		newline = "\r\n"
	}
	lines := strings.Split(content, "\n")
	if _, end := findImportBlock(lines); end != -1 {
		lines = append(lines[:end], append([]string{line + strings.TrimSuffix(newline, "\n")}, lines[end:]...)...)
		return strings.Join(lines, "\n"), true
	}
	block := importBlockBegin + newline + line + newline + importBlockEnd + newline
	if content == "" {
		return block, true
	}
	return block + newline + content, true
}

// removeImportLine deletes line from the managed block and drops the block
// (with the blank line addImportLine put after it) once it is empty.
func malformedImportBlock(content string) bool {
	open := false
	for _, line := range strings.Split(content, "\n") {
		switch strings.TrimSpace(line) {
		case importBlockBegin:
			if open {
				return true
			}
			open = true
		case importBlockEnd:
			if !open {
				return true
			}
			open = false
		}
	}
	return open
}

func removeImportLine(content, line string) (string, bool) {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	changed := false
	inBlock := false
	for _, l := range lines {
		switch strings.TrimSpace(l) {
		case importBlockBegin:
			inBlock = true
		case importBlockEnd:
			inBlock = false
		}
		if inBlock && strings.TrimSpace(l) == line {
			changed = true
			continue
		}
		out = append(out, l)
	}
	if !changed {
		return content, false
	}
	// Remove empty complete blocks, preserving the user's surrounding text.
	for i := 0; i < len(out); i++ {
		if strings.TrimSpace(out[i]) != importBlockBegin {
			continue
		}
		j := i + 1
		for j < len(out) && strings.TrimSpace(out[j]) == "" {
			j++
		}
		if j < len(out) && strings.TrimSpace(out[j]) == importBlockEnd {
			end := j + 1
			if end < len(out)-1 && strings.TrimSpace(out[end]) == "" {
				end++
			}
			out = append(out[:i], out[end:]...)
			i--
		}
	}
	return strings.Join(out, "\n"), true
}

// ExtraFileStatus reports "synced", "drift", "modified" (a symlink-mode
// target was replaced by a different real file), "not synced" (no target
// file), or "no source".
func ExtraFileStatus(f ExtraFile) string {
	if config.ValidateExtraMode(f.Mode) != nil {
		return "invalid mode"
	}
	if !f.sourceExists() {
		return "no source"
	}
	if _, err := os.Lstat(f.Target); os.IsNotExist(err) {
		return "not synced"
	}
	if f.Mode == "import" {
		data, err := os.ReadFile(f.Target)
		if err == nil && !f.isOurLink() && f.hasImport(string(data)) {
			return "synced"
		}
		return "drift"
	}
	info, err := os.Lstat(f.Target)
	if err != nil {
		return "drift"
	}
	if f.Mode == "copy" {
		if info.Mode().IsRegular() {
			if contentEqual(f.Source, f.Target) {
				return "synced"
			}
			if !isOurExtraCopy(f.Target) {
				return "modified"
			}
		}
		return "drift"
	}
	if utils.IsSymlinkOrJunction(f.Target) {
		if f.isOurLink() && fileLinkUsable(f.Target) {
			return "synced"
		}
		return "drift"
	}
	if info.Mode().IsRegular() && !contentEqual(f.Source, f.Target) && !isOurExtraCopy(f.Target) {
		return "modified"
	}
	return "drift"
}

// RestoreExtraTarget undoes a single-file extra target when it is removed. It
// removes what skillshare wrote (its symlink, a copy, or its import line) and
// puts back the state recorded at attach time: the file it replaced, or no
// file. An attached target the user edited is saved as a drift backup first.
// A file skillshare never attached to is left alone. It reports whether
// anything was changed.
func RestoreExtraTarget(f ExtraFile) (bool, error) {
	if f.Mode == "import" {
		return restoreExtraImport(f)
	}

	info, err := os.Lstat(f.Target)
	owned, drift := extraRestoreOwnership(f, info)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return false, fmt.Errorf("failed to inspect target: %w", err)
	case owned:
		if err := os.Remove(f.Target); err != nil {
			return false, fmt.Errorf("failed to remove target: %w", err)
		}
	case drift:
		if err := backupExtraDrift(f.Target, DriftReasonRestore); err != nil {
			return false, err
		}
		if err := os.Remove(f.Target); err != nil {
			return false, fmt.Errorf("failed to remove modified target: %w", err)
		}
	default:
		// Not ours to undo; the target is being detached all the same.
		clearExtraAttach(f.Target)
		return false, nil
	}

	return true, putBackExtraRestorePoint(f.Target)
}

// extraRestoreOwnership shares restore's removal and drift decisions with preview.
func extraRestoreOwnership(f ExtraFile, info os.FileInfo) (owned, drift bool) {
	if info == nil {
		return false, false
	}
	owned = f.isOurLink() || f.Mode == "copy" && info.Mode().IsRegular() && (contentEqual(f.Source, f.Target) || isOurExtraCopy(f.Target))
	return owned, !owned && info.Mode().IsRegular() && extraAttached(f.Target)
}

func restoreExtraImport(f ExtraFile) (bool, error) {
	data, err := os.ReadFile(f.Target)
	if os.IsNotExist(err) {
		clearExtraAttach(f.Target)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read target: %w", err)
	}
	updated, changed := f.removeImport(string(data))
	_, remaining := splitImportBlock(updated)
	if !changed {
		if len(remaining) == 0 {
			clearExtraAttach(f.Target)
		}
		return false, nil
	}
	// Only the managed block was left: the file is what skillshare wrote, so
	// put back the state recorded at attach time (no file, or the replaced one).
	if strings.TrimSpace(updated) == "" && extraAttached(f.Target) {
		if err := os.Remove(f.Target); err != nil {
			return false, fmt.Errorf("failed to remove target: %w", err)
		}
		return true, putBackExtraRestorePoint(f.Target)
	}
	if dest, err := os.ReadFile(filepath.Join(extraBackupDir(f.Target), attachRestoreLink)); err == nil {
		original, err := os.ReadFile(resolveReadlink(string(dest), f.Target))
		if err == nil && updated == string(original) {
			if err := os.Remove(f.Target); err != nil {
				return false, err
			}
			return true, putBackExtraRestorePoint(f.Target)
		}
	}
	info, err := os.Stat(f.Target)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(f.Target, []byte(updated), info.Mode().Perm()); err != nil {
		return false, fmt.Errorf("failed to write target: %w", err)
	}
	if len(remaining) == 0 {
		clearExtraAttach(f.Target)
	}
	return true, nil
}

// CollectBackExtraFile resolves a "modified" target by keeping the target's
// content: it becomes the source (the old source is backed up), then the
// target is linked again. In copy mode the target stays a copy.
func CollectBackExtraFile(f ExtraFile, projectRoot string) error {
	info, err := os.Lstat(f.Target)
	if err != nil {
		return fmt.Errorf("failed to inspect target: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("target %s is not a regular file", f.Target)
	}
	data, err := os.ReadFile(f.Target)
	if err != nil {
		return fmt.Errorf("failed to read target: %w", err)
	}
	perm := os.FileMode(0644)
	if srcInfo, statErr := os.Stat(f.Source); statErr == nil {
		existing, _ := os.ReadFile(f.Source)
		if !bytes.Equal(existing, data) {
			if err := backupExtraFile(f.Source, BackupReasonCollect); err != nil {
				return err
			}
		}
		perm = srcInfo.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(f.Source), 0755); err != nil {
		return fmt.Errorf("failed to create source dir: %w", err)
	}
	if err := os.WriteFile(f.Source, data, perm); err != nil {
		return fmt.Errorf("failed to write source: %w", err)
	}
	return replaceDriftedTarget(f, projectRoot)
}

// ReapplyExtraFile resolves a "modified" target by keeping the source: the
// target file is backed up and replaced by the source again.
func ReapplyExtraFile(f ExtraFile, projectRoot string) error {
	return replaceDriftedTarget(f, projectRoot)
}

// replaceDriftedTarget links a modified target back to its source. The edited
// file is kept as a drift backup, not a regular one: a later restore must put
// back what was there before the target was attached, not the edit.
func replaceDriftedTarget(f ExtraFile, projectRoot string) error {
	if info, err := os.Lstat(f.Target); err == nil && info.Mode().IsRegular() {
		if err := backupExtraDrift(f.Target, DriftReasonOverwrite); err != nil {
			return err
		}
		if err := os.Remove(f.Target); err != nil {
			return fmt.Errorf("failed to remove modified target: %w", err)
		}
	}
	result, err := SyncExtraFile(f, false, projectRoot)
	if err != nil {
		return err
	}
	if result.Skipped > 0 {
		return fmt.Errorf("%s", strings.Join(result.Warnings, "; "))
	}
	return nil
}

// RestoreExtraFileTargets undoes every target of a removed single-file extra:
// its links, copies, or import lines go, and files it replaced come back.
// resolve turns a configured target path into a directory. Directory extras
// are left alone (sync cleans their orphans). It reports how many targets
// changed.
func RestoreExtraFileTargets(extra config.ExtraConfig, sourceDir string, resolve func(string) string) (int, error) {
	if extra.File == "" {
		return 0, nil
	}
	restored := 0
	var errs []string
	for _, t := range extra.Targets {
		changed, err := RestoreExtraTarget(NewExtraFile(sourceDir, extra.File, resolve(t.Path), t.As, t.Mode))
		if err != nil {
			errs = append(errs, err.Error())
		}
		if changed {
			restored++
		}
	}
	if len(errs) > 0 {
		return restored, fmt.Errorf("could not restore %q targets: %s", extra.Name, strings.Join(errs, "; "))
	}
	return restored, nil
}

// ForgetExtraTarget drops the attach-time record of a target that is detached
// from the config without being restored, so a later attach records afresh.
func ForgetExtraTarget(f ExtraFile) { clearExtraAttach(f.Target) }
