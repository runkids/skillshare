package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
)

var (
	restoreCopyDir  = copyDir
	restoreCopyFile = copyFile
)

// RestoreOptions holds options for restore operation
type RestoreOptions struct {
	Force bool // Overwrite existing files
}

// ValidateRestore checks if a restore would succeed without modifying the destination.
func ValidateRestore(backupPath, targetName, destPath string, opts RestoreOptions) error {
	targetBackupPath, _, err := resolveBackupTargetPath(backupPath, targetName)
	if err != nil {
		return err
	}

	manifest, err := loadSnapshotManifest(targetBackupPath)
	if err != nil {
		return err
	}
	if manifest != nil {
		return validateSnapshotRestore(targetBackupPath, targetName, manifest, destPath, opts)
	}

	// Check if destination exists
	info, err := os.Lstat(destPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			if !opts.Force {
				entries, _ := os.ReadDir(destPath)
				if len(entries) > 0 {
					return fmt.Errorf("destination is not empty: %s (use --force to overwrite)", destPath)
				}
			}
			return nil
		}
		return fmt.Errorf("destination exists and is not a directory: %s", destPath)
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access destination: %w", err)
	}

	return nil
}

// RestoreToPath restores a backup to a specific path.
// backupPath is the full path to the backup (e.g., ~/.config/skillshare/backups/2024-01-15_14-30-45)
// targetName is the name of the target to restore (e.g., "claude")
// destPath is where to restore to (e.g., ~/.claude/skills)
func RestoreToPath(backupPath, targetName, destPath string, opts RestoreOptions) error {
	if err := ValidateRestore(backupPath, targetName, destPath, opts); err != nil {
		return err
	}

	targetBackupPath, _, err := resolveBackupTargetPath(backupPath, targetName)
	if err != nil {
		return err
	}
	manifest, err := loadSnapshotManifest(targetBackupPath)
	if err != nil {
		return err
	}
	if manifest != nil {
		return restoreSnapshot(targetBackupPath, targetName, manifest, destPath)
	}

	return restoreDirAtomic(targetBackupPath, destPath)
}

// RestoreLatest restores the most recent backup for a target from the global backup dir.
func RestoreLatest(targetName, destPath string, opts RestoreOptions) (string, error) {
	return RestoreLatestInDir(BackupDir(), targetName, destPath, opts)
}

// RestoreLatestInDir restores the most recent backup for a target from the specified dir.
// Returns the timestamp of the restored backup.
func RestoreLatestInDir(backupDir, targetName, destPath string, opts RestoreOptions) (string, error) {
	backups, err := ListInDir(backupDir)
	if err != nil {
		return "", err
	}

	// Find most recent backup containing the target
	for _, b := range backups {
		if _, ok, err := config.ResolveTargetNameCandidate(targetName, b.Targets); err != nil {
			return "", fmt.Errorf("resolve backup target for %q in %s: %w", targetName, b.Timestamp, err)
		} else if ok {
			if err := RestoreToPath(b.Path, targetName, destPath, opts); err != nil {
				return "", err
			}
			return b.Timestamp, nil
		}
	}

	return "", fmt.Errorf("no backup found for target '%s'", targetName)
}

// FindBackupsForTarget returns all backups that contain the specified target from the global dir.
func FindBackupsForTarget(targetName string) ([]BackupInfo, error) {
	return FindBackupsForTargetInDir(BackupDir(), targetName)
}

// FindBackupsForTargetInDir returns all backups that contain the specified target.
func FindBackupsForTargetInDir(backupDir, targetName string) ([]BackupInfo, error) {
	allBackups, err := ListInDir(backupDir)
	if err != nil {
		return nil, err
	}

	var result []BackupInfo
	for _, b := range allBackups {
		if matched, ok, err := config.ResolveTargetNameCandidate(targetName, b.Targets); err != nil {
			return nil, fmt.Errorf("resolve backup target for %q in %s: %w", targetName, b.Timestamp, err)
		} else if ok {
			normalized := b
			if matched != targetName && !slices.Contains(normalized.Targets, targetName) {
				normalized.Targets = append(append([]string(nil), normalized.Targets...), targetName)
			}
			result = append(result, normalized)
		}
	}

	return result, nil
}

// GetBackupByTimestamp finds a backup by its timestamp from the global dir.
func GetBackupByTimestamp(timestamp string) (*BackupInfo, error) {
	return GetBackupByTimestampInDir(BackupDir(), timestamp)
}

