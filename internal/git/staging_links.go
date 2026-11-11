package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"skillshare/internal/gitenv"
	"skillshare/internal/sourcewalk"
	"skillshare/internal/utils"
)

// SourceLinkWarnings describes first-level links that staging will record. It is
// advisory: neither the index nor ignore files are changed, including in previews.
func SourceLinkWarnings(root, skills, operation string) []string {
	rel, err := filepath.Rel(root, skills)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	// Staging inspects link entries regardless of the discovery follow policy.
	entries, err := sourcewalk.ReadDir(skills, sourcewalk.Options{})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("%s: cannot inspect source links: %v", operation, err)}
	}
	var warnings []string
	for _, entry := range entries {
		if !utils.IsLinkMode(filepath.Join(skills, entry.Name()), entry.Type()) {
			continue
		}
		path := filepath.ToSlash(filepath.Join(rel, entry.Name()))
		indexed := gitenv.Command("--literal-pathspecs", "ls-files", "--error-unmatch", "--", path)
		indexed.Dir = root
		inIndex := indexed.Run() == nil
		if !inIndex {
			ignored := gitenv.Command("check-ignore", "-q", "--", path)
			ignored.Dir = root
			if ignored.Run() == nil {
				continue
			}
		}
		pattern := "/" + strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", " ", "\\ ").Replace(path)
		warning := fmt.Sprintf("%s will stage source link %q from %q; machine-local link targets may break on other machines. Add `%s` to %s", operation, path, root, pattern, filepath.Join(root, ".gitignore"))
		if inIndex {
			warning += fmt.Sprintf("; this link is already indexed, so adding an ignore pattern will not untrack it. To untrack manually, run from the git root: git rm --cached -- %s", shellQuoteLink(":(literal)"+path))
		}
		warnings = append(warnings, warning)
	}
	return warnings
}

func shellQuoteLink(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}
