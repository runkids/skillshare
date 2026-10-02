package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const releaseAsset = "skillshare_1.0.0_linux_amd64.tar.gz"

// releaseArchive builds the platform asset the self-uploader expects: a zip
// holding skillshare.exe on Windows, a tar.gz holding skillshare elsewhere.
func releaseArchive(t *testing.T, payload string) []byte {
	t.Helper()

	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		w, err := zw.Create("skillshare.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(payload)); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "skillshare", Mode: 0755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// upgradeServer serves the archive and a checksums file written independently,
// so a test can put the two out of agreement. An empty checksums body 404s.
func upgradeServer(t *testing.T, archive []byte, checksums string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/"+releaseAsset, func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(archive); err != nil {
			t.Error(err)
		}
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		if checksums == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if _, err := w.Write([]byte(checksums)); err != nil {
			t.Error(err)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func runningBinary(t *testing.T) string {
	t.Helper()

	dest := filepath.Join(t.TempDir(), "skillshare")
	if err := os.WriteFile(dest, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	return dest
}

func readBinary(t *testing.T, dest string) string {
	t.Helper()

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDownloadAndReplace_VerifiedArchiveReplacesBinary(t *testing.T) {
	archive := releaseArchive(t, "new binary")
	srv := upgradeServer(t, archive, sha256Hex(archive)+"  "+releaseAsset+"\n")
	dest := runningBinary(t)

	if err := downloadAndReplace(srv.URL+"/"+releaseAsset, srv.URL+"/checksums.txt", dest, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := readBinary(t, dest); got != "new binary" {
		t.Errorf("dest = %q, want the archive payload", got)
	}
}

func TestDownloadAndReplace_MismatchedChecksumLeavesBinaryAlone(t *testing.T) {
	archive := releaseArchive(t, "tampered payload")
	expected := sha256Hex(releaseArchive(t, "genuine payload"))
	srv := upgradeServer(t, archive, expected+"  "+releaseAsset+"\n")
	dest := runningBinary(t)

	err := downloadAndReplace(srv.URL+"/"+releaseAsset, srv.URL+"/checksums.txt", dest, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v, want a checksum mismatch", err)
	}
	if got := readBinary(t, dest); got != "old binary" {
		t.Errorf("the running binary was replaced despite the mismatch: %q", got)
	}
}

func TestDownloadAndReplace_AbsentAssetEntryLeavesBinaryAlone(t *testing.T) {
	archive := releaseArchive(t, "new binary")
	srv := upgradeServer(t, archive, sha256Hex(archive)+"  other_asset.tar.gz\n")
	dest := runningBinary(t)

	err := downloadAndReplace(srv.URL+"/"+releaseAsset, srv.URL+"/checksums.txt", dest, nil)
	if err == nil || !strings.Contains(err.Error(), "not found in checksums.txt") {
		t.Fatalf("err = %v, want the asset to be reported missing", err)
	}
	if got := readBinary(t, dest); got != "old binary" {
		t.Errorf("dest = %q, want the running binary untouched", got)
	}
}

func TestDownloadAndReplace_UnreachableChecksumsLeavesBinaryAlone(t *testing.T) {
	archive := releaseArchive(t, "new binary")
	srv := upgradeServer(t, archive, "")
	dest := runningBinary(t)

	err := downloadAndReplace(srv.URL+"/"+releaseAsset, srv.URL+"/checksums.txt", dest, nil)
	if err == nil || !strings.Contains(err.Error(), "checksums.txt returned HTTP 404") {
		t.Fatalf("err = %v, want the checksums.txt status reported", err)
	}
	if got := readBinary(t, dest); got != "old binary" {
		t.Errorf("dest = %q, want the running binary untouched", got)
	}
}

func TestDownloadArchive_InterruptedDownloadLeavesNoTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TMPDIR", tmpDir)
	t.Setenv("TMP", tmpDir)
	t.Setenv("TEMP", tmpDir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1024")
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	if _, err := downloadArchive(srv.URL+"/"+releaseAsset, nil); err == nil {
		t.Fatal("expected an error for an interrupted download")
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("interrupted download left %d file(s) in the temp dir", len(entries))
	}
}
