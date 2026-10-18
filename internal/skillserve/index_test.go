package skillserve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// textSkill is a skill Executor accepts; skillMD has list metadata, which it rejects.
func textSkill(name string) string {
	return "---\nname: " + name + "\ndescription: Use when testing " + name + "\n---\n# " + name + "\n"
}

func indexServer(t *testing.T, src, token string) http.Handler {
	t.Helper()
	srv, err := NewServer(&Builder{Source: src}, Options{Warn: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	return HTTPHandler(srv, token)
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+path, nil))
	return rec
}

type testIndex struct {
	Skills []indexEntry `json:"skills"`
}

func readIndex(t *testing.T, h http.Handler) testIndex {
	t.Helper()
	rec := get(t, h, IndexPath+"index.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("index: status %d: %s", rec.Code, rec.Body)
	}
	var idx testIndex
	if err := json.Unmarshal(rec.Body.Bytes(), &idx); err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestIndex_ServesSkillsAndFilesTheWayExecutorFetchesThem(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf"))
	writeFile(t, filepath.Join(src, "pdf/refs/a b.md"), "notes")
	h := indexServer(t, src, "")

	first := get(t, h, IndexPath+"index.json").Body.String()
	idx := readIndex(t, h)
	if len(idx.Skills) != 1 || idx.Skills[0].Name != "pdf" || strings.Join(idx.Skills[0].Files, ",") != "SKILL.md,refs/a b.md" {
		t.Fatalf("index = %+v", idx)
	}
	if !strings.HasPrefix(idx.Skills[0].Version, "sha256:") {
		t.Errorf("version = %q", idx.Skills[0].Version)
	}
	// Executor encodes each path segment and compares the index it reads again byte for byte.
	if rec := get(t, h, IndexPath+"pdf/refs/a%20b.md"); rec.Code != http.StatusOK || rec.Body.String() != "notes" {
		t.Errorf("file: status %d, body %q", rec.Code, rec.Body)
	}
	if again := get(t, h, IndexPath+"index.json").Body.String(); again != first {
		t.Errorf("index changed between reads:\n%s\n%s", first, again)
	}
	if rec := get(t, h, IndexPath+"pdf/missing.md"); rec.Code != http.StatusNotFound {
		t.Errorf("unlisted file: status %d, want 404", rec.Code)
	}
}

func TestIndex_VersionChangesWithContent(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf"))
	h := indexServer(t, src, "")
	before := readIndex(t, h).Skills[0].Version
	writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf")+"more\n")
	if after := readIndex(t, h).Skills[0].Version; after == before {
		t.Errorf("version stayed %s after the content changed", before)
	}
}

// One skill Executor rejects fails its whole load, so the index leaves such skills out.
func TestIndex_LeavesOutSkillsExecutorWouldReject(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "ok/SKILL.md"), textSkill("ok"))
	writeFile(t, filepath.Join(src, "tagged/SKILL.md"), skillMD("tagged")) // metadata.tags is a list
	writeFile(t, filepath.Join(src, "logo/SKILL.md"), textSkill("logo"))
	writeFile(t, filepath.Join(src, "logo/logo.png"), "\x89PNG\xff")
	writeFile(t, filepath.Join(src, "_a/review/SKILL.md"), textSkill("review"))
	writeFile(t, filepath.Join(src, "_b/review/SKILL.md"), textSkill("review"))
	c, err := (&Builder{Source: src}).Build()
	if err != nil {
		t.Fatal(err)
	}

	idx := c.Index()
	if idx.Skills != 1 || idx.Err != nil {
		t.Fatalf("index has %d skills, err %v; want only ok", idx.Skills, idx.Err)
	}
	want := map[string]string{"tagged": "metadata.tags", "logo": "logo.png is not UTF-8", "_a/review": "used by 2 skills", "_b/review": "used by 2 skills"}
	for _, s := range idx.Skipped {
		if !strings.Contains(s.Reason, want[s.Path]) || want[s.Path] == "" {
			t.Errorf("skipped %s: %s", s.Path, s.Reason)
		}
		delete(want, s.Path)
	}
	if len(want) != 0 {
		t.Errorf("not skipped: %v", want)
	}
}

