package tooling

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CopyDir recursively copies src into dst, skipping .git directories.
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ReplaceDir atomically replaces dst with a fresh copy of src.
func ReplaceDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return CopyDir(src, dst)
}

// MergeDir recursively copies src into dst without removing dst first.
// When a file/dir type conflicts, the destination path is replaced.
func MergeDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			if existing, statErr := os.Stat(target); statErr == nil && !existing.IsDir() {
				if err := os.Remove(target); err != nil {
					return err
				}
			}
			return os.MkdirAll(target, info.Mode())
		}
		if existing, statErr := os.Stat(target); statErr == nil && existing.IsDir() {
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
		return copyFile(path, target, info.Mode())
	})
}

// WriteJSON writes pretty JSON to path, creating parent directories.
func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

// ReadJSON reads JSON from path into dst. Missing files are not errors.
func ReadJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	return json.Unmarshal(data, dst)
}

// EnsureManagedTableEntry adds or updates a simple TOML boolean key in a table.
func EnsureManagedTableEntry(content, header, key string, value bool) string {
	lines := strings.Split(content, "\n")
	sectionLine := "[" + header + "]"
	valueLine := fmt.Sprintf("%s = %t", key, value)
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != sectionLine {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				lines = append(lines[:j], append([]string{valueLine}, lines[j:]...)...)
				return strings.Join(lines, "\n")
			}
			if strings.HasPrefix(trimmed, key+" =") {
				lines[j] = valueLine
				return strings.Join(lines, "\n")
			}
		}
		lines = append(lines, valueLine)
		return strings.Join(lines, "\n")
	}
	if strings.TrimSpace(content) != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + sectionLine + "\n" + valueLine + "\n"
}

// EnsureManagedTOMLBool adds or updates a boolean key in a TOML table path.
// The implementation is line-oriented but table-aware, so unrelated content is preserved.
func EnsureManagedTOMLBool(content string, tablePath []string, key string, value bool) string {
	sectionLine := "[" + strings.Join(tablePath, ".") + "]"
	valueLine := fmt.Sprintf("%s = %t", key, value)
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != sectionLine {
			continue
		}
		insertAt := len(lines)
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				insertAt = j
				break
			}
			if strings.HasPrefix(trimmed, key+" =") {
				lines[j] = valueLine
				return strings.Join(lines, "\n")
			}
		}
		lines = append(lines[:insertAt], append([]string{valueLine}, lines[insertAt:]...)...)
		return strings.Join(lines, "\n")
	}
	if strings.TrimSpace(content) != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if strings.TrimSpace(content) != "" {
		content += "\n"
	}
	return content + sectionLine + "\n" + valueLine + "\n"
}

// ManagedJSONMapMerge rewrites a top-level object key containing event arrays.
// Unmanaged entries are kept, managed entries are dropped when shouldRemove returns true,
// and replacement entries are appended in sorted key order for stable output.
func ManagedJSONMapMerge(current map[string][]map[string]any, replacements map[string][]map[string]any, shouldRemove func(map[string]any) bool) map[string][]map[string]any {
	out := make(map[string][]map[string]any, len(current)+len(replacements))
	for event, entries := range current {
		var kept []map[string]any
		for _, entry := range entries {
			if shouldRemove != nil && shouldRemove(entry) {
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) > 0 {
			out[event] = kept
		}
	}
	keys := make([]string, 0, len(replacements))
	for event := range replacements {
		keys = append(keys, event)
	}
	sort.Strings(keys)
	for _, event := range keys {
		out[event] = append(out[event], replacements[event]...)
	}
	return out
}
