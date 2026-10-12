package memory

import (
	"io/fs"
	"sort"
	"time"
)

type Overview struct {
	Root        string `json:"root"`
	Count       int    `json:"count"`
	Initialized bool   `json:"initialized"`
	Notes       []Note `json:"notes"`
}

// Recent counts entries from metadata and reads at most two starters and four candidates.
func Recent(root string) (Overview, error) {
	return recent(root, Read)
}

func recent(root string, read func(string, string) (Note, error)) (Overview, error) {
	result := Overview{Root: root, Initialized: true, Notes: []Note{}}
	starters := map[string]Note{}
	for _, starter := range starterNotes {
		note, err := read(root, starter.path)
		result.Initialized = result.Initialized && err == nil
		if err == nil {
			note.Content = ""
			starters[starter.path] = note
		}
	}
	type candidate struct {
		path     string
		modified time.Time
	}
	candidates := []candidate{}
	err := walkNotes(root, func(path string, entry fs.DirEntry) error {
		result.Count++
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxNoteBytes || starters[path].Starter {
			return nil
		}
		candidates = append(candidates, candidate{path, info.ModTime()})
		return nil
	})
	if err != nil {
		return result, err
	}
	onlyStarters := result.Count > 0 && result.Count == len(starters)
	for _, note := range starters {
		onlyStarters = onlyStarters && note.Starter
	}
	if onlyStarters {
		for _, starter := range starterNotes {
			if note, found := starters[starter.path]; found {
				result.Notes = append(result.Notes, note)
			}
		}
		return result, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modified.Equal(candidates[j].modified) {
			return candidates[i].path < candidates[j].path
		}
		return candidates[i].modified.After(candidates[j].modified)
	})
	if len(candidates) > 4 {
		candidates = candidates[:4]
	}
	for _, candidate := range candidates {
		note, cached := starters[candidate.path]
		if !cached {
			note, err = read(root, candidate.path)
			if err != nil {
				continue
			}
		}
		note.Content = ""
		result.Notes = append(result.Notes, note)
	}
	return result, nil
}
