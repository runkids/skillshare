package trash

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"skillshare/internal/config"
)

const defaultMaxAge = 7 * 24 * time.Hour // 7 days

const reservedAgentTrashDir = "agents"

// validateTrashName rejects names that could cause path traversal when joined
// with a base directory.  Allowed: plain names ("skill"), nested names
// ("org/team-skill").  Rejected: empty, absolute, ".", "..", any segment
// containing NUL, or any segment equal to "..".
func validateTrashName(name string) error {
	if name == "" {
		return fmt.Errorf("trash name must not be empty")
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("trash name must not contain NUL")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("trash name must not be absolute: %s", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			return fmt.Errorf("trash name must not contain path traversal segment %q in %q", part, name)
		}
	}
	// Trash names use logical slash-separated paths.  Backslash is rejected
	// even on Unix to unconditionally block Windows-style traversal syntax.
	if strings.Contains(name, `\`) {
		return fmt.Errorf("trash name must not contain backslash: %s", name)
	}
	return nil
}

// trashLogicalName builds a slash-separated logical name for a trash entry.
// It normalizes OS path separators to '/' so that List() returns names
// compatible with validateTrashName() (which rejects backslashes).
func trashLogicalName(parentRel, name string) string {
	if parentRel == "." {
		return name
	}

	// filepath.ToSlash converts the current OS path separator to slash.
	// Do not translate literal backslashes on Unix; they are filename
	// characters, not path separators, and should remain invalid input.
	parentRel = filepath.ToSlash(parentRel)
	return path.Join(parentRel, name)
}

// ensureUnderBase asserts that candidate resolves to a path under base.
// Both are resolved to absolute paths before comparison.
// candidate == base is allowed (used by RestoreAgent where targetDir may
// equal destDir).
func ensureUnderBase(base, candidate string) error {
	return ensureUnderBaseRel(base, candidate, true)
}

// ensureStrictlyUnderBase is like ensureUnderBase but rejects candidate == base.
// Used by destructive sinks (e.g. Cleanup) that must never target the base
// directory itself.
func ensureStrictlyUnderBase(base, candidate string) error {
	return ensureUnderBaseRel(base, candidate, false)
}

func ensureUnderBaseRel(base, candidate string, allowEqual bool) error {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return fmt.Errorf("resolve base %q: %w", base, err)
	}

	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return fmt.Errorf("resolve candidate %q: %w", candidate, err)
	}

	baseAbs = filepath.Clean(baseAbs)
	candidateAbs = filepath.Clean(candidateAbs)

	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return fmt.Errorf("path %q escapes base %q: %w", candidate, base, err)
	}

	if rel == "." {
		if allowEqual {
			return nil
		}
		return fmt.Errorf("path %q equals base %q — refusing to operate on base", candidate, base)
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %q escapes base %q", candidate, base)
	}

	return nil
}

// TrashDir returns the global trash directory path.
func TrashDir() string {
	return filepath.Join(config.DataDir(), "trash")
}

// ProjectTrashDir returns the project-level trash directory path.
func ProjectTrashDir(root string) string {
	return filepath.Join(root, ".skillshare", "trash")
}

// AgentTrashDir returns the global trash directory for agents.
func AgentTrashDir() string {
	return filepath.Join(config.DataDir(), "trash", "agents")
}

// ProjectAgentTrashDir returns the project-level trash directory for agents.
func ProjectAgentTrashDir(root string) string {
	return filepath.Join(root, ".skillshare", "trash", "agents")
}

// MoveAgentToTrash moves an agent file (and its metadata) to the trash directory.
func MoveAgentToTrash(agentFile, metaFile, name, trashBase string) (string, error) {
	if err := validateTrashName(name); err != nil {
		return "", fmt.Errorf("invalid trash name: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	trashDir := filepath.Join(trashBase, name+"_"+timestamp)

	if err := ensureUnderBase(trashBase, trashDir); err != nil {
		return "", fmt.Errorf("trash path unsafe: %w", err)
	}

	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create agent trash dir: %w", err)
	}

	// Move agent .md file
	destFile := filepath.Join(trashDir, filepath.Base(agentFile))
	if err := os.Rename(agentFile, destFile); err != nil {
		// Fallback: copy then delete
		data, readErr := os.ReadFile(agentFile)
		if readErr != nil {
			return "", fmt.Errorf("failed to read agent for trash: %w", readErr)
		}
		if writeErr := os.WriteFile(destFile, data, 0644); writeErr != nil {
			return "", fmt.Errorf("failed to write agent to trash: %w", writeErr)
		}
		os.Remove(agentFile)
	}

	// Move metadata if exists
	if metaFile != "" {
		if _, err := os.Stat(metaFile); err == nil {
			destMeta := filepath.Join(trashDir, filepath.Base(metaFile))
			if err := os.Rename(metaFile, destMeta); err != nil {
				data, _ := os.ReadFile(metaFile)
				os.WriteFile(destMeta, data, 0644)
				os.Remove(metaFile)
			}
		}
	}

	return trashDir, nil
}

// TrashEntry holds information about a trashed item.
type TrashEntry struct {
	Name      string    // Original skill name
	Timestamp string    // Timestamp portion of dir name
	Path      string    // Full path to trashed directory
	Date      time.Time // Parsed or stat-based date
	Size      int64     // Total size in bytes
	Kind      string    // "skill" or "agent" — set by caller
}

// MoveToTrash moves a skill directory to the trash.
// Uses os.Rename for atomic same-device moves, falls back to copy+delete.
func MoveToTrash(srcPath, name, trashBase string) (string, error) {
	if err := validateTrashName(name); err != nil {
		return "", fmt.Errorf("invalid trash name: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	trashName := name + "_" + timestamp
	trashPath := filepath.Join(trashBase, trashName)

	if err := ensureUnderBase(trashBase, trashPath); err != nil {
		return "", fmt.Errorf("trash path unsafe: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(trashPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create trash directory: %w", err)
	}

	// Try atomic rename first (same device)
	if err := os.Rename(srcPath, trashPath); err == nil {
		return trashPath, nil
	}

	// Fallback: copy then delete (cross-device)
	if err := copyDir(srcPath, trashPath); err != nil {
		return "", fmt.Errorf("failed to move to trash: %w", err)
	}

	if err := os.RemoveAll(srcPath); err != nil {
		return trashPath, fmt.Errorf("copied to trash but failed to remove original: %w", err)
	}

	return trashPath, nil
}

// List returns all trashed items sorted by date (newest first).
// Walks recursively to find nested entries (e.g., "org/_team-skills_<ts>").
func List(trashBase string) []TrashEntry {
	var items []TrashEntry
	base := filepath.Clean(trashBase)

	filepath.WalkDir(trashBase, func(path string, d fs.DirEntry, err error) error {
		if shouldSkipReservedAgentTrashSubtree(base, path, d) {
			return fs.SkipDir
		}
		if err != nil || !d.IsDir() || path == base {
			return nil
		}

		name, ts := parseTrashName(d.Name())
		if name == "" {
			return nil // intermediate directory — keep walking
		}

		// Compute full nested name from relative path
		parentRel, _ := filepath.Rel(trashBase, filepath.Dir(path))
		fullName := trashLogicalName(parentRel, name)

		date, parseErr := time.Parse("2006-01-02_15-04-05", ts)
		if parseErr != nil {
			if info, serr := d.Info(); serr == nil {
				date = info.ModTime()
			}
		}

		items = append(items, TrashEntry{
			Name:      fullName,
			Timestamp: ts,
			Path:      path,
			Date:      date,
			Size:      dirSize(path),
		})

		return fs.SkipDir // don't descend into trashed content
	})

	sort.Slice(items, func(i, j int) bool {
		return items[i].Date.After(items[j].Date)
	})

	return items
}

// shouldSkipReservedAgentTrashSubtree prevents the skills trash root from
// recursively listing agent trash entries under the reserved "trash/agents" path.
// Agent trash is listed separately from AgentTrashDir()/ProjectAgentTrashDir().
func shouldSkipReservedAgentTrashSubtree(base, path string, d fs.DirEntry) bool {
	if d == nil || !d.IsDir() {
		return false
	}
	if filepath.Base(base) != "trash" {
		return false
	}
	return filepath.Clean(path) == filepath.Join(base, reservedAgentTrashDir)
}

// Cleanup removes trashed items older than maxAge.
// Returns the number of items removed.
func Cleanup(trashBase string, maxAge time.Duration) (int, error) {
	if maxAge == 0 {
		maxAge = defaultMaxAge
	}

	items := List(trashBase)
	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, item := range items {
		if item.Date.Before(cutoff) {
			if err := ensureStrictlyUnderBase(trashBase, item.Path); err != nil {
				return removed, fmt.Errorf("skip unsafe item %s: %w", item.Name, err)
			}
			if err := os.RemoveAll(item.Path); err != nil {
				return removed, fmt.Errorf("failed to clean up %s: %w", item.Name, err)
			}
			cleanEmptyParents(item.Path, trashBase)
			removed++
		}
	}

	return removed, nil
}

// cleanEmptyParents removes empty parent directories between path and stopAt.
func cleanEmptyParents(path, stopAt string) {
	stopAt = filepath.Clean(stopAt)
	dir := filepath.Dir(filepath.Clean(path))
	for dir != stopAt && dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}

// TotalSize returns the total size of all trashed items in bytes.
func TotalSize(trashBase string) int64 {
	items := List(trashBase)
	var total int64
	for _, item := range items {
		total += item.Size
	}
	return total
}

// FindByName returns the most recent trashed item matching the given skill name.
// Returns nil if not found.
func FindByName(trashBase, name string) *TrashEntry {
	items := List(trashBase)
	for i := range items {
		if items[i].Name == name {
			return &items[i] // List is sorted newest-first
		}
	}
	return nil
}

// Restore moves a trashed skill back to the destination directory.
// Returns an error if the destination already exists.
func Restore(entry *TrashEntry, destDir string) error {
	if err := validateTrashName(entry.Name); err != nil {
		return fmt.Errorf("invalid trash entry name: %w", err)
	}

	destPath := filepath.Join(destDir, entry.Name)

	if err := ensureUnderBase(destDir, destPath); err != nil {
		return fmt.Errorf("restore path unsafe: %w", err)
	}

	// Ensure parent directory exists for nested names (e.g., "org/_team-skills")
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("'%s' already exists in %s (use --force on uninstall to replace)", entry.Name, destDir)
	}

	// Try atomic rename first
	if err := os.Rename(entry.Path, destPath); err == nil {
		return nil
	}

	// Fallback: copy then delete
	if err := copyDir(entry.Path, destPath); err != nil {
		return fmt.Errorf("failed to restore: %w", err)
	}

	if err := os.RemoveAll(entry.Path); err != nil {
		return fmt.Errorf("restored but failed to remove trash entry: %w", err)
	}

	return nil
}

// RestoreAgent restores agent files from a trashed directory back to the agent source.
// Unlike Restore (which moves the whole directory), this copies individual files
// from the trashed directory to destDir (since agents are file-based, not directory-based).
func RestoreAgent(entry *TrashEntry, destDir string) error {
	if err := validateTrashName(entry.Name); err != nil {
		return fmt.Errorf("invalid trash entry name: %w", err)
	}

	// Reconstruct subdirectory for nested agents (e.g., "demo/my-agent" → destDir/demo/)
	targetDir := destDir
	if subDir := filepath.Dir(entry.Name); subDir != "." {
		targetDir = filepath.Join(destDir, subDir)
	}

	if err := ensureUnderBase(destDir, targetDir); err != nil {
		return fmt.Errorf("restore path unsafe: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent destination: %w", err)
	}

	// Read files from the trashed directory
	entries, err := os.ReadDir(entry.Path)
	if err != nil {
		return fmt.Errorf("failed to read trashed agent: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		srcPath := filepath.Join(entry.Path, e.Name())
		destPath := filepath.Join(targetDir, e.Name())

		if _, statErr := os.Stat(destPath); statErr == nil {
			return fmt.Errorf("'%s' already exists in %s", e.Name(), targetDir)
		}

		// Try rename, fallback to copy
		if renameErr := os.Rename(srcPath, destPath); renameErr != nil {
			data, readErr := os.ReadFile(srcPath)
			if readErr != nil {
				return fmt.Errorf("failed to read %s: %w", e.Name(), readErr)
			}
			if writeErr := os.WriteFile(destPath, data, 0644); writeErr != nil {
				return fmt.Errorf("failed to write %s: %w", e.Name(), writeErr)
			}
		}
	}

	// Remove the trashed directory
	if removeErr := os.RemoveAll(entry.Path); removeErr != nil {
		return fmt.Errorf("restored but failed to remove trash entry: %w", removeErr)
	}

	return nil
}

// parseTrashName splits "skillname_YYYY-MM-DD_HH-MM-SS" into name and timestamp.
func parseTrashName(dirName string) (string, string) {
	// Timestamp format: YYYY-MM-DD_HH-MM-SS (19 chars)
	const tsLen = 19 // "2006-01-02_15-04-05"
	// Need at least "x_" + timestamp
	if len(dirName) < tsLen+2 {
		return "", ""
	}

	// The timestamp is always the last 19 characters, preceded by "_"
	sep := len(dirName) - tsLen - 1
	if dirName[sep] != '_' {
		return "", ""
	}

	name := dirName[:sep]
	ts := dirName[sep+1:]

	// Validate timestamp format
	if _, err := time.Parse("2006-01-02_15-04-05", ts); err != nil {
		return "", ""
	}

	return name, ts
}

// dirSize calculates total size of a directory.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// copyDir copies a directory recursively.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := os.Lstat(srcPath)
		if err != nil {
			continue
		}

		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

		if info.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
