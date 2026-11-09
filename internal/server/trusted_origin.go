package server

import (
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ParseTrustedOrigins normalizes the origins given with --trusted-origin, such as the
// https://skills.example.com a reverse proxy serves the dashboard at, and drops duplicates.
func ParseTrustedOrigins(values []string) ([]string, error) {
	var origins []string
	for _, value := range values {
		origin, err := normalizeOrigin(value)
		if err != nil {
			return nil, fmt.Errorf("--trusted-origin %q must be a scheme and host, such as https://skills.example.com", value)
		}
		if !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	return origins, nil
}

// normalizeOrigin returns scheme://host[:port] in lower case, without a default port or a
// trailing dot.
func normalizeOrigin(value string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if (scheme != "http" && scheme != "https") || host == "" || strings.Contains(host, "*") ||
		u.User != nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("not an origin")
	}
	port := u.Port()
	if port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("invalid port")
		}
		if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
			port = ""
		}
	}
	if port != "" {
		return scheme + "://" + net.JoinHostPort(host, port), nil
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host, nil
}

// SetTrustedOrigins sets the origins, already normalized by ParseTrustedOrigins, that the
// dashboard answers at besides localhost and IP addresses.
func (s *Server) SetTrustedOrigins(origins []string) {
	s.trustedOrigins = origins
}

// servedHandler is what the listener serves: s.handler, for allowed Hosts only.
func (s *Server) servedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r) {
			s.refuseHost(w, r)
			return
		}
		s.handler.ServeHTTP(w, r)
	})
}

// maxLoggedRefusals bounds refusedHosts, since any client can send any Host.
const maxLoggedRefusals = 100

// refuseHost answers a request for an untrusted Host with the setting that would allow it:
// a page for a browser, JSON for the API, and one server log line per Host for the operator.
func (s *Server) refuseHost(w http.ResponseWriter, r *http.Request) {
	origin := suggestedOrigin(r.Host)
	setting := "--trusted-origin set to this address"
	if origin != "" {
		setting = "--trusted-origin " + origin
	}
	s.refusedMu.Lock()
	host := strings.ToLower(r.Host)
	first := !s.refusedHosts[host] && len(s.refusedHosts) < maxLoggedRefusals
	if first {
		if s.refusedHosts == nil {
			s.refusedHosts = map[string]bool{}
		}
		s.refusedHosts[host] = true
	}
	s.refusedMu.Unlock()
	if first {
		log.Printf("refused requests for Host %q: the dashboard answers only at localhost, IP addresses and trusted origins; to allow it, restart with %s", r.Host, setting)
	}

	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		writeError(w, http.StatusForbidden, fmt.Sprintf("The dashboard does not answer at %s: open it by localhost or an IP address, or restart it with %s (or SKILLSHARE_UI_TRUSTED_ORIGINS), using the scheme the browser shows", r.Host, setting))
		return
	}
	shown := origin
	if shown == "" {
		shown = "https://your.domain"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprintf(w, refusedPage, html.EscapeString(r.Host), html.EscapeString("skillshare ui stop\nskillshare ui start --trusted-origin "+shown), html.EscapeString(shown))
}

// suggestedOrigin guesses the origin to trust for a refused Host: a reverse proxy usually
// serves HTTPS on the default port, while a Host with a port usually reaches the dashboard
// directly over HTTP. It returns "" when the Host is not a usable origin.
func suggestedOrigin(host string) string {
	scheme := "https"
	if _, port := splitHostPortDefault(host); port != "" {
		scheme = "http"
	}
	origin, err := normalizeOrigin(scheme + "://" + host)
	if err != nil {
		return ""
	}
	return origin
}

const refusedPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Address not trusted</title>
<style>body{font-family:system-ui,sans-serif;max-width:42rem;margin:3rem auto;padding:0 1rem;line-height:1.55;color:#1f2328;background:#fff}
pre,code{font-family:ui-monospace,monospace}pre{background:#f3f4f6;padding:.75rem 1rem;border-radius:6px;overflow-x:auto}
@media (prefers-color-scheme:dark){body{color:#e6e6e6;background:#16181d}pre{background:#23262d}a{color:#7cb7ff}}</style></head>
<body><h1>This address is not trusted</h1>
<p>The Skillshare dashboard does not answer at <code>%s</code>. It answers only at localhost, an IP address and the origins you trust, so that another website cannot reach it through DNS rebinding.</p>
<p>To open it here, stop the dashboard and start it again with this address trusted, keeping any other flags you use:</p>
<pre>%s</pre>
<p>Or set <code>SKILLSHARE_UI_TRUSTED_ORIGINS=%s</code>. Use the scheme your address bar shows, <code>https://</code> or <code>http://</code>. See <a href="https://skillshare.runkids.cc/docs/reference/commands/ui#reverse-proxy">Reverse Proxy</a>.</p>
</body></html>
`

// hostAllowed reports whether the request names the dashboard by a host that DNS rebinding
// cannot use. A rebinding page points its own domain at this machine, so its requests carry
// that domain as Host and match it as Origin; the dashboard therefore answers only at
// localhost, an IP address, the host it was started with, and the origins the user trusted.
func (s *Server) hostAllowed(r *http.Request) bool {
	host, _ := splitHostPortDefault(r.Host)
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return true
	}
	if bindHost, _, err := net.SplitHostPort(s.addr); err == nil && bindHost != "" && strings.EqualFold(host, strings.TrimSuffix(bindHost, ".")) {
		return true
	}
	// A trusted origin matches when the Host, read with its scheme, is that origin, and an
	// Origin header, when sent, is that same origin.
	origin := r.Header.Get("Origin")
	for _, trusted := range s.trustedOrigins {
		scheme, _, _ := strings.Cut(trusted, "://")
		if asOrigin, err := normalizeOrigin(scheme + "://" + r.Host); err != nil || asOrigin != trusted {
			continue
		}
		if origin == "" {
			return true
		}
		if normalized, err := normalizeOrigin(origin); err == nil && normalized == trusted {
			return true
		}
	}
	return false
}