// A parent lists its nested skill's files under paths inside the parent.
func TestIndex_ListsNestedSkillFilesRelativeToEachSkill(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "suite/SKILL.md"), textSkill("suite"))
	writeFile(t, filepath.Join(src, "suite/child/SKILL.md"), textSkill("child"))
	h := indexServer(t, src, "")
	files := map[string]string{}
	for _, e := range readIndex(t, h).Skills {
		files[e.Name] = strings.Join(e.Files, ",")
	}
	if files["suite"] != "SKILL.md,child/SKILL.md" || files["child"] != "SKILL.md" {
		t.Fatalf("files = %v", files)
	}
	if body := get(t, h, IndexPath+"suite/SKILL.md").Body.String(); body != textSkill("suite") {
		t.Errorf("suite/SKILL.md = %q", body)
	}
}

// Executor decodes each path segment with decodeURIComponent; a name that fails it fails the load.
func TestIndex_LeavesOutSkillsWithFileNamesExecutorCannotLoad(t *testing.T) {
	for _, name := range []string{"50%.md", "a%2Fb.md", `back\slash.md`} {
		src := t.TempDir()
		writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf"))
		writeFile(t, filepath.Join(src, "pdf", name), "x")
		c, err := (&Builder{Source: src}).Build()
		if err != nil {
			t.Fatal(err)
		}
		if idx := c.Index(); idx.Skills != 0 || len(idx.Skipped) != 1 || !strings.Contains(idx.Skipped[0].Reason, "file name") {
			t.Errorf("%s: %d skills, skipped %v", name, idx.Skills, idx.Skipped)
		}
	}
}

func TestIndex_RefusesAnIndexOverExecutorsFileLimit(t *testing.T) {
	src := t.TempDir()
	for _, d := range []string{"a", "b", "c"} {
		writeFile(t, filepath.Join(src, d+"/SKILL.md"), textSkill(d))
		for i := range 400 {
			writeFile(t, filepath.Join(src, d, fmt.Sprintf("f%d.md", i)), "")
		}
	}
	h := indexServer(t, src, "")
	if rec := get(t, h, IndexPath+"index.json"); rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "--target") {
		t.Errorf("status %d, body %q; want 500 naming --target", rec.Code, rec.Body)
	}
}

func TestIndex_RequiresTheBearerToken(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf"))
	h := indexServer(t, src, "s3cret")
	if rec := get(t, h, IndexPath+"index.json"); rec.Code != http.StatusUnauthorized {
		t.Errorf("without token: status %d, want 401", rec.Code)
	}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+IndexPath+"pdf/SKILL.md", nil)
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("with token: status %d, want 200", rec.Code)
	}
}

// A TLS proxy in front of a loopback server may keep the public Host; the token
// already proves the request is not a rebinding page.
func TestHTTPHandler_AcceptsProxiedHostWithToken(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf"))
	h := indexServer(t, src, "s3cret")
	local := context.WithValue(context.Background(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8765})
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "https://skills.example.com"+IndexPath+"index.json", nil),
		httptest.NewRequest(http.MethodPost, "https://skills.example.com/", strings.NewReader(`{}`)),
	} {
		req = req.WithContext(local)
		req.Header.Set("Authorization", "Bearer s3cret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusForbidden {
			t.Errorf("%s %s: 403 %s", req.Method, req.URL.Path, rec.Body)
		}
	}
}

// A page that rebinds its own host name to 127.0.0.1 must not read a server that has no token.
func TestIndex_RejectsReboundHostOnLoopback(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "pdf/SKILL.md"), textSkill("pdf"))
	h := indexServer(t, src, "")
	req := httptest.NewRequest(http.MethodGet, "http://evil.example"+IndexPath+"index.json", nil)
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8765}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status %d, want 403", rec.Code)
	}
}
