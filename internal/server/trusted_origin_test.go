package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseTrustedOrigins(t *testing.T) {
	got, err := ParseTrustedOrigins([]string{"HTTPS://Skills.Example.com/", "https://skills.example.com:443", "http://box.tail.ts.net:8080"})
	want := []string{"https://skills.example.com", "http://box.tail.ts.net:8080"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseTrustedOrigins = %v, %v; want %v", got, err, want)
	}
}

func TestParseTrustedOriginsRejectsWhatIsNotAnOrigin(t *testing.T) {
	for _, value := range []string{"", "skills.example.com", "ftp://skills.example.com", "https://skills.example.com/app", "https://user@skills.example.com", "https://*.example.com", "https://skills.example.com?x=1", "https://skills.example.com:99999"} {
		if _, err := ParseTrustedOrigins([]string{value}); err == nil {
			t.Errorf("%q accepted", value)
		}
	}
}

// The served handler answers only Hosts that DNS rebinding cannot use: localhost, IP
// addresses, and the hosts of origins the user trusted with --trusted-origin.
func TestServedHandlerAnswersOnlyAllowedHosts(t *testing.T) {
	s, _ := newTestServer(t)
	s.SetTrustedOrigins([]string{"https://skills.example.com", "https://other.example.com"})
	tests := []struct {
		name, host, origin string
		want               int
	}{
		{"localhost", "localhost:19420", "", http.StatusOK},
		{"ip address", "192.168.1.20:19420", "", http.StatusOK},
		{"ipv6 loopback", "[::1]:19420", "", http.StatusOK},
		{"rebinding domain", "evil.example:19420", "", http.StatusForbidden},
		{"trusted origin", "skills.example.com", "", http.StatusOK},
		{"trusted origin with its own Origin", "skills.example.com", "https://skills.example.com", http.StatusOK},
		{"trusted host reached over http", "skills.example.com", "http://skills.example.com", http.StatusForbidden},
		{"trusted host on another port", "skills.example.com:8443", "", http.StatusForbidden},
		{"trusted host with its default port", "skills.example.com:443", "", http.StatusOK},
		{"trusted host with a trailing dot", "Skills.Example.com.", "", http.StatusOK},
		{"another trusted origin as Origin", "skills.example.com", "https://other.example.com", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rr := httptest.NewRecorder()
			s.servedHandler().ServeHTTP(rr, req)
			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func TestMCPRequestAllowedAcceptsATrustedOrigin(t *testing.T) {
	s := &Server{addr: "127.0.0.1:19420", trustedOrigins: []string{"https://skills.example.com"}}
	req := httptest.NewRequest(http.MethodPost, "/api/mcp/import", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	req.Host = "skills.example.com"
	req.Header.Set("Origin", "https://skills.example.com")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if !s.mcpRequestAllowed(req) {
		t.Fatal("trusted origin refused")
	}
}

func TestUIRestartHelperArgsKeepTrustedOrigins(t *testing.T) {
	s := &Server{addr: "127.0.0.1:19420", trustedOrigins: []string{"https://a.example", "https://b.example"}}
	got, err := s.uiRestartHelperArgs(false)
	want := []string{"__ui-restart", "--no-open", "--host", "127.0.0.1", "--port", "19420", "--trusted-origin", "https://a.example", "--trusted-origin", "https://b.example", "--global"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, %v", got, err)
	}
}

// skillshare ui --host mybox.lan opens and health-checks the dashboard by that name.
func TestServedHandlerAnswersAtTheHostItWasStartedWith(t *testing.T) {
	s := &Server{addr: "mybox.lan:19420", handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Host = "mybox.lan:19420"
	rr := httptest.NewRecorder()
	s.servedHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
}

func refusedResponse(t *testing.T, s *Server, host, accept string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rr := httptest.NewRecorder()
	s.servedHandler().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d", rr.Code)
	}
	return rr
}

// A browser opening the dashboard at an untrusted name gets a page with the setting to copy.
func TestRefusedBrowserGetsTheOriginToTrust(t *testing.T) {
	rr := refusedResponse(t, &Server{addr: "127.0.0.1:19420"}, "skills.example.com", "text/html,application/xhtml+xml")
	if body := rr.Body.String(); !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/html") || !strings.Contains(body, "--trusted-origin https://skills.example.com") {
		t.Fatalf("%s\n%s", rr.Header().Get("Content-Type"), body)
	}
}

func TestRefusedAPIRequestNamesTheOriginToTrust(t *testing.T) {
	rr := refusedResponse(t, &Server{addr: "0.0.0.0:19420"}, "box.lan:19420", "")
	if body := rr.Body.String(); !strings.Contains(body, "--trusted-origin http://box.lan:19420") {
		t.Fatal(body)
	}
}

// The server log names each refused Host once, so the operator can find why.
func TestRefusedHostIsLoggedOnce(t *testing.T) {
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	s := &Server{addr: "127.0.0.1:19420"}
	refusedResponse(t, s, "skills.example.com", "")
	refusedResponse(t, s, "skills.example.com", "")
	if got := strings.Count(buf.String(), "--trusted-origin https://skills.example.com"); got != 1 {
		t.Fatalf("logged %d times:\n%s", got, buf.String())
	}
}
