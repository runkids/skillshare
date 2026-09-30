package sync

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/utils"
)

// keepExtraBackups bounds the backups kept per replaced file.
const keepExtraBackups = 10

// extraBackupDir holds the backup history of one file, keyed by a digest of
// its absolute path: <state>/extras/backups/<digest>/<unix-nano>[.<reason>].bak
// (the fixed-width time prefix keeps name order time order), plus a
// "path" file naming the original. The state at attach time is recorded as a
// "created" marker (no file existed), a "restore" copy of the file that was
// replaced, or a "restore-link" naming the target of a replaced symlink;
// restore uses only that record, never a newer history backup.
func extraBackupDir(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return filepath.Join(config.StateDir(), "extras", "backups", fmt.Sprintf("%x", sum[:8]))
}

// Why a history backup was taken; stored in its file name.
const (
	BackupReasonConvert = "convert" // before a file is converted to or renamed as AGENTS.md
	BackupReasonShim    = "shim"    // before @AGENTS.md is added to a project file
	BackupReasonImport  = "import"  // before an @import line is added
	BackupReasonEdit    = "edit"    // before a dashboard edit
	BackupReasonCollect = "collect" // before a target's edit is collected into the shared file
	BackupReasonAttach  = "attach"  // the file replaced when a target was first attached
	BackupReasonRestore = "restore" // before an older version is restored
	BackupReasonMigrate = "migrate" // before a sync drops settings a release retired
)

// Why a drift backup (an edit skillshare replaced) was taken.
const (
	DriftReasonOverwrite = "overwrite" // the edit was overwritten from the dashboard
	DriftReasonMode      = "mode"      // the edit was replaced by a mode switch
	DriftReasonRestore   = "restore"   // the edit was replaced by restoring the target
)

// extraBackupName returns a new backup file name recording reason.
func extraBackupName(reason string) string {
	if reason == "" {
		return fmt.Sprintf("%019d.bak", time.Now().UnixNano())
	}
	return fmt.Sprintf("%019d.%s.bak", time.Now().UnixNano(), reason)
}

// backupExtraFile saves the current content of path before skillshare replaces
// it. A copy identical to the latest backup is not stored again.
func backupExtraFile(path, reason string) error {
	_, err := storeExtraBackup(path, reason)
	return err
}

// storeExtraBackup is backupExtraFile returning the new backup's file name,
// or "" when the content matched the latest backup.
func storeExtraBackup(path, reason string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("back up %s: %w", path, err)
	}
	if latest, ok, _ := latestExtraBackup(path); ok && bytes.Equal(latest, data) {
		return "", nil
	}
	dir := extraBackupDir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("back up %s: %w", path, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "path"), []byte(filepath.Clean(path)), 0644); err != nil {
		return "", fmt.Errorf("back up %s: %w", path, err)
	}
	name := extraBackupName(reason)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		return "", fmt.Errorf("back up %s: %w", path, err)
	}
	pruneExtraBackups(dir)
	return name, nil
}

// backupExtraDrift saves an edited target that skillshare is about to relink.
// It lives apart from the regular backups so restore never picks it.
func backupExtraDrift(path, reason string) error {
	data, err := os.ReadFile(path)
	// Preserve link identity even for a dangling link; never read its destination
	// as if it were an edited regular file.
	if dest, linkErr := os.Readlink(path); linkErr == nil {
		kind := "symlink"
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink == 0 && utils.IsLinkMode(path, info.Mode()) {
			kind = "junction"
		}
		data, err = []byte(kind+": "+dest+"\n"), nil
	}
	if err != nil {
		return fmt.Errorf("back up %s: %w", path, err)
	}
	dir := filepath.Join(extraBackupDir(path), "drift")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("back up %s: %w", path, err)
	}
	// The path file lets the file history find a target that has only drift backups.
	if err := os.WriteFile(filepath.Join(extraBackupDir(path), "path"), []byte(filepath.Clean(path)), 0644); err != nil {
		return fmt.Errorf("back up %s: %w", path, err)
	}
	name := extraBackupName(reason)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
		return fmt.Errorf("back up %s: %w", path, err)
	}
	pruneExtraBackups(dir)
	return nil
}

// extraBackupNames returns the backup file names of dir, oldest first.
func extraBackupNames(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".bak") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // zero-padded nanosecond prefixes sort by time
	return names
}

func latestExtraBackup(path string) ([]byte, bool, error) {
	dir := extraBackupDir(path)
	names := extraBackupNames(dir)
	if len(names) == 0 {
		return nil, false, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, names[len(names)-1]))
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// pruneExtraBackups is best effort: a backup that fails to delete only costs
// disk space.
func pruneExtraBackups(dir string) {
	names := extraBackupNames(dir)
	for len(names) > keepExtraBackups {
		_ = os.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
}

// Attach-time records, one per target path. Exactly one is kept at a time.
const (
	attachCreated         = "created"          // no file existed
	attachRestore         = "restore"          // copy of the file that was replaced
	attachRestoreJunction = "restore-junction" // original Windows junction
	attachRestoreLink     = "restore-link"     // link target of a symlink that was replaced
)

func markExtraCreated(path string) error {
	return recordExtraAttach(path, attachCreated, nil, 0644)
}

// recordExtraAttach writes the attach-time record name for path, replacing
// any other record.
func recordExtraAttach(path, name string, data []byte, perm os.FileMode) error {
	dir := extraBackupDir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "path"), []byte(filepath.Clean(path)), 0644); err != nil {
		return err
	}
	clearExtraAttach(path)
	return os.WriteFile(filepath.Join(dir, name), data, perm)
}

