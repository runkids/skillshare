package server

import (
	"os"
	"sync"
	"time"

	"skillshare/internal/install"
)

// The dashboard lists a repository's skills and then installs the ones picked.
// The listing's clone is kept for a while so the install copies from it
// instead of downloading the repository again.
const (
	discoveryKeepFor = 10 * time.Minute
	// Clones can be hundreds of MB, and some listings are never installed.
	discoveryKeepMax = 3
)

type keptDiscovery struct {
	discovery *install.DiscoveryResult
	expires   time.Time
	// used orders the last listing or install, so the least recent goes first.
	used uint64
}

// discoveryCache holds git discoveries by the source and branch they cloned.
// An install takes one out and gives it back when done, so no two requests
// read one clone at once and nothing removes a clone in use.
type discoveryCache struct {
	mu     sync.Mutex
	kept   map[string]keptDiscovery
	used   uint64
	closed bool
}

// discoveryKey names what a clone holds. Call it before cloning, which fills
// in the source's commit.
func discoveryKey(source *install.Source) string {
	return source.Raw + "\x00" + source.Branch
}

// keep stores a listing's clone in place of an older one for the same source.
// A local source costs nothing to list again, so only git clones are kept.
func (c *discoveryCache) keep(key string, d *install.DiscoveryResult) {
	if !d.Source.IsGit() {
		return
	}
	c.store(key, keptDiscovery{discovery: d, expires: time.Now().Add(discoveryKeepFor)}, true)
	time.AfterFunc(discoveryKeepFor, c.trim)
}

// take hands an install the clone kept for key, and a function to give it
// back. It returns nil when there is none to use.
func (c *discoveryCache) take(key string) (*install.DiscoveryResult, func()) {
	c.mu.Lock()
	k, ok := c.kept[key]
	delete(c.kept, key)
	c.mu.Unlock()
	if !ok {
		return nil, nil
	}
	if _, err := os.Stat(k.discovery.RepoPath); err != nil || !time.Now().Before(k.expires) {
		removeClones([]*install.DiscoveryResult{k.discovery})
		return nil, nil
	}
	return k.discovery, func() { c.store(key, k, false) }
}

// store puts k under key, then drops what has expired and the least recently
// used clone past the limit. A clone given back loses to a newer listing.
func (c *discoveryCache) store(key string, k keptDiscovery, replace bool) {
	c.mu.Lock()
	var drop []*install.DiscoveryResult
	old, exists := c.kept[key]
	if c.closed || !time.Now().Before(k.expires) || (exists && !replace) {
		drop = append(drop, k.discovery)
	} else {
		if exists {
			drop = append(drop, old.discovery)
		}
		if c.kept == nil {
			c.kept = map[string]keptDiscovery{}
		}
		c.used++
		k.used = c.used
		c.kept[key] = k
	}
	drop = append(drop, c.trimLocked()...)
	c.mu.Unlock()
	removeClones(drop)
}

func (c *discoveryCache) trim() {
	c.mu.Lock()
	drop := c.trimLocked()
	c.mu.Unlock()
	removeClones(drop)
}

func (c *discoveryCache) trimLocked() []*install.DiscoveryResult {
	var drop []*install.DiscoveryResult
	now := time.Now()
	for key, k := range c.kept {
		if !now.Before(k.expires) {
			drop = append(drop, k.discovery)
			delete(c.kept, key)
		}
	}
	// store adds one clone at a time, so one over the limit is the most there can be.
	if len(c.kept) > discoveryKeepMax {
		var least string
		for key, k := range c.kept {
			if least == "" || k.used < c.kept[least].used {
				least = key
			}
		}
		drop = append(drop, c.kept[least].discovery)
		delete(c.kept, least)
	}
	return drop
}

// close removes every kept clone. A clone in use is removed when it is given back.
func (c *discoveryCache) close() {
	c.mu.Lock()
	c.closed = true
	var drop []*install.DiscoveryResult
	for _, k := range c.kept {
		drop = append(drop, k.discovery)
	}
	c.kept = nil
	c.mu.Unlock()
	removeClones(drop)
}

func removeClones(drop []*install.DiscoveryResult) {
	for _, d := range drop {
		install.CleanupDiscovery(d)
	}
}
