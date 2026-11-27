// Package filepreview lists and reads files for the dashboard's read-only file views.
package filepreview

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// MaxSize keeps a stray bundle or binary from being sent to the browser whole.
const MaxSize = 1 << 20

// ErrTooLarge reports a file over MaxSize.
var ErrTooLarge = errors.New("too large to preview")

// List returns the files under dir as sorted slash paths, skipping node_modules: dependencies
// are not what the author wrote, and there can be tens of thousands of them. A missing dir
// lists nothing.
func List(dir string) ([]string, error) {
	// WalkDir does not enter a root that is a symlink, such as a config folder kept in a
	// dotfiles repo; Read follows it through os.OpenRoot, so List must too.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	files := []string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == dir && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipAll
			}
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	slices.Sort(files)
	return files, err
}

// Read reads one file under dir. os.Root refuses any path, or symlink, that leaves dir.
func Read(dir, rel string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(filepath.FromSlash(rel))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLimited(file, rel)
}

// ReadFile reads the file at path, which the caller has already resolved and checked.
func ReadFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLimited(file, path)
}

func readLimited(file io.Reader, name string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(file, MaxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxSize {
		return nil, fmt.Errorf("%s is %w", name, ErrTooLarge)
	}
	return data, nil
}
