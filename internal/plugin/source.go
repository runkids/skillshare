package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var githubPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

// acquire checks out remote sources in an ephemeral directory, without running
// repository code or registering anything with an Agent. Callers must clean up.
func acquire(ctx context.Context, source string) (string, string, func(), error) {
	cleanup := func() {}
	if strings.HasPrefix(source, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", cleanup, err
		}
		source = filepath.Join(home, source[2:])
	}
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		p, err := filepath.Abs(source)
		return p, p, cleanup, err
	}
	if githubPattern.MatchString(source) {
		source = "https://github.com/" + source + ".git"
	}
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", cleanup, fmt.Errorf("use an existing directory, owner/repo, or HTTPS Git URL without credentials")
	}
	root, err := os.MkdirTemp("", "skillshare-plugin-")
	if err != nil {
		return "", "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(root) }
	_, err = runCommand(ctx, "", "git", "-c", "core.hooksPath=/dev/null", "clone", "--depth", "1", "--", source, root)
	if err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("download plugin source: %w", err)
	}
	return root, source, cleanup, nil
}

// treeDigest rejects links and special files, bounding both preview and copying.
// The entire package tree is retained, including scripts and referenced assets.
func treeDigest(root string) (string, error) {
	h := sha256.New()
	var size int64
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported link or special file: %s", rel)
		}
		count++
		size += info.Size()
		if count > 20000 || size > 100*1024*1024 {
			return fmt.Errorf("plugin source exceeds 20,000 files or 100 MiB")
		}
		fmt.Fprintf(h, "%s\x00%d\x00%d\x00", filepath.ToSlash(rel), info.Mode().Perm(), info.Size())
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(h, f)
		_ = f.Close()
		return err
	})
	return hex.EncodeToString(h.Sum(nil)), err
}

func Discover(ctx context.Context, source string) (*Discovery, error) {
	root, normalized, cleanup, err := acquire(ctx, source)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return discoverRoot(root, normalized)
}

func discoverRoot(root, source string) (*Discovery, error) {
	digest, err := treeDigest(root)
	if err != nil {
		return nil, err
	}
	result := &Discovery{Source: source, Digest: digest, Candidates: []Candidate{}}
	// Prefer the portable/Codex catalog when a repository publishes both.
	for _, path := range []string{".agents/plugins/marketplace.json", ".claude-plugin/marketplace.json", ".cursor-plugin/marketplace.json"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var catalog struct {
			Name    string `json:"name"`
			Plugins []struct {
				Name   string          `json:"name"`
				Source json.RawMessage `json:"source"`
			} `json:"plugins"`
		}
		if err := json.Unmarshal(data, &catalog); err != nil {
			return nil, fmt.Errorf("invalid marketplace: %w", err)
		}
		if !namePattern.MatchString(catalog.Name) {
			return nil, fmt.Errorf("marketplace requires a valid name")
		}
		seen := map[string]bool{}
		for _, entry := range catalog.Plugins {
			if !namePattern.MatchString(entry.Name) || seen[entry.Name] {
				return nil, fmt.Errorf("invalid or duplicate plugin name: %s", entry.Name)
			}
			seen[entry.Name] = true
			var rel string
			if json.Unmarshal(entry.Source, &rel) != nil {
				var src struct {
					Source string `json:"source"`
					Path   string `json:"path"`
				}
				_ = json.Unmarshal(entry.Source, &src)
				if src.Source == "local" {
					rel = src.Path
				}
			}
			c := Candidate{Name: entry.Name, Marketplace: catalog.Name, Targets: []string{}, Components: []string{}}
			if rel == "" {
				c.Problem = "This entry uses an external source. Add its Git repository directly, or install with the native client and import it."
			} else {
				clean := filepath.Clean(rel)
				if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
					return nil, fmt.Errorf("plugin path escapes marketplace: %s", rel)
				}
				c, err = inspect(filepath.Join(root, clean))
				if err != nil {
					return nil, fmt.Errorf("%s: %w", entry.Name, err)
				}
				if c.Name != entry.Name {
					return nil, fmt.Errorf("catalog and manifest names differ for %s", entry.Name)
				}
				c.Path = filepath.ToSlash(clean)
				c.Marketplace = catalog.Name
			}
			result.Candidates = append(result.Candidates, c)
		}
		return result, nil
	}
	c, err := inspect(root)
	if err != nil {
		return nil, err
	}
	c.Path = "."
	result.Candidates = append(result.Candidates, c)
	return result, nil
}

func copyTree(root, dest string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		to := filepath.Join(dest, rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(to, 0755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(to, data, info.Mode().Perm())
	})
}
