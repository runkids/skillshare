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
}

// discoveryCache holds git discoveries by the source and branch they cloned.
// An install takes one out and gives it back when done, so no two requests
// read one clone at once and nothing removes a clone in use.
type discoveryCache struct {
	mu     sync.Mutex
	kept   map[string]keptDiscovery
	closed bool
}

// discoveryKey names what a clone holds. Call it before cloning, which fills
// in the source's commit.
func discoveryKey(source *install.Source) string {
	return source.Raw + "\x00" + source.Branch
}

// keep stores a discovery in place of an older one for the same source.
// Anything but a git clone is left where it is.
func (c *discoveryCache) keep(key string, d *install.DiscoveryResult) {
	if !d.Source.IsGit() {
		return
	}
	c.mu.Lock()
	var drop []*install.DiscoveryResult
	if old, ok := c.kept[key]; ok {
		drop = append(drop, old.discovery)
		delete(c.kept, key)
	}
	c.mu.Unlock()
	removeClones(drop)
	c.put(key, keptDiscovery{discovery: d, expires: time.Now().Add(discoveryKeepFor)})
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
	return k.discovery, func() { c.put(key, k) }
}

// put stores k unless it has expired, the server is stopping, or a newer
// discovery took its place, then drops what has expired or is past the limit.
func (c *discoveryCache) put(key string, k keptDiscovery) {
	c.mu.Lock()
	var drop []*install.DiscoveryResult
	if _, newer := c.kept[key]; newer || c.closed || !time.Now().Before(k.expires) {
		drop = append(drop, k.discovery)
	} else {
		if c.kept == nil {
			c.kept = map[string]keptDiscovery{}
		}
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
	for len(c.kept) > discoveryKeepMax {
		oldest := ""
		for key, k := range c.kept {
			if oldest == "" || k.expires.Before(c.kept[oldest].expires) {
				oldest = key
			}
		}
		drop = append(drop, c.kept[oldest].discovery)
		delete(c.kept, oldest)
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
