package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitStatusPaths returns every path git status reports, tracked or not.
func gitStatusPaths(dir string) map[string]bool {
	cmd := exec.Command("git", "status", "--porcelain", "-z", "-uall", "--no-renames")
	cmd.Dir = dir
	out, err := cmd.Output()
	paths := map[string]bool{}
	if err != nil {
		return paths
	}
	for entry := range strings.SplitSeq(string(out), "\x00") {
		if len(entry) > 3 {
			paths[entry[3:]] = true
		}
	}
	return paths
}

// GitCheckoutRecovery snapshots dirty paths and returns best-effort cleanup for a
// failed checkout. Only previously clean paths holding the incoming revision are
// restored, so compatible user edits are never touched.
func GitCheckoutRecovery(dir, revision string) func() {
	dirtyBefore := gitStatusPaths(dir)
	return func() {
		blob := func(rev, path string) string {
			cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", rev+":"+path)
			cmd.Dir = dir
			out, _ := cmd.Output() // empty when the path is absent at rev
			return strings.TrimSpace(string(out))
		}
		for path := range gitStatusPaths(dir) {
			if dirtyBefore[path] {
				continue
			}
			upstream := blob(revision, path)
			full := filepath.Join(dir, path)
			if info, err := os.Lstat(full); err == nil {
				hash := exec.Command("git", "hash-object", "--", path)
				if info.Mode()&os.ModeSymlink != 0 {
					target, err := os.Readlink(full)
					if err != nil {
						continue
					}
					hash = exec.Command("git", "hash-object", "--stdin")
					hash.Stdin = strings.NewReader(target)
				}
				hash.Dir = dir
				out, err := hash.Output()
				if err != nil || upstream == "" || strings.TrimSpace(string(out)) != upstream {
					continue
				}
			} else if upstream != "" {
				continue // missing here but present upstream: not the pull's doing
			}
			if blob("HEAD", path) != "" {
				restore := exec.Command("git", "checkout", "HEAD", "--", ":(literal)"+path)
				restore.Dir = dir
				restore.Run() // best-effort; the pull error is what gets reported
				continue
			}
			unstage := exec.Command("git", "rm", "--cached", "--quiet", "--ignore-unmatch", "--", ":(literal)"+path)
			unstage.Dir = dir
			unstage.Run()
			clean := exec.Command("git", "clean", "-f", "--", ":(literal)"+path)
			clean.Dir = dir
			clean.Run() // only this newly introduced path, never a broad clean
		}
	}
}
