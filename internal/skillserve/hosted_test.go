package skillserve

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"skillshare/internal/profile"
)

const hostedMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`

func hostedFixture(t *testing.T, edit *EditOptions) (string, *httptest.Server) {
	t.Helper()
	src := t.TempDir()
	for _, n := range []string{"alpha", "beta", "shared"} {
		writeFile(t, filepath.Join(src, n, "SKILL.md"), textSkill(n))
	}
	builders := map[string]*Builder{}
	for name, own := range map[string]string{"writing": "alpha", "coding": "beta", "unbound": "alpha"} {
		sel := hostedSelector(name, own, "shared")
		builders[name] = &Builder{Source: src, Profile: &sel}
	}
	h, err := NewHostedHandler(builders, []ProfileBinding{{"writing", "write-secret"}, {"coding", "code-secret"}}, Options{Version: "test", Refresh: time.Hour, Edit: edit})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return src, srv
}

func hostedRequest(t *testing.T, srv *httptest.Server, token, method, path, body string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	if method == "POST" {
		var rpc struct {
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
				URI  string `json:"uri"`
			} `json:"params"`
		}
		_ = json.Unmarshal([]byte(body), &rpc)
		req.Header.Set("Mcp-Method", rpc.Method)
		if rpc.Params.Name != "" {
			req.Header.Set("Mcp-Name", rpc.Params.Name)
		} else if rpc.Params.URI != "" {
			req.Header.Set("Mcp-Name", rpc.Params.URI)
		}
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, data
}

func hostedRPC(t *testing.T, srv *httptest.Server, token, method, params string) []byte {
	t.Helper()
	if params != "" {
		params += ","
	}
	status, _, data := hostedRequest(t, srv, token, "POST", "/", `{"jsonrpc":"2.0","id":1,"method":"`+method+`","params":{`+params+hostedMeta+`}}`)
	if status != 200 && !(status == 400 && bytes.Contains(data, []byte(`"error"`))) {
		t.Fatalf("%s HTTP %d: %s", method, status, data)
	}
	if bytes.HasPrefix(data, []byte("event:")) {
		for line := range strings.SplitSeq(string(data), "\n") {
			if strings.HasPrefix(line, "data: ") {
				return []byte(strings.TrimPrefix(line, "data: "))
			}
		}
		t.Fatalf("missing SSE data: %s", data)
	}
	return data
}

// One listener must never dispatch an account to another account's catalog or files.
func TestHostedHTTP_ProfileIsolation(t *testing.T) {
	_, srv := hostedFixture(t, nil)
	for _, tc := range []struct{ token, own, foreign string }{{"write-secret", "alpha", "beta"}, {"code-secret", "beta", "alpha"}} {
		t.Run(tc.own, func(t *testing.T) {
			t.Parallel()
			data := hostedRPC(t, srv, tc.token, "skills/list", "")
			var result struct {
				Result struct {
					Skills []*Skill `json:"skills"`
				} `json:"result"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			var uris []string
			for _, s := range result.Result.Skills {
				uris = append(uris, s.URI)
			}
			if !slices.Equal(uris, []string{"skill://" + tc.own + "/SKILL.md", "skill://shared/SKILL.md"}) {
				t.Fatalf("catalog: %s", data)
			}
			for _, method := range []string{"skills/get", "resources/read"} {
				for _, n := range []string{tc.own, tc.foreign} {
					data = hostedRPC(t, srv, tc.token, method, `"uri":"skill://`+n+`/SKILL.md"`)
					if bytes.Contains(data, []byte(`"error"`)) != (n == tc.foreign) {
						t.Fatalf("%s %s: %s", method, n, data)
					}
				}
			}
			data = hostedRPC(t, srv, tc.token, "tools/call", `"name":"list_skills","arguments":{}`)
			if !bytes.Contains(data, []byte("skill://"+tc.own+"/SKILL.md")) || bytes.Contains(data, []byte("skill://"+tc.foreign+"/SKILL.md")) {
				t.Fatalf("fallback list: %s", data)
			}
			for _, n := range []string{tc.own, tc.foreign} {
				data = hostedRPC(t, srv, tc.token, "tools/call", `"name":"read_skill","arguments":{"uri":"skill://`+n+`/SKILL.md"}`)
				if bytes.Contains(data, []byte(`"isError":true`)) != (n == tc.foreign) {
					t.Fatalf("fallback read %s: %s", n, data)
				}
			}
			status, headers, data := hostedRequest(t, srv, tc.token, "GET", IndexPath+"index.json?profile="+tc.foreign, "")
			if status != 200 || !strings.Contains(headers.Get("Cache-Control"), "private") {
				t.Fatalf("index %d %v: %s", status, headers, data)
			}
			var idx testIndex
			if err := json.Unmarshal(data, &idx); err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, s := range idx.Skills {
				names = append(names, s.Name)
				for _, f := range s.Files {
					status, headers, data = hostedRequest(t, srv, tc.token, "GET", IndexPath+s.Name+"/"+f, "")
					if status != 200 || !strings.Contains(headers.Get("Cache-Control"), "private") {
						t.Fatalf("file %d %v: %s", status, headers, data)
					}
				}
			}
			if !slices.Equal(names, []string{tc.own, "shared"}) {
				t.Fatalf("index names %v", names)
			}
			status, _, _ = hostedRequest(t, srv, tc.token, "GET", IndexPath+tc.foreign+"/SKILL.md", "")
			if status != 404 {
				t.Fatalf("foreign file HTTP %d", status)
			}
		})
	}
}

