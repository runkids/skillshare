package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	ghclient "skillshare/internal/github"
	"skillshare/internal/install"
	"skillshare/internal/search"
)

var (
	previewCache    sync.Map
	previewCacheTTL = 5 * time.Minute
	previewCacheMax = 200
)

type previewCacheEntry struct {
	data      *search.SkillPreview
	expiresAt time.Time
}

type previewCacheKey struct {
	source, branch, policy string
}

func previewKey(source, branch string, opts install.ParseOptions) previewCacheKey {
	policy, _ := json.Marshal(opts)
	return previewCacheKey{source: source, branch: branch, policy: string(policy)}
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing required parameter: source")
		return
	}
	branch := r.URL.Query().Get("branch")

	s.mu.RLock()
	parseOpts := s.parseOpts()
	s.mu.RUnlock()

	// Bind cached content to both the requested ref and configured host scope.
	cacheKey := previewKey(source, branch, parseOpts)
	if entry, ok := previewCache.Load(cacheKey); ok {
		ce := entry.(*previewCacheEntry)
		if time.Now().Before(ce.expiresAt) {
			writeJSON(w, ce.data)
			return
		}
		previewCache.Delete(cacheKey)
	}

	client := ghclient.NewClient()
	preview, err := search.FetchPreview(client, source, branch, parseOpts)
	if err != nil {
		var rlErr *ghclient.RateLimitError
		if errors.As(err, &rlErr) {
			w.Header().Set("Retry-After", rlErr.Reset)
			writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		if errors.Is(err, search.ErrSkillNotFound) || errors.Is(err, search.ErrPreviewUnsupported) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Evict expired entries and enforce max size
	now := time.Now()
	count := 0
	previewCache.Range(func(k, v any) bool {
		count++
		if now.After(v.(*previewCacheEntry).expiresAt) {
			previewCache.Delete(k)
			count--
		}
		return true
	})
	if count >= previewCacheMax {
		// Over limit after purge — drop all (simple reset)
		previewCache.Range(func(k, _ any) bool {
			previewCache.Delete(k)
			return true
		})
	}

	previewCache.Store(cacheKey, &previewCacheEntry{
		data:      preview,
		expiresAt: now.Add(previewCacheTTL),
	})

	writeJSON(w, preview)
}
