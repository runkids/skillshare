package server

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
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
