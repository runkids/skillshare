package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	snapshotManifestFilename       = ".skillshare-backup-snapshot.json"
	legacySnapshotManifestFilename = ".skillshare-manifest.json"
	snapshotManifestVersion        = 1
)

type SnapshotRestoreBaseMode string

const (
	SnapshotRestoreBaseTarget      SnapshotRestoreBaseMode = "target"
	SnapshotRestoreBaseParent      SnapshotRestoreBaseMode = "parent"
	SnapshotRestoreBaseGrandparent SnapshotRestoreBaseMode = "grandparent"
)

type SnapshotOptions struct {
	RestoreBaseMode    SnapshotRestoreBaseMode
	TargetRelativePath string
}

type SnapshotPath struct {
	RelativePath      string
	SourcePath        string
	FollowTopSymlinks bool
}

type snapshotManifest struct {
	Version            int                     `json:"version"`
	RestoreBaseMode    SnapshotRestoreBaseMode `json:"restore_base_mode,omitempty"`
	TargetRelativePath string                  `json:"target_relative_path,omitempty"`
	Entries            []snapshotManifestEntry `json:"entries"`
}

type snapshotManifestEntry struct {
	RelativePath string `json:"relative_path"`
	Kind         string `json:"kind"`
	StoragePath  string `json:"storage_path,omitempty"`
}

func CreateSnapshot(targetName string, paths []SnapshotPath, opts SnapshotOptions) (string, error) {
	paths = normalizeSnapshotPaths(paths)
	if len(paths) == 0 {
		return "", nil
	}

	targetRelativePath := opts.TargetRelativePath
	if strings.TrimSpace(targetRelativePath) == "" {
		switch opts.RestoreBaseMode {
		case "", SnapshotRestoreBaseTarget:
			targetRelativePath = "."
		default:
			return "", fmt.Errorf("snapshot target relative path is required for restore base mode %s", opts.RestoreBaseMode)
		}
	}
	cleanTargetRelativePath, err := validateSnapshotRelativePath(targetRelativePath)
	if err != nil {
		return "", fmt.Errorf("invalid snapshot target relative path %q: %w", targetRelativePath, err)
	}

	backupDir := BackupDir()
	if backupDir == "" {
		return "", fmt.Errorf("cannot determine backup directory: home directory not found")
	}

	_, backupPath, err := allocateBackupPath(backupDir, targetName)
	if err != nil {
		return "", fmt.Errorf("failed to allocate backup path: %w", err)
	}
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	manifest := snapshotManifest{
		Version:            snapshotManifestVersion,
		RestoreBaseMode:    opts.RestoreBaseMode,
		TargetRelativePath: cleanTargetRelativePath,
		Entries:            make([]snapshotManifestEntry, 0, len(paths)),
	}

	for i, path := range paths {
		entry, err := snapshotEntryForPath(backupPath, i, path)
		if err != nil {
			return "", err
		}
		manifest.Entries = append(manifest.Entries, entry)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode snapshot manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(backupPath, snapshotManifestFilename), data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write snapshot manifest: %w", err)
	}

	return backupPath, nil
}

func normalizeSnapshotPaths(paths []SnapshotPath) []SnapshotPath {
	byRelative := make(map[string]SnapshotPath, len(paths))
	for _, path := range paths {
		relative := cleanSnapshotRelativePath(path.RelativePath)
		if relative == "" || filepath.IsAbs(relative) {
			continue
		}
		path.RelativePath = relative

		existing, ok := byRelative[relative]
		if !ok {
			byRelative[relative] = path
			continue
		}
		if path.FollowTopSymlinks {
			existing.FollowTopSymlinks = true
		}
		if existing.SourcePath == "" {
			existing.SourcePath = path.SourcePath
		}
		byRelative[relative] = existing
	}

	normalized := make([]SnapshotPath, 0, len(byRelative))
	for _, path := range byRelative {
		normalized = append(normalized, path)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].RelativePath < normalized[j].RelativePath
	})
	return normalized
}

func cleanSnapshotRelativePath(relative string) string {
	if relative == "" {
		return "."
	}
	return filepath.Clean(relative)
}

func snapshotEntryForPath(backupPath string, index int, path SnapshotPath) (snapshotManifestEntry, error) {
	info, err := os.Stat(path.SourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshotManifestEntry{
				RelativePath: path.RelativePath,
				Kind:         "absent",
			}, nil
		}
		return snapshotManifestEntry{}, fmt.Errorf("failed to inspect snapshot path %s: %w", path.SourcePath, err)
	}

	storagePath := filepath.Join("entries", fmt.Sprintf("%03d", index))
	fullStoragePath := filepath.Join(backupPath, storagePath)
	if info.IsDir() {
		if err := os.MkdirAll(filepath.Dir(fullStoragePath), 0o755); err != nil {
			return snapshotManifestEntry{}, fmt.Errorf("failed to prepare snapshot directory: %w", err)
		}
		var copyErr error
		if path.FollowTopSymlinks {
			copyErr = copyDirFollowTopSymlinks(path.SourcePath, fullStoragePath)
		} else {
			copyErr = copyDir(path.SourcePath, fullStoragePath)
		}
		if copyErr != nil {
			return snapshotManifestEntry{}, fmt.Errorf("failed to snapshot directory %s: %w", path.SourcePath, copyErr)
		}
		return snapshotManifestEntry{
			RelativePath: path.RelativePath,
			Kind:         "dir",
			StoragePath:  filepath.ToSlash(storagePath),
		}, nil
	}
	if !info.Mode().IsRegular() {
		return snapshotManifestEntry{}, fmt.Errorf("unsupported snapshot path type: %s", path.SourcePath)
	}

	if err := os.MkdirAll(filepath.Dir(fullStoragePath), 0o755); err != nil {
		return snapshotManifestEntry{}, fmt.Errorf("failed to prepare snapshot file: %w", err)
	}
	if err := copyFile(path.SourcePath, fullStoragePath); err != nil {
		return snapshotManifestEntry{}, fmt.Errorf("failed to snapshot file %s: %w", path.SourcePath, err)
	}
	return snapshotManifestEntry{
		RelativePath: path.RelativePath,
		Kind:         "file",
		StoragePath:  filepath.ToSlash(storagePath),
	}, nil
}

