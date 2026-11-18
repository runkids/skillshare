package check

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"skillshare/internal/install"
)

func TestArchiveResolution(t *testing.T) {
	var requests atomic.Int32
	body := "archive bytes"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.RawQuery != "download=1" {
			t.Errorf("query lost: %s", r.URL)
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	old := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	defer func() { http.DefaultTransport = old }()
	raw := srv.URL + "/pack.zip?download=1"
	hash, err := install.ArchiveHash(raw)
	if err != nil {
		t.Fatal(err)
	}
	r := Plan([]Skill{
		{Name: "a", Entry: &install.MetadataEntry{Source: raw + "#skill=a", Type: "archive-subdir", ArchiveSHA256: hash}},
		{Name: "b", Entry: &install.MetadataEntry{Source: raw + "#skill=b", Type: "archive-subdir", ArchiveSHA256: hash}},
		{Name: "no-baseline", Entry: &install.MetadataEntry{Source: raw, Type: "archive"}},
	}, "")
	if r.Remotes() != 1 || r.RemoteSkills() != 3 {
		t.Fatalf("archive treated as local or fetched per member: %+v", r)
	}
	for _, parallel := range []bool{false, true} {
		body = "archive bytes"
		for _, want := range []string{"up_to_date", "update_available"} {
			requests.Store(0)
			if want == "update_available" {
				body = "changed archive bytes"
			}
			done := 0
			results, err := r.Run(context.Background(), Options{Parallel: parallel, OnRemoteDone: func(n int) { done += n }})
			if err != nil {
				t.Fatal(err)
			}
			if done != 3 {
				t.Fatalf("progress = %d", done)
			}
			for _, result := range results {
				expected := want
				if result.Name == "no-baseline" {
					expected = "update_available"
				}
				if result.Local || result.Status != expected {
					t.Fatalf("got %+v, want remote %s", result, want)
				}
			}
			if requests.Load() != 1 {
				t.Fatalf("downloaded %d times", requests.Load())
			}
		}
	}
	srv.Close()
	results, err := r.Run(context.Background(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Status != "error" {
			t.Fatalf("unreachable ZIP: %+v", result)
		}
	}

}
