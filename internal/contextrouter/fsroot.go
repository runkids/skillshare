package contextrouter

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// Why a file the config names could not be measured.
const (
	ErrMissing     = "missing"
	ErrNotFile     = "not_file"
	ErrOutside     = "outside"
	ErrTooLarge    = "too_large"
	ErrInvalidUTF8 = "invalid_utf8"
)

// cleanRel turns a path from the config into a slash path inside the router
// folder. ok is false for a path that is empty, absolute, or leaves the folder.
func cleanRel(p string) (string, bool) {
	p = strings.ReplaceAll(p, "\\", "/")
	if p == "" || strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) {
		return "", false
	}
	if len(p) >= 2 && p[1] == ':' {
		return "", false // a Windows drive
	}
	p = path.Clean(p)
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// readRegular reads rel, already cleaned, through root, so a symlink that
// leaves the folder fails. The opened handle decides whether it is a regular
// file, and at most max bytes are accepted even if the file grows meanwhile.
func readRegular(root *os.Root, rel string, max int64) (text string, code string) {
	f, err := root.OpenFile(rel, os.O_RDONLY|nonblock, 0)
	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return "", ErrMissing
		case strings.Contains(err.Error(), "path escapes"):
			return "", ErrOutside
		}
		return "", ErrNotFile
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrNotFile
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return "", ErrNotFile
	}
	if int64(len(data)) > max {
		return "", ErrTooLarge
	}
	text, ok := normalizeNewlines(data)
	if !ok {
		return "", ErrInvalidUTF8
	}
	return text, ""
}