// GetBackupByTimestampInDir finds a backup by its timestamp in the specified dir.
func GetBackupByTimestampInDir(backupDir, timestamp string) (*BackupInfo, error) {
	backups, err := ListInDir(backupDir)
	if err != nil {
		return nil, err
	}

	for _, b := range backups {
		if b.Timestamp == timestamp {
			return &b, nil
		}
	}

	return nil, fmt.Errorf("backup not found: %s", timestamp)
}

// BackupVersion describes a single timestamped backup for a target.
type BackupVersion struct {
	Timestamp     time.Time
	Label         string // formatted: "2006-01-02 15:04:05" or ".000" when needed
	Dir           string // full path to target dir inside this backup
	SkillBaseDir  string
	SkillCount    int
	TotalSize     int64
	SkillNames    []string
	SnapshotPaths []string
	Manifest      bool
}

// ListBackupVersions returns all backup versions for a target, newest first.
// Returns nil, nil for a non-existent directory.
func ListBackupVersions(backupDir, targetName string) ([]BackupVersion, error) {
	return listBackupVersions(backupDir, targetName, true)
}

// ListBackupVersionsLite is like ListBackupVersions but skips the expensive
// dirSize() walk. Versions will have TotalSize = -1. Use this for TUI list
// population where size is computed lazily on demand.
func ListBackupVersionsLite(backupDir, targetName string) ([]BackupVersion, error) {
	return listBackupVersions(backupDir, targetName, false)
}

// DirSize calculates the total size of a directory by walking all files.
// Exported so callers (e.g. TUI) can compute size on demand for a single version.
func DirSize(path string) int64 {
	return dirSize(path)
}

func listBackupVersions(backupDir, targetName string, computeSize bool) ([]BackupVersion, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var versions []BackupVersion
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ts, parseErr := parseBackupTimestamp(entry.Name())
		if parseErr != nil {
			continue
		}

		targetDir, _, err := resolveBackupTargetPath(filepath.Join(backupDir, entry.Name()), targetName)
		if err != nil {
			if strings.Contains(err.Error(), "not found in backup") {
				continue
			}
			return nil, fmt.Errorf("resolve backup target for %q in %s: %w", targetName, entry.Name(), err)
		}
		if targetDir == "" {
			continue
		}
		info, statErr := os.Stat(targetDir)
		if statErr != nil || !info.IsDir() {
			continue
		}

		skillBaseDir := targetDir
		skillNames, snapshotPaths, err := backupVersionContents(targetDir)
		if err != nil {
			return nil, err
		}
		manifest, err := loadSnapshotManifest(targetDir)
		if err != nil {
			return nil, err
		}
		if manifest != nil {
			for _, entry := range manifest.Entries {
				if entry.Kind == "dir" && snapshotSkillEntryRelativePath(entry.RelativePath) {
					skillBaseDir, err = resolveSnapshotStoragePath(targetDir, entry.StoragePath)
					if err != nil {
						return nil, err
					}
					break
				}
			}
		}

		var totalSize int64 = -1
		if computeSize {
			totalSize = dirSize(targetDir)
		}

		versions = append(versions, BackupVersion{
			Timestamp:     ts,
			Label:         formatBackupTimestampLabel(ts),
			Dir:           targetDir,
			SkillBaseDir:  skillBaseDir,
			SkillCount:    len(skillNames),
			TotalSize:     totalSize,
			SkillNames:    skillNames,
			SnapshotPaths: snapshotPaths,
			Manifest:      manifest != nil,
		})
	}

	// Sort newest first
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})

	return versions, nil
}

func validateSnapshotRestore(targetBackupPath, targetName string, manifest *snapshotManifest, destPath string, opts RestoreOptions) error {
	restoreRoot, err := snapshotValidatedRestoreBasePath(destPath, targetName, manifest)
	if err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		resolvedPath, err := resolveSnapshotRestorePath(restoreRoot, entry.RelativePath)
		if err != nil {
			return err
		}
		if entry.Kind == "file" || entry.Kind == "dir" {
			sourcePath, err := resolveSnapshotStoragePath(targetBackupPath, entry.StoragePath)
			if err != nil {
				return err
			}
			if _, err := os.Stat(sourcePath); err != nil {
				return fmt.Errorf("cannot access snapshot storage %s: %w", sourcePath, err)
			}
		}
		if err := validateRestorePath(resolvedPath, opts); err != nil {
			return err
		}
	}
	return nil
}