func TestHostedHTTP_AuthenticationAndSecurity(t *testing.T) {
	_, srv := hostedFixture(t, nil)
	for _, auth := range []string{"", "Basic write-secret", "Bearer wrong", "Bearer write-secret extra", "Bearer"} {
		for _, path := range []string{"/", IndexPath + "index.json", IndexPath + "alpha/SKILL.md"} {
			req, _ := http.NewRequest("POST", srv.URL+path, strings.NewReader("not JSON"))
			req.Header.Set("Authorization", auth)
			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 401 {
				t.Fatalf("unauthorized path %s status %d", path, resp.StatusCode)
			}
		}
	}
	req, _ := http.NewRequest("POST", srv.URL, strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer write-secret")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "https://evil.example")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-origin status %d", resp.StatusCode)
	}
	req, _ = http.NewRequest("GET", srv.URL+IndexPath+"index.json", nil)
	req.Host = "proxy.example"
	req.Header.Set("Authorization", "Bearer write-secret")
	resp, err = srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("authenticated proxy status %d", resp.StatusCode)
	}
}

func hostedPreview(t *testing.T, srv *httptest.Server, token, uri, content string) string {
	t.Helper()
	data := hostedRPC(t, srv, token, "resources/read", `"uri":"`+uri+`"`)
	var res struct {
		Result struct {
			Contents []struct {
				Text string `json:"text"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &res); err != nil || len(res.Result.Contents) != 1 {
		t.Fatalf("read: %s %v", data, err)
	}
	args, _ := json.Marshal(previewInput{URI: uri, BaseRevision: skillDigest(res.Result.Contents[0].Text), Content: content})
	data = hostedRPC(t, srv, token, "tools/call", `"name":"preview_skill_edit","arguments":`+string(args))
	// JSON escapes the diff but not the preview id.
	return idOf(t, string(data))
}

func hostedApply(t *testing.T, srv *httptest.Server, token, id string) []byte {
	return hostedRPC(t, srv, token, "tools/call", `"name":"apply_skill_edit","arguments":{"preview_id":"`+id+`"}`)
}

func TestHostedEdit_IsolatesAndRechecks(t *testing.T) {
	for _, change := range []string{"disable", "remove", "unwritable", "reroute"} {
		t.Run(change, func(t *testing.T) {
			edit := &EditOptions{Skills: []string{"*"}, PreviewTTL: time.Hour, Threshold: "CRITICAL"}
			src, srv := hostedFixture(t, edit)
			uri := "skill://shared/SKILL.md"
			id := hostedPreview(t, srv, "write-secret", uri, textSkill("shared")+"changed\n")
			data := hostedApply(t, srv, "code-secret", id)
			if !bytes.Contains(data, []byte(`"isError":true`)) {
				t.Fatalf("foreign preview applied: %s", data)
			}
			switch change {
			case "disable":
				writeFile(t, filepath.Join(src, ".skillignore"), "shared\n")
			case "remove":
				if err := os.Remove(filepath.Join(src, "shared/SKILL.md")); err != nil {
					t.Fatal(err)
				}
			case "unwritable":
				writeFile(t, filepath.Join(src, ".metadata.json"), `{"version":1,"entries":{"shared":{"source":"github.com/acme/skills/shared"}}}`)
			case "reroute":
				writeFile(t, filepath.Join(src, "shared/SKILL.md"), "---\nname: shared\ndescription: fixture\ntargets: [codex]\n---\n")
			}
			before, _ := os.ReadFile(filepath.Join(src, "shared/SKILL.md"))
			data = hostedApply(t, srv, "write-secret", id)
			if !bytes.Contains(data, []byte(`"isError":true`)) {
				t.Fatalf("lost authorization applied: %s", data)
			}
			after, _ := os.ReadFile(filepath.Join(src, "shared/SKILL.md"))
			if !bytes.Equal(before, after) {
				t.Fatal("rejected edit wrote source")
			}
		})
	}
}

func TestHostedEdit_SharedRevisionConflict(t *testing.T) {
	src, srv := hostedFixture(t, &EditOptions{Skills: []string{"shared"}, PreviewTTL: time.Hour, Threshold: "CRITICAL"})
	uri := "skill://shared/SKILL.md"
	ids := []string{hostedPreview(t, srv, "write-secret", uri, textSkill("shared")+"writing\n"), hostedPreview(t, srv, "code-secret", uri, textSkill("shared")+"coding\n")}
	// Populate both Executor snapshots before saving to catch missing sibling invalidation.
	for _, token := range []string{"write-secret", "code-secret"} {
		hostedRequest(t, srv, token, "GET", IndexPath+"index.json", "")
	}
	results := make([][]byte, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, token := range []string{"write-secret", "code-secret"} {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results[i] = hostedApply(t, srv, token, ids[i]) }()
	}
	close(start)
	wg.Wait()
	successes, conflicts := 0, 0
	for _, r := range results {
		if bytes.Contains(r, []byte("Saved ")) {
			successes++
		}
		if bytes.Contains(r, []byte("conflict:")) {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("want save/conflict, got %s", results)
	}
	saved := readFile(t, filepath.Join(src, "shared/SKILL.md"))
	for _, token := range []string{"write-secret", "code-secret"} {
		data := hostedRPC(t, srv, token, "resources/read", `"uri":"`+uri+`"`)
		if bytes.Contains(data, []byte(`"error"`)) {
			t.Fatalf("stale catalog: %s", data)
		}
		data = hostedRPC(t, srv, token, "skills/get", `"uri":"`+uri+`"`)
		if !bytes.Contains(data, []byte(skillDigest(saved))) {
			t.Fatalf("stale resource digest: %s", data)
		}
		_, _, data = hostedRequest(t, srv, token, "GET", IndexPath+"index.json", "")
		var idx testIndex
		if err := json.Unmarshal(data, &idx); err != nil {
			t.Fatal(err)
		}
		wantVersion := skillDigest("SKILL.md\x00" + skillDigest(saved) + "\n")
		for _, entry := range idx.Skills {
			if entry.Name == "shared" && entry.Version != wantVersion {
				t.Fatalf("stale shared index version: %s want %s", entry.Version, wantVersion)
			}
		}
		_, _, data = hostedRequest(t, srv, token, "GET", IndexPath+"shared/SKILL.md", "")
		if string(data) != saved {
			t.Fatalf("stale index file %s want %s", data, saved)
		}
	}
}

func TestHostedHTTP_RejectsInvalidBindings(t *testing.T) {
	src := t.TempDir()
	sel := hostedSelector("writing", "*")
	builders := map[string]*Builder{"writing": {Source: src, Profile: &sel}}
	for _, bindings := range [][]ProfileBinding{nil, {{"writing", ""}}, {{"missing", "secret"}}, {{"writing", "secret"}, {"writing", "other"}}} {
		_, err := NewHostedHandler(builders, bindings, Options{})
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe validation error: %v", err)
		}
	}
	bad := profile.Resolve(profile.Catalog{}, profile.State{Active: "unbound"}, "fixture.yaml", nil)
	badBuilder := &Builder{Source: filepath.Join(src, "missing"), Profile: &bad}
	builders["unbound"] = badBuilder
	if _, err := NewHostedHandler(builders, []ProfileBinding{{"writing", "secret"}}, Options{}); err == nil {
		t.Fatal("unusable unbound profile accepted")
	}
}

func FuzzHostedAuthorization(f *testing.F) {
	for _, s := range []string{"", "Bearer fixture", "Bearer other", "Basic fixture", "Bearer fixture extra"} {
		f.Add(s)
	}
	src := f.TempDir()
	sel := hostedSelector("writing", "*")
	h, err := NewHostedHandler(map[string]*Builder{"writing": {Source: src, Profile: &sel}}, []ProfileBinding{{"writing", "fixture"}}, Options{})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, auth string) {
		req := httptest.NewRequest("GET", "http://127.0.0.1"+IndexPath+"index.json", nil)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		want := 401
		if auth == "Bearer fixture" {
			want = 200
		}
		if rec.Code != want {
			t.Fatalf("authorization status %d want %d", rec.Code, want)
		}
	})
}

// Keep digest expectations on the actual file bytes returned over the protocol.
func skillDigest(s string) string { return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(s))) }
