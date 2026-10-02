package db

import (
	"database/sql"
	"sync"
	"time"
)

// projectURLCacheTTL bounds how long an instance keeps a project's tracker fields after another instance changed them (#486).
const projectURLCacheTTL = 30 * time.Second

// projectURLFields are the only project fields a task's tracker link is built from. None of them is a credential.
type projectURLFields struct {
	IssueTracker, GithubRepo, GitRemoteUrl, TrackerUrl, GitlabUrl, GitlabProject string
}

// projectURLCache keeps the link fields of the projects a task list resolves, so a board of N tracker tickets reads
// each project once instead of N times (#486).
//
// It is process-local and standard library only: ADR 0030 rules out a shared cache. The instance that writes a project
// clears it synchronously, under d.mu; every other instance sees the change once its entry expires, at most
// projectURLCacheTTL later. Misses and read errors are not stored. Credentials never are: the settings, which carry the
// tracker tokens, are read per call (see externalURLResolver).
//
// The zero value is ready to use, so a DB assembled without openWith works too.
type projectURLCache struct {
	mu      sync.Mutex
	entries map[string]projectURLEntry
	// gen counts the clears, so a fill that read the database before a clear is not stored after it.
	gen uint64
	// now is the clock, time.Now when nil. Tests move it past the TTL.
	now func() time.Time
}

type projectURLEntry struct {
	fields  projectURLFields
	expires time.Time
}

func (c *projectURLCache) clock() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}

// get returns the fields cached for ref and, on a miss, the generation the fill must hand back to put.
func (c *projectURLCache) get(ref string) (projectURLFields, uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[ref]; ok {
		if c.clock().Before(e.expires) {
			return e.fields, c.gen, true
		}
		delete(c.entries, ref)
	}
	return projectURLFields{}, c.gen, false
}

func (c *projectURLCache) put(ref string, f projectURLFields, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return
	}
	if c.entries == nil {
		c.entries = map[string]projectURLEntry{}
	}
	c.entries[ref] = projectURLEntry{fields: f, expires: c.clock().Add(projectURLCacheTTL)}
}

// clear drops everything: a default flip or a slug rename touches several keys, and project writes are rare.
func (c *projectURLCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.entries = nil
}

// projectURLFieldsUnsafe reads the link fields of the project ref names, by id or slug as tasks.project_id stores it.
// False when no project matches or the read failed; neither is cached. getProjectByIDUnsafe itself stays uncached:
// UpdateProjectAs merges into what it reads under FOR UPDATE, and a stale read there would lose an update.
func (d *DB) projectURLFieldsUnsafe(ref string) (projectURLFields, bool) {
	f, gen, ok := d.projectURLs.get(ref)
	if ok {
		return f, true
	}
	var glURL, glProj sql.NullString
	err := d.conn.QueryRow(`SELECT issue_tracker, github_repo, git_remote_url, tracker_url, gitlab_url, gitlab_project FROM projects WHERE id = ? OR slug = ?`, ref, ref).
		Scan(&f.IssueTracker, &f.GithubRepo, &f.GitRemoteUrl, &f.TrackerUrl, &glURL, &glProj)
	if err != nil {
		return projectURLFields{}, false
	}
	f.GitlabUrl, f.GitlabProject = glURL.String, glProj.String
	d.projectURLs.put(ref, f, gen)
	return f, true
}