func validateRestorePath(destPath string, opts RestoreOptions) error {
	info, err := os.Lstat(destPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot access destination: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if info.IsDir() {
		if !opts.Force {
			entries, _ := os.ReadDir(destPath)
			if len(entries) > 0 {
				return fmt.Errorf("destination is not empty: %s (use --force to overwrite)", destPath)
			}
		}
		return nil
	}
	if !opts.Force {
		return fmt.Errorf("destination exists and is not a directory: %s", destPath)
	}
	return nil
}

func restoreSnapshot(targetBackupPath, targetName string, manifest *snapshotManifest, destPath string) error {
	restoreRoot, err := snapshotValidatedRestoreBasePath(destPath, targetName, manifest)
	if err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		resolvedPath, err := resolveSnapshotRestorePath(restoreRoot, entry.RelativePath)
		if err != nil {
			return err
		}

		switch entry.Kind {
		case "absent":
			if err := os.RemoveAll(resolvedPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove %s: %w", resolvedPath, err)
			}
		case "file":
			storagePath, err := resolveSnapshotStoragePath(targetBackupPath, entry.StoragePath)
			if err != nil {
				return err
			}
			if err := restoreFileAtomic(storagePath, resolvedPath); err != nil {
				return fmt.Errorf("failed to restore file %s: %w", resolvedPath, err)
			}
		case "dir":
			storagePath, err := resolveSnapshotStoragePath(targetBackupPath, entry.StoragePath)
			if err != nil {
				return err
			}
			if err := restoreDirAtomic(storagePath, resolvedPath); err != nil {
				return fmt.Errorf("failed to restore directory %s: %w", resolvedPath, err)
			}
		default:
			return fmt.Errorf("unsupported snapshot entry kind: %s", entry.Kind)
		}
	}
	return nil
}

func restoreDirAtomic(srcPath, destPath string) error {
	return restorePathAtomic(destPath, true, func(stagedPath string) error {
		return restoreCopyDir(srcPath, stagedPath)
	})
}

func restoreFileAtomic(srcPath, destPath string) error {
	return restorePathAtomic(destPath, false, func(stagedPath string) error {
		return restoreCopyFile(srcPath, stagedPath)
	})
}

func restorePathAtomic(destPath string, dir bool, populate func(stagedPath string) error) error {
	parentDir := filepath.Dir(destPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("failed to create restore directory: %w", err)
	}

	stagedPath, cleanup, err := createRestoreStagePath(parentDir, dir)
	if err != nil {
		return err
	}
	keepStage := true
	defer func() {
		if keepStage {
			cleanup()
		}
	}()

	if err := populate(stagedPath); err != nil {
		return err
	}
	if err := replaceWithRestoreStage(destPath, stagedPath); err != nil {
		return err
	}
	keepStage = false
	return nil
}

func createRestoreStagePath(parentDir string, dir bool) (string, func(), error) {
	if dir {
		stageDir, err := os.MkdirTemp(parentDir, ".skillshare-restore-dir-*")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create restore staging directory: %w", err)
		}
		return stageDir, func() { _ = os.RemoveAll(stageDir) }, nil
	}

	stageFile, err := os.CreateTemp(parentDir, ".skillshare-restore-file-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create restore staging file: %w", err)
	}
	stagePath := stageFile.Name()
	if err := stageFile.Close(); err != nil {
		_ = os.Remove(stagePath)
		return "", nil, fmt.Errorf("failed to close restore staging file: %w", err)
	}
	return stagePath, func() { _ = os.Remove(stagePath) }, nil
}

func replaceWithRestoreStage(destPath, stagedPath string) error {
	previousPath, hadPrevious, err := moveExistingRestorePathAside(destPath)
	if err != nil {
		return err
	}

	if err := os.Rename(stagedPath, destPath); err != nil {
		if hadPrevious {
			if rollbackErr := os.Rename(previousPath, destPath); rollbackErr != nil {
				return fmt.Errorf("failed to replace restore path %s: %w (rollback failed: %v)", destPath, err, rollbackErr)
			}
		}
		return fmt.Errorf("failed to replace restore path %s: %w", destPath, err)
	}

	if hadPrevious {
		if err := os.RemoveAll(previousPath); err != nil {
			return fmt.Errorf("restored %s but failed to remove previous path backup %s: %w", destPath, previousPath, err)
		}
	}
	return nil
}

