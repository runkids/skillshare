package plugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"skillshare/internal/utils"
)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
var githubPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`)

// Codex accepts only ASCII letters, digits, `_`, and `-` in marketplace names.
var marketUnsafe = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// acquire checks out remote sources in an ephemeral directory, without running
// repository code or registering anything with an Agent. Callers must clean up.
func acquire(ctx context.Context, source string) (string, string, func(), error) {
	return acquireRef(ctx, source, "")
}

func acquireRef(ctx context.Context, source, ref string) (string, string, func(), error) {
	cleanup := func() {}
	if strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\x00\r\n ") {
		return "", "", cleanup, fmt.Errorf("invalid source ref")
	}
	if strings.HasPrefix(source, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", cleanup, err
		}
		source = filepath.Join(home, source[2:])
	}
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		if ref != "" {
			return "", "", cleanup, fmt.Errorf("source ref requires a remote Git source")
		}
		p, err := filepath.Abs(source)
		return p, p, cleanup, err
	}
	if isNpmSource(source) {
		return "", "", cleanup, fmt.Errorf("npm packages can't be previewed: Pi downloads them when it installs them. Add one to a Pi target with: skillshare plugin add %s --target pi", source)
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
	_, err = runCommand(ctx, "", nil, "git", "-c", "core.hooksPath=/dev/null", "clone", "--depth", "1", "--", source, root)
	if err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("download plugin source: %w", err)
	}
	if ref != "" {
		if _, err = runCommand(ctx, root, nil, "git", "fetch", "--depth", "1", "origin", ref); err == nil {
			_, err = runCommand(ctx, root, nil, "git", "-c", "core.hooksPath=/dev/null", "checkout", "--detach", "FETCH_HEAD")
		}
		if err != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("resolve source ref: %w", err)
		}
	}
	return root, source, cleanup, nil
}

// treeDigest includes safe relative links, leaves out unsafe ones and rejects special files.
// The entire package tree is retained, including scripts and referenced assets.
func treeDigest(root string) (string, error) {
	digest, _, err := hashTree(root)
	return digest, err
}

// hashTree is treeDigest plus the links it left out, relative to root.
func hashTree(root string) (string, map[string]bool, error) {
	skipped, err := validateSourceLinks(root)
	if err != nil {
		return "", nil, err
	}
	h := sha256.New()
	var size int64
	count := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
		if skipped[rel] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if utils.IsLinkMode(path, info.Mode()) {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00link\x00%s\x00", filepath.ToSlash(rel), link)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported special file: %s", rel)
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
	return hex.EncodeToString(h.Sum(nil)), skipped, err
}

func Discover(ctx context.Context, source string) (*Discovery, error) {
	return DiscoverRef(ctx, source, "")
}

func DiscoverRef(ctx context.Context, source, ref string) (*Discovery, error) {
	return DiscoverOptions(ctx, source, ref, "")
}

func DiscoverOptions(ctx context.Context, source, ref, entry string) (*Discovery, error) {
	root, normalized, cleanup, err := acquireRef(ctx, source, ref)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	d, err := discoverRoot(root, normalized, entry)
	if err == nil {
		d.SourceRef = ref
		if strings.HasPrefix(normalized, "https://") {
			if commit, e := runCommand(ctx, root, nil, "git", "rev-parse", "HEAD"); e == nil {
				d.Commit = strings.TrimSpace(string(commit))
			} else {
				return nil, e
			}
		}
	}
	return d, err
}

func discoverRoot(root, source string, explicit ...string) (*Discovery, error) {
	digest, skipped, err := hashTree(root)
	if err != nil {
		return nil, err
	}
	result := &Discovery{TargetDefinitions: TargetDefinitions(), Source: source, Digest: digest, Candidates: []Candidate{}}
	if len(skipped) > 0 {
		links := slices.Sorted(maps.Keys(skipped))
		if len(links) > 3 {
			links = append(links[:3], "...")
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("Skipped %d broken or unsafe links; they are left out of the install: %s", len(skipped), strings.Join(links, ", ")))
	}
	seenAt := map[string]int{}
	// Read all native catalogs; the same package may be advertised more than once.
	for _, path := range []string{".agents/plugins/marketplace.json", ".claude-plugin/marketplace.json", ".cursor-plugin/marketplace.json", ".github/plugin/marketplace.json", ".plugin/marketplace.json", "marketplace.json"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var catalog struct {
			Name    string                       `json:"name"`
			Plugins []map[string]json.RawMessage `json:"plugins"`
		}
		if err := json.Unmarshal(data, &catalog); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: invalid marketplace: %v", path, err))
			continue
		}
		if !namePattern.MatchString(catalog.Name) {
			result.Warnings = append(result.Warnings, path+": marketplace requires a valid name")
			continue
		}
		seen := map[string]bool{}
		for _, fields := range catalog.Plugins {
			var name string
			_ = json.Unmarshal(fields["name"], &name)
			if !namePattern.MatchString(name) || seen[name] {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: invalid or duplicate plugin name: %s", path, name))
				continue
			}
			seen[name] = true
			var rel string
			if json.Unmarshal(fields["source"], &rel) != nil {
				var src struct {
					Source string `json:"source"`
					Path   string `json:"path"`
					URL    string `json:"url"`
				}
				_ = json.Unmarshal(fields["source"], &src)
				if src.Source == "local" {
					rel = src.Path
				} else if src.Source == "url" && (src.URL == "." || strings.HasPrefix(src.URL, "./")) {
					rel = src.URL
				}
			}
			// One broken entry marks its own row; the rest of the marketplace stays installable.
			c := Candidate{Name: name, Marketplace: catalog.Name, Targets: []string{}, Components: []string{}}
			clean := filepath.Clean(rel)
			switch {
			case rel == "":
				c.block("plugins.problem.externalSource", "This entry uses an external source. Add its Git repository directly, or install with the native client and import it.", nil)
			case filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)):
				c.block("plugins.problem.pathEscapes", "Plugin path escapes the marketplace: "+rel, map[string]string{"path": rel})
			default:
				dir := filepath.Join(root, clean)
				found, err := inspect(dir, explicit...)
				_, claudeManifest := found.TargetInfo["claude"]
				switch {
				case err != nil && path == ".claude-plugin/marketplace.json" && !claudeManifest:
					// Claude makes plugin.json optional; the catalog entry then names and defines the plugin.
					found = claudeCatalogCandidate(dir, name, fields)
				case err != nil:
					found.block("plugins.problem.noManifest", name+": "+err.Error(), nil)
				case found.Name != name:
					// Claude installs under the catalog's name; Codex refuses the mismatch.
					// ponytail: other Agents are unverified, so they are blocked too; unblock one once its CLI is checked.
					args := map[string]string{"name": name, "manifest": found.Name}
					for target, info := range found.TargetInfo {
						if target != "claude" && info.Problem == "" {
							info.block("plugins.problem.catalogNameDiffers", "Catalog and manifest names differ for "+name, args)
							found.TargetInfo[target] = info
						}
					}
					found.collectTargets()
					if len(found.Targets) == 0 {
						found.block("plugins.problem.catalogNameDiffers", "Catalog and manifest names differ for "+name, args)
					}
				}
				c = found
				c.Name, c.Path, c.Marketplace = name, filepath.ToSlash(clean), catalog.Name
				if path == ".claude-plugin/marketplace.json" {
					c.catalogEntry = claudeEntryFields(fields)
				}
			}
			if i, ok := seenAt[c.Name]; ok {
				// An external entry has no path; a catalog that points inside the source wins over it.
				// The same folder may be readable by one catalog only (Claude allows no plugin.json).
				switch previous := result.Candidates[i].Path; {
				case c.Path == "" || (c.Path == previous && (c.Problem != "" || result.Candidates[i].Problem == "")):
				case previous == "" || c.Path == previous:
					result.Candidates[i] = c
				case result.Candidates[i].Problem != "":
					result.Candidates[i] = c
				case c.Problem == "":
					mergeCatalogPath(&result.Candidates[i], c, catalogOwner[path])
				}
				continue
			}
			seenAt[c.Name] = len(result.Candidates)
			result.Candidates = append(result.Candidates, c)
		}
	}
	if len(result.Candidates) > 0 {
		return result, nil
	}
	c, err := inspect(root, explicit...)
	if err != nil {
		return nil, err
	}
	c.Path = "."
	result.Candidates = append(result.Candidates, c)
	return result, nil
}

// catalogOwner is the Agent each native catalog is written for.
var catalogOwner = map[string]string{".agents/plugins/marketplace.json": "codex", ".claude-plugin/marketplace.json": "claude", ".cursor-plugin/marketplace.json": "cursor", ".github/plugin/marketplace.json": "copilot", ".plugin/marketplace.json": "copilot"}

// mergeCatalogPath folds in the same plugin that another catalog places in another folder
// (one folder per Agent). An Agent keeps the folder found first, unless its own catalog says otherwise.
func mergeCatalogPath(kept *Candidate, c Candidate, owner string) {
	for target, info := range c.TargetInfo {
		if existing, ok := kept.TargetInfo[target]; info.Problem != "" || (ok && existing.Problem == "" && target != owner) {
			continue
		}
		info.Path = c.Path
		kept.TargetInfo[target] = info
	}
	if kept.catalogEntry == nil {
		kept.catalogEntry = c.catalogEntry
	}
	kept.collectTargets()
}

func copyTree(root, dest string) error {
	_, skipped, err := hashTree(root)
	if err != nil {
		return err
	}
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
		if skipped[rel] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if utils.IsLinkMode(path, info.Mode()) {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, to)
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
