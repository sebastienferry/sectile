package db

import (
	"sync"
	"time"

	"tasks/internal/models"
)

// trackerCacheTTL bounds how long an instance keeps a tracker after another instance changed it (#486).
const trackerCacheTTL = 30 * time.Second

// trackerCache keeps the trackers a task list resolves, so a board of N tracker tickets reads each tracker once
// instead of N times (#486). It holds trackers rather than projects since #741: a ticket's link and its client are
// built from the tracker it belongs to.
//
// It is process-local and standard library only: ADR 0030 rules out a shared cache. The instance that writes a tracker
// or a project clears it synchronously, under d.mu; every other instance sees the change once its entry expires, at
// most trackerCacheTTL later. Misses and read errors are not stored. Credentials never are: a tracker carries none,
// and the settings, which carry the tracker tokens, are read per call (see externalURLResolver).
//
// The zero value is ready to use, so a DB assembled without openWith works too.
type trackerCache struct {
	mu      sync.Mutex
	entries map[string]trackerCacheEntry
	// gen counts the clears, so a fill that read the database before a clear is not stored after it.
	gen uint64
	// now is the clock, time.Now when nil. Tests move it past the TTL.
	now func() time.Time
	// members is every project's trackers and label (#741), which decide the
	// projects of a ticket; membersExpire bounds it like an entry.
	members       *membershipIndex
	membersExpire time.Time
}

type trackerCacheEntry struct {
	tracker models.Tracker
	expires time.Time
}

func (c *trackerCache) clock() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}

// get returns the tracker cached for id and, on a miss, the generation the fill must hand back to put.
func (c *trackerCache) get(id string) (models.Tracker, uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[id]; ok {
		if c.clock().Before(e.expires) {
			return e.tracker, c.gen, true
		}
		delete(c.entries, id)
	}
	return models.Tracker{}, c.gen, false
}

func (c *trackerCache) put(id string, t models.Tracker, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return
	}
	if c.entries == nil {
		c.entries = map[string]trackerCacheEntry{}
	}
	c.entries[id] = trackerCacheEntry{tracker: t, expires: c.clock().Add(trackerCacheTTL)}
}

// clear drops everything: tracker and project writes are rare, and a project write may relink its tracker.
func (c *trackerCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.entries = nil
	c.members = nil
}

// getMembership returns the cached membership index and, on a miss, the
// generation the fill must hand back to putMembership.
func (c *trackerCache) getMembership() (*membershipIndex, uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.members != nil && c.clock().Before(c.membersExpire) {
		return c.members, c.gen, true
	}
	c.members = nil
	return nil, c.gen, false
}

func (c *trackerCache) putMembership(index *membershipIndex, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return
	}
	c.members, c.membersExpire = index, c.clock().Add(trackerCacheTTL)
}

// trackerByIDUnsafe reads one tracker through the cache. Nil when no tracker has that id or the read failed; neither
// is cached. The caller gets a copy it may not write through: the slices and the map are shared with the cache.
// getProjectByIDUnsafe stays uncached: UpdateProjectAs merges into what it reads under FOR UPDATE, and a stale read
// there would lose an update.
func (d *DB) trackerByIDUnsafe(id string) *models.Tracker {
	if id == "" {
		return nil
	}
	if t, _, ok := d.trackerCache.get(id); ok {
		return &t
	}
	_, gen, _ := d.trackerCache.get(id)
	t, err := trackerByIDOn(d.conn, id)
	if err != nil || t == nil {
		return nil
	}
	d.trackerCache.put(id, *t, gen)
	return t
}
