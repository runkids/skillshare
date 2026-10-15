package skillserve

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SEP-2640 methods.
const (
	methodList = "skills/list"
	methodGet  = "skills/get"
)

const pageSize = 100

type listSkillsParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

type listSkillsResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string   `json:"resultType"`
	Skills     []*Skill `json:"skills"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

type getSkillParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

type getSkillResult struct {
	mcp.ResultBase
	mcp.Cacheable
	ResultType string `json:"resultType"`
	Skill      *Skill `json:"skill"`
}

// Options configures NewServer.
type Options struct {
	Version string
	// Warn receives each skip warning when it first appears.
	Warn io.Writer
	// Refresh is how long a catalog is reused before the source is read again;
	// it is also the ttlMs hint. Zero rebuilds on every request.
	Refresh time.Duration
	// Edit, when set, adds the tools that edit existing skill files.
	Edit *EditOptions
}

type service struct {
	b    *Builder
	opts Options

	mu     sync.Mutex
	cat    *Catalog
	built  time.Time
	warned map[string]bool

	edit     *EditOptions
	editMu   sync.Mutex // serializes saves and guards previews
	previews map[string]*preview
}

// NewServer returns an MCP server for the skills b selects, read-only unless
// opts.Edit is set. The first
// catalog is built here, so source errors and initial warnings surface at once.
func NewServer(b *Builder, opts Options) (*mcp.Server, error) {
	if opts.Warn == nil {
		opts.Warn = io.Discard
	}
	svc := &service{b: b, opts: opts, edit: opts.Edit, previews: map[string]*preview{}}
	if _, err := svc.catalog(); err != nil {
		return nil, err
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "skillshare", Version: opts.Version}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{
			Resources:  &mcp.ResourceCapabilities{},
			Extensions: map[string]any{extensionID: map[string]any{}},
		},
	})
	srv.AddResourceTemplate(&mcp.ResourceTemplate{Name: "skill-file", URITemplate: "skill://{+path}"}, svc.read)
	if err := mcp.AddReceivingCustomMethod(srv, methodList, svc.list); err != nil {
		return nil, err
	}
	if err := mcp.AddReceivingCustomMethod(srv, methodGet, svc.get); err != nil {
		return nil, err
	}
	svc.addTools(srv)
	if svc.edit != nil {
		svc.addEditTools(srv)
	}
	return srv, nil
}

// HTTPHandler serves srv over Streamable HTTP. A non-empty token must be sent
// as a bearer token on every request. Cross-origin browser requests are
// refused, so a web page cannot reach a loopback server that has no token.
func HTTPHandler(srv *mcp.Server, token string) http.Handler {
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{
		Stateless:             true,
		CrossOriginProtection: http.NewCrossOriginProtection(),
	})
	if token == "" {
		return h
	}
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *service) catalog() (*Catalog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cat != nil && time.Since(s.built) < s.opts.Refresh {
		return s.cat, nil
	}
	c, err := s.b.Build()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(c.Skipped))
	for _, skip := range c.Skipped {
		w := skip.String()
		seen[w] = true
		if !s.warned[w] {
			fmt.Fprintln(s.opts.Warn, "warning: "+w)
		}
	}
	s.cat, s.built, s.warned = c, time.Now(), seen
	return c, nil
}

func (s *service) cacheable() mcp.Cacheable {
	return mcp.Cacheable{TTLMs: int(s.opts.Refresh.Milliseconds()), CacheScope: "private"}
}

func invalidParams(msg string) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: msg}
}

func (s *service) list(_ context.Context, _ *mcp.ServerSession, p *listSkillsParams) (*listSkillsResult, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	// The cursor is the last URI returned, not an offset: the catalog may be rebuilt
	// between pages, and skills added or removed before it must not shift the next page.
	start := 0
	if p != nil && p.Cursor != "" {
		if !strings.HasPrefix(p.Cursor, "skill://") {
			return nil, invalidParams("invalid cursor")
		}
		start = sort.Search(len(c.Skills), func(i int) bool { return c.Skills[i].URI > p.Cursor })
	}
	end := min(start+pageSize, len(c.Skills))
	res := &listSkillsResult{Cacheable: s.cacheable(), ResultType: "complete", Skills: c.Skills[start:end]}
	if end < len(c.Skills) {
		res.NextCursor = c.Skills[end-1].URI
	}
	return res, nil
}

func (s *service) get(_ context.Context, _ *mcp.ServerSession, p *getSkillParams) (*getSkillResult, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, invalidParams("missing uri")
	}
	skill, ok := c.Skill(p.URI)
	if !ok {
		return nil, invalidParams("skill not found: " + p.URI)
	}
	return &getSkillResult{Cacheable: s.cacheable(), ResultType: "complete", Skill: skill}, nil
}

func (s *service) read(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	c, err := s.catalog()
	if err != nil {
		return nil, err
	}
	uri := req.Params.URI
	data, err := c.Read(uri)
	if errors.Is(err, errNotFound) {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	if err != nil {
		return nil, err
	}
	text := utf8.Valid(data)
	content := &mcp.ResourceContents{URI: uri, MIMEType: mimeType(uri, text)}
	if text {
		content.Text = string(data)
	} else {
		content.Blob = data
	}
	return &mcp.ReadResourceResult{Cacheable: s.cacheable(), Contents: []*mcp.ResourceContents{content}}, nil
}

func mimeType(uri string, text bool) string {
	ext := path.Ext(uri)
	if ext == ".md" {
		return "text/markdown"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	if text {
		return "text/plain"
	}
	return "application/octet-stream"
}
