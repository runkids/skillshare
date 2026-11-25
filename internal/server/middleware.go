package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// writeJSON writes a JSON response with 200 status
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response
func writeError(w http.ResponseWriter, code int, msg string) {
	writeCodedError(w, code, defaultErrorCode(code, msg), msg, nil)
}

// writeCodedError writes a JSON error response with a stable machine-readable
// code while preserving the legacy string `error` field for existing callers.
func writeCodedError(w http.ResponseWriter, status int, errCode, msg string, params map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"error":        msg,
		"error_code":   errCode,
		"error_params": params,
	}
	if params == nil {
		delete(body, "error_params")
	}
	json.NewEncoder(w).Encode(body)
}

func defaultErrorCode(status int, msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not found"):
		return "not_found"
	case strings.Contains(lower, "invalid"):
		return "validation"
	case strings.Contains(lower, "required"):
		return "validation"
	case strings.Contains(lower, "conflict") || strings.Contains(lower, "already exists"):
		return "conflict"
	}
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "unauthorized"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation"
	default:
		if status >= 500 {
			return "internal"
		}
		return "generic"
	}
}

// sendSSE writes a single Server-Sent Event frame and flushes.
func sendSSE(w http.ResponseWriter, f http.Flusher, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	f.Flush()
}

// longRequestTimeout outlasts one git network step (180 s), so a failed clone
// still reaches the dashboard as an error. Some git calls have no timeout of
// their own, so the deadline stays finite.
const longRequestTimeout = 10 * time.Minute

// withLongWriteDeadline lets a handler that clones, fetches, downloads or
// waits on another program outlast the server's 30-second WriteTimeout. Past that timeout the
// connection drops with no reply, and the dashboard reports a lost server even
// when the work succeeded.
func withLongWriteDeadline(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(longRequestTimeout))
		next(w, r)
	}
}

// initSSE sets SSE headers, disables the write deadline, and returns a
// mutex-protected safeSend function. If the ResponseWriter does not support
// flushing, an error response is written and ok is false.
func initSSE(w http.ResponseWriter) (safeSend func(string, any), ok bool) {
	flusher, isFlusher := w.(http.Flusher)
	if !isFlusher {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return nil, false
	}
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	var mu sync.Mutex
	return func(event string, data any) {
		mu.Lock()
		defer mu.Unlock()
		sendSSE(w, flusher, event, data)
	}, true
}
