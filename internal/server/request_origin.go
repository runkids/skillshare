package server

import (
	"net/http"
	"net/url"
)

// rejectCrossSiteWrites refuses a request that changes state when a browser sent it from
// another site. A page can submit a form or a no-cors fetch to the dashboard without a CORS
// preflight, and the handlers accept a text/plain or empty body, so without this any site the
// user visits could install, sync or push. Browsers label such requests with Origin or
// Sec-Fetch-Site; a client that sends neither, such as curl, is not driven by a web page.
func rejectCrossSiteWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !sameOriginRequest(r) {
				writeError(w, http.StatusForbidden, "Request from another site refused: open the dashboard directly to make changes")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireSameOrigin applies the same check to a GET route that changes state, such as an
// update stream that EventSource can only open with GET.
func requireSameOrigin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameOriginRequest(r) {
			writeError(w, http.StatusForbidden, "Request from another site refused: open the dashboard directly to make changes")
			return
		}
		next(w, r)
	}
}

// sameOriginRequest reports whether a browser request came from the page it was sent to:
// Fetch Metadata, when present, must say same-origin, and Origin, when present, must name
// the host and port the request was sent to.
func sameOriginRequest(r *http.Request) bool {
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return sameHost(u.Host, r.Host)
}