func loadSnapshotManifest(targetBackupPath string) (*snapshotManifest, error) {
	manifest, err := loadSnapshotManifestFile(targetBackupPath, snapshotManifestFilename, false)
	if err != nil {
		return nil, err
	}
	if manifest != nil {
		return manifest, nil
	}
	return loadSnapshotManifestFile(targetBackupPath, legacySnapshotManifestFilename, true)
}

func loadSnapshotManifestFile(targetBackupPath, filename string, allowLegacySyncManifest bool) (*snapshotManifest, error) {
	data, err := os.ReadFile(filepath.Join(targetBackupPath, filename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read snapshot manifest: %w", err)
	}

	manifest, ok, err := parseSnapshotManifest(data, allowLegacySyncManifest)
	if err != nil {
		return nil, fmt.Errorf("failed to parse snapshot manifest: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return manifest, nil
}

func parseSnapshotManifest(data []byte, allowLegacySyncManifest bool) (*snapshotManifest, bool, error) {
	var raw struct {
		Version            *int                    `json:"version"`
		RestoreBaseMode    SnapshotRestoreBaseMode `json:"restore_base_mode"`
		TargetRelativePath string                  `json:"target_relative_path"`
		Entries            []snapshotManifestEntry `json:"entries"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false, err
	}

	if raw.Version == nil {
		if allowLegacySyncManifest && len(raw.Entries) == 0 {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("missing snapshot manifest version")
	}
	if *raw.Version != snapshotManifestVersion {
		return nil, false, fmt.Errorf("unsupported snapshot manifest version: %d", *raw.Version)
	}
	if raw.RestoreBaseMode != "" && !raw.RestoreBaseMode.valid() {
		return nil, false, fmt.Errorf("unsupported snapshot restore base mode: %s", raw.RestoreBaseMode)
	}
	if raw.TargetRelativePath != "" {
		cleaned, err := validateSnapshotRelativePath(raw.TargetRelativePath)
		if err != nil {
			return nil, false, fmt.Errorf("invalid snapshot target relative path %q: %w", raw.TargetRelativePath, err)
		}
		raw.TargetRelativePath = cleaned
	}

	return &snapshotManifest{
		Version:            *raw.Version,
		RestoreBaseMode:    raw.RestoreBaseMode,
		TargetRelativePath: raw.TargetRelativePath,
		Entries:            raw.Entries,
	}, true, nil
}

func validateSnapshotRelativePath(relative string) (string, error) {
	cleaned := cleanSnapshotRelativePath(relative)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid snapshot path %q: absolute paths are not allowed", cleaned)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid snapshot path %q: path traversal is not allowed", cleaned)
	}
	return cleaned, nil
}

func validateSnapshotStoragePath(storagePath string) (string, error) {
	cleaned := path.Clean(filepath.ToSlash(strings.TrimSpace(storagePath)))
	if cleaned == "." {
		return "", fmt.Errorf("invalid snapshot storage path %q: empty paths are not allowed", storagePath)
	}
	if path.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid snapshot storage path %q: absolute paths are not allowed", cleaned)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid snapshot storage path %q: path traversal is not allowed", cleaned)
	}
	return cleaned, nil
}

func resolveSnapshotStoragePath(backupRoot, storagePath string) (string, error) {
	cleaned, err := validateSnapshotStoragePath(storagePath)
	if err != nil {
		return "", err
	}
	return resolvePathWithinRoot(backupRoot, filepath.FromSlash(cleaned), "snapshot storage path")
}

func resolvePathWithinRoot(root, relative, label string) (string, error) {
	root = filepath.Clean(root)
	resolved := filepath.Clean(filepath.Join(root, relative))
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("invalid %s %q: %w", label, relative, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid %s %q: path traversal is not allowed", label, relative)
	}
	return resolved, nil
}

func snapshotSkillEntryRelativePath(relative string) bool {
	cleaned := cleanSnapshotRelativePath(relative)
	return cleaned == "." || strings.EqualFold(filepath.Base(cleaned), "skills")
}

func (mode SnapshotRestoreBaseMode) valid() bool {
	switch mode {
	case SnapshotRestoreBaseTarget, SnapshotRestoreBaseParent, SnapshotRestoreBaseGrandparent:
		return true
	default:
		return false
	}
}
