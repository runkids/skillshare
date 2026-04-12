package server

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errInvalidSkillFilePath = errors.New("invalid file path")

func writeSkillFile(skillRoot, relativePath string, content []byte) (string, error) {
	absPath, mode, err := resolveWritableSkillMarkdownPath(skillRoot, relativePath)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(filepath.Dir(absPath), ".skillshare-write-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return "", fmt.Errorf("chmod temp file: %w", err)
	}
	if err := replaceFile(tmpName, absPath); err != nil {
		return "", fmt.Errorf("replace file: %w", err)
	}

	return filepath.Base(absPath), nil
}

func containsTraversalSegment(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func ensureResolvedPathWithinRoot(root, absPath string) error {
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(absPath)
	relPath, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return errInvalidSkillFilePath
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	current := resolvedRoot
	for _, segment := range parts[:len(parts)-1] {
		if segment == "" || segment == "." {
			continue
		}

		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return os.ErrNotExist
			}
			return fmt.Errorf("stat path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errInvalidSkillFilePath
		}
	}

	return nil
}

func resolveWritableSkillMarkdownPath(skillRoot, relativePath string) (string, os.FileMode, error) {
	if relativePath == "" || filepath.IsAbs(relativePath) || containsTraversalSegment(relativePath) {
		return "", 0, errInvalidSkillFilePath
	}

	normalized := filepath.Clean(filepath.FromSlash(relativePath))
	if normalized == "." || normalized == "" || filepath.IsAbs(normalized) || strings.HasPrefix(normalized, ".."+string(filepath.Separator)) {
		return "", 0, errInvalidSkillFilePath
	}
	if strings.ToLower(filepath.Ext(normalized)) != ".md" {
		return "", 0, errInvalidSkillFilePath
	}

	root := filepath.Clean(skillRoot)
	absPath := filepath.Clean(filepath.Join(root, normalized))
	rel, err := filepath.Rel(root, absPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", 0, errInvalidSkillFilePath
	}

	if err := ensureResolvedPathWithinRoot(root, absPath); err != nil {
		return "", 0, err
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, os.ErrNotExist
		}
		return "", 0, fmt.Errorf("stat file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", 0, errInvalidSkillFilePath
	}

	return absPath, info.Mode().Perm(), nil
}
