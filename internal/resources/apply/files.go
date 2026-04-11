package apply

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"skillshare/internal/resources/adapters"
)

var writeFileAtomically = WriteFileAtomic

// CompiledFiles writes compiled resource files in a stable order and reports
// which files changed versus already matched the compiled content.
func CompiledFiles(files []adapters.CompiledFile, dryRun bool) ([]string, []string, error) {
	if files == nil {
		files = []adapters.CompiledFile{}
	}

	sorted := append([]adapters.CompiledFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	updated := make([]string, 0, len(sorted))
	skipped := make([]string, 0, len(sorted))
	for _, file := range sorted {
		same, err := compiledFileMatches(file.Path, file.Content)
		if err != nil {
			return nil, nil, err
		}
		if same {
			skipped = append(skipped, file.Path)
			continue
		}

		updated = append(updated, file.Path)
		if dryRun {
			continue
		}
		if err := writeFileAtomically(file.Path, []byte(file.Content), 0o644); err != nil {
			return nil, nil, err
		}
	}

	return updated, skipped, nil
}

// WriteFileAtomic writes a file by staging a temp file in the same directory
// and renaming it into place. The destination is either fully updated or left
// unchanged.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".compiled-tmp-*")
	if err != nil {
		return err
	}

	tempPath := tempFile.Name()
	cleanupWith := func(writeErr error) error {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return writeErr
	}

	if _, err := tempFile.Write(data); err != nil {
		return cleanupWith(err)
	}
	if err := tempFile.Chmod(perm); err != nil {
		return cleanupWith(err)
	}
	if err := tempFile.Close(); err != nil {
		return cleanupWith(err)
	}
	if err := replaceFile(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return err
	}

	return nil
}

func compiledFileMatches(path, content string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return string(data) == content, nil
}
