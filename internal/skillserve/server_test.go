package skillserve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, src string) *mcp.ClientSession {
	t.Helper()
	return connectWith(t, src, nil)
}

func connectWith(t *testing.T, src string, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	return connectServer(t, src, Options{Version: "test", Warn: io.Discard}, opts)
}

func connectServer(t *testing.T, src string, so Options, opts *mcp.ClientOptions) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv, err := NewServer(&Builder{Source: src}, so)
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1"}, opts)
	if err := mcp.AddSendingCustomMethod[*listSkillsParams, *listSkillsResult](client, methodList); err != nil {
		t.Fatal(err)
	}
	if err := mcp.AddSendingCustomMethod[*getSkillParams, *getSkillResult](client, methodGet); err != nil {
		t.Fatal(err)
	}
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func listSkills(t *testing.T, cs *mcp.ClientSession, cursor string) *listSkillsResult {
	t.Helper()
	res, err := mcp.CallCustomMethod[*listSkillsParams, *listSkillsResult](context.Background(), cs, methodList, &listSkillsParams{Cursor: cursor})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestServer_DeclaresSkillsExtension(t *testing.T) {
	cs := connect(t, t.TempDir())
	caps := cs.InitializeResult().Capabilities
	if _, ok := caps.Extensions[extensionID]; !ok || caps.Resources == nil {
		t.Errorf("capabilities = %+v, want resources and %s", caps, extensionID)
	}
}

func TestServer_ListPaginatesWholeEntries(t *testing.T) {
	src := t.TempDir()
	for i := range pageSize + 1 {
		name := fmt.Sprintf("s%03d", i)
		writeFile(t, filepath.Join(src, name, "SKILL.md"), skillMD(name))
	}
	cs := connect(t, src)

	first := listSkills(t, cs, "")
	if len(first.Skills) != pageSize || first.NextCursor == "" || first.ResultType != "complete" || first.CacheScope == "" {
		t.Fatalf("first page: %d skills, cursor %q, resultType %q, cacheScope %q", len(first.Skills), first.NextCursor, first.ResultType, first.CacheScope)
	}
	second := listSkills(t, cs, first.NextCursor)
	if len(second.Skills) != 1 || second.NextCursor != "" {
		t.Errorf("second page: %d skills, cursor %q, want the last skill and no cursor", len(second.Skills), second.NextCursor)
	}
}

// The catalog may be rebuilt between pages; a skill added ahead of the cursor
// must not shift the next page onto entries already returned.
func TestServer_ListContinuesAfterTheLastSkillReturned(t *testing.T) {
	src := t.TempDir()
	for i := range pageSize + 1 {
		name := fmt.Sprintf("s%03d", i)
		writeFile(t, filepath.Join(src, name, "SKILL.md"), skillMD(name))
	}
	cs := connect(t, src) // Refresh 0: every call rebuilds

	first := listSkills(t, cs, "")
	writeFile(t, filepath.Join(src, "a000", "SKILL.md"), skillMD("a000"))
	second := listSkills(t, cs, first.NextCursor)
	if len(second.Skills) != 1 || second.Skills[0].URI != "skill://s100/SKILL.md" {
		t.Errorf("second page = %v, want only skill://s100/SKILL.md", second.Skills)
	}
}

func TestServer_GetUnknownSkillIsInvalidParams(t *testing.T) {
	cs := connect(t, t.TempDir())
	_, err := mcp.CallCustomMethod[*getSkillParams, *getSkillResult](context.Background(), cs, methodGet, &getSkillParams{URI: "skill://missing/SKILL.md"})
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("err = %v, want -32602", err)
	}
}

func TestServer_ReadsSkillFileAsMarkdown(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "doc/SKILL.md"), skillMD("doc"))
	cs := connect(t, src)

	res, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "skill://doc/SKILL.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 || res.Contents[0].Text != skillMD("doc") || res.Contents[0].MIMEType != "text/markdown" {
		t.Errorf("contents = %+v", res.Contents)
	}
}

func TestHTTPHandler_RequiresBearerToken(t *testing.T) {
	srv, err := NewServer(&Builder{Source: t.TempDir()}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	h := HTTPHandler(srv, "s3cret")
	for _, auth := range []string{"", "Bearer wrong"} {
		req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/", strings.NewReader(`{}`))
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status %d, want 401", auth, rec.Code)
		}
	}
}

// A web page in the browser must not reach a loopback server that has no token.
func TestHTTPHandler_RejectsCrossOriginBrowserRequests(t *testing.T) {
	srv, err := NewServer(&Builder{Source: t.TempDir()}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	HTTPHandler(srv, "").ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-origin request: status %d, want 403", rec.Code)
	}
}

func TestServer_SeesNewSkillWithoutRestart(t *testing.T) {
	src := t.TempDir()
	cs := connect(t, src)
	if got := listSkills(t, cs, ""); len(got.Skills) != 0 {
		t.Fatalf("got %d skills, want 0", len(got.Skills))
	}
	writeFile(t, filepath.Join(src, "late/SKILL.md"), skillMD("late"))

	res, err := mcp.CallCustomMethod[*getSkillParams, *getSkillResult](context.Background(), cs, methodGet, &getSkillParams{URI: "skill://late/SKILL.md"})
	if err != nil || res.Skill == nil || res.Skill.Frontmatter["name"] != "late" {
		t.Errorf("get late = %+v, %v", res, err)
	}
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(*mcp.TextContent).Text, res.IsError
}

func TestServer_ToolsListAndReadSkillsForClientsWithoutTheExtension(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "pdf", "SKILL.md"), skillMD("pdf"))
	writeFile(t, filepath.Join(src, "pdf", "references", "guide.md"), "# Guide\n")
	writeFile(t, filepath.Join(src, "review", "SKILL.md"), skillMD("review"))
	cs := connect(t, src)

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 2 {
		t.Fatalf("tools/list = %v, %v; want list_skills and read_skill", tools, err)
	}
	for _, tool := range tools.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s annotations = %+v, want readOnlyHint so clients do not ask to approve a read", tool.Name, tool.Annotations)
		}
	}
	list, _ := callTool(t, cs, "list_skills", map[string]any{"query": "PDF"})
	if !strings.Contains(list, "skill://pdf/SKILL.md") || strings.Contains(list, "review") {
		t.Errorf("list_skills query=PDF:\n%s", list)
	}
	body, _ := callTool(t, cs, "read_skill", map[string]any{"uri": "skill://pdf/SKILL.md"})
	if !strings.Contains(body, "# pdf") || !strings.Contains(body, "skill://pdf/references/guide.md") {
		t.Errorf("read_skill SKILL.md should return the body and list the other files:\n%s", body)
	}
	if msg, isErr := callTool(t, cs, "read_skill", map[string]any{"uri": "skill://pdf/../review/SKILL.md"}); !isErr {
		t.Errorf("read_skill outside the skill = %q, want a tool error", msg)
	}
}

func TestServer_HidesToolsFromClientsWithTheExtension(t *testing.T) {
	caps := &mcp.ClientCapabilities{}
	caps.AddExtension(extensionID, map[string]any{})
	cs := connectWith(t, t.TempDir(), &mcp.ClientOptions{Capabilities: caps})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Tools) != 0 {
		t.Errorf("tools = %d, want none: the client loads skills natively", len(res.Tools))
	}
}
