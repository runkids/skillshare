package skillserve

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// IndexPath is where the skill index for Executor's wellKnownSkills loader is
// served. The format is the one that loader reads (Executor 570209c5:
// {skills: [{name, version, files}]}), not the agent-skills discovery RFC v0.2.
const IndexPath = "/.well-known/agent-skills/"

// Executor's skill loader limits for one load, which reads the index twice.
const (
	indexMaxFiles     = 1000
	indexMaxFileBytes = 2_000_000
	indexMaxBytes     = 20_000_000
)

// executorName is Executor's skill name rule, checked on the exact name.
var executorName = regexp.MustCompile(`^[\p{L}\p{N}]+(?:-[\p{L}\p{N}]+)*$`)

type indexEntry struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

// Index is the view of one catalog that Executor can load. One skill Executor
// rejects fails its whole load, so such skills are left out and listed in Skipped.
type Index struct {
	JSON    []byte
	Skills  int
	Files   int
	Bytes   int64
	Skipped []Skip
	// Err is set when the index as a whole is over Executor's limits.
	Err   error
	files map[string]string // "<name>/<path>" -> catalog URI
}

// Index builds the Executor index of c. It reads every listed file, to leave
// out skills with files Executor cannot decode as UTF-8 text.
func (c *Catalog) Index() *Index {
	idx := &Index{files: map[string]string{}}
	uses := map[string]int{}
	for _, s := range c.Skills {
		uses[skillName(s)]++
	}
	entries := []indexEntry{}
	for _, s := range c.Skills {
		name := skillName(s)
		reason := ""
		if n := uses[name]; n > 1 {
			reason = fmt.Sprintf("name %q is used by %d skills; Executor needs unique names", name, n)
		} else {
			reason = c.executorReason(s, name)
		}
		if reason != "" {
			idx.Skipped = append(idx.Skipped, Skip{Path: s.src.RelPath, Reason: reason})
			continue
		}
		e := indexEntry{Name: name}
		lines := make([]string, 0, len(s.Resources))
		for _, f := range s.Resources {
			e.Files = append(e.Files, f.rel)
			lines = append(lines, f.rel+"\x00"+f.Digest+"\n")
			idx.files[name+"/"+f.rel] = f.URI
			idx.Files++
			idx.Bytes += f.Size
		}
		slices.Sort(e.Files)
		slices.Sort(lines)
		e.Version = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(strings.Join(lines, ""))))
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(x, y indexEntry) int { return strings.Compare(x.Name, y.Name) })
	slices.SortFunc(idx.Skipped, func(x, y Skip) int { return strings.Compare(x.Path, y.Path) })
	idx.Skills = len(entries)
	idx.JSON, _ = json.Marshal(map[string]any{"skills": entries})
	switch {
	case idx.Files > indexMaxFiles:
		idx.Err = fmt.Errorf("the index lists %d files; Executor loads at most %d. Serve fewer skills with --target", idx.Files, indexMaxFiles)
	case len(idx.JSON) > indexMaxFileBytes:
		idx.Err = fmt.Errorf("the index is %d bytes; Executor reads at most %d. Serve fewer skills with --target", len(idx.JSON), indexMaxFileBytes)
	case idx.Bytes+2*int64(len(idx.JSON)) > indexMaxBytes:
		idx.Err = fmt.Errorf("the skills and index are %d bytes; Executor loads at most %d. Serve fewer skills with --target", idx.Bytes+2*int64(len(idx.JSON)), indexMaxBytes)
	}
	return idx
}

func skillName(s *Skill) string {
	name, _ := s.Frontmatter["name"].(string)
	return name
}

// executorReason returns why Executor would reject a skill that skillpkg
// accepted, or "".
func (c *Catalog) executorReason(s *Skill, name string) string {
	if strings.ToLower(name) != name || !executorName.MatchString(name) {
		return fmt.Sprintf("Executor needs a lowercase name of letters, digits and single hyphens, not %q", name)
	}
	for _, key := range []string{"license", "allowed-tools"} {
		if v, set := s.Frontmatter[key]; set {
			if _, ok := v.(string); !ok {
				return fmt.Sprintf("Executor needs %s to be text", key)
			}
		}
	}
	if v, set := s.Frontmatter["metadata"]; set {
		m, ok := v.(map[string]any)
		if !ok {
			return "Executor needs metadata to map keys to text"
		}
		for k, mv := range m {
			if _, ok := mv.(string); !ok {
				return fmt.Sprintf("Executor needs metadata.%s to be text", k)
			}
		}
	}
	for _, f := range s.Resources {
		rel := f.rel
		if !executorPath(rel) {
			return fmt.Sprintf("Executor cannot load the file name %q", rel)
		}
		if f.Size > indexMaxFileBytes {
			return fmt.Sprintf("%s is larger than Executor's %d-byte file limit", rel, indexMaxFileBytes)
		}
		data, err := c.Read(f.URI)
		if err != nil {
			return err.Error()
		}
		if !utf8.Valid(data) {
			return fmt.Sprintf("%s is not UTF-8 text, which Executor cannot load", rel)
		}
	}
	return ""
}

// executorPath mirrors Executor's resourcePath: each segment must be UTF-8 and
// survive decodeURIComponent without becoming empty, ".", "..", or holding a separator.
func executorPath(rel string) bool {
	if !utf8.ValidString(rel) || strings.ContainsAny(rel, "\\\x00") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		d, err := url.PathUnescape(seg)
		if err != nil || !utf8.ValidString(d) || d == "" || d == "." || d == ".." || strings.ContainsAny(d, "/\\\x00") {
			return false
		}
	}
	return true
}

// serveIndex answers GET requests for the index and the files it lists.
func (s *service) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c, idx, err := s.index()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if idx.Err != nil {
		http.Error(w, idx.Err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	rel := strings.TrimPrefix(r.URL.Path, IndexPath)
	if rel == "index.json" {
		w.Header().Set("Content-Type", "application/json")
		w.Write(idx.JSON) //nolint:errcheck
		return
	}
	uri, ok := idx.files[rel]
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := c.Read(uri)
	if err != nil {
		// Changed since it was indexed; the next index lists the new version.
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data) //nolint:errcheck
}

// Index returns the Executor index of the current catalog.
func (s *Server) Index() (*Index, error) {
	_, idx, err := s.svc.index()
	return idx, err
}

// index returns the current catalog and its index, building the index once per catalog.
func (s *service) index() (*Catalog, *Index, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, nil, err
	}
	s.idxMu.Lock()
	defer s.idxMu.Unlock()
	if s.idxFor != c {
		s.idx, s.idxFor = c.Index(), c
	}
	return c, s.idx, nil
}

// rebound reports a request that reached a loopback listener under another
// host name, as a DNS rebinding page would. It mirrors the MCP handler's check.
func rebound(r *http.Request) bool {
	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	return ok && local != nil && loopback(local.String()) && !loopback(r.Host)
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = strings.Trim(addr, "[]")
	}
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}
