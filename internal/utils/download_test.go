package utils

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// payload is larger than minParallelSize and not a multiple of downloadParts.
var payload = bytes.Repeat([]byte("0123456789abcdef"), (minParallelSize+12345)/16)

func download(t *testing.T, srv *httptest.Server, maxSize int64) ([]byte, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "dl-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	client := &http.Client{Timeout: 10 * time.Second}
	if err := DownloadToFile(client, srv.URL+"/asset", f, maxSize, nil); err != nil {
		return nil, err
	}
	return os.ReadFile(f.Name())
}

func TestDownloadToFile_ParallelRangesReassembleExactBytes(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.ServeContent(w, r, "asset", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

	got, err := download(t, srv, int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %d bytes, want the exact %d-byte payload", len(got), len(payload))
	}
	if n := requests.Load(); n != 1+downloadParts {
		t.Errorf("requests = %d, want a probe plus %d ranges", n, downloadParts)
	}
}

func TestDownloadToFile_ServerWithoutRangesStreamsWholeFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	got, err := download(t, srv, int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %d bytes, want the exact %d-byte payload", len(got), len(payload))
	}
}

func TestDownloadToFile_FailedRangeReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The probe and the first range succeed; every other range fails.
		if !strings.HasPrefix(r.Header.Get("Range"), "bytes=0-") {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, "asset", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

	if _, err := download(t, srv, int64(len(payload))); err == nil {
		t.Fatal("expected an error when a range fails")
	}
}

func TestDownloadToFile_RejectsOversizeBeforeDownloading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "asset", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()

	_, err := download(t, srv, int64(len(payload))-1)
	if err == nil || !strings.Contains(err.Error(), "exceeds the maximum size") {
		t.Fatalf("err = %v, want the maximum size error", err)
	}
}

func TestDownloadToFile_RangesKeepTheClientTransport(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "asset", time.Time{}, bytes.NewReader(payload))
	}))
	defer srv.Close()
	f, err := os.CreateTemp(t.TempDir(), "dl-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// srv.Client() trusts the test server's certificate; http.DefaultTransport does not.
	if err := DownloadToFile(srv.Client(), srv.URL+"/asset", f, int64(len(payload)), nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %d bytes, want the exact %d-byte payload", len(got), len(payload))
	}
}
