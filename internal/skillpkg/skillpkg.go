// Package skillpkg reads one skill directory as an Agent Skills package: its
// full frontmatter and every regular file, within the SEP-2640 limits. It has
// no configuration dependencies, so sync can share its naming rules.
package skillpkg

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// Per-skill limits from SEP-2640, and the Agent Skills field limits in characters.
const (
	MaxFiles         = 512
	MaxBytes         = 16 << 20
	MaxDescription   = 1024
	MaxCompatibility = 500
)

// File is one regular file of a package. Digest and Size come from one read, so
// they describe the same bytes even when the file changes while it is loaded.
type File struct {
	Path   string // absolute
	Rel    string // slash-separated, relative to the package directory
	Digest string // sha256:<hex>
	Size   int64
	Text   bool // whether the accepted bytes are valid UTF-8, matching resource reads
}

// Package is a valid skill directory. Its SKILL.md entry is the read the
// frontmatter was parsed from.
type Package struct {
	Dir         string // resolved
	Frontmatter map[string]any
	Files       []File
	Directories []string // slash-relative real directories; "." is the package root
}

var errTooLarge = fmt.Errorf("is larger than %d MiB", MaxBytes>>20)

// Load reads the skill in dir. The error says why the directory is not a
// valid package. .git directories are left out; links are neither listed nor read.
func Load(dir string) (*Package, error) {
	dirName := filepath.Base(filepath.Clean(dir)) // a followed link's name, not its target's
	dir = utils.ResolveSymlink(dir)
	// A linked SKILL.md would be missing from the manifest, which lists no links.
	skillPath := filepath.Join(dir, "SKILL.md")
	info, err := os.Lstat(skillPath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("SKILL.md is not a regular file")
	}
	content, err := readAtMost(skillPath, info, MaxBytes)
	if err != nil {
		return nil, err
	}
	fm, err := utils.ParseFrontmatterMap(content)
	if err != nil {
		return nil, fmt.Errorf("SKILL.md has %w", err)
	}
	name, _ := fm["name"].(string)
	if reason := ValidateName(name, dirName); reason != "" {
		return nil, fmt.Errorf("%s", reason)
	}
	desc, _ := fm["description"].(string)
	if strings.TrimSpace(desc) == "" {
		return nil, fmt.Errorf("SKILL.md is missing a description")
	}
	if n := utf8.RuneCountInString(desc); n > MaxDescription {
		return nil, fmt.Errorf("description has %d characters; the limit is %d", n, MaxDescription)
	}
	// metadata is left unchecked: the format wants string values, but lists there are
	// common and tool clients read them fine, so skipping those skills would cost more.
	compat, ok := fm["compatibility"].(string)
	if _, set := fm["compatibility"]; set && (!ok || compat == "") {
		return nil, fmt.Errorf("compatibility must be non-empty text")
	}
	if n := utf8.RuneCountInString(compat); n > MaxCompatibility {
		return nil, fmt.Errorf("compatibility has %d characters; the limit is %d", n, MaxCompatibility)
	}

	p := &Package{Dir: dir, Frontmatter: fm, Files: []File{{Path: skillPath, Rel: "SKILL.md", Digest: Digest(content), Size: int64(len(content)), Text: utf8.Valid(content)}}}
	// Every read is bounded by what is left of the package budget, so a file that
	// grows while it is read cannot push the package past the limit.
	left := MaxBytes - int64(len(content))
	err = sourcewalk.Walk(dir, sourcewalk.Options{}, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			p.Directories = append(p.Directories, filepath.ToSlash(rel))
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "SKILL.md" { // SKILL.md is listed from the read above
			return err
		}
		if len(p.Files) == MaxFiles {
			return fmt.Errorf("has more than %d files", MaxFiles)
		}
		sum, n, text, err := hashAtMost(path, info, left)
		if err != nil {
			return err
		}
		left -= n
		p.Files = append(p.Files, File{Path: path, Rel: filepath.ToSlash(rel), Digest: sum, Size: n, Text: text})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Digest is the manifest digest of b.
func Digest(b []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }

// openSame opens path and checks that it is the file info (from Lstat) described,
// so a file swapped for a link after it was checked is not followed.
func openSame(path string, info os.FileInfo) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if got, err := f.Stat(); err != nil || !os.SameFile(info, got) {
		f.Close()
		return nil, fmt.Errorf("%s changed while it was read", filepath.Base(path))
	}
	return f, nil
}

// readAtMost reads the file, failing past limit bytes.
func readAtMost(path string, info os.FileInfo, limit int64) ([]byte, error) {
	f, err := openSame(path, info)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errTooLarge
	}
	return data, nil
}

// hashAtMost inspects the same bounded stream for digest, size and UTF-8 validity.
func hashAtMost(path string, info os.FileInfo, limit int64) (string, int64, bool, error) {
	f, err := openSame(path, info)
	if err != nil {
		return "", 0, false, err
	}
	defer f.Close()
	h := sha256.New()
	r := io.LimitReader(f, limit+1)
	var buf [32 << 10]byte
	var n int64
	pending, text := 0, true
	for {
		nr, readErr := r.Read(buf[pending:])
		h.Write(buf[pending : pending+nr])
		n += int64(nr)
		if text {
			end := pending + nr
			cut := end
			// Keep only a trailing incomplete rune (at most three bytes).
			// FullRune also recognizes invalid encodings, which Valid rejects.
			for cut > 0 && end-cut < utf8.UTFMax-1 && !utf8.RuneStart(buf[cut-1]) {
				cut--
			}
			if cut > 0 && !utf8.FullRune(buf[cut-1:end]) {
				cut--
			} else {
				cut = end
			}
			text = utf8.Valid(buf[:cut])
			pending = copy(buf[:], buf[cut:end])
		}
		if readErr == io.EOF {
			text = text && pending == 0
			break
		}
		if readErr != nil {
			return "", 0, false, readErr
		}
		if !text {
			pending = 0
		}
	}
	if n > limit {
		return "", 0, false, errTooLarge
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), n, text, nil
}

// ValidateName returns why a SKILL.md name breaks the Agent Skills naming rules
// (including name == directory name), or "" when it follows them. Like the
// reference validator, it checks lowercase Unicode letters and digits after NFKC
// normalization and counts the limit in characters.
func ValidateName(name, dirName string) string {
	// Not trimmed: surrounding whitespace is not a letter, digit or hyphen, so it fails below.
	exact := name
	name = norm.NFKC.String(name)
	if name == "" {
		return "SKILL.md is missing a name"
	}
	if utf8.RuneCountInString(name) > 64 {
		return fmt.Sprintf("SKILL.md name %q is longer than 64 characters", name)
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return fmt.Sprintf("SKILL.md name %q cannot start or end with '-'", name)
	}
	if strings.Contains(name, "--") {
		return fmt.Sprintf("SKILL.md name %q cannot contain consecutive hyphens", name)
	}
	if name != strings.ToLower(name) || strings.ContainsFunc(name, func(r rune) bool {
		return r != '-' && !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		return fmt.Sprintf("SKILL.md name %q must use only lowercase letters, numbers, and hyphens", name)
	}
	// Exactly, not only after NFKC: the directory is the skill path's last segment,
	// which the Skills extension requires to equal the name.
	if exact != dirName {
		return fmt.Sprintf("SKILL.md name %q does not match directory name %q", exact, dirName)
	}
	return ""
}
