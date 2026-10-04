// Package memory manages user-owned Markdown notes independently of native agent memory.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"skillshare/internal/config"
	syncpkg "skillshare/internal/sync"
	"skillshare/internal/utils"
)

const MaxNoteBytes = 1 << 20

var ErrConflict = errors.New("note changed on disk; reload it before saving")
var writeMu sync.Mutex

type Note struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Version string `json:"version"`
	Invalid string `json:"invalid,omitempty"`
}

// Extra reuses the existing named source, including its per-extra override.
func Extra(extras []config.ExtraConfig) (config.ExtraConfig, bool, error) {
	for _, extra := range extras {
		if strings.EqualFold(extra.Name, "memory") {
			if extra.File != "" {
				return extra, true, fmt.Errorf("extra %q must be a folder to manage memory notes", extra.Name)
			}
			return extra, true, nil
		}
	}
	return config.ExtraConfig{Name: "memory"}, false, nil
}

func GlobalRoot(cfg *config.Config) (string, error) {
	extra, _, err := Extra(cfg.Extras)
	if err != nil {
		return "", err
	}
	return config.ResolveExtrasSourceDir(extra, cfg.EffectiveExtrasSource(), cfg.EffectiveSkillsSource()), nil
}

func ProjectRoot(cfg *config.ProjectConfig, root string) (string, error) {
	extra, _, err := Extra(cfg.Extras)
	if err != nil {
		return "", err
	}
	if err := config.ValidateProjectExtraSource(extra.Source); err != nil {
		return "", err
	}
	return config.ResolveExtrasSourceDirProject(extra, cfg.EffectiveExtrasSource(root), root), nil
}

func notePath(root, rel string) (string, error) {
	if strings.ContainsAny(rel, `\:`) || !strings.EqualFold(filepath.Ext(rel), ".md") {
		return "", fmt.Errorf("note path must be a relative Markdown filename")
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("note path cannot contain hidden folders or traversal")
		}
	}
	abs, err := config.ValidateTargetFilePath(root, rel)
	if err != nil {
		return "", err
	}
	// Notes are real source files, not links into a native memory store.
	p := root
	for _, part := range strings.Split(rel, "/") {
		p = filepath.Join(p, part)
		_, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && utils.IsSymlinkOrJunction(p) {
			return "", fmt.Errorf("note path cannot contain symbolic links")
		}
	}
	if info, err := os.Lstat(abs); err == nil && !info.Mode().IsRegular() {
		return "", fmt.Errorf("note is not a regular file")
	}
	return abs, nil
}

func Read(root, rel string) (Note, error) {
	abs, err := notePath(root, rel)
	if err != nil {
		return Note{}, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return Note{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Note{}, err
	}
	if !info.Mode().IsRegular() {
		return Note{}, fmt.Errorf("note is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxNoteBytes+1))
	if err != nil {
		return Note{}, err
	}
	if len(data) > MaxNoteBytes || !utf8.Valid(data) {
		return Note{}, fmt.Errorf("note must be UTF-8 text of at most 1 MiB")
	}
	sum := sha256.Sum256(data)
	note := Note{Path: rel, Content: string(data), Version: hex.EncodeToString(sum[:]), Title: strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))}
	frontmatter := strings.HasPrefix(note.Content, "---\n") || strings.HasPrefix(note.Content, "---\r\n")
	for i, line := range strings.Split(note.Content, "\n") {
		line = strings.TrimSpace(line)
		if frontmatter {
			if i > 0 && line == "---" {
				frontmatter = false
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			note.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}
	return note, nil
}

func List(root, query string) ([]Note, error) {
	notes := []Note{}
	query = strings.ToLower(strings.TrimSpace(query))
	walkRoot, err := filepath.EvalSymlinks(root)
	if os.IsNotExist(err) {
		return notes, nil
	}
	if err != nil {
		return notes, err
	}
	err = filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, err error) error {
		if os.IsNotExist(err) && path == walkRoot {
			return nil
		}
		if err != nil {
			return err
		}
		if path == walkRoot {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if utils.IsSymlinkOrJunction(path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, path)
		if err != nil {
			return err
		}
		note, err := Read(root, filepath.ToSlash(rel))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			note = Note{Path: filepath.ToSlash(rel), Title: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), Invalid: err.Error()}
		}
		if query == "" || strings.Contains(strings.ToLower(note.Path+"\n"+note.Content), query) {
			note.Content = ""
			notes = append(notes, note)
		}
		return nil
	})
	sort.Slice(notes, func(i, j int) bool { return notes[i].Path < notes[j].Path })
	return notes, err
}

// Write requires the last read version; an empty version creates a new note.
func Write(root, rel, content, version string) (Note, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	if len(content) > MaxNoteBytes || !utf8.ValidString(content) {
		return Note{}, fmt.Errorf("note must be UTF-8 text of at most 1 MiB")
	}
	abs, err := notePath(root, rel)
	if err != nil {
		return Note{}, err
	}
	current, err := Read(root, rel)
	if err != nil && !os.IsNotExist(err) {
		return Note{}, err
	}
	if err == nil && current.Version != version || os.IsNotExist(err) && version != "" {
		return Note{}, ErrConflict
	}
	perm := os.FileMode(0644)
	if err == nil {
		if current.Content == content {
			return current, nil
		}
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return Note{}, statErr
		}
		perm = info.Mode().Perm()
		if err := syncpkg.BackupFile(abs, syncpkg.BackupReasonEdit); err != nil {
			return Note{}, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return Note{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".memory-*")
	if err != nil {
		return Note{}, err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return Note{}, err
	}
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return Note{}, err
	}
	if err := tmp.Close(); err != nil {
		return Note{}, err
	}
	if err := commitNote(root, rel, tmp.Name(), version); err != nil {
		return Note{}, err
	}
	return Read(root, rel)
}

// Delete backs up the saved note and requires its last read version.
func Delete(root, rel, version string) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	abs, err := notePath(root, rel)
	if err != nil {
		return err
	}
	current, err := Read(root, rel)
	if err != nil {
		return err
	}
	if version == "" || current.Version != version {
		return ErrConflict
	}
	if err := syncpkg.BackupFile(abs, syncpkg.BackupReasonDelete); err != nil {
		return err
	}
	return removeNote(root, rel, version)
}

func Init(root string) error {
	starters := []struct{ path, content string }{
		{"INDEX.md", "# Shared memory\n\nKeep durable notes in this folder. Link relevant notes here with standard Markdown links.\n\n## Notes\n\n- [Lessons learned](LEARNED.md)\n"},
		{"LEARNED.md", "# Lessons learned\n\nRecord durable lessons and verified solutions here. Your memory guidance says when to update these notes. Verify changeable claims before reusing them.\n\n## Lesson title\n\n- Date:\n- Context:\n- Conclusion:\n- Evidence:\n"},
	}
	for _, starter := range starters {
		if _, err := Read(root, starter.path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if _, err := Write(root, starter.path, starter.content, ""); err != nil {
			return err
		}
	}
	return nil
}
