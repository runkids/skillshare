package skillserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"skillshare/internal/config"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Keep the client contract independent of the server's custom result type.
type directoryParams struct {
	mcp.ParamsBase
	URI    string `json:"uri"`
	Cursor string `json:"cursor,omitempty"`
}
type directoryResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string          `json:"resultType"`
	Resources  []*mcp.Resource `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

func directory(t *testing.T, cs *mcp.ClientSession, uri, cursor string) *directoryResult {
	t.Helper()
	r, err := mcp.CallCustomMethod[*directoryParams, *directoryResult](context.Background(), cs, "resources/directory/read", &directoryParams{URI: uri, Cursor: cursor})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestServer_DirectoryBrowsingAndResourceMetadata(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "_team/doc/SKILL.md"), skillMD("doc"))
	writeFile(t, filepath.Join(src, "_team/doc/ref space/a #.md"), "guide")
	writeFile(t, filepath.Join(src, "_team/doc/child/SKILL.md"), skillMD("child"))
	writeFile(t, filepath.Join(src, "_team/doc/.git/HEAD"), "hidden")
	writeFile(t, filepath.Join(src, "_team/doc/raw.unknown-extension"), "data")
	writeFile(t, filepath.Join(src, "_team/doc/LICENSE"), "License text\n")
	// Valid multibyte text across the streaming hash buffer boundary, and
	// invalid/incomplete encodings, must agree with resources/read's full bytes.
	writeFile(t, filepath.Join(src, "_team/doc/mime/boundary"), strings.Repeat("a", 32767)+"界")
	writeFile(t, filepath.Join(src, "_team/doc/mime/boundary-two"), strings.Repeat("a", 32767)+"é")
	writeFile(t, filepath.Join(src, "_team/doc/mime/boundary-four"), strings.Repeat("a", 32767)+"😀")
	writeFile(t, filepath.Join(src, "_team/doc/mime/empty"), "")
	writeFile(t, filepath.Join(src, "_team/doc/mime/invalid"), "\xff"+strings.Repeat("a", 65536))
	writeFile(t, filepath.Join(src, "_team/doc/mime/truncated"), strings.Repeat("a", 8192)+"\xe7\x95")
	if err := os.MkdirAll(filepath.Join(src, "_team/doc/empty/deep"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(src, "_team/doc/link")); err != nil {
		t.Fatal(err)
	}
	cs := connectServer(t, src, Options{Refresh: time.Hour}, nil)
	ext := cs.InitializeResult().Capabilities.Extensions[extensionID]
	data, _ := json.Marshal(ext)
	var capability map[string]any
	if err := json.Unmarshal(data, &capability); err != nil || capability["directoryRead"] != true {
		t.Errorf("directory capability = %s", data)
	}
	root := directory(t, cs, "skill://_team/doc", "")
	want := []*mcp.Resource{
		{URI: "skill://_team/doc/LICENSE", Name: "LICENSE", MIMEType: "text/plain"},
		{URI: "skill://_team/doc/SKILL.md", Name: "doc", Description: "Use when testing doc", MIMEType: "text/markdown"},
		{URI: "skill://_team/doc/child", Name: "child", MIMEType: "inode/directory"},
		{URI: "skill://_team/doc/empty", Name: "empty", MIMEType: "inode/directory"},
		{URI: "skill://_team/doc/mime", Name: "mime", MIMEType: "inode/directory"},
		{URI: "skill://_team/doc/raw.unknown-extension", Name: "raw.unknown-extension", MIMEType: "text/plain"},
		{URI: "skill://_team/doc/ref%20space", Name: "ref space", MIMEType: "inode/directory"},
	}
	if !reflect.DeepEqual(root.Resources, want) {
		t.Errorf("entries = %+v, want %+v", root.Resources, want)
	}
	if root.ResultType != "complete" || root.TTLMs != 3600000 || root.CacheScope != "private" {
		t.Errorf("cache result = %+v", root)
	}
	mimes := directory(t, cs, "skill://_team/doc/mime", "")
	wantMIMEs := map[string]string{
		"skill://_team/doc/LICENSE":               "text/plain",
		"skill://_team/doc/raw.unknown-extension": "text/plain",
		"skill://_team/doc/mime/boundary":         "text/plain",
		"skill://_team/doc/mime/boundary-two":     "text/plain",
		"skill://_team/doc/mime/boundary-four":    "text/plain",
		"skill://_team/doc/mime/empty":            "text/plain",
		"skill://_team/doc/mime/invalid":          "application/octet-stream",
		"skill://_team/doc/mime/truncated":        "application/octet-stream",
	}
	for _, resource := range append(root.Resources, mimes.Resources...) {
		mime, tested := wantMIMEs[resource.URI]
		if !tested {
			continue
		}
		read, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: resource.URI})
		if err != nil {
			t.Fatal(err)
		}
		if resource.MIMEType != mime || len(read.Contents) != 1 || read.Contents[0].MIMEType != mime {
			t.Errorf("%s directory MIME = %s, read = %+v; want %s", resource.URI, resource.MIMEType, read.Contents, mime)
		}
		delete(wantMIMEs, resource.URI)
	}
	if len(wantMIMEs) != 0 {
		t.Errorf("MIME resources missing: %v", wantMIMEs)
	}
	deep := directory(t, cs, "skill://_team/doc/empty/deep", "")
	if deep.Resources == nil || len(deep.Resources) != 0 {
		t.Errorf("empty directory = %+v", deep)
	}
	ref := directory(t, cs, "skill://_team/doc/ref%20space", "")
	if len(ref.Resources) != 1 || ref.Resources[0].URI != "skill://_team/doc/ref%20space/a%20%23.md" || ref.Resources[0].Name != "a #.md" {
		t.Errorf("escaped entry = %+v", ref)
	}
	child := directory(t, cs, "skill://_team/doc/child", "")
	if len(child.Resources) != 1 || child.Resources[0].Name != "child" || child.Resources[0].Description != "Use when testing child" {
		t.Errorf("nested metadata = %+v", child)
	}
	// Snapshot metadata remains available without payload reads until refresh.
	if err := os.Remove(filepath.Join(src, "_team/doc/SKILL.md")); err != nil {
		t.Fatal(err)
	}
	listed, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Resources) != 2 || !reflect.DeepEqual(listed.Resources[0], want[1]) || listed.CacheScope != "private" || listed.TTLMs != 3600000 {
		t.Errorf("resources/list = %+v", listed)
	}
}

func TestServer_DirectoryRejectsNonDirectoriesAndForeignCursors(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "doc/SKILL.md"), skillMD("doc"))
	cs := connect(t, src)
	for _, p := range []*directoryParams{
		nil, {}, {URI: "file:///doc"}, {URI: "skill://doc/"}, {URI: "skill://doc/.."},
		{URI: "skill://doc/%2f"}, {URI: "skill://missing"}, {URI: "skill://doc/SKILL.md"},
		{URI: "skill://doc", Cursor: "skill://other/SKILL.md"}, {URI: "skill://doc", Cursor: "skill://doc/sub/file"},
		{URI: "skill://doc", Cursor: "bad"},
	} {
		_, err := mcp.CallCustomMethod[*directoryParams, *directoryResult](context.Background(), cs, "resources/directory/read", p)
		var rpc *jsonrpc.Error
		if !errors.As(err, &rpc) || rpc.Code != -32602 {
			t.Errorf("params %+v: %v, want -32602", p, err)
		}
	}
}

func TestServer_ResourceAndDirectoryPagesSurviveRefresh(t *testing.T) {
	src := t.TempDir()
	for i := range 101 {
		n := fmt.Sprintf("s%03d", i)
		writeFile(t, filepath.Join(src, n, "SKILL.md"), skillMD(n))
		writeFile(t, filepath.Join(src, "s000", fmt.Sprintf("part-%03d.txt", i)), "part")
	}
	cs := connect(t, src)
	first, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Resources) != 100 || first.NextCursor != "skill://s099/SKILL.md" {
		t.Fatalf("first resource page = %+v", first)
	}
	dirs := directory(t, cs, "skill://s000", "")
	if len(dirs.Resources) != 100 || dirs.NextCursor != "skill://s000/part-098.txt" {
		t.Fatalf("first directory page = %+v", dirs)
	}
	writeFile(t, filepath.Join(src, "a000/SKILL.md"), skillMD("a000"))
	writeFile(t, filepath.Join(src, "s000/a.txt"), "new before cursor")
	writeFile(t, filepath.Join(src, "s101/SKILL.md"), skillMD("s101"))
	writeFile(t, filepath.Join(src, "s000/part-101.txt"), "new after cursor")
	for _, rel := range []string{"s001/SKILL.md", "s000/part-000.txt"} {
		if err := os.Remove(filepath.Join(src, rel)); err != nil {
			t.Fatal(err)
		}
	}
	next, err := cs.ListResources(context.Background(), &mcp.ListResourcesParams{Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Resources) != 2 || next.Resources[0].URI != "skill://s100/SKILL.md" || next.Resources[1].URI != "skill://s101/SKILL.md" || next.NextCursor != "" {
		t.Errorf("continued resources = %+v", next)
	}
	last := directory(t, cs, "skill://s000", dirs.NextCursor)
	if len(last.Resources) != 3 || last.Resources[0].URI != "skill://s000/part-099.txt" || last.Resources[2].URI != "skill://s000/part-101.txt" || last.NextCursor != "" {
		t.Errorf("continued directory = %+v", last)
	}
	end := directory(t, cs, "skill://s000", "skill://s000/zzz")
	if end.Resources == nil || len(end.Resources) != 0 {
		t.Errorf("exhausted page = %+v", end)
	}
}

func TestCatalog_DirectoriesRespectSkillAcceptance(t *testing.T) {
	for _, tc := range []struct {
		name          string
		child, parent string
		exclude       []string
		disabled      bool
		want          []string
	}{
		{name: "nested accepted", parent: "suite", child: "child", want: []string{"skill://suite", "skill://suite/child"}},
		{name: "excluded child skips parent", parent: "suite", child: "child", exclude: []string{"suite__child"}},
		{name: "disabled child skips parent", parent: "suite", child: "child", disabled: true},
		{name: "invalid child skips parent", parent: "suite", child: "wrong-name"},
		{name: "invalid parent leaves only child root", parent: "wrong-name", child: "child", want: []string{"skill://suite/child"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := t.TempDir()
			writeFile(t, filepath.Join(src, "suite/SKILL.md"), skillMD(tc.parent))
			writeFile(t, filepath.Join(src, "suite/child/SKILL.md"), skillMD(tc.child))
			if tc.disabled {
				writeFile(t, filepath.Join(src, ".skillignore"), "suite/child\n")
			}
			c, err := (&Builder{Source: src, Target: "claude", Skills: &config.ResourceTargetConfig{Exclude: tc.exclude}}).Build()
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for uri := range c.byDir {
				got = append(got, uri)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("served directories = %v, want %v", got, tc.want)
			}
		})
	}
}

func BenchmarkCachedResourcePage(b *testing.B) {
	for _, n := range []int{100, 10000, 100000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			c := &Catalog{}
			for i := range n {
				c.Skills = append(c.Skills, &Skill{URI: fmt.Sprintf("skill://s%06d/SKILL.md", i), Frontmatter: map[string]any{"name": "fixture", "description": "fixture"}})
			}
			s := &service{cat: c, built: time.Now(), opts: Options{Refresh: time.Hour}}
			p := &mcp.ListResourcesParams{Cursor: fmt.Sprintf("skill://s%06d/SKILL.md", n/2)}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := s.listResources(p); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