func moveExistingRestorePathAside(destPath string) (string, bool, error) {
	if _, err := os.Lstat(destPath); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("cannot access destination: %w", err)
	}

	previousPath, err := reserveRestoreSiblingPath(filepath.Dir(destPath))
	if err != nil {
		return "", false, err
	}
	if err := os.Rename(destPath, previousPath); err != nil {
		return "", false, fmt.Errorf("failed to move existing restore path %s aside: %w", destPath, err)
	}
	return previousPath, true, nil
}

func reserveRestoreSiblingPath(parentDir string) (string, error) {
	reserved, err := os.CreateTemp(parentDir, ".skillshare-restore-old-*")
	if err != nil {
		return "", fmt.Errorf("failed to reserve restore backup path: %w", err)
	}
	reservedPath := reserved.Name()
	if err := reserved.Close(); err != nil {
		_ = os.Remove(reservedPath)
		return "", fmt.Errorf("failed to close restore backup reservation: %w", err)
	}
	if err := os.Remove(reservedPath); err != nil {
		return "", fmt.Errorf("failed to reserve restore backup path: %w", err)
	}
	return reservedPath, nil
}

func resolveSnapshotRestorePath(destPath, relative string) (string, error) {
	cleaned, err := validateSnapshotRelativePath(relative)
	if err != nil {
		return "", err
	}
	return resolvePathWithinRoot(destPath, cleaned, "snapshot path")
}

func backupVersionContents(targetDir string) ([]string, []string, error) {
	manifest, err := loadSnapshotManifest(targetDir)
	if err != nil {
		return nil, nil, err
	}
	if manifest == nil {
		skillEntries, readErr := os.ReadDir(targetDir)
		if readErr != nil {
			return nil, nil, nil
		}

		var skillNames []string
		for _, se := range skillEntries {
			if se.IsDir() {
				skillNames = append(skillNames, se.Name())
			}
		}
		sort.Strings(skillNames)
		return skillNames, nil, nil
	}

	snapshotPaths := make([]string, 0, len(manifest.Entries))
	var skillNames []string
	for _, entry := range manifest.Entries {
		snapshotPaths = append(snapshotPaths, cleanSnapshotRelativePath(entry.RelativePath))
		if entry.Kind != "dir" || !snapshotSkillEntryRelativePath(entry.RelativePath) {
			continue
		}
		skillDir, err := resolveSnapshotStoragePath(targetDir, entry.StoragePath)
		if err != nil {
			return nil, nil, err
		}
		entries, readErr := os.ReadDir(skillDir)
		if readErr != nil {
			return nil, nil, readErr
		}
		for _, se := range entries {
			if se.IsDir() {
				skillNames = append(skillNames, se.Name())
			}
		}
	}
	sort.Strings(skillNames)
	sort.Strings(snapshotPaths)
	return skillNames, snapshotPaths, nil
}

func snapshotRestoreBasePath(destPath string, manifest *snapshotManifest) (string, error) {
	cleaned := filepath.Clean(destPath)
	switch manifest.RestoreBaseMode {
	case "":
		// Legacy snapshots inferred the restore base from the destination path shape.
	case SnapshotRestoreBaseTarget:
		return cleaned, nil
	case SnapshotRestoreBaseParent:
		return filepath.Dir(cleaned), nil
	case SnapshotRestoreBaseGrandparent:
		return filepath.Dir(filepath.Dir(cleaned)), nil
	default:
		return "", fmt.Errorf("unsupported snapshot restore base mode: %s", manifest.RestoreBaseMode)
	}

	for _, entry := range manifest.Entries {
		if cleanSnapshotRelativePath(entry.RelativePath) == "." {
			return cleaned, nil
		}
	}
	if strings.EqualFold(filepath.Base(cleaned), "skills") {
		return filepath.Dir(cleaned), nil
	}
	return cleaned, nil
}

func snapshotValidatedRestoreBasePath(destPath, targetName string, manifest *snapshotManifest) (string, error) {
	restoreRoot, err := snapshotRestoreBasePath(destPath, manifest)
	if err != nil {
		return "", err
	}
	if err := validateSnapshotRestoreTargetPath(restoreRoot, destPath, targetName, manifest); err != nil {
		return "", err
	}
	return restoreRoot, nil
}

