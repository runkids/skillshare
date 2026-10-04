// Package skillfollow edits the .skillfollow declaration files and the
// matching .gitignore lines in a skills source. Every write goes through the
// source's sourcefs.Root and replaces the file atomically, so a failed write
// leaves the previous content in place. Reading and classifying declarations
// stays in internal/sourcewalk.
package skillfollow

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/install"
	"skillshare/internal/sourcefs"
	"skillshare/internal/sourcewalk"
)

const (
	// File holds committed declarations.
	File = ".skillfollow"
	// LocalFile holds machine-local declarations, merged after File.
	LocalFile = ".skillfollow.local"
	// LocalIgnoreLine keeps LocalFile out of Git.
	LocalIgnoreLine = "/" + LocalFile
	// IgnoreFile is the source's .gitignore, where declared link lines go.
	IgnoreFile = ".gitignore"
)

// Files lists the declaration files in merge order.
var Files = []string{File, LocalFile}

// sameName matches a declaration line to a name the way discovery does, so
// on Windows `unfollow team` removes a `Team` line. Tests swap it.
var sameName = sourcewalk.SameEntryName

// Declares reports whether file in the source declares name.
func Declares(root *sourcefs.Root, file, name string) (bool, error) {
	content, err := read(root, file)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(content, "\n") {
		if sameName(strings.TrimSpace(line), name) {
			return true, nil
		}
	}
	return false, nil
}

// AddEntry appends name to file, keeping every existing line, comment, and
// blank line in place. It reports false when file already declares name.
func AddEntry(root *sourcefs.Root, file, name string) (bool, error) {
	content, err := read(root, file)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(content, "\n") {
		if sameName(strings.TrimSpace(line), name) {
			return false, nil
		}
	}
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += newline
	}
	return true, root.WriteFileAtomic(file, []byte(content+name+newline), 0644)
}

// RemoveEntry drops every line of file that declares name and keeps all
// other lines as they are. It returns the first removed line's spelling,
// which differs from name only on Windows, or "" when file does not declare
// name.
func RemoveEntry(root *sourcefs.Root, file, name string) (string, error) {
	content, err := read(root, file)
	if err != nil {
		return "", err
	}
	lines := strings.Split(content, "\n")
	kept := lines[:0]
	declared := ""
	for _, line := range lines {
		entry := strings.TrimSpace(line)
		if !sameName(entry, name) {
			kept = append(kept, line)
		} else if declared == "" {
			declared = entry
		}
	}
	if declared == "" {
		return "", nil
	}
	return declared, root.WriteFileAtomic(file, []byte(strings.Join(kept, "\n")), 0644)
}

// AddIgnoreLine puts line in the managed block of the source's .gitignore.
// It reports false when the block already has it.
func AddIgnoreLine(root *sourcefs.Root, line string) (bool, error) {
	content, err := read(root, IgnoreFile)
	if err != nil {
		return false, err
	}
	updated, changed := install.AddGitIgnoreLines(content, []string{line})
	if !changed {
		return false, nil
	}
	return true, root.WriteFileAtomic(IgnoreFile, []byte(updated), 0644)
}

// RemoveIgnoreLine drops line from the managed block of the source's
// .gitignore. Lines outside the block belong to the user and stay.
func RemoveIgnoreLine(root *sourcefs.Root, line string) (bool, error) {
	content, err := read(root, IgnoreFile)
	if err != nil {
		return false, err
	}
	updated, changed := install.RemoveGitIgnoreLines(content, []string{line})
	if !changed {
		return false, nil
	}
	return true, root.WriteFileAtomic(IgnoreFile, []byte(updated), 0644)
}

// read returns a first-level file's content, or "" when it does not exist.
func read(root *sourcefs.Root, file string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root.Dir(), file))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}
