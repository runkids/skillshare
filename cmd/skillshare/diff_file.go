package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"

	"skillshare/internal/install"
)

// fileDiffEntry represents a single file-level difference between source and destination.
type fileDiffEntry struct {
	RelPath  string
	Action   string // "add", "modify", "delete"
	SrcSize  int64
	DstSize  int64
	SrcMtime time.Time
	DstMtime time.Time
}

// diffSkillFiles walks both directories and returns file-level diffs.
// src-only = add, dst-only = delete, both but different content = modify.
// Results are sorted by RelPath. If dstDir doesn't exist, all src files are "add".
func diffSkillFiles(srcDir, dstDir string) []fileDiffEntry {
	srcFiles := walkFiles(srcDir)
	dstFiles := walkFiles(dstDir)

	entries := make([]fileDiffEntry, 0)

	// Files in source
	for rel, srcInfo := range srcFiles {
		dstInfo, inDst := dstFiles[rel]
		if !inDst {
			entries = append(entries, fileDiffEntry{
				RelPath:  rel,
				Action:   "add",
				SrcSize:  srcInfo.Size(),
				SrcMtime: srcInfo.ModTime(),
			})
			continue
		}
		// Both exist — check if content differs
		if srcInfo.Size() != dstInfo.Size() || !fileContentsEqual(filepath.Join(srcDir, filepath.FromSlash(rel)), filepath.Join(dstDir, filepath.FromSlash(rel))) {
			entries = append(entries, fileDiffEntry{
				RelPath:  rel,
				Action:   "modify",
				SrcSize:  srcInfo.Size(),
				DstSize:  dstInfo.Size(),
				SrcMtime: srcInfo.ModTime(),
				DstMtime: dstInfo.ModTime(),
			})
		}
	}

	// Files only in destination
	for rel, dstInfo := range dstFiles {
		if _, inSrc := srcFiles[rel]; !inSrc {
			entries = append(entries, fileDiffEntry{
				RelPath:  rel,
				Action:   "delete",
				DstSize:  dstInfo.Size(),
				DstMtime: dstInfo.ModTime(),
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].RelPath < entries[j].RelPath
	})

	return entries
}

// walkFiles walks a directory and returns a map of relPath -> FileInfo.
// Skips .git directories and .skillshare-meta.json. Returns empty map if root doesn't exist.
func walkFiles(root string) map[string]os.FileInfo {
	result := make(map[string]os.FileInfo)
	if root == "" {
		return result
	}
	if _, err := os.Stat(root); err != nil {
		return result
	}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == install.MetaFileName {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		// Normalize path separators to forward slash
		rel = strings.ReplaceAll(rel, "\\", "/")
		result[rel] = info
		return nil
	})

	return result
}

// fileContentsEqual compares two files by streaming byte comparison.
// Returns false if either file is unreadable or contents differ.
func fileContentsEqual(pathA, pathB string) bool {
	fA, err := os.Open(pathA)
	if err != nil {
		return false
	}
	defer fA.Close()

	fB, err := os.Open(pathB)
	if err != nil {
		return false
	}
	defer fB.Close()

	const bufSize = 32 * 1024
	bufA := make([]byte, bufSize)
	bufB := make([]byte, bufSize)
	for {
		nA, errA := fA.Read(bufA)
		nB, errB := fB.Read(bufB)
		if nA != nB || !bytes.Equal(bufA[:nA], bufB[:nB]) {
			return false
		}
		if errA == io.EOF && errB == io.EOF {
			return true
		}
		if errA != nil || errB != nil {
			return false
		}
	}
}

// maxDiffFileSize is the maximum file size for generating unified diffs.
// Files larger than this are skipped to avoid performance issues.
const maxDiffFileSize = 1_000_000 // 1MB

// generateUnifiedDiff produces a unified diff between two files.
// dstContent is treated as "old", srcContent as "new".
// Returns "(binary file differs)" if either file contains null bytes in the first 8000 chars.
// Returns "(file too large for diff)" if either file exceeds maxDiffFileSize.
func generateUnifiedDiff(srcPath, dstPath string) string {
	if fileExceedsSize(srcPath, maxDiffFileSize) || fileExceedsSize(dstPath, maxDiffFileSize) {
		return "(file too large for diff)"
	}

	srcContent := readFileString(srcPath)
	dstContent := readFileString(dstPath)

	if isBinary(srcContent) || isBinary(dstContent) {
		return "(binary file differs)"
	}

	dmp := diffmatchpatch.New()
	// dstContent is "old", srcContent is "new"
	a, b, lineArray := dmp.DiffLinesToChars(dstContent, srcContent)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	diffs = dmp.DiffCleanupSemantic(diffs)

	return formatUnifiedDiff(diffs)
}

// readFileString reads a file and returns its content as a string.
// Returns "" on error.
func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// fileExceedsSize returns true if the file at path is larger than limit bytes.
// Returns false if the file doesn't exist or can't be stat'd.
func fileExceedsSize(path string, limit int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() > limit
}

// isBinary checks whether a string contains null bytes in the first 8000 characters.
func isBinary(content string) bool {
	limit := len(content)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// formatUnifiedDiff formats diffmatchpatch.Diff slices as a unified diff string.
// Insert lines are prefixed with "+ ", Delete lines with "- ", Equal lines with "  ".
func formatUnifiedDiff(diffs []diffmatchpatch.Diff) string {
	var b strings.Builder

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		// If the text ends with \n, Split produces a trailing empty string.
		// Don't emit a line for that trailing empty element.
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		var prefix string
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			prefix = "+ "
		case diffmatchpatch.DiffDelete:
			prefix = "- "
		case diffmatchpatch.DiffEqual:
			prefix = "  "
		}

		for _, line := range lines {
			b.WriteString(prefix)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderFileStat returns a git-style stat summary for file diffs.
//
//	add:    "      + relpath (N bytes)"
//	delete: "      - relpath (N bytes)"
//	modify: "      ~ relpath (old -> new bytes)"
func renderFileStat(files []fileDiffEntry) string {
	var b strings.Builder
	for _, f := range files {
		switch f.Action {
		case "add":
			fmt.Fprintf(&b, "      + %s (%d bytes)\n", f.RelPath, f.SrcSize)
		case "delete":
			fmt.Fprintf(&b, "      - %s (%d bytes)\n", f.RelPath, f.DstSize)
		case "modify":
			fmt.Fprintf(&b, "      ~ %s (%d → %d bytes)\n", f.RelPath, f.DstSize, f.SrcSize)
		}
	}
	return b.String()
}
