package skillserve

import (
	"context"
	"net/url"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const methodReadDirectory = "resources/directory/read"

type readDirectoryParams struct {
	mcp.ParamsBase
	URI    string `json:"uri"`
	Cursor string `json:"cursor,omitempty"`
}

type readDirectoryResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string          `json:"resultType"`
	Resources  []*mcp.Resource `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

func skillResource(s *Skill) *mcp.Resource {
	name, _ := s.Frontmatter["name"].(string)
	desc, _ := s.Frontmatter["description"].(string)
	return &mcp.Resource{URI: s.URI, Name: name, Description: desc, MIMEType: "text/markdown"}
}

func parentURI(uri string) string {
	i := strings.LastIndex(uri, "/")
	if i < len("skill://") {
		return ""
	}
	return uri[:i]
}

// indexDirectories runs after all acceptance and nested file ownership decisions.
// Only directories recorded by accepted packages can acquire children.
func (c *Catalog) indexDirectories() {
	add := func(uri, mime string) {
		parent := parentURI(uri)
		if _, ok := c.byDir[parent]; !ok {
			return
		}
		name, _ := url.PathUnescape(strings.TrimPrefix(uri, parent+"/"))
		r := &mcp.Resource{URI: uri, Name: name, MIMEType: mime}
		if s := c.bySkill[uri]; s != nil {
			r = skillResource(s)
		}
		c.byDir[parent] = append(c.byDir[parent], r)
	}
	for uri := range c.byDir {
		add(uri, "inode/directory")
	}
	for uri, file := range c.byFile {
		add(uri, mimeType(uri, file.text))
	}
	for _, children := range c.byDir {
		sort.Slice(children, func(i, j int) bool { return children[i].URI < children[j].URI })
	}
}

func (s *service) readDirectory(_ context.Context, _ *mcp.ServerSession, p *readDirectoryParams) (*readDirectoryResult, error) {
	if p == nil {
		return nil, invalidParams("missing uri")
	}
	uri, ok := canonical(p.URI)
	if !ok {
		return nil, invalidParams("invalid directory uri")
	}
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	children, ok := c.byDir[uri]
	if !ok {
		return nil, invalidParams("directory not found: " + p.URI)
	}
	cursor := ""
	if p.Cursor != "" {
		cursor, ok = canonical(p.Cursor)
		if !ok || parentURI(cursor) != uri {
			return nil, invalidParams("invalid directory cursor")
		}
	}
	start, end, next := resourcePage(len(children), cursor, func(i int) string { return children[i].URI })
	return &readDirectoryResult{Cacheable: s.cacheable(), ResultType: "complete", Resources: children[start:end], NextCursor: next}, nil
}

// resourcePage uses the last URI rather than an offset across refreshed snapshots.
func resourcePage(n int, cursor string, uri func(int) string) (start, end int, next string) {
	start = sort.Search(n, func(i int) bool { return uri(i) > cursor })
	end = min(start+pageSize, n)
	if end < n {
		next = uri(end - 1)
	}
	return
}

func (s *service) resourceListMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "resources/list" {
			return next(ctx, method, req)
		}
		r := req.(*mcp.ListResourcesRequest)
		return s.listResources(r.Params)
	}
}

func (s *service) listResources(p *mcp.ListResourcesParams) (*mcp.ListResourcesResult, error) {
	cursor := ""
	if p != nil && p.Cursor != "" {
		var ok bool
		cursor, ok = canonical(p.Cursor)
		if !ok {
			return nil, invalidParams("invalid resource cursor")
		}
	}
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	start, end, next := resourcePage(len(c.Skills), cursor, func(i int) string { return c.Skills[i].URI })
	resources := make([]*mcp.Resource, 0, end-start)
	for _, skill := range c.Skills[start:end] {
		resources = append(resources, skillResource(skill))
	}
	return &mcp.ListResourcesResult{Cacheable: s.cacheable(), Resources: resources, NextCursor: next}, nil
}
