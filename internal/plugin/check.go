package plugin

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

// memo runs each key's read once and gives every caller its answer, also callers that ask
// while it is still running.
type memo[T any] struct {
	mu    sync.Mutex
	calls map[string]*memoCall[T]
}

type memoCall[T any] struct {
	once  sync.Once
	value T
	err   error
}

func (m *memo[T]) get(key string, read func() (T, error)) (T, error) {
	m.mu.Lock()
	if m.calls == nil {
		m.calls = map[string]*memoCall[T]{}
	}
	c, ok := m.calls[key]
	if !ok {
		c = &memoCall[T]{}
		m.calls[key] = c
	}
	m.mu.Unlock()
	c.once.Do(func() { c.value, c.err = read() })
	return c.value, c.err
}

// reads is what the previews of one operation share. Nothing outlives the operation.
type reads struct {
	sources memo[*Discovery]
	// heads are the commits remote sources point at, which a check compares before downloading anything.
	heads memo[string]
	// latests are npm's latest versions of the packages Pi follows.
	latests memo[[]int]
	// hosts is set only for a check: an update changes what an Agent has between plugins.
	hosts *memo[Host]
}

// sharedHost reads an Agent once per check, however many plugins ask about it.
func (s *Service) sharedHost(ctx context.Context, target string) Host {
	if s.reads == nil || s.reads.hosts == nil {
		return s.host(ctx, target)
	}
	h, _ := s.reads.hosts.get(target, func() (Host, error) { return s.host(ctx, target), nil })
	return h
}

// Check looks for updates to every plugin at once and passes each plugin's plan to each as soon
// as it is ready, one call at a time. The plugins share one read of each Agent and each source,
// and a plugin waits only on the Agents it is bound to, so an Agent that hangs holds up only its
// own plugins. There is no cap on how many run together: a capped plugin waiting on a hung Agent
// would keep the others from starting.
func (s *Service) Check(ctx context.Context, r Request, each func(name string, p *Plan, err error)) error {
	if r.Action != "check" || r.Name != "" {
		return fmt.Errorf("check every plugin takes the check action without a name")
	}
	r.Targets = slices.Clone(r.Targets)
	for i, target := range r.Targets {
		r.Targets[i] = NormalizeTarget(target)
	}
	if err := s.validate(r); err != nil {
		return err
	}
	d, err := s.load()
	if err != nil {
		return err
	}
	check := *s
	check.reads = &reads{hosts: &memo[Host]{}}
	var mu sync.Mutex
	var wg sync.WaitGroup
	// Ask every Agent at once, so a plugin bound to several does not ask them one after another.
	targets := map[string]bool{}
	for _, p := range d.packages {
		for target := range p.Bindings {
			targets[target] = len(r.Targets) == 0 || slices.Contains(r.Targets, target)
		}
	}
	for target, ask := range targets {
		if ask {
			wg.Go(func() { check.sharedHost(ctx, target) })
		}
	}
	for name := range d.packages {
		wg.Go(func() {
			one := r
			one.Name = name
			p, err := check.preview(ctx, one, nil)
			mu.Lock()
			defer mu.Unlock()
			each(name, p, err)
		})
	}
	wg.Wait()
	return nil
}

// checkAll is Check as one plan in name order, failing as the first plugin that failed does.
func (s *Service) checkAll(ctx context.Context, r Request) (*Plan, error) {
	plans, errs := map[string]*Plan{}, map[string]error{}
	if err := s.Check(ctx, r, func(name string, p *Plan, err error) { plans[name], errs[name] = p, err }); err != nil {
		return nil, err
	}
	all := &Plan{Changes: []Change{}}
	var revisions strings.Builder
	for _, name := range slices.Sorted(maps.Keys(plans)) {
		if errs[name] != nil {
			return nil, errs[name]
		}
		all.Changes = append(all.Changes, plans[name].Changes...)
		all.Blocked = all.Blocked || plans[name].Blocked
		revisions.WriteString(plans[name].Revision)
	}
	all.Revision = hash([]byte(revisions.String()))
	return all, nil
}

// sourceUnmoved reports whether a remote source still points at the commit b installed, so a
// check need not download it: the same commit is the same content. Anything it cannot tell,
// such as a pinned ref or a source that does not answer, is left to the download.
func (s *Service) sourceUnmoved(ctx context.Context, b Binding, source, ref, plugin, entry string) bool {
	if ref != "" || b.Commit == "" || b.Digest == "" || b.Pending != "" || source != b.Source || ref != b.SourceRef || plugin != b.Plugin || entry != b.Entry {
		return false
	}
	head, err := s.reads.heads.get(source, func() (string, error) { return remoteHead(ctx, source) })
	return err == nil && head == b.Commit
}
