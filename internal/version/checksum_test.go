package version

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseChecksum(t *testing.T) {
	input := "abc123  skillshare_1.0.0_linux_amd64.tar.gz\ndef456  skillshare-ui-dist.tar.gz\n"
	hash, err := ParseChecksum(strings.NewReader(input), "skillshare-ui-dist.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "def456" {
		t.Errorf("got %s, want def456", hash)
	}
}

func TestParseChecksum_NotFound(t *testing.T) {
	input := "abc123  other-file.tar.gz\n"
	_, err := ParseChecksum(strings.NewReader(input), "skillshare-ui-dist.tar.gz")
	if err == nil {
		t.Error("expected error for missing checksum")
	}
}

func TestFetchChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			w.Write([]byte("aaa111  skillshare_1.0.0_linux_amd64.tar.gz\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	got, err := FetchChecksum(srv.URL+"/checksums.txt", "skillshare_1.0.0_linux_amd64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "aaa111" {
		t.Errorf("got %q, want aaa111", got)
	}

	_, err = FetchChecksum(srv.URL+"/missing.txt", "skillshare_1.0.0_linux_amd64.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "returned HTTP 404") {
		t.Errorf("err = %v, want the checksums.txt status to be reported", err)
	}
}