func validateSnapshotRestoreTargetPath(restoreRoot, destPath, targetName string, manifest *snapshotManifest) error {
	expectedRelativePath, err := snapshotTargetRelativePath(targetName, manifest)
	if err != nil {
		return err
	}

	currentRelativePath, err := filepath.Rel(restoreRoot, filepath.Clean(destPath))
	if err != nil {
		return fmt.Errorf("resolve restore destination %s: %w", destPath, err)
	}
	currentRelativePath, err = validateSnapshotRelativePath(currentRelativePath)
	if err != nil {
		return fmt.Errorf("invalid restore destination %s: %w", destPath, err)
	}

	if currentRelativePath != expectedRelativePath {
		return fmt.Errorf("snapshot target path mismatch: backup was created for %s, current destination resolves to %s", expectedRelativePath, currentRelativePath)
	}
	return nil
}

func snapshotTargetRelativePath(targetName string, manifest *snapshotManifest) (string, error) {
	if manifest.TargetRelativePath != "" {
		return manifest.TargetRelativePath, nil
	}

	inferred, ok, err := legacySnapshotTargetRelativePath(targetName, manifest)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("snapshot target path is ambiguous in legacy manifest; cannot safely restore without target_relative_path")
	}
	return inferred, nil
}

func legacySnapshotTargetRelativePath(targetName string, manifest *snapshotManifest) (string, bool, error) {
	if manifest.RestoreBaseMode == SnapshotRestoreBaseTarget {
		return ".", true, nil
	}

	candidates := make(map[string]struct{}, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if !snapshotSkillEntryRelativePath(entry.RelativePath) {
			continue
		}
		cleaned, err := validateSnapshotRelativePath(entry.RelativePath)
		if err != nil {
			return "", false, err
		}
		candidates[cleaned] = struct{}{}
	}

	switch len(candidates) {
	case 0:
		return knownGlobalSnapshotTargetRelativePath(targetName, manifest)
	case 1:
		for candidate := range candidates {
			return candidate, true, nil
		}
	default:
		return "", false, fmt.Errorf("snapshot target path is ambiguous in legacy manifest; multiple candidate skill paths found")
	}

	return "", false, nil
}

func knownGlobalSnapshotTargetRelativePath(targetName string, manifest *snapshotManifest) (string, bool, error) {
	if manifest.RestoreBaseMode == "" {
		return "", false, nil
	}

	target, ok := config.LookupGlobalTarget(targetName)
	if !ok {
		return "", false, nil
	}

	cleanTargetPath := filepath.Clean(target.Path)
	var restoreRoot string
	switch manifest.RestoreBaseMode {
	case SnapshotRestoreBaseTarget:
		return ".", true, nil
	case SnapshotRestoreBaseParent:
		restoreRoot = filepath.Dir(cleanTargetPath)
	case SnapshotRestoreBaseGrandparent:
		restoreRoot = filepath.Dir(filepath.Dir(cleanTargetPath))
	default:
		return "", false, nil
	}

	relativePath, err := filepath.Rel(restoreRoot, cleanTargetPath)
	if err != nil {
		return "", false, fmt.Errorf("resolve known target path %s for %s: %w", cleanTargetPath, targetName, err)
	}
	cleanedRelativePath, err := validateSnapshotRelativePath(relativePath)
	if err != nil {
		return "", false, fmt.Errorf("invalid known target path %s for %s: %w", cleanTargetPath, targetName, err)
	}
	return cleanedRelativePath, true, nil
}

func resolveBackupTargetPath(backupPath, targetName string) (string, string, error) {
	candidates, err := backupTargetNames(backupPath)
	if err != nil {
		return "", "", err
	}

	resolvedName, ok, err := config.ResolveTargetNameCandidate(targetName, candidates)
	if err != nil {
		return "", "", fmt.Errorf("target '%s' is ambiguous in backup: %w", targetName, err)
	}
	if !ok {
		return "", "", fmt.Errorf("target '%s' not found in backup", targetName)
	}

	return filepath.Join(backupPath, resolvedName), resolvedName, nil
}

func backupTargetNames(backupPath string) ([]string, error) {
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("target backup directory does not exist: %s", backupPath)
		}
		return nil, fmt.Errorf("cannot access backup: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