// recordExtraRestorePoint backs up the file at path when a target is attached
// over it and records that content as what restore puts back. importLine, left
// in the file by an earlier import mode, is not part of the user's file and is
// dropped from the record.
func recordExtraRestorePoint(path, importLine string) error {
	if err := backupExtraFile(path, BackupReasonAttach); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if stripped, changed := removeImportLine(string(data), importLine); changed {
			data = []byte(stripped)
		}
		err = recordExtraAttach(path, attachRestore, data, 0600)
	}
	if err != nil {
		return fmt.Errorf("back up %s: %w", path, err)
	}
	return nil
}

// recordExtraRestoreLink records the symlink at path, which is not ours, as
// what restore puts back.
func recordExtraRestoreLink(path string) error {
	dest, err := os.Readlink(path)
	if err == nil {
		kind := attachRestoreLink
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSymlink == 0 && utils.IsLinkMode(path, info.Mode()) {
			kind = attachRestoreJunction
		}
		err = recordExtraAttach(path, kind, []byte(dest), 0644)
	}
	if err != nil {
		return fmt.Errorf("back up %s: %w", path, err)
	}
	return nil
}

// extraAttached reports whether an attach-time state is recorded for path.
func extraAttached(path string) bool {
	dir := extraBackupDir(path)
	for _, name := range []string{attachCreated, attachRestore, attachRestoreLink, attachRestoreJunction} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// putBackExtraRestorePoint recreates at path (which must not exist) the state
// recorded at attach time: the replaced file, the replaced symlink, or nothing
// when no file existed. Without a record it falls back to the latest backup.
// The record is dropped once restored.
func putBackExtraRestorePoint(path string) error {
	dir := extraBackupDir(path)
	switch {
	case extraCreated(path):
	case fileExists(filepath.Join(dir, attachRestoreJunction)):
		dest, err := os.ReadFile(filepath.Join(dir, attachRestoreJunction))
		if err == nil {
			err = createJunction(path, string(dest))
		}
		if err != nil {
			return fmt.Errorf("failed to restore %s: %w", path, err)
		}
	case fileExists(filepath.Join(dir, attachRestoreLink)):
		dest, err := os.ReadFile(filepath.Join(dir, attachRestoreLink))
		if err == nil {
			err = os.Symlink(string(dest), path)
		}
		if err != nil {
			return fmt.Errorf("failed to restore %s: %w", path, err)
		}
	default:
		data, err := os.ReadFile(filepath.Join(dir, attachRestore))
		ok := err == nil
		if os.IsNotExist(err) {
			data, ok, err = latestExtraBackup(path)
		}
		if err != nil {
			return fmt.Errorf("failed to read backup of %s: %w", path, err)
		}
		if ok {
			if err := os.WriteFile(path, data, 0644); err != nil {
				return fmt.Errorf("failed to restore %s: %w", path, err)
			}
		}
	}
	clearExtraAttach(path)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// clearExtraAttach drops the attach-time record of path.
func clearExtraAttach(path string) {
	dir := extraBackupDir(path)
	for _, name := range []string{attachCreated, attachRestore, attachRestoreLink, attachRestoreJunction, extraWritten, extraImportBase} {
		_ = os.Remove(filepath.Join(dir, name))
	}
}

// extraWritten holds the hash of the copy skillshare last wrote at a target,
// so a later sync can tell its own unedited copy from a user edit.
const extraWritten = "written"

// recordExtraWritten remembers the file now at path as skillshare's copy.
func recordExtraWritten(path string) error {
	h, err := utils.FileHash(path)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(extraBackupDir(path), extraWritten), []byte(h), 0644)
}

func clearExtraWritten(path string) {
	_ = os.Remove(filepath.Join(extraBackupDir(path), extraWritten))
}

// extraImportBase holds the target's own content (without this extra's
// import line) when it left import mode, so switching back rebuilds the file
// as it was then rather than as it was at attach time. Restore ignores it.
const extraImportBase = "import-base"

func recordExtraImportBase(path, content string) error {
	dir := extraBackupDir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, extraImportBase), []byte(content), 0600)
}

func clearExtraImportBase(path string) {
	_ = os.Remove(filepath.Join(extraBackupDir(path), extraImportBase))
}

func hasExtraWritten(path string) bool {
	return fileExists(filepath.Join(extraBackupDir(path), extraWritten))
}

// extraRestoreBase returns the file content to rebuild when switching back to
// import: the content recorded when the target left import mode, else the one
// recorded at attach time, or "" when no file existed or a link was replaced
// (a link's content is not the user's to rebuild).
func extraRestoreBase(path string) string {
	dir := extraBackupDir(path)
	if data, err := os.ReadFile(filepath.Join(dir, extraImportBase)); err == nil {
		return string(data)
	}
	data, err := os.ReadFile(filepath.Join(dir, attachRestore))
	if err != nil {
		return ""
	}
	return string(data)
}

// isOurExtraCopy reports whether the file at path is still exactly the copy
// skillshare last wrote there.
func isOurExtraCopy(path string) bool {
	want, err := os.ReadFile(filepath.Join(extraBackupDir(path), extraWritten))
	if err != nil {
		return false
	}
	h, err := utils.FileHash(path)
	return err == nil && h == string(want)
}

func extraCreated(path string) bool {
	_, err := os.Stat(filepath.Join(extraBackupDir(path), "created"))
	return err == nil
}
