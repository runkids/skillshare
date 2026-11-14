package skillserve

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"slices"
)

// ProfileBinding authorizes one credential to exactly one named profile.
// Token values must come from secret environment variables, never CLI arguments.
type ProfileBinding struct{ Profile, Token string }

// NewHostedHandler eagerly loads every profile, including unbound profiles,
// and dispatches authenticated MCP and Executor requests to isolated servers.
func NewHostedHandler(builders map[string]*Builder, bindings []ProfileBinding, opts Options) (http.Handler, error) {
	if len(bindings) == 0 {
		return nil, fmt.Errorf("hosted serving requires profile bindings")
	}
	profiles, tokens := map[string]bool{}, map[[32]byte]bool{}
	for _, b := range bindings {
		if builders[b.Profile] == nil {
			return nil, fmt.Errorf("unknown profile %q", b.Profile)
		}
		if profiles[b.Profile] {
			return nil, fmt.Errorf("duplicate profile binding %q", b.Profile)
		}
		if b.Token == "" {
			return nil, fmt.Errorf("profile %q has an empty credential", b.Profile)
		}
		hash := sha256.Sum256([]byte("Bearer " + b.Token))
		if tokens[hash] {
			return nil, fmt.Errorf("profile %q reuses a credential", b.Profile)
		}
		profiles[b.Profile], tokens[hash] = true, true
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
		srv, err := NewServer(b, opts)
		if err != nil {
			return nil, fmt.Errorf("profile %q: %w", name, err)
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
		routes = append(routes, route{sha256.Sum256([]byte("Bearer " + b.Token)), HTTPHandler(servers[b.Profile], b.Token)})
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
