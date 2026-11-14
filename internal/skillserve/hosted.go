package skillserve

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// ProfileBinding authorizes one credential to exactly one named profile.
// Token values must come from secret environment variables, never CLI arguments.
type ProfileBinding struct{ Profile, Token string }

// NewHostedHandler eagerly loads every profile, including unbound profiles,
// and dispatches authenticated MCP and Executor requests to isolated servers.
func NewHostedHandler(builders map[string]*Builder, bindings []ProfileBinding, opts Options) (http.Handler, error) {
	if err := ValidateProfileBindings(builders, bindings); err != nil {
		return nil, err
	}
	servers := map[string]*Server{}
	writes := &sourceWrites{}
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		b := builders[name]
		if b == nil || b.Profile == nil || b.Profile.Name() != name {
			return nil, fmt.Errorf("profile %q requires explicit membership", name)
		}
		profileOpts := opts
		if opts.Warn != nil {
			profileOpts.Warn = profileWarnings{opts.Warn, name}
		}
		srv, err := NewServer(b, profileOpts)
		if err != nil {
			return nil, fmt.Errorf("profile %q: %w", name, err)
		}
		if opts.Warn != nil {
			idx := srv.svc.indexFor(srv.svc.cat)
			for _, skip := range idx.Skipped {
				fmt.Fprintf(srv.svc.opts.Warn, "warning: %s left out of the Executor index: %s\n", skip.Path, skip.Reason)
			}
			if idx.Err != nil {
				fmt.Fprintln(srv.svc.opts.Warn, "warning: "+idx.Err.Error())
			}
		}
		srv.svc.writes = writes
		writes.services = append(writes.services, srv.svc)
		servers[name] = srv
	}
	type route struct {
		credential [32]byte
		handler    http.Handler
	}
	routes := make([]route, 0, len(bindings))
	for _, b := range bindings {
		routes = append(routes, route{sha256.Sum256([]byte("Bearer " + b.Token)), httpHandler(servers[b.Profile], true)})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		var selected http.Handler
		for _, route := range routes {
			if subtle.ConstantTimeCompare(got[:], route.credential[:]) == 1 {
				selected = route.handler
			}
		}
		if selected == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		selected.ServeHTTP(w, r)
	}), nil
}

// ValidateProfileBindings checks credentials without discovering source or
// constructing services, so --check and serving use the same validation owner.
func ValidateProfileBindings(builders map[string]*Builder, bindings []ProfileBinding) error {
	if len(bindings) == 0 {
		return fmt.Errorf("hosted serving requires profile bindings")
	}
	profiles, tokens := map[string]bool{}, map[[32]byte]bool{}
	for _, b := range bindings {
		if builders[b.Profile] == nil {
			return fmt.Errorf("unknown profile %q", b.Profile)
		}
		if profiles[b.Profile] {
			return fmt.Errorf("duplicate profile binding %q", b.Profile)
		}
		if !ValidProfileToken(b.Token) {
			return fmt.Errorf("profile %q requires a nonempty credential without trailing whitespace or forbidden HTTP header bytes", b.Profile)
		}
		hash := sha256.Sum256([]byte("Bearer " + b.Token))
		if tokens[hash] {
			return fmt.Errorf("profile %q reuses a credential", b.Profile)
		}
		profiles[b.Profile], tokens[hash] = true, true
	}
	return nil
}

// ValidProfileToken reports whether an opaque credential survives HTTP header
// serialization and parsing unchanged. It never normalizes secret bytes.
func ValidProfileToken(token string) bool {
	return token != "" && strings.TrimRight(token, " \t") == token && httpguts.ValidHeaderFieldValue(token)
}

type profileWarnings struct {
	io.Writer
	name string
}

func (w profileWarnings) Write(p []byte) (int, error) {
	_, err := fmt.Fprintf(w.Writer, "profile %q: %s", w.name, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
