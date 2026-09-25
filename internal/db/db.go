package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/secrets"
	"tasks/internal/skills"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

type SkillJob struct {
	ActivityID string
	TaskID     string
	ProjectID  string
	SkillID    string
	Prompt     string
	// ActingUser is whoever asked for the work, carried onto the queue so a
	// tracker credential belonging to a person can still be resolved once the
	// request that started it is gone. Empty means nobody asked, which is what
	// a timer does, and resolves to the server credential.
	ActingUser string
	// Mode is the execution mode already resolved by ResolveSkillMode. The
	// agent applies it; it never re-reads project settings to decide, so a
	// stale agent configuration cannot open a window inside a full chain run.
	Mode string
	// Model is the one-off model override of this launch, empty when none was
	// given. Like Mode it is resolved before the job is filed, so the agent
	// applies it without re-reading any configuration.
	Model         string
	RemovedLabels []string
	// TrackerStatus is the status named as the tracker spells it. When set, the
	// transition targets it directly instead of folding the internal status onto
	// a guessed tracker state, which cannot distinguish two columns sharing a
	// stage.
	TrackerStatus string
	// SyncTitle / SyncDescription / SyncPriority say which fields the user
	// actually edited. Pushing a field that did not change is not harmless: Jira
	// stores rich text, Taskacao a flattened copy, so re-sending an untouched
	// description destroys its formatting.
	SyncTitle       bool
	SyncDescription bool
	SyncPriority    bool
	// ChainStopStage porte l'étape où s'arrête la chaîne quand ce job en est un
	// pas ; vide, le job ne chaîne rien. Porté par le job et non par l'interface :
	// la chaîne doit survivre à la fermeture de l'onglet. Il est recopié sur le
	// run distant, seul endroit où il survit au lancement, puisque c'est la fin
	// du run qui décide s'il y a un pas suivant.
	ChainStopStage string
	// Op porte l'écriture tracker à effectuer quand SkillID vaut "tracker_op" :
	// assignation, rattachement à un épic, découpe d'épic, labels d'horizon.
	Op *TrackerOp
	// Sync carries what tells a background synchronisation from one somebody
	// asked for, when SkillID starts with "sync_".
	Sync SyncOptions
}

// SyncOptions is what the background loop adds to a synchronisation: how far
// back to read, and the fact that nobody is watching it.
type SyncOptions struct {
	// WindowMin narrows the read to the work items the tracker has touched in
	// the last so many minutes. Zero reads the whole project.
	WindowMin int
	// Background marks a pass nobody asked for. Such a pass that brings nothing
	// back leaves no activity: it runs every few minutes, and a row per pass
	// per project buries the entries the feed exists for.
	Background bool
}

// ProjectLimiter serializes background AI agent skill workers per project.
type ProjectLimiter struct {
	mu      sync.Mutex
	cond    *sync.Cond
	running map[string]int
}

func newProjectLimiter() *ProjectLimiter {
	l := &ProjectLimiter{
		running: make(map[string]int),
	}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *ProjectLimiter) Acquire(projectID string, limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		if limit < 1 {
			limit = 1
		}
		if l.running[projectID] < limit {
			l.running[projectID]++
			return
		}
		l.cond.Wait()
	}
}

func (l *ProjectLimiter) Release(projectID string) {
	l.mu.Lock()
	if l.running[projectID] > 0 {
		l.running[projectID]--
	}
	l.cond.Broadcast()
	l.mu.Unlock()
}

type DB struct {
	agentOperations AgentOperations
	trackers        *trackerapi.Client
	trackerRegistry *tracker.Registry
	// serverKey opens the credentials a user did not seal behind a passphrase.
	// It lives outside the database, so a copy of the database alone is useless.
	// serverKeyErr says it could not be read, in which case it must never be
	// used: a zero key is a valid AES key, and encrypting under it would be
	// worse than refusing.
	serverKey    secrets.Key
	serverKeyErr error
	// unlocked holds the keys derived from sealing passphrases, for this
	// server's lifetime only.
	unlocked unlockedKeys
	// prEvidenceLookup stands in for the forge answer on every route. It gets
	// the repository asked (the foreign identity for a pull request in another
	// repository), the branch and the prUrl the caller gave.
	prEvidenceLookup func(repo, branch, prURL string) (trackerapi.PullRequest, error)
	// prDiscoveryLookup stands in for the tracker read that answers which pull
	// requests belong to an issue, so the discovery step is testable without a
	// live forge. See internal/db/prdiscovery.go.
	prDiscoveryLookup func(projectID, key string) ([]models.TaskPullRequest, error)
	conn              *sqlConn
	// dialect carries what differs between the engines: placeholder
	// rebinding, DDL type names, whether the legacy migrations apply. Every
	// query and all 210 methods below are shared. See docs/adrs/0016.
	dialect  dialect
	cfg      Config
	mu       sync.RWMutex
	jobQueue chan SkillJob
	limiter  *ProjectLimiter
	// auto porte l'état de la boucle de synchronisation de fond.
	auto              *autoSync
	cancelMap         map[string]context.CancelFunc
	cancelMu          sync.Mutex
	postBackListeners []PostBackListener
	postBackMu        sync.RWMutex
	// relayed receives the events other server instances published. See
	// internal/db/bus.go.
	relayed   []func(BusMessage)
	relayedMu sync.RWMutex
	// jobs counts the queue work in flight, so Close can wait for it instead of
	// pulling the database out from under a write.
	jobs inFlightJobs
	// instanceID names this process among the server instances sharing the
	// database. Every activity it creates or starts executing carries it. See
	// internal/db/instances.go.
	instanceID string
	// instanceAddress is where the other instances reach this one's internal
	// endpoints. See internal/db/presence.go.
	instanceAddress string
}

// NewDB opens a SQLite database at dbPath. It is the path-shaped entry point the
// desktop application and the tests use; Open is the general one.
func NewDB(dbPath string) (*DB, error) {
	return Open(SQLiteConfig(dbPath))
}

// Open connects the store to whichever engine cfg names. SQLite is the default
// and the only engine the desktop application ships with; PostgreSQL is the
// alternative an operator can point a server at.
func Open(cfg Config) (*DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	d, err := newDialect(cfg)
	if err != nil {
		return nil, err
	}
	return openWith(cfg, d)
}

// openWith is Open once the dialect is chosen. It exists as its own function so
// a test can open SQLite through a dialect that skips the legacy migrations and
// compare the two schemas; nothing else should call it.
func openWith(cfg Config, d dialect) (*DB, error) {
	conn, err := d.Open(cfg)
	if err != nil {
		return nil, err
	}
	// A pool hands out connections lazily, so a DSN pointing nowhere would not
	// be noticed until the first query — by which time the server is up and
	// answering with errors. Failing here keeps a misconfiguration a startup
	// failure.
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("cannot reach the %s database: %w", d.Name(), err)
	}

	trackerClient := trackerapi.NewClient()
	// The key sits beside the database: an operator who backs one up without the
	// other ends up with a copy that opens nothing.
	//
	// It is not required to serve: a deployment on a read-only volume, or one
	// that never stores a personal credential, must still start. Only the
	// operations that need the key refuse, and they say why.
	serverKey, serverKeyErr := secrets.ServerKey(d.SecretKeyDir(cfg))
	if serverKeyErr != nil {
		log.Printf("⚠️  Clé de chiffrement indisponible (%v) : les accès tracker personnels non scellés seront refusés. Définissez %s pour la fournir.", serverKeyErr, secrets.KeyEnvVar)
	}
	db := &DB{
		serverKey:       serverKey,
		serverKeyErr:    serverKeyErr,
		conn:            newSQLConn(conn, d),
		dialect:         d,
		cfg:             cfg,
		trackers:        trackerClient,
		trackerRegistry: trackerapi.NewDefaultRegistry(trackerClient),
		jobQueue:        make(chan SkillJob, 100),
		limiter:         newProjectLimiter(),
		cancelMap:       make(map[string]context.CancelFunc),
		instanceID:      uuid.NewString(),
	}
	// The client resolves its credentials through the store, which is the only
	// component able to read the settings and the project override.
	trackerClient.Resolve = db.trackerCredentials
	// And the acting user's own credential, where they stored one.
	trackerClient.ResolveUser = db.UserTrackerCredentialsFor
	// The one path that may change the schema: the baseline on a database this
	// scheme has never seen, then every numbered migration it has not applied.
	// A failure here stops the server rather than serving requests against a
	// schema the code does not have. See internal/db/migrations.go.
	if err := db.migrateSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Work the previous process was running when it stopped. This is not a
	// migration and must never become one: it runs on every start, not once.
	db.recoverInterruptedRuns()

	// Says what it found and changes nothing: a token stored under an identity
	// no account resolves is a person's problem to settle, not a migration's.
	// See internal/db/orphancredentials.go for why neither deleting nor
	// rebinding is done here.
	db.reportOrphanedTrackerCredentials()

	// Start background queue worker
	go db.startQueueWorker()

	if err := db.seedIfEmpty(); err != nil {
		log.Printf("Warning: error seeding default data: %v", err)
	}

	return db, nil
}

// inFlightJobs counts the queue work that is waiting or running. A
// sync.WaitGroup cannot do this job: its Add was called by the worker
// goroutine, which races Close's Wait every time the counter sits at zero, and
// the race detector fails the whole package on it. An atomic counter has no
// such rule, and counting from the moment a job is queued rather than from the
// moment it starts closes the window where a job admitted to the queue just as
// the drain observed zero would run against a connection already closed.
type inFlightJobs struct{ running atomic.Int64 }

func (f *inFlightJobs) begin() { f.running.Add(1) }
func (f *inFlightJobs) end()   { f.running.Add(-1) }

// drain waits until nothing is queued or running, and reports whether it got
// there before the bound expired.
func (f *inFlightJobs) drain(bound time.Duration) bool {
	deadline := time.Now().Add(bound)
	for f.running.Load() > 0 {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

// enqueueJob puts one job in the queue, counting it in flight from here so a
// shutdown waits for work that is queued and not yet started. A full queue is
// handed to a goroutine rather than blocking the HTTP request that caused it.
func (d *DB) enqueueJob(job SkillJob) {
	d.jobs.begin()
	select {
	case d.jobQueue <- job:
	default:
		go func() { d.jobQueue <- job }()
	}
}

// Close waits for the queue work already running before closing the database.
// Those jobs write, and closing under them turns an ordinary write into
// "database is closed"; in a test it also races the temporary directory's own
// cleanup, which then fails on a directory that is not empty. The wait is
// bounded so one stuck job cannot hold a shutdown open.
func (d *DB) Close() error {
	d.jobs.drain(5 * time.Second)
	return d.conn.Close()
}

func (d *DB) TrackerRegistry() *tracker.Registry {
	if d.trackerRegistry == nil {
		d.trackerRegistry = trackerapi.NewDefaultRegistry(d.trackers)
	}
	return d.trackerRegistry
}

func (d *DB) TrackerForProject(proj *models.Project) (tracker.TicketingSystem, error) {
	return d.TrackerRegistry().ForProject(proj)
}

func (d *DB) TrackerForTask(task *models.Task) (tracker.TicketingSystem, error) {
	var proj *models.Project
	if task != nil && task.ProjectID != "" {
		proj, _ = d.GetProjectByID(task.ProjectID)
	}
	return d.TrackerRegistry().ForTask(task, proj)
}

func (d *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			theme TEXT NOT NULL DEFAULT 'dark',
			accent_color TEXT NOT NULL DEFAULT 'indigo',
			language TEXT NOT NULL DEFAULT 'fr',
			density TEXT NOT NULL DEFAULT 'standard',
			default_view TEXT NOT NULL DEFAULT 'board',
			detail_mode TEXT NOT NULL DEFAULT 'panel',
			user_name TEXT NOT NULL DEFAULT 'Developer',
			user_email TEXT NOT NULL DEFAULT 'dev@example.com',
			user_avatar TEXT NOT NULL DEFAULT '',
			ai_provider TEXT NOT NULL DEFAULT 'agy',
			ai_command_template TEXT NOT NULL DEFAULT 'agy -p "{prompt}"',
			ai_command_template_autonomous TEXT NOT NULL DEFAULT '',
			ai_model TEXT NOT NULL DEFAULT '',
			ai_skill_models TEXT NOT NULL DEFAULT '{}',
			ai_provider_models TEXT NOT NULL DEFAULT '{}',
			repo_path TEXT NOT NULL DEFAULT '.',
			issue_tracker TEXT NOT NULL DEFAULT 'local',
			github_repo TEXT NOT NULL DEFAULT '',
			prompt_clarify TEXT NOT NULL DEFAULT '',
			prompt_specify TEXT NOT NULL DEFAULT '',
			prompt_implement TEXT NOT NULL DEFAULT '',
			prompt_adjust TEXT NOT NULL DEFAULT '',
			prompt_handoff TEXT NOT NULL DEFAULT '',
			prompt_create_pr TEXT NOT NULL DEFAULT '',
			prompt_pick TEXT NOT NULL DEFAULT '',
			editor_command TEXT NOT NULL DEFAULT 'code',
			ui_scale INTEGER NOT NULL DEFAULT 100,
			auto_sync_enabled INTEGER NOT NULL DEFAULT 0,
			auto_sync_interval_sec INTEGER NOT NULL DEFAULT 60,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			jira_url TEXT NOT NULL DEFAULT '',
			jira_email TEXT NOT NULL DEFAULT '',
			jira_api_token TEXT NOT NULL DEFAULT '',
			jira_project TEXT NOT NULL DEFAULT '',
			github_api_url TEXT NOT NULL DEFAULT '',
			github_token TEXT NOT NULL DEFAULT '',
			gitlab_url TEXT NOT NULL DEFAULT '',
			gitlab_project TEXT NOT NULL DEFAULT '',
			gitlab_token TEXT NOT NULL DEFAULT '',
			spec_framework TEXT NOT NULL DEFAULT 'speckit',
			external_terminal_command TEXT NOT NULL DEFAULT ''
		);`,
		// user_settings holds the personal half of the settings: one row per
		// account, created on the first save and seeded, until then, from the
		// deployment row (ADR 0015). The deployment half stays in settings.
		`CREATE TABLE IF NOT EXISTS user_settings (
			user_id TEXT PRIMARY KEY,
			theme TEXT NOT NULL DEFAULT 'dark',
			accent_color TEXT NOT NULL DEFAULT 'indigo',
			language TEXT NOT NULL DEFAULT 'fr',
			density TEXT NOT NULL DEFAULT 'standard',
			default_view TEXT NOT NULL DEFAULT 'board',
			detail_mode TEXT NOT NULL DEFAULT 'panel',
			ui_scale INTEGER NOT NULL DEFAULT 100,
			user_name TEXT NOT NULL DEFAULT '',
			user_email TEXT NOT NULL DEFAULT '',
			user_avatar TEXT NOT NULL DEFAULT '',
			editor_command TEXT NOT NULL DEFAULT '',
			external_terminal_command TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			icon TEXT NOT NULL DEFAULT 'Folder',
			color TEXT NOT NULL DEFAULT 'indigo',
			repo_path TEXT NOT NULL DEFAULT '',
			repo_paths TEXT NOT NULL DEFAULT '[]',
			use_worktrees INTEGER NOT NULL DEFAULT 1,
			default_skill_mode TEXT NOT NULL DEFAULT '',
			full_chain_stop_stage TEXT NOT NULL DEFAULT 'reviewed',
			board_id TEXT NOT NULL DEFAULT '',
			tracker_columns TEXT NOT NULL DEFAULT '[]',
			sprints TEXT NOT NULL DEFAULT '[]',
			issue_types TEXT NOT NULL DEFAULT '[]',
			mono_repo INTEGER NOT NULL DEFAULT 1,
			stage_columns TEXT NOT NULL DEFAULT '{}',
			github_repo TEXT NOT NULL DEFAULT '',
			issue_tracker TEXT NOT NULL DEFAULT 'local',
			is_default INTEGER NOT NULL DEFAULT 0,
			-- Kept so an older binary still opens a base written by this one.
			-- Nothing reads it: a stage's internal status is fixed by the code.
			stage_mapping TEXT NOT NULL DEFAULT '{}',
			auto_sync_enabled INTEGER NOT NULL DEFAULT 0,
			auto_sync_interval_min INTEGER NOT NULL DEFAULT 5,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			git_remote_url TEXT NOT NULL DEFAULT '',
			tracker_url TEXT NOT NULL DEFAULT '',
			github_api_url TEXT NOT NULL DEFAULT '',
			github_token TEXT NOT NULL DEFAULT '',
			gitlab_url TEXT NOT NULL DEFAULT '',
			gitlab_project TEXT NOT NULL DEFAULT '',
			gitlab_token TEXT NOT NULL DEFAULT '',
			jira_project TEXT NOT NULL DEFAULT '',
			pr_creation_stage TEXT NOT NULL DEFAULT 'implemented',
			skill_overrides TEXT NOT NULL DEFAULT '{}',
			setup_providers TEXT NOT NULL DEFAULT '[]',
			spec_framework TEXT NOT NULL DEFAULT '',
			tty_mode TEXT NOT NULL DEFAULT 'integrated',
			external_terminal_command TEXT NOT NULL DEFAULT '',
			ai_provider TEXT NOT NULL DEFAULT '',
			ai_command_template TEXT NOT NULL DEFAULT '',
			ai_command_template_autonomous TEXT NOT NULL DEFAULT '',
			ai_model TEXT NOT NULL DEFAULT '',
			ai_skill_models TEXT NOT NULL DEFAULT '{}',
			owner_user_id TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL DEFAULT 'default',
			key TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'backlog',
			priority TEXT NOT NULL DEFAULT 'medium',
			labels TEXT NOT NULL DEFAULT '[]',
			pinned INTEGER NOT NULL DEFAULT 0,
			assignee TEXT NOT NULL DEFAULT '',
			assignee_avatar TEXT NOT NULL DEFAULT '',
			position INTEGER NOT NULL DEFAULT 0,
			due_date TEXT,
			branch_name TEXT,
			pr_url TEXT,
			pr_links TEXT NOT NULL DEFAULT '[]',
			pr_links_detached INTEGER NOT NULL DEFAULT 0,
			repo_path TEXT NOT NULL DEFAULT '',
			sprint TEXT NOT NULL DEFAULT '',
			team TEXT NOT NULL DEFAULT '',
			team_id TEXT NOT NULL DEFAULT '',
			tracker_created_at DATETIME,
			tracker_updated_at DATETIME,
			status_changed_at DATETIME,
			tracker_status TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'local',
			external_url TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			issue_type TEXT NOT NULL DEFAULT '',
			parent_key TEXT NOT NULL DEFAULT '',
			parent_title TEXT NOT NULL DEFAULT '',
			parent_type TEXT NOT NULL DEFAULT '',
			UNIQUE(project_id, key)
		);`,
		taskActivitiesSchema("task_activities"),
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_position ON tasks(status, position);`,
		`CREATE INDEX IF NOT EXISTS idx_activities_task ON task_activities(task_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_activities_status ON task_activities(status);`,
		`CREATE INDEX IF NOT EXISTS idx_activities_created ON task_activities(created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS user_project_bookmarks (
			user_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, project_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_user_project_bookmarks_user ON user_project_bookmarks (user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_user_project_bookmarks_project ON user_project_bookmarks (project_id);`,
		// pinned_tasks was created lazily by the pin helpers, which reach an
		// engine that skips the legacy migrations too late: the schema has to
		// declare it like any other live table.
		`CREATE TABLE IF NOT EXISTS pinned_tasks (
			task_id   TEXT PRIMARY KEY,
			pinned_at TEXT NOT NULL
		);`,
	}

	for _, query := range queries {
		if _, err := d.conn.Exec(query); err != nil {
			return err
		}
	}

	// Everything below repairs databases created by earlier versions: columns
	// added after their table, a table rebuilt for a constraint it lacked,
	// statuses renamed, timestamps rewritten. A database created today starts
	// complete, so the engines that have no such history skip all of it. See
	// docs/adrs/0016.
	if d.dialect.RunsLegacyMigrations() {
		d.applyLegacyMigrations()
	}
	// The columns an engine without those migrations still has to be given are
	// reconciled from openWith, once every table exists. See lateColumns.

	// task_activities: task_id points at a task again, and project_id carries a
	// project activity. This runs on both engines and outside the legacy block
	// above — a PostgreSQL database created since #304 holds the very
	// "sync-<x>" rows the backfill exists for, and leaving them behind would
	// make the restored foreign key impossible to create. It runs after the
	// legacy migrations so that, under SQLite, the table it rebuilds already has
	// every column those migrations add.
	if err := d.dialect.MigrateActivityAttachment(d.conn, backfillActivityAttachment); err != nil {
		log.Printf("[task_activities] attachment migration failed: %v", err)
	}
	// After the migration, never with the other indexes: on a database that
	// still has the old table, project_id does not exist yet and the statement
	// would fail the whole schema initialisation.
	if _, err := d.conn.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_project ON task_activities(project_id, created_at DESC);`); err != nil {
		log.Printf("[task_activities] idx_activities_project: %v", err)
	}

	// Seed default workspace only if projects table is completely empty
	var projectsCount int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projectsCount)
	if projectsCount == 0 {
		_, _ = d.conn.Exec(`
			INSERT INTO projects (id, name, slug, description, icon, color, repo_path, github_repo, issue_tracker, is_default)
			VALUES ('default', 'Default Project', 'default', 'Primary workspace repository', 'Folder', 'emerald', '.', '', 'local', 1);
		`)
	}

	return nil
}

// dropRetiredColumns removes the storage of features the app no longer has:
// the Linear tracker, the delivery/personal project type and the daily digest.
// Leaving the columns behind would keep serving stale values to anything that
// reads the table with SELECT *, and keep the digests growing on disk.
//
// Each statement is idempotent by failure: a database created after the removal
// no longer declares these columns, and SQLite then refuses the ALTER with an
// error this deliberately ignores, exactly like the ADD COLUMN migrations above.
func (d *DB) dropRetiredColumns() {
	for _, statement := range []string{
		"ALTER TABLE projects DROP COLUMN linear_team;",
		"ALTER TABLE projects DROP COLUMN project_type;",
		"ALTER TABLE settings DROP COLUMN linear_team;",
		"ALTER TABLE settings DROP COLUMN prompt_digest_agenda;",
		"DROP TABLE IF EXISTS daily_digests;",
	} {
		_, _ = d.conn.Exec(statement)
	}
}

func (d *DB) migrateTasksKeyUnique() {
	var sqlSchema string
	_ = d.conn.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='tasks'").Scan(&sqlSchema)
	if strings.Contains(sqlSchema, "key TEXT NOT NULL UNIQUE") || (strings.Contains(sqlSchema, "key TEXT NOT NULL") && !strings.Contains(sqlSchema, "UNIQUE(project_id, key)")) {
		_, err := d.conn.Exec(`
			PRAGMA foreign_keys=OFF;
			CREATE TABLE IF NOT EXISTS tasks_new (
				id TEXT PRIMARY KEY,
				project_id TEXT NOT NULL DEFAULT 'default',
				key TEXT NOT NULL,
				title TEXT NOT NULL,
				description TEXT NOT NULL DEFAULT '',
				status TEXT NOT NULL DEFAULT 'backlog',
				priority TEXT NOT NULL DEFAULT 'medium',
				labels TEXT NOT NULL DEFAULT '[]',
				pinned INTEGER NOT NULL DEFAULT 0,
				assignee TEXT NOT NULL DEFAULT '',
				assignee_avatar TEXT NOT NULL DEFAULT '',
				position INTEGER NOT NULL DEFAULT 0,
				due_date TEXT,
				branch_name TEXT,
				pr_url TEXT,
				repo_path TEXT NOT NULL DEFAULT '',
				sprint TEXT NOT NULL DEFAULT '',
				team TEXT NOT NULL DEFAULT '',
				team_id TEXT NOT NULL DEFAULT '',
				tracker_created_at DATETIME,
				tracker_updated_at DATETIME,
				status_changed_at DATETIME,
				tracker_status TEXT NOT NULL DEFAULT '',
				source TEXT NOT NULL DEFAULT 'local',
				external_url TEXT,
				issue_type TEXT NOT NULL DEFAULT '',
				parent_key TEXT NOT NULL DEFAULT '',
				parent_title TEXT NOT NULL DEFAULT '',
				parent_type TEXT NOT NULL DEFAULT '',
				created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(project_id, key)
			);
			INSERT OR REPLACE INTO tasks_new (
				id, project_id, key, title, description, status, priority, labels, pinned, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_created_at, tracker_updated_at, status_changed_at, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, created_at, updated_at
			)
			SELECT
				id, project_id, key, title, description, status, priority, labels, pinned, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_created_at, tracker_updated_at, status_changed_at, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, created_at, updated_at
			FROM tasks;
			DROP TABLE tasks;
			ALTER TABLE tasks_new RENAME TO tasks;
			CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
			CREATE INDEX IF NOT EXISTS idx_tasks_position ON tasks(status, position);
			CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
			CREATE INDEX IF NOT EXISTS idx_tasks_team ON tasks(team);
			CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee);
			CREATE INDEX IF NOT EXISTS idx_tasks_sprint ON tasks(sprint);
			CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_key);
			CREATE INDEX IF NOT EXISTS idx_tasks_pinned ON tasks(pinned);
			PRAGMA foreign_keys=ON;
		`)
		if err != nil {
			log.Printf("[migrateTasksKeyUnique] failed: %v", err)
		}
	}
}

// lateColumn is one column declared after PostgreSQL support shipped.
//
// The engines that skip the legacy migrations are given a complete schema when
// their database is created, and nothing afterwards: CREATE TABLE IF NOT EXISTS
// adds nothing to a table that already exists, and no ALTER is ever replayed. A
// database created by an earlier version therefore keeps the schema it was born
// with, and every write path naming a newer column fails on it with
// `column "..." does not exist`.
//
// So each column added to a CREATE TABLE from that point on is listed here as
// well. This is the whole list a reviewer has to read, and the list the upgrade
// test drives; forgetting to extend it is what shipped #327's blocked_at to a
// deployment that could no longer block an account.
type lateColumn struct {
	table      string
	name       string
	definition string
}

// addStatement is the idempotent form, which the SQLite spelling of ADD COLUMN
// cannot express. The DATETIME in a definition is rewritten to the engine's own
// type name on its way out, like every other schema statement.
func (c lateColumn) addStatement() string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s;", c.table, c.name, c.definition)
}

// lateColumns is that list, oldest first.
//
// It deliberately starts at PostgreSQL support (#296) rather than at the first
// column ever added: the ~90 columns before it only ever went missing from a
// SQLite file, which the legacy migrations repair, and a PostgreSQL database
// has never existed without them. See docs/adrs/0016.
var lateColumns = []lateColumn{
	{table: "tasks", name: "pr_links_detached", definition: "INTEGER NOT NULL DEFAULT 0"},
	{table: "projects", name: "owner_user_id", definition: "TEXT NOT NULL DEFAULT ''"},
	// #327: a blocked account keeps its row, its history and its ownership of
	// past executions, and only its sign-in stops opening. NULL is the normal
	// state, so every account an upgrade finds stays open.
	{table: "users", name: "blocked_at", definition: "DATETIME"},
}

// reconcileLateColumns gives the database whichever of lateColumns it lacks.
//
// It runs on every start, on the engines that have no legacy migrations, and
// after every table is created so a statement may name any of them. The error
// is ignored for the same reason the legacy migrations ignore theirs: SQLite
// cannot spell IF NOT EXISTS, and this path is reached under SQLite only by the
// dialect the schema-parity test opens.
func (d *DB) reconcileLateColumns() {
	for _, column := range lateColumns {
		_, _ = d.conn.Exec(column.addStatement())
	}
}

// applyLegacyMigrations brings a database written by an earlier version up to
// the current schema. It is a no-op on an engine whose databases are always
// created complete, and every statement in it is deliberately
// error-tolerant: re-adding a column that is already there is how it detects
// it has already run.
func (d *DB) applyLegacyMigrations() {
	// stage_mapping is dead weight, kept only so the schema stays identical
	// across versions; see the CREATE TABLE above.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN stage_mapping TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN git_remote_url TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN tracker_url TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN skill_overrides TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN setup_providers TEXT NOT NULL DEFAULT '[]';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN repo_paths TEXT NOT NULL DEFAULT '[]';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN pr_creation_stage TEXT NOT NULL DEFAULT 'implemented';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN use_worktrees INTEGER NOT NULL DEFAULT 1;")
	// default_skill_mode : le mode d'exécution des skills quand ni le lancement
	// ni la skill n'en fixe un. Vide vaut « interactif », le comportement
	// historique, donc les projets existants ne changent pas.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN default_skill_mode TEXT NOT NULL DEFAULT '';")
	// full_chain_stop_stage : l'étape où s'arrête une exécution en chaîne.
	// 'reviewed' est la constante historique.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN full_chain_stop_stage TEXT NOT NULL DEFAULT 'reviewed';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN board_id TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN tracker_columns TEXT NOT NULL DEFAULT '[]';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN sprints TEXT NOT NULL DEFAULT '[]';")
	// issue_types : les types de tickets qu'un projet importe. Une liste vide vaut
	// « les types par défaut », ce qui laisse les projets existants inchangés.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN issue_types TEXT NOT NULL DEFAULT '[]';")
	// mono_repo : un projet tenu dans un seul dépôt. La branche courante et le
	// sélecteur de branche n'ont de sens que là ; sur un projet dont les tickets
	// s'étalent sur plusieurs dépôts, ils montrent la branche d'un dépôt choisi
	// au hasard. Vrai par défaut, ce qui est le comportement d'avant ce réglage.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN mono_repo INTEGER NOT NULL DEFAULT 1;")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN stage_columns TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN ai_provider TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN ai_command_template TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN ai_command_template_autonomous TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN ai_model TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN ai_skill_models TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN spec_framework TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN jira_project TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN tty_mode TEXT NOT NULL DEFAULT 'integrated';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN external_terminal_command TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN external_terminal_command TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN project_id TEXT NOT NULL DEFAULT 'default';")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN branch_name TEXT;")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN pr_url TEXT;")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN repo_path TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN sprint TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN team TEXT NOT NULL DEFAULT '';")
	// team_id : le nom d'équipe ne suffit pas pour lire ses membres, l'API des
	// équipes est indexée par identifiant.
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN team_id TEXT NOT NULL DEFAULT '';")
	// Dates du tracker, distinctes de created_at / updated_at qui portent l'heure
	// d'import sur un ticket synchronisé. Sans elles, « ouvert depuis N jours »
	// se calculerait sur la date d'import, ce qui serait inventé.
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN tracker_created_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN tracker_updated_at DATETIME;")
	// Entrée dans la catégorie de statut : c'est de là que se compte « en cours
	// depuis N jours ».
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN status_changed_at DATETIME;")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_team ON tasks(team);")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee);")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN tracker_status TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_sprint ON tasks(sprint);")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN source TEXT NOT NULL DEFAULT 'local';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN external_url TEXT;")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN issue_type TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN parent_key TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN parent_title TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN parent_type TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_key);")
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;")
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_pinned ON tasks(pinned);")
	d.migrateTasksKeyUnique()
	d.migratePinnedTasks()
	// pr_links : l'ensemble ordonné des pull requests d'un ticket. Un ticket
	// produit couramment plusieurs PR (une première fusionnée, puis une suite
	// poussée sur la même branche) et pr_url seule ne peut en tenir qu'une.
	// Déclarée après la reconstruction historique de `tasks`, qui ne la connaît
	// pas et l'effacerait.
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN pr_links TEXT NOT NULL DEFAULT '[]';")
	// Reprise des lignes existantes : la PR déjà enregistrée devient le seul
	// lien de l'ensemble. Le garde-fou `pr_links = '[]'` rend l'ordre idempotent,
	// et n'exhume pas un lien qu'un humain a détaché depuis l'interface.
	_, _ = d.conn.Exec(`UPDATE tasks
		SET pr_links = json_array(json_object('url', TRIM(pr_url), 'branch', COALESCE(branch_name, '')))
		WHERE pr_links = '[]' AND pr_url IS NOT NULL AND TRIM(pr_url) != '';`)
	// pr_links_detached : un humain a retiré tous les liens depuis la fiche du
	// ticket. La redécouverte automatique respecte ce geste et se tait, jusqu'à
	// ce qu'une redécouverte soit demandée explicitement ou qu'un lien soit
	// rattaché par le workflow. Voir internal/db/prdiscovery.go.
	_, _ = d.conn.Exec("ALTER TABLE tasks ADD COLUMN pr_links_detached INTEGER NOT NULL DEFAULT 0;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN prompt TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN started_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN completed_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN error TEXT NOT NULL DEFAULT '';")
	// How a run was launched, which is what tells, once it is over, whether it
	// handed the workflow back. run_mode is the resolved execution mode,
	// launch_stage the stage the task sat on when the run started, and
	// chain_stop_stage is set only on a step of a full chain run, to the stage
	// that chain stops at. All three default to empty, which reads as "unknown"
	// on every run recorded before this.
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN run_mode TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN launch_stage TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN chain_stop_stage TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN waiting_since DATETIME;")
	// The owner of an activity; empty on rows written before ownership existed.
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN user_id TEXT NOT NULL DEFAULT '';")
	// The engine a run actually ran against. run_provider and run_model are
	// written at launch from the server's own resolution, then corrected by the
	// agent, which alone sees the workstation override. Empty reads as unknown.
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN run_provider TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN run_model TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN detail_mode TEXT NOT NULL DEFAULT 'panel';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_provider TEXT NOT NULL DEFAULT 'agy';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_command_template TEXT NOT NULL DEFAULT 'agy -p \"{prompt}\"';")
	// Additive: an older binary ignores the column, and an empty one means an
	// autonomous launch falls back to the interactive command.
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_command_template_autonomous TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_model TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_skill_models TEXT NOT NULL DEFAULT '{}';")
	// Which models each provider may run. Empty means "use the list Sectile
	// ships", so an installation that never opened the setting still offers
	// models at launch.
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_provider_models TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN repo_path TEXT NOT NULL DEFAULT '.';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN issue_tracker TEXT NOT NULL DEFAULT 'local';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN github_repo TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_clarify TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_specify TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_implement TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_adjust TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_handoff TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_create_pr TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_pick TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN editor_command TEXT NOT NULL DEFAULT 'code';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN spec_framework TEXT NOT NULL DEFAULT 'speckit';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ui_scale INTEGER NOT NULL DEFAULT 100;")
	// Boucle de synchronisation de fond : éteinte par défaut, c'est un appel
	// périodique au tracker et personne ne doit le découvrir après coup.
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN auto_sync_enabled INTEGER NOT NULL DEFAULT 0;")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN auto_sync_interval_sec INTEGER NOT NULL DEFAULT 60;")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_project TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_url TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_email TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_api_token TEXT NOT NULL DEFAULT '';")
	// Tracker connection parameters, held in the user configuration so they no
	// longer require a server environment variable and a restart.
	for _, column := range []string{"github_api_url", "github_token", "gitlab_url", "gitlab_project", "gitlab_token"} {
		_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN " + column + " TEXT NOT NULL DEFAULT '';")
		_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN " + column + " TEXT NOT NULL DEFAULT '';")
	}
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN auto_sync_enabled INTEGER NOT NULL DEFAULT 0;")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN auto_sync_interval_min INTEGER NOT NULL DEFAULT 5;")
	// The owner the background synchronisation borrows a credential from. A
	// project written before the column has none, and keeps the historical
	// behaviour, the server credential, until somebody saves it.
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN owner_user_id TEXT NOT NULL DEFAULT '';")

	// Migrate the legacy 'openfeature' Spec-Driven Design option to 'openspec'.
	// OpenFeature is a feature-flag standard, not an SDD framework: the two
	// supported frameworks are GitHub Spec Kit and OpenSpec.
	_, _ = d.conn.Exec("UPDATE projects SET spec_framework = 'openspec' WHERE spec_framework = 'openfeature';")
	_, _ = d.conn.Exec("UPDATE settings SET spec_framework = 'openspec' WHERE spec_framework = 'openfeature';")

	// Migrate legacy stage names to 5-stage workflow
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_clarify' WHERE status = 'backlog';")
	// Retire le statut interne historique, qui ne doit plus apparaître dans les
	// réponses ni dans l'interface. Les tickets déjà concernés gardent leur
	// étape métier : ils deviennent `clarified`.
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'clarified' WHERE status = 'to_specify';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_implement' WHERE status = 'in_progress';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_test' WHERE status = 'to_validate';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_close' WHERE status = 'done';")

	d.dropRetiredColumns()

	// Runs last: it sweeps the date columns of the schema as it stands once
	// every table and column above exists.
	d.repairNumericZoneTimestamps()
}

func (d *DB) seedIfEmpty() error {
	var settingsCount int
	err := d.conn.QueryRow("SELECT COUNT(*) FROM settings WHERE id = 1").Scan(&settingsCount)
	if err != nil {
		return err
	}
	if settingsCount == 0 {
		_, err = d.conn.Exec(`
			INSERT INTO settings (id, theme, accent_color, language, density, default_view, user_name, user_email, user_avatar, ai_provider, ai_command_template, repo_path, issue_tracker, github_repo)
			VALUES (1, 'dark', 'indigo', 'en', 'standard', 'board', 'Developer', 'dev@example.com', '', 'agy', 'agy -p "{prompt}"', '.', 'local', '')
		`)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DB) SeedDemoData() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Ensure default settings
	_, err := d.conn.Exec(`
		INSERT INTO settings (id, theme, accent_color, language, density, default_view, user_name, user_email, user_avatar, ai_provider, ai_command_template, repo_path, issue_tracker, github_repo)
		VALUES (1, 'dark', 'indigo', 'en', 'standard', 'board', 'Developer', 'dev@example.com', '', 'agy', 'agy -p "{prompt}"', '.', 'local', '')
		ON CONFLICT(id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP;
	`)
	if err != nil {
		return err
	}

	// Clean tasks and activities for fresh sync
	if _, err := d.conn.Exec("DELETE FROM task_activities"); err != nil {
		return err
	}
	if _, err := d.conn.Exec("DELETE FROM tasks"); err != nil {
		return err
	}

	// Seed clean demo tasks for default project
	now := time.Now()
	demoTasks := []struct {
		ID, Key, Title, Desc, Status, Priority, Branch, Labels string
		Pos                                                    int
	}{
		{"task-1", "TASK-1", "Initialize workspace configuration and metadata", "Setup project structure, metadata, and continuous integration pipeline.", "finished", "high", "TASK-1-init-workspace", `["devops", "repo"]`, 1},
		{"task-2", "TASK-2", "Configure multi-tracker sync and issue mappings", "Implement generic abstractions for GitHub, Jira, and Local SQLite storage.", "to_implement", "high", "TASK-2-configure-trackers", `["tracker", "sync", "backend"]`, 2},
		{"task-3", "TASK-3", "Refine Kanban board drag and drop interactions", "Ensure optimistic UI updates and smooth animations across all workflow stages.", "clarified", "medium", "TASK-3-kanban-board-dnd", `["ui", "kanban", "frontend"]`, 3},
		{"task-4", "TASK-4", "Implement interactive terminal session manager", "Provide browser-based PTY terminal with contextual environment variables and WebSocket streaming.", "to_test", "high", "TASK-4-terminal-session", `["pty", "terminal", "websocket"]`, 4},
		{"task-5", "TASK-5", "Integrate automated AI skill runner pipeline", "Orchestrate clarify, specify, code, and PR generation skills directly in isolated worktrees.", "to_close", "high", "TASK-5-ai-skills-pipeline", `["ai", "agent", "skills"]`, 5},
		{"task-6", "TASK-6", "Add live Git diff and branch inspector", "Display syntax-highlighted file diffs and branch status against the main repository.", "to_clarify", "low", "TASK-6-git-diff-inspector", `["git", "diff", "ui"]`, 6},
	}

	for _, dt := range demoTasks {
		_, _ = d.conn.Exec(`
			INSERT INTO tasks (id, project_id, key, title, description, status, priority, labels, assignee, position, branch_name, source, created_at, updated_at)
			VALUES (?, 'default', ?, ?, ?, ?, ?, ?, 'Developer', ?, ?, 'local', ?, ?)
		`, dt.ID, dt.Key, dt.Title, dt.Desc, dt.Status, dt.Priority, dt.Labels, dt.Pos, dt.Branch, now.Format(time.RFC3339), now)
	}

	return nil
}

func (d *DB) ImportOrUpdateTasks(syncedTasks []models.Task) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	var importErrs []string
	for _, t := range syncedTasks {
		labelsJSON, _ := json.Marshal(t.Labels)
		isPinned := HasPinnedLabel(t.Labels)
		pinnedVal := 0
		if isPinned {
			pinnedVal = 1
		}

		projID := t.ProjectID
		if projID == "" {
			var defaultProjID string
			_ = d.conn.QueryRow("SELECT id FROM projects WHERE is_default = 1 LIMIT 1").Scan(&defaultProjID)
			if defaultProjID == "" {
				defaultProjID = "default"
			}
			projID = defaultProjID
		}

		var existingID string
		err := d.conn.QueryRow("SELECT id FROM tasks WHERE project_id = ? AND (key = ? OR id = ?)", projID, t.Key, t.ID).Scan(&existingID)

		src := t.Source
		if src == "" {
			if strings.HasPrefix(t.Key, "GH-#") || strings.HasPrefix(t.Key, "gh-") || strings.HasPrefix(t.Key, "#") {
				src = "github"
			} else {
				src = "local"
			}
		}

		// Calculate task.Status dynamically based on project's trackerColumns + stageColumns mapping
		if proj, _ := d.getProjectByIDUnsafe(projID); proj != nil && len(proj.TrackerColumns) > 0 {
			stName := strings.TrimSpace(t.TrackerStatus)
			if stName == "" && len(t.Labels) > 0 {
				stName = t.Labels[len(t.Labels)-1]
			}
			if stName != "" {
				// Find which column contains this status
				matchedCol := ""
				for _, col := range proj.TrackerColumns {
					for _, s := range col.Statuses {
						if strings.EqualFold(strings.TrimSpace(s), stName) {
							matchedCol = col.Name
							break
						}
					}
					if matchedCol != "" {
						break
					}
				}

				// If matched column found, find corresponding stage
				if matchedCol != "" && proj.StageColumns != nil {
					for stageID, cols := range proj.StageColumns {
						for _, colName := range cols {
							if strings.EqualFold(colName, matchedCol) {
								t.Status = models.Status(stageID)
								break
							}
						}
					}
				}
			}
		}

		if err == sql.ErrNoRows {
			// Insert new task
			newID := t.ID
			if ts, ok := d.TrackerRegistry().Get(src); ok && ts != nil {
				newID = ts.FormatTaskID(projID, t.Key, t.ID)
			} else if newID == "" {
				newID = uuid.New().String()
			}
			if isPinned {
				_, _ = d.conn.Exec(`
					INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
					ON CONFLICT(task_id) DO NOTHING
				`, newID, t.UpdatedAt.Format(time.RFC3339))
			}
			if _, insErr := d.conn.Exec(`
				INSERT INTO tasks (id, project_id, key, title, description, status, priority, labels, pinned, assignee, assignee_avatar, creator, creator_avatar, position, due_date, source, external_url, issue_type, parent_key, parent_title, parent_type, sprint, team, team_id, tracker_status, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, newID, projID, t.Key, t.Title, t.Description, string(t.Status), string(t.Priority), string(labelsJSON), pinnedVal, t.Assignee, t.AssigneeAvatar, t.Creator, t.CreatorAvatar, t.Position, t.DueDate, src, t.ExternalURL, t.IssueType, t.ParentKey, t.ParentTitle, t.ParentType, t.Sprint, t.Team, t.TeamID, t.TrackerStatus, t.TrackerCreatedAt, t.TrackerUpdatedAt, t.StatusChangedAt, t.CreatedAt, now); insErr != nil {
				// Never swallow this: a silent failure here makes a sync report
				// "N tickets imported" while the board stays empty.
				log.Printf("[DB.ImportOrUpdateTasks] insert of %s failed: %v", t.Key, insErr)
				importErrs = append(importErrs, fmt.Sprintf("%s: %v", t.Key, insErr))
			}
		} else if err == nil {
			if isPinned {
				_, _ = d.conn.Exec(`
					INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
					ON CONFLICT(task_id) DO NOTHING
				`, existingID, t.UpdatedAt.Format(time.RFC3339))
			} else {
				_, _ = d.conn.Exec(`DELETE FROM pinned_tasks WHERE task_id = ? OR task_id = ?`, existingID, t.Key)
			}
			// Update existing task title/desc/status/labels/source
			if _, updErr := d.conn.Exec(`
				UPDATE tasks
				SET title = ?, description = ?, status = ?, priority = ?, labels = ?, pinned = ?, assignee = ?, assignee_avatar = ?, source = ?, external_url = ?,
				    creator = CASE WHEN ? != '' THEN ? ELSE creator END,
				    creator_avatar = CASE WHEN ? != '' THEN ? ELSE creator_avatar END,
				    issue_type = CASE WHEN ? != '' THEN ? ELSE issue_type END,
				    parent_key = CASE WHEN ? != '' THEN ? ELSE parent_key END,
				    parent_title = CASE WHEN ? != '' THEN ? ELSE parent_title END,
				    parent_type = CASE WHEN ? != '' THEN ? ELSE parent_type END,
				    sprint = CASE WHEN ? != '' THEN ? ELSE sprint END,
				    team = CASE WHEN ? != '' THEN ? ELSE team END,
				    team_id = CASE WHEN ? != '' THEN ? ELSE team_id END,
				    tracker_status = CASE WHEN ? != '' THEN ? ELSE tracker_status END,
				    tracker_created_at = COALESCE(?, tracker_created_at),
				    tracker_updated_at = COALESCE(?, tracker_updated_at),
				    status_changed_at = COALESCE(?, status_changed_at),
				    updated_at = ?
				WHERE id = ?
			`, t.Title, t.Description, string(t.Status), string(t.Priority), string(labelsJSON), pinnedVal, t.Assignee, t.AssigneeAvatar, src, t.ExternalURL,
				t.Creator, t.Creator, t.CreatorAvatar, t.CreatorAvatar,
				t.IssueType, t.IssueType, t.ParentKey, t.ParentKey, t.ParentTitle, t.ParentTitle, t.ParentType, t.ParentType,
				t.Sprint, t.Sprint, t.Team, t.Team, t.TeamID, t.TeamID, t.TrackerStatus, t.TrackerStatus, t.TrackerCreatedAt, t.TrackerUpdatedAt, t.StatusChangedAt, now, existingID); updErr != nil {
				log.Printf("[DB.ImportOrUpdateTasks] update of %s failed: %v", t.Key, updErr)
				importErrs = append(importErrs, fmt.Sprintf("%s: %v", t.Key, updErr))
			}
		}
	}

	if len(importErrs) > 0 {
		shown := importErrs
		if len(shown) > 3 {
			shown = shown[:3]
		}
		return fmt.Errorf("%d/%d tickets n'ont pas pu être enregistrés (ex: %s)",
			len(importErrs), len(syncedTasks), strings.Join(shown, "; "))
	}
	return nil
}

func (d *DB) computeExternalURLUnsafe(t *models.Task) *string {
	if t.ExternalURL != nil && *t.ExternalURL != "" {
		urlStr := *t.ExternalURL
		if strings.HasPrefix(urlStr, "https://api.github.com/repos/") {
			// Convert https://api.github.com/repos/{owner}/{repo}/issues/{num} -> https://github.com/{owner}/{repo}/issues/{num}
			parts := strings.TrimPrefix(urlStr, "https://api.github.com/repos/")
			converted := "https://github.com/" + parts
			return &converted
		}
		return t.ExternalURL
	}
	var proj *models.Project
	if t.ProjectID != "" {
		proj, _ = d.getProjectByIDUnsafe(t.ProjectID)
	}
	source := t.Source
	if source == "" && proj != nil {
		source = proj.IssueTracker
	}
	switch source {
	case "github":
		var repo string
		if proj != nil && proj.GithubRepo != "" {
			repo = proj.GithubRepo
		} else if proj != nil && proj.GitRemoteUrl != "" {
			repo = trackerapi.CleanGithubRepo(proj.GitRemoteUrl)
		}
		cleanNum := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(t.Key, "GH-#"), "gh-"), "#")
		if repo != "" {
			u := fmt.Sprintf("https://github.com/%s/issues/%s", repo, cleanNum)
			return &u
		}
	case "jira":
		base := ""
		if proj != nil && proj.TrackerUrl != "" {
			base = proj.TrackerUrl
		} else if s, _ := d.getSettingsUnsafe(); s != nil && s.JiraUrl != "" {
			base = s.JiraUrl
		}
		if base != "" {
			u := fmt.Sprintf("%s/browse/%s", strings.TrimSuffix(base, "/"), t.Key)
			return &u
		}
	}
	return nil
}

// TaskFacets lists the distinct tracker values present in the board, so the UI
// can offer a filter only when the tracker actually feeds the field. A GitHub or
// local project simply returns empty lists.
// containerIssueTypes are work item types that hold other work items rather than
// being work of their own. They stay out of the board and the list unless asked
// for by name: an epic is a container, shown as such by the roadmap, and a
// hundred and fifty of them among the cards is a hundred and fifty rows of
// something nobody works on.
//
// They remain in the database: the tickets under them carry their key, and the
// roadmap reads that.
var containerIssueTypes = []string{"Epic", "Initiative"}

// unassignedFilterValue is the sentinel the assignee filter uses to ask for the
// work items nobody owns. An empty parameter cannot say it: it means "no filter".
const unassignedFilterValue = "__unassigned__"

type TaskFacetMacro struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Count int    `json:"count"`
}

type TaskFacets struct {
	Sprints      []string         `json:"sprints"`
	Teams        []string         `json:"teams"`
	Macros       []TaskFacetMacro `json:"macros"`
	NoMacroCount int              `json:"noMacroCount"`
	// Assignees are the people carried by the project's work items, in the
	// tracker's own spelling. The team members are served separately: somebody
	// can be in a team without owning a single ticket yet.
	Assignees []string `json:"assignees"`
	// UnassignedCount lets the filter offer "unassigned" only when there is
	// something to show under it.
	UnassignedCount int `json:"unassignedCount"`
	// TrackerStatuses are the tracker's own status names present on the board,
	// most used first. The internal status folds a dozen tracker states onto six
	// values, which is too lossy to choose what to display.
	TrackerStatuses []TaskFacetValue `json:"trackerStatuses"`
	// Statuses, Sources and Labels count the project's work items per internal
	// status, per tracker of origin and per label.
	//
	// These counts exist because a filter's own counter must not be computed on
	// the filtered list: doing so made every counter shrink as soon as a filter
	// was set, and fall to zero once two were combined. The facets ignore every
	// filter but the project.
	Statuses []TaskFacetValue `json:"statuses"`
	Sources  []TaskFacetValue `json:"sources"`
	// IssueTypes are the tracker's own work item types present on the board. A
	// project may import a dozen of them (Bug, Technical debt, Corrective
	// action…), and telling them apart on a card starts with knowing which exist.
	IssueTypes []TaskFacetValue `json:"issueTypes"`
	Labels     []TaskFacetValue `json:"labels"`
	// Total is the project's work item count, all filters ignored.
	Total int `json:"total"`
}

// TaskFacetValue is one filterable value with the number of work items behind it.
type TaskFacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

func projectScope(projectID, userID string) (string, []interface{}) {
	if projectID != "" && projectID != "all" {
		return "(project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))", []interface{}{projectID, projectID, projectID}
	}
	if strings.TrimSpace(userID) != "" {
		return "(project_id IN (SELECT project_id FROM user_project_bookmarks WHERE user_id = ?) OR project_id IN (SELECT slug FROM projects WHERE id IN (SELECT project_id FROM user_project_bookmarks WHERE user_id = ?)))", []interface{}{userID, userID}
	}
	return "", nil
}

// TaskScope says which part of the board a task list or its facets cover: one
// project, the user's bookmarked projects ("all"), or one of the user's saved
// views, which then takes precedence over the project.
type TaskScope struct {
	UserID    string
	ProjectID string
	ViewID    string
}

// taskScopeUnsafe returns the scope as two SQL conditions over tasks: the
// projects, and the labels a saved view asks for. They are kept apart because
// the macros table shares the project column but carries no labels.
func (d *DB) taskScopeUnsafe(scope TaskScope) (projectCond string, projectArgs []interface{}, labelCond string, labelArgs []interface{}, err error) {
	if strings.TrimSpace(scope.ViewID) == "" {
		projectCond, projectArgs = projectScope(scope.ProjectID, scope.UserID)
		return projectCond, projectArgs, "", nil, nil
	}
	view, err := d.getBoardViewUnsafe(scope.UserID, scope.ViewID)
	if err != nil {
		return "", nil, "", nil, err
	}
	projectCond, projectArgs = viewProjectScope(view.ProjectIDs)
	labelCond, labelArgs = viewLabelScope(view.Labels, d.lowerASCII("labels"))
	return projectCond, projectArgs, labelCond, labelArgs, nil
}

// lockDefaultProjectUnsafe serialises every change to which project is the
// default, across server instances, by locking the settings row: a row lock
// cannot cover a project that is not inserted yet, and "one default project"
// spans every row of the table. It is always taken before any project row, so
// two writers never wait on each other in opposite orders. The settings row is
// seeded at open; it is inserted here too, for a database emptied since.
func (d *DB) lockDefaultProjectUnsafe(tx *sqlTx) error {
	return d.lockSettingsUnsafe(tx)
}

// lockSettingsUnsafe locks the single settings row until the transaction ends.
func (d *DB) lockSettingsUnsafe(tx *sqlTx) error {
	if _, err := tx.Exec("INSERT INTO settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING"); err != nil {
		return err
	}
	var id int
	return tx.QueryRow("SELECT id FROM settings WHERE id = 1" + d.forUpdate()).Scan(&id)
}

// sourceConverting marks a local task while ConvertTaskToRemote creates its
// tracker issue: the claim that keeps a second conversion, on this instance or
// another, from creating a second issue. It never reaches a reader.
const sourceConverting = "converting"

// convertClaimExpiry is how long a conversion claim holds. A tracker call takes
// seconds; a claim this old was left by a server that stopped mid-conversion.
const convertClaimExpiry = 5 * time.Minute

// taskSource is where a task comes from: its recorded source, else what its key
// looks like. A task being converted is still local until the issue exists.
func taskSource(source sql.NullString, key string) string {
	switch {
	case source.Valid && source.String == sourceConverting:
		return "local"
	case source.Valid && source.String != "":
		return source.String
	case strings.HasPrefix(key, "#") || strings.HasPrefix(key, "GH-#") || strings.HasPrefix(key, "gh-"):
		return "github"
	default:
		return "local"
	}
}

// forUpdate is the row-locking clause of the engine, appended to a SELECT run
// inside a transaction. See dialect.ForUpdate.
func (d *DB) forUpdate() string {
	if d == nil || d.dialect == nil {
		return ""
	}
	return d.dialect.ForUpdate()
}

// lowerASCII folds a TEXT expression the way asciiLower folds the value it is
// compared against: A-Z and nothing else, on either engine and whatever the
// server's collation. Both sides must fold the same characters, otherwise
// lowering `É` on one side alone makes `Équipe` miss `Équipe`.
func (d *DB) lowerASCII(expr string) string {
	if d == nil || d.dialect == nil {
		return "LOWER(" + expr + ")"
	}
	return d.dialect.LowerASCII(expr)
}

// foldSearch folds a TEXT expression for free-text search: see
// dialect.FoldSearch.
func (d *DB) foldSearch(expr string) string {
	if d == nil || d.dialect == nil {
		return "LOWER(" + expr + ")"
	}
	return d.dialect.FoldSearch(expr)
}

// asciiLower lowers A-Z, and leaves every other character as it is — an
// accented letter included. It is the Go half of lowerASCII.
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, s)
}

// viewProjectScope selects the view's projects. tasks.project_id may hold a
// project's slug rather than its id, as projectScope already allows for.
func viewProjectScope(projectIDs []string) (string, []interface{}) {
	if len(projectIDs) == 0 {
		return "1 = 0", nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(projectIDs)), ", ")
	args := make([]interface{}, 0, 2*len(projectIDs))
	for _, id := range projectIDs {
		args = append(args, id)
	}
	for _, id := range projectIDs {
		args = append(args, id)
	}
	return fmt.Sprintf("(project_id IN (%s) OR project_id IN (SELECT slug FROM projects WHERE id IN (%s)))", placeholders, placeholders), args
}

// viewLabelScope keeps the tickets carrying at least one of the labels, whole
// and regardless of case. tasks.labels is a JSON array written by
// json.Marshal, so a whole label is exactly its quoted JSON token: `"backend"`
// is found in `["Backend","ops"]` and not in `["backend-api"]`. LIKE wildcards
// in a label are escaped.
//
// lowered is the SQL that folds the labels column, and must be the engine's
// lowerASCII: the label is folded here with asciiLower, so case is ignored for
// ASCII letters and any other character has to match its own spelling.
func viewLabelScope(labels []string, lowered string) (string, []interface{}) {
	if len(labels) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(labels))
	args := make([]interface{}, 0, len(labels))
	for _, label := range labels {
		token, _ := json.Marshal(asciiLower(label))
		clauses = append(clauses, lowered+" LIKE ? ESCAPE '!'")
		args = append(args, "%"+escapeLike(string(token))+"%")
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

// searchPredicate matches query anywhere in any of columns, ignoring case and,
// on PostgreSQL, diacritics. LIKE wildcards typed in the query are literal.
// Column and pattern go through the same SQL fold, so they can never be folded
// differently.
func (d *DB) searchPredicate(query string, columns ...string) (string, []interface{}) {
	pattern := "%" + escapeLike(query) + "%"
	folded := d.foldSearch("?")
	parts := make([]string, 0, len(columns))
	args := make([]interface{}, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, d.foldSearch(column)+" LIKE "+folded+" ESCAPE '!'")
		args = append(args, pattern)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func joinScope(projectCond string, projectArgs []interface{}, labelCond string, labelArgs []interface{}) (string, []interface{}) {
	if labelCond == "" {
		return projectCond, projectArgs
	}
	if projectCond == "" {
		return labelCond, labelArgs
	}
	return projectCond + " AND " + labelCond, append(append([]interface{}{}, projectArgs...), labelArgs...)
}

// GetTaskFacets returns the sprints and teams found on the tasks of a project,
// or of the whole board when projectID is empty. The values must come from a
// dedicated query rather than from the filtered task list, otherwise selecting
// a sprint would empty the very dropdown it was picked from.
func (d *DB) GetTaskFacets(projectID string) (*TaskFacets, error) {
	return d.GetTaskFacetsForUser("", projectID)
}

func (d *DB) GetTaskFacetsForUser(userID, projectID string) (*TaskFacets, error) {
	return d.GetTaskFacetsInScope(TaskScope{UserID: userID, ProjectID: projectID})
}

// GetTaskFacetsInScope is GetTaskFacetsForUser over any scope, a saved view
// included. A view that is not the user's returns ErrBoardViewNotFound.
func (d *DB) GetTaskFacetsInScope(scope TaskScope) (*TaskFacets, error) {
	userID, projectID := scope.UserID, scope.ProjectID
	if userID != "" && scope.ViewID == "" && (projectID == "" || projectID == "all") {
		_ = d.EnsureDefaultBookmark(userID)
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	facets := &TaskFacets{
		Sprints:         []string{},
		Teams:           []string{},
		Macros:          []TaskFacetMacro{},
		Assignees:       []string{},
		TrackerStatuses: []TaskFacetValue{},
		Statuses:        []TaskFacetValue{},
		Sources:         []TaskFacetValue{},
		IssueTypes:      []TaskFacetValue{},
		Labels:          []TaskFacetValue{},
	}

	projectCond, projectArgs, labelCond, labelArgs, err := d.taskScopeUnsafe(scope)
	if err != nil {
		return nil, err
	}
	scopeCond, scopeArgs := joinScope(projectCond, projectArgs, labelCond, labelArgs)
	scopeSQL := ""
	if scopeCond != "" {
		scopeSQL = " AND " + scopeCond
	}

	for _, column := range []string{"sprint", "team"} {
		query := fmt.Sprintf("SELECT DISTINCT %s FROM tasks WHERE %s != ''%s ORDER BY %s DESC", column, column, scopeSQL, column)

		rows, err := d.conn.Query(query, scopeArgs...)
		if err != nil {
			return facets, err
		}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				continue
			}
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if column == "sprint" {
				facets.Sprints = append(facets.Sprints, value)
			} else {
				facets.Teams = append(facets.Teams, value)
			}
		}
		rows.Close()
	}

	// Les personnes sont triées par nom : un ordre décroissant sur une colonne
	// texte n'a de sens que pour un sprint, dont le nom porte le numéro.
	assigneeQuery := "SELECT assignee, COUNT(*) FROM tasks WHERE TRIM(assignee) != ''" + scopeSQL
	unassignedQuery := "SELECT COUNT(*) FROM tasks WHERE TRIM(assignee) = ''" + scopeSQL
	assigneeQuery += " GROUP BY assignee ORDER BY COUNT(*) DESC, assignee ASC"

	if rows, err := d.conn.Query(assigneeQuery, scopeArgs...); err == nil {
		for rows.Next() {
			var name string
			var count int
			if err := rows.Scan(&name, &count); err != nil {
				continue
			}
			if name = strings.TrimSpace(name); name != "" {
				facets.Assignees = append(facets.Assignees, name)
			}
		}
		rows.Close()
	}
	_ = d.conn.QueryRow(unassignedQuery, scopeArgs...).Scan(&facets.UnassignedCount)

	// Macros / Milestones
	macroCounts := make(map[string]int)
	macroTitles := make(map[string]string)
	macroQuery := "SELECT parent_key, parent_title, COUNT(*) FROM tasks WHERE (TRIM(parent_key) != '' OR TRIM(parent_title) != '')" + scopeSQL
	noMacroQuery := "SELECT COUNT(*) FROM tasks WHERE TRIM(parent_key) = '' AND TRIM(parent_title) = ''" + scopeSQL
	macroQuery += " GROUP BY parent_key, parent_title ORDER BY COUNT(*) DESC"

	if rows, err := d.conn.Query(macroQuery, scopeArgs...); err == nil {
		for rows.Next() {
			var pKey, pTitle string
			var count int
			if err := rows.Scan(&pKey, &pTitle, &count); err != nil {
				continue
			}
			pKey = strings.TrimSpace(pKey)
			pTitle = strings.TrimSpace(pTitle)
			k := pKey
			if k == "" {
				k = pTitle
			}
			if k != "" {
				macroCounts[k] += count
				if pTitle != "" {
					macroTitles[k] = pTitle
				}
			}
		}
		rows.Close()
	}
	_ = d.conn.QueryRow(noMacroQuery, scopeArgs...).Scan(&facets.NoMacroCount)

	d.ensureMacrosTable()
	// Scan macros table for existing macros
	macroTableQuery := "SELECT key, title FROM macros WHERE 1=1"
	macroArgs := []interface{}{}
	if projectCond != "" {
		macroTableQuery += " AND " + projectCond
		macroArgs = append(macroArgs, projectArgs...)
	}
	if rows, err := d.conn.Query(macroTableQuery, macroArgs...); err == nil {
		for rows.Next() {
			var k, t string
			if err := rows.Scan(&k, &t); err == nil {
				k = strings.TrimSpace(k)
				t = strings.TrimSpace(t)
				if k != "" {
					if _, exists := macroCounts[k]; !exists {
						macroCounts[k] = 0
					}
					if t != "" {
						macroTitles[k] = t
					}
				}
			}
		}
		rows.Close()
	}

	for k, count := range macroCounts {
		facets.Macros = append(facets.Macros, TaskFacetMacro{
			Key:   k,
			Title: macroTitles[k],
			Count: count,
		})
	}
	sort.Slice(facets.Macros, func(i, j int) bool {
		if facets.Macros[i].Count != facets.Macros[j].Count {
			return facets.Macros[i].Count > facets.Macros[j].Count
		}
		return facets.Macros[i].Key < facets.Macros[j].Key
	})

	statusQuery := "SELECT tracker_status, COUNT(*) FROM tasks WHERE TRIM(tracker_status) != ''" + scopeSQL
	statusQuery += " GROUP BY tracker_status ORDER BY COUNT(*) DESC, tracker_status ASC"

	if rows, err := d.conn.Query(statusQuery, scopeArgs...); err == nil {
		for rows.Next() {
			var value string
			var count int
			if err := rows.Scan(&value, &count); err != nil {
				continue
			}
			if value = strings.TrimSpace(value); value != "" {
				facets.TrackerStatuses = append(facets.TrackerStatuses, TaskFacetValue{Value: value, Count: count})
			}
		}
		rows.Close()
	}

	// Statut interne et tracker d'origine : deux regroupements simples, comptés
	// sur le même périmètre que le reste.
	for _, column := range []string{"status", "source", "issue_type"} {
		countQuery := fmt.Sprintf("SELECT %s, COUNT(*) FROM tasks WHERE TRIM(%s) != ''%s", column, column, scopeSQL)
		countQuery += fmt.Sprintf(" GROUP BY %s ORDER BY COUNT(*) DESC", column)

		rows, err := d.conn.Query(countQuery, scopeArgs...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var value string
			var count int
			if err := rows.Scan(&value, &count); err != nil {
				continue
			}
			if value = strings.TrimSpace(value); value == "" {
				continue
			}
			switch column {
			case "status":
				facets.Statuses = append(facets.Statuses, TaskFacetValue{Value: value, Count: count})
			case "source":
				facets.Sources = append(facets.Sources, TaskFacetValue{Value: value, Count: count})
			default:
				facets.IssueTypes = append(facets.IssueTypes, TaskFacetValue{Value: value, Count: count})
			}
		}
		rows.Close()
	}

	// Les labels sont stockés en JSON dans une colonne : ils se comptent en
	// mémoire, sur les seules valeurs, ce qui reste négligeable à l'échelle d'un
	// projet.
	labelQuery := "SELECT labels FROM tasks WHERE labels != '' AND labels != '[]'" + scopeSQL
	labelCounts := map[string]int{}
	if rows, err := d.conn.Query(labelQuery, scopeArgs...); err == nil {
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err != nil {
				continue
			}
			var labels []string
			if err := json.Unmarshal([]byte(raw), &labels); err != nil {
				continue
			}
			for _, label := range labels {
				if label = strings.TrimSpace(label); label != "" {
					labelCounts[label]++
				}
			}
		}
		rows.Close()
	}
	for label, count := range labelCounts {
		facets.Labels = append(facets.Labels, TaskFacetValue{Value: label, Count: count})
	}
	sort.Slice(facets.Labels, func(i, j int) bool {
		if facets.Labels[i].Count != facets.Labels[j].Count {
			return facets.Labels[i].Count > facets.Labels[j].Count
		}
		return facets.Labels[i].Value < facets.Labels[j].Value
	})

	totalQuery := "SELECT COUNT(*) FROM tasks"
	if scopeCond != "" {
		totalQuery += " WHERE " + scopeCond
	}
	_ = d.conn.QueryRow(totalQuery, scopeArgs...).Scan(&facets.Total)

	return facets, nil
}

// GetTasks lists the tasks matching the filters. pinnedOnly restricts to the
// pinned tickets, which is the fastest way back to the two or three chantiers in
// flight when the board carries three hundred.
func (d *DB) GetTasks(query, status, priority, label, projectID, sprint, team, assignee, macro string, trackerStatuses, issueTypes []string, pinnedOnly bool) ([]models.Task, error) {
	return d.GetTasksForUser("", query, status, priority, label, projectID, sprint, team, assignee, macro, trackerStatuses, issueTypes, pinnedOnly)
}

func (d *DB) GetTasksForUser(userID, query, status, priority, label, projectID, sprint, team, assignee, macro string, trackerStatuses, issueTypes []string, pinnedOnly bool) ([]models.Task, error) {
	return d.GetTasksInScope(TaskScope{UserID: userID, ProjectID: projectID}, query, status, priority, label, sprint, team, assignee, macro, trackerStatuses, issueTypes, pinnedOnly)
}

// GetTasksInScope is GetTasksForUser over any scope, a saved view included:
// the view's projects and labels select the tickets, and every other filter
// narrows them further. A view that is not the user's returns
// ErrBoardViewNotFound.
func (d *DB) GetTasksInScope(scope TaskScope, query, status, priority, label, sprint, team, assignee, macro string, trackerStatuses, issueTypes []string, pinnedOnly bool) ([]models.Task, error) {
	userID, projectID := scope.UserID, scope.ProjectID
	if userID != "" && scope.ViewID == "" && (projectID == "" || projectID == "all") {
		_ = d.EnsureDefaultBookmark(userID)
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var conditions []string
	var args []interface{}

	projectCond, projectArgs, labelCond, labelArgs, err := d.taskScopeUnsafe(scope)
	if err != nil {
		return nil, err
	}
	if scopeCond, scopeArgs := joinScope(projectCond, projectArgs, labelCond, labelArgs); scopeCond != "" {
		conditions = append(conditions, scopeCond)
		args = append(args, scopeArgs...)
	}

	if pinnedOnly {
		conditions = append(conditions, "pinned = 1")
	}

	if query != "" {
		// The parent counts in the search: looking for an epic's key or title
		// must bring back its children, the natural way to isolate a piece of
		// work when no ticket carries the epic in its own title.
		cond, searchArgs := d.searchPredicate(query, "key", "title", "description", "labels", "assignee", "parent_key", "parent_title")
		conditions = append(conditions, cond)
		args = append(args, searchArgs...)
	}

	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	if priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, priority)
	}

	if label != "" {
		conditions = append(conditions, "labels LIKE ?")
		args = append(args, "%"+label+"%")
	}

	if sprint != "" {
		conditions = append(conditions, "sprint = ?")
		args = append(args, sprint)
	}

	if team != "" {
		conditions = append(conditions, "team = ?")
		args = append(args, team)
	}

	if macro != "" {
		if strings.EqualFold(macro, unassignedFilterValue) || strings.EqualFold(macro, "__no_macro__") || strings.EqualFold(macro, "none") {
			conditions = append(conditions, "(TRIM(parent_key) = '' AND TRIM(parent_title) = '')")
		} else {
			conditions = append(conditions, "(parent_key = ? OR parent_title = ?)")
			args = append(args, macro, macro)
		}
	}

	// Les conteneurs sont écartés par défaut, et seulement par défaut : les
	// demander nommément les ramène, ce qui est le sens du sélecteur de types.
	if len(issueTypes) == 0 {
		placeholders := make([]string, 0, len(containerIssueTypes))
		for _, ct := range containerIssueTypes {
			placeholders = append(placeholders, "?")
			args = append(args, ct)
		}
		conditions = append(conditions, fmt.Sprintf("(issue_type IS NULL OR issue_type NOT IN (%s))", strings.Join(placeholders, ", ")))
	}

	// Types de tickets retenus. Un board qui porte douze types n'est lisible qu'en
	// pouvant n'en regarder qu'un : les correctives d'un côté, les stories de
	// l'autre.
	if len(issueTypes) > 0 {
		placeholders := make([]string, 0, len(issueTypes))
		for _, it := range issueTypes {
			it = strings.TrimSpace(it)
			if it == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, it)
		}
		if len(placeholders) > 0 {
			conditions = append(conditions, fmt.Sprintf("issue_type IN (%s)", strings.Join(placeholders, ", ")))
		}
	}

	// Statuts du tracker retenus : c'est le choix explicite de ce qu'on veut voir,
	// et il remplace avantageusement le masquage des terminés, qui ne connaissait
	// que le statut interne.
	if len(trackerStatuses) > 0 {
		placeholders := make([]string, 0, len(trackerStatuses))
		for _, st := range trackerStatuses {
			st = strings.TrimSpace(st)
			if st == "" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, st)
		}
		if len(placeholders) > 0 {
			conditions = append(conditions, fmt.Sprintf("tracker_status IN (%s)", strings.Join(placeholders, ", ")))
		}
	}

	// L'assigné se filtre côté base comme l'équipe : le raccourci « Mes tâches »
	// et le sélecteur de personne portent le nom tel que le tracker l'écrit, et
	// aucune vue ne refiltrait la liste côté client.
	if assignee != "" {
		if strings.EqualFold(assignee, unassignedFilterValue) {
			conditions = append(conditions, "TRIM(assignee) = ''")
		} else {
			conditions = append(conditions, "assignee = ?")
			args = append(args, assignee)
		}
	}

	sqlQuery := "SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, creator, creator_avatar, position, due_date, branch_name, pr_url, pr_links, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at FROM tasks"
	if len(conditions) > 0 {
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}
	sqlQuery += " ORDER BY status, position ASC, created_at DESC"

	rows, err := d.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var labelsJSON string
		var dueDate, branchName, prURL, repoPath, sprint, team, teamID, trackerStatus, source, extURL, issueType, parentKey, parentTitle, parentType sql.NullString
		var prLinksJSON sql.NullString
		var trackerCreatedAt, trackerUpdatedAt, statusChangedAt sql.NullTime
		var statusStr, priorityStr string

		err := rows.Scan(
			&t.ID,
			&t.ProjectID,
			&t.Key,
			&t.Title,
			&t.Description,
			&statusStr,
			&priorityStr,
			&labelsJSON,
			&t.Assignee,
			&t.AssigneeAvatar,
			&t.Creator,
			&t.CreatorAvatar,
			&t.Position,
			&dueDate,
			&branchName,
			&prURL,
			&prLinksJSON,
			&repoPath,
			&sprint,
			&team,
			&teamID,
			&trackerStatus,
			&source,
			&extURL,
			&issueType,
			&parentKey,
			&parentTitle,
			&parentType,
			&trackerCreatedAt,
			&trackerUpdatedAt,
			&statusChangedAt,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		t.Status = models.Status(statusStr)
		t.Priority = models.Priority(priorityStr)
		if dueDate.Valid {
			t.DueDate = &dueDate.String
		}
		if branchName.Valid {
			t.BranchName = &branchName.String
		}
		if prURL.Valid {
			t.PrURL = &prURL.String
		}
		t.PrLinks = decodePullRequestLinks(prLinksJSON.String)
		if repoPath.Valid && repoPath.String != "" {
			p := repoPath.String
			t.RepoPath = &p
		}
		if sprint.Valid {
			t.Sprint = sprint.String
		}
		if team.Valid {
			t.Team = team.String
			t.TeamID = teamID.String
			if trackerCreatedAt.Valid {
				created := trackerCreatedAt.Time
				t.TrackerCreatedAt = &created
			}
			if trackerUpdatedAt.Valid {
				updated := trackerUpdatedAt.Time
				t.TrackerUpdatedAt = &updated
			}
			if statusChangedAt.Valid {
				changed := statusChangedAt.Time
				t.StatusChangedAt = &changed
			}
		}
		if trackerStatus.Valid {
			t.TrackerStatus = trackerStatus.String
		}
		t.Source = taskSource(source, t.Key)

		if extURL.Valid && extURL.String != "" {
			t.ExternalURL = &extURL.String
		} else {
			t.ExternalURL = d.computeExternalURLUnsafe(&t)
		}

		t.IssueType = issueType.String
		t.ParentKey = parentKey.String
		t.ParentTitle = parentTitle.String
		t.ParentType = parentType.String

		_ = json.Unmarshal([]byte(labelsJSON), &t.Labels)
		if t.Labels == nil {
			t.Labels = []string{}
		}
		t.Pinned = HasPinnedLabel(t.Labels)

		tasks = append(tasks, t)
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	return tasks, nil
}

func (d *DB) GetTaskByID(id string) (*models.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var t models.Task
	var labelsJSON string
	var dueDate, branchName, prURL, repoPath, sprint, team, teamID, trackerStatus, source, extURL, issueType, parentKey, parentTitle, parentType sql.NullString
	var prLinksJSON sql.NullString
	var trackerCreatedAt, trackerUpdatedAt, statusChangedAt sql.NullTime
	var statusStr, priorityStr string

	err := d.conn.QueryRow(`
		SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, creator, creator_avatar, position, due_date, branch_name, pr_url, pr_links, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
		FROM tasks WHERE id = ?
	`, id).Scan(
		&t.ID,
		&t.ProjectID,
		&t.Key,
		&t.Title,
		&t.Description,
		&statusStr,
		&priorityStr,
		&labelsJSON,
		&t.Assignee,
		&t.AssigneeAvatar,
		&t.Creator,
		&t.CreatorAvatar,
		&t.Position,
		&dueDate,
		&branchName,
		&prURL,
		&prLinksJSON,
		&repoPath,
		&sprint,
		&team,
		&teamID,
		&trackerStatus,
		&source,
		&extURL,
		&issueType,
		&parentKey,
		&parentTitle,
		&parentType,
		&trackerCreatedAt,
		&trackerUpdatedAt,
		&statusChangedAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		err = d.conn.QueryRow(`
			SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, creator, creator_avatar, position, due_date, branch_name, pr_url, pr_links, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
			FROM tasks WHERE key = ? LIMIT 1
		`, id).Scan(
			&t.ID,
			&t.ProjectID,
			&t.Key,
			&t.Title,
			&t.Description,
			&statusStr,
			&priorityStr,
			&labelsJSON,
			&t.Assignee,
			&t.AssigneeAvatar,
			&t.Creator,
			&t.CreatorAvatar,
			&t.Position,
			&dueDate,
			&branchName,
			&prURL,
			&prLinksJSON,
			&repoPath,
			&sprint,
			&team,
			&teamID,
			&trackerStatus,
			&source,
			&extURL,
			&issueType,
			&parentKey,
			&parentTitle,
			&parentType,
			&trackerCreatedAt,
			&trackerUpdatedAt,
			&statusChangedAt,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	t.Status = models.Status(statusStr)
	t.Priority = models.Priority(priorityStr)
	if dueDate.Valid {
		t.DueDate = &dueDate.String
	}
	if branchName.Valid {
		t.BranchName = &branchName.String
	}
	if prURL.Valid {
		t.PrURL = &prURL.String
	}
	t.PrLinks = decodePullRequestLinks(prLinksJSON.String)
	if repoPath.Valid && repoPath.String != "" {
		p := repoPath.String
		t.RepoPath = &p
	}
	if sprint.Valid {
		t.Sprint = sprint.String
	}
	if team.Valid {
		t.Team = team.String
		t.TeamID = teamID.String
		if trackerCreatedAt.Valid {
			created := trackerCreatedAt.Time
			t.TrackerCreatedAt = &created
		}
		if trackerUpdatedAt.Valid {
			updated := trackerUpdatedAt.Time
			t.TrackerUpdatedAt = &updated
		}
		if statusChangedAt.Valid {
			changed := statusChangedAt.Time
			t.StatusChangedAt = &changed
		}
	}
	if trackerStatus.Valid {
		t.TrackerStatus = trackerStatus.String
	}
	t.Source = taskSource(source, t.Key)

	if extURL.Valid && extURL.String != "" {
		t.ExternalURL = &extURL.String
	} else {
		t.ExternalURL = d.computeExternalURLUnsafe(&t)
	}

	t.IssueType = issueType.String
	t.ParentKey = parentKey.String
	t.ParentTitle = parentTitle.String
	t.ParentType = parentType.String
	_ = json.Unmarshal([]byte(labelsJSON), &t.Labels)
	if t.Labels == nil {
		t.Labels = []string{}
	}
	t.Pinned = HasPinnedLabel(t.Labels)

	activities, _ := d.getTaskActivitiesUnsafe(t.ID)
	t.Activities = activities

	return &t, nil
}

func repoPathValue(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// ResolveTaskRepoPath returns the repository a task works in: its own pinned
// path first, then its project's, then the global setting. Trackers where one
// epic spans several codebases need the per-ticket override.
func (d *DB) ResolveTaskRepoPath(task *models.Task) string {
	if task == nil {
		return ""
	}
	if p := repoPathValue(task.RepoPath); p != "" {
		return p
	}
	if task.ProjectID != "" {
		if proj, _ := d.GetProjectByID(task.ProjectID); proj != nil && proj.RepoPath != "" {
			return proj.RepoPath
		}
	}
	if settings, _ := d.GetSettings(); settings != nil && settings.RepoPath != "" {
		return settings.RepoPath
	}
	return ""
}

// TaskWorktreesEnabled reports whether a task runs in its own Git worktree or
// directly in the clone. It is a per-project choice: the isolation is valuable
// when several agents work in parallel, and pure overhead on a solo project.
// Projects with no explicit setting keep the historical behaviour, enabled.
func (d *DB) TaskWorktreesEnabled(task *models.Task) bool {
	if task == nil || task.ProjectID == "" {
		return true
	}
	proj, err := d.GetProjectByID(task.ProjectID)
	if err != nil || proj == nil {
		return true
	}
	return proj.UseWorktrees
}

// GenerateTaskBranchName formats a valid, clean git branch name for a task.
func GenerateTaskBranchName(key, title string) string {
	cleanKey := strings.TrimSpace(key)
	var kb strings.Builder
	for _, r := range strings.ToLower(cleanKey) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			kb.WriteRune(r)
		}
	}
	cleanKey = kb.String()
	if cleanKey == "" {
		cleanKey = "task"
	}

	var tb strings.Builder
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			tb.WriteRune(r)
		} else {
			tb.WriteRune('-')
		}
	}
	slug := tb.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 35 {
		slug = strings.TrimRight(slug[:35], "-")
	}
	if slug == "" {
		slug = "work"
	}
	return fmt.Sprintf("%s-%s", cleanKey, slug)
}

// SanitizeBranchName removes characters illegal in git branch names. The rule
// itself lives in models: the agent needs it too, and the agent binary must not
// link the database package.
func SanitizeBranchName(branch string) string {
	return models.SanitizeBranchName(branch)
}

func (d *DB) EnsureTaskWorktree(mainRepoPath string, task *models.Task) (string, string, error) {
	if task == nil {
		return "", "", fmt.Errorf("task is required")
	}
	var info models.WorktreeInfo
	if err := d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "prepare_workspace"}, &info); err != nil {
		return "", "", err
	}
	task.BranchName = &info.Branch
	task.WorktreePath = &info.WorktreePath
	d.mu.Lock()
	_, err := d.conn.Exec("UPDATE tasks SET branch_name=?,worktree_path=? WHERE id=?", info.Branch, info.WorktreePath, task.ID)
	d.mu.Unlock()
	return info.WorktreePath, info.Branch, err
}

func (d *DB) RemoveTaskWorktree(mainRepoPath, taskID string) error {
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return fmt.Errorf("task not found")
	}
	return d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "remove_workspace"}, nil)
}

func (d *DB) GetTaskWorktreeInfo(taskID string) (*models.WorktreeInfo, error) {
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("task not found")
	}
	var info models.WorktreeInfo
	if err = d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "workspace_info"}, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (d *DB) GetTaskGitDiff(taskID string) (*models.GitDiffResult, error) {
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("task not found")
	}
	var info models.GitDiffResult
	if err = d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "git_diff"}, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (d *DB) resolveRepoPathUnsafe(projectIDOrPath string) string {
	if proj, _ := d.getProjectByIDUnsafe(projectIDOrPath); proj != nil {
		return proj.RepoPath
	}
	return projectIDOrPath
}

func (d *DB) GetGitStatus(projectIDOrPath string) (*models.GitStatusInfo, error) {
	var result models.GitStatusInfo
	if err := d.callAgent(agentprotocol.Operation{ProjectID: projectIDOrPath, Action: "git_status"}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *DB) GetGitBranches(projectIDOrPath string) (*models.GitBranchesInfo, error) {
	var result models.GitBranchesInfo
	if err := d.callAgent(agentprotocol.Operation{ProjectID: projectIDOrPath, Action: "git_branches"}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *DB) SwitchGitBranch(projectIDOrPath, targetBranch string, create bool) (*models.GitStatusInfo, error) {
	var result models.GitStatusInfo
	if err := d.callAgent(agentprotocol.Operation{ProjectID: projectIDOrPath, Action: "git_checkout", Branch: targetBranch, Create: create}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *DB) CleanAllLocalBranches(projectIDOrPath string) (*models.CleanBranchesResult, error) {
	var result models.CleanBranchesResult
	if err := d.callAgent(agentprotocol.Operation{ProjectID: projectIDOrPath, Action: "git_clean"}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *DB) DeleteGitBranch(projectIDOrPath string, branchName string, deleteRemote bool) error {
	return d.callAgent(agentprotocol.Operation{ProjectID: projectIDOrPath, Action: "git_delete", Branch: branchName, DeleteRemote: deleteRemote}, nil)
}

// getNextTaskKey reads through q, a transaction that holds the project row
// locked, so two local creations on two server instances never pick one key.
func (d *DB) getNextTaskKey(q interface {
	Query(string, ...any) (*sql.Rows, error)
}, projectID string, prefix string) (string, error) {
	if prefix == "" {
		prefix = "TASK"
	}
	prefix = strings.ToUpper(strings.TrimSpace(prefix))

	query := "SELECT key FROM tasks WHERE UPPER(key) LIKE ?"
	args := []interface{}{prefix + "-%"}
	if projectID != "" {
		query = "SELECT key FROM tasks WHERE project_id = ? AND UPPER(key) LIKE ?"
		args = []interface{}{projectID, prefix + "-%"}
	}

	rows, err := q.Query(query, args...)
	if err != nil {
		return fmt.Sprintf("%s-1", prefix), nil
	}
	defer rows.Close()

	maxNum := 0
	prefixWithDash := prefix + "-"
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err == nil {
			upperK := strings.ToUpper(strings.TrimSpace(k))
			if strings.HasPrefix(upperK, prefixWithDash) {
				numPart := strings.TrimPrefix(upperK, prefixWithDash)
				if num, err := strconv.Atoi(numPart); err == nil {
					if num > maxNum {
						maxNum = num
					}
				}
			}
		}
	}

	return fmt.Sprintf("%s-%d", prefix, maxNum+1), nil
}

func (d *DB) getNextGithubTaskKey(projectID string) string {
	query := "SELECT key FROM tasks WHERE key LIKE '#%' OR key LIKE 'GH-%'"
	var rows *sql.Rows
	var err error
	if projectID != "" {
		query = "SELECT key FROM tasks WHERE project_id = ? AND (key LIKE '#%' OR key LIKE 'GH-%')"
		rows, err = d.conn.Query(query, projectID)
	} else {
		rows, err = d.conn.Query(query)
	}
	if err != nil {
		return "#1"
	}
	defer rows.Close()

	maxNum := 0
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err == nil {
			s := strings.TrimSpace(k)
			s = strings.TrimPrefix(s, "GH-#")
			s = strings.TrimPrefix(s, "gh-")
			s = strings.TrimPrefix(s, "GH-")
			s = strings.TrimPrefix(s, "#")
			if num, err := strconv.Atoi(s); err == nil {
				if num > maxNum {
					maxNum = num
				}
			}
		}
	}
	return fmt.Sprintf("#%d", maxNum+1)
}

func GetStageLabelForStatus(status models.Status) string {
	clean := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(string(status), "-", "_")))
	switch clean {
	case "to_clarify", "backlog", "todo", "idea", "open", "new", "untouched":
		return "new"
	case "clarified", "cadré", "cadre", "clarify":
		return "clarified"
	case "to_implement", "specified", "spec", "specced", "in_progress", "progress", "code", "coding", "dev", "doing":
		return "specified"
	case "to_test", "to_validate", "implemented", "testing", "test", "validate", "validating", "qa":
		return "implemented"
	case "to_close", "reviewed", "review", "in_review", "pr", "mr":
		return "reviewed"
	case "finished", "done", "closed", "terminé", "termine", "completed":
		return "finished"
	default:
		// Fallback keywords check
		if strings.Contains(clean, "done") || strings.Contains(clean, "close") || strings.Contains(clean, "finish") || strings.Contains(clean, "termin") {
			return "finished"
		}
		if strings.Contains(clean, "review") || strings.Contains(clean, "pr") {
			return "reviewed"
		}
		if strings.Contains(clean, "test") || strings.Contains(clean, "validat") {
			return "implemented"
		}
		if strings.Contains(clean, "progress") || strings.Contains(clean, "code") || strings.Contains(clean, "implement") {
			return "specified"
		}
		if strings.Contains(clean, "specify") || strings.Contains(clean, "spec") || strings.Contains(clean, "clarif") {
			return "clarified"
		}
		return "new"
	}
}

// workflowLabelVariants est la liste des libellés d'étape, dans les casses que
// Sectile et les trackers utilisent. Jira distingue la casse, donc retirer un
// label exige de viser la bonne graphie : on les vise toutes.
var workflowLabelVariants = []string{
	"untouched", "new", "clarified", "specified", "implemented", "reviewed", "finished", "closed",
	"Untouched", "New", "Clarified", "Specified", "Implemented", "Reviewed", "Finished",
	"#untouched", "#new", "#clarified", "#specified", "#implemented", "#reviewed", "#finished", "#closed",
	"#Untouched", "#New", "#Clarified", "#Specified", "#Implemented", "#Reviewed", "#Finished",
}

// StaleWorkflowLabels liste les labels d'étape à retirer côté tracker quand on
// pose targetLabel. Sans ça, un ticket accumule clarified, specified,
// implemented… dans Jira/GitHub alors que Sectile n'en montre qu'un.
func StaleWorkflowLabels(targetLabel string) []string {
	target := strings.ToLower(strings.TrimLeft(strings.TrimSpace(targetLabel), "#"))
	out := []string{}
	for _, variant := range workflowLabelVariants {
		clean := strings.ToLower(strings.TrimLeft(strings.TrimSpace(variant), "#"))
		if clean == target {
			continue
		}
		out = append(out, variant)
	}
	return out
}

// SetWorkflowLabel replaces the stage label in existingLabels with targetLabel.
// Callers pass a stage name in whatever spelling they have, with or without the
// "#" prefix and in any case: this function decides the spelling that gets
// written, always "#<stage>" in lower case. Other labels keep their own case
// and prefix untouched.
func SetWorkflowLabel(existingLabels []string, targetLabel string) []string {
	var result []string
	cleanTarget := strings.TrimLeft(strings.TrimSpace(targetLabel), "#")
	for _, l := range existingLabels {
		clean := strings.ToLower(strings.TrimLeft(strings.TrimSpace(l), "#"))
		if clean == "" {
			continue
		}
		isWorkflow := false
		for _, wl := range []string{"untouched", "new", "clarified", "specified", "implemented", "reviewed", "finished", "closed"} {
			if clean == wl {
				isWorkflow = true
				break
			}
		}
		if !isWorkflow {
			origClean := strings.TrimLeft(strings.TrimSpace(l), "#")
			if strings.HasPrefix(l, "#") {
				result = append(result, "#"+origClean)
			} else {
				result = append(result, origClean)
			}
		}
	}
	if cleanTarget != "" {
		result = append(result, "#"+strings.ToLower(cleanTarget))
	}
	return result
}

// CreateTask creates a work item with no acting user, which is what background
// and machine callers do: the remote creation then uses the server credential.
func (d *DB) CreateTask(req models.CreateTaskRequest) (*models.Task, error) {
	return d.CreateTaskAs(context.Background(), req)
}

// CreateTaskAs creates a work item on behalf of whoever the context names. On a
// tracker that attributes a creation to the account its token belongs to, this
// is what puts the person's own name on the ticket they just created rather
// than a shared service account.
func (d *DB) CreateTaskAs(ctx context.Context, req models.CreateTaskRequest) (*models.Task, error) {
	// The tracker is called with no lock held (a store lock across an HTTP call
	// stalls every writer for its duration): what it needs is read first, and
	// the row is written afterwards in a transaction of its own.
	d.mu.RLock()
	settings, _ := d.getSettingsUnsafe()
	if settings == nil {
		settings = &models.Settings{
			GithubRepo: "",
			RepoPath:   ".",
		}
	}

	projID := req.ProjectID
	proj, _ := d.getProjectByIDUnsafe(projID)
	if proj == nil {
		projects, _ := d.getProjectsUnsafe()
		if len(projects) > 0 {
			proj = &projects[0]
			projID = proj.ID
		}
	}
	d.mu.RUnlock()

	githubRepo := settings.GithubRepo
	jiraProject := settings.JiraProject
	jiraUrl := settings.JiraUrl
	trackerName := settings.IssueTracker
	if trackerName == "" {
		trackerName = "local"
	}

	prefix := "TASK"
	if proj != nil {
		if proj.IssueTracker != "" {
			trackerName = proj.IssueTracker
		}
		if proj.GithubRepo != "" {
			githubRepo = proj.GithubRepo
		} else if proj.GitRemoteUrl != "" {
			githubRepo = models.CleanGithubRepo(proj.GitRemoteUrl)
		}
		if proj.JiraProject != "" {
			jiraProject = proj.JiraProject
		}
		if proj.TrackerUrl != "" {
			jiraUrl = proj.TrackerUrl
		}
		if trackerName == "jira" && proj.JiraProject != "" {
			prefix = proj.JiraProject
		} else if proj.Slug != "" {
			cleanSlug := strings.ToUpper(strings.ReplaceAll(proj.Slug, "-", ""))
			if len(cleanSlug) > 6 {
				prefix = cleanSlug[:6]
			} else {
				prefix = cleanSlug
			}
		}
	}

	// A Jira-tracked project with no explicit key falls back to the slug, which
	// is what earlier Taskacao builds used as the acli --project argument.
	if jiraProject == "" && proj != nil {
		jiraProject = strings.ToUpper(strings.ReplaceAll(proj.Slug, "-", ""))
	}

	// Issue tracker / source defaults to project tracker, but respects requested source if specified
	if req.Source == "" {
		req.Source = trackerName
	}

	if req.RequireRemoteCreation && req.Source != "local" && req.Source != "github" {
		return nil, fmt.Errorf("remote task creation is not supported for tracker %q", req.Source)
	}
	// Action Create -> Status: to_clarify, Label: New
	if req.Status == "" {
		req.Status = models.StatusToClarify
	}
	req.Labels = SetWorkflowLabel(req.Labels, "#new")

	var key string
	var extURL *string
	var id string = uuid.New().String()
	now := time.Now()

	if req.Priority == "" {
		req.Priority = models.PriorityMedium
	}

	// Remote creation requires confirmation from the server HTTP adapter.
	local := true
	ts, tsErr := d.TrackerForProject(proj)
	if tsErr == nil && ts != nil && req.Source != "local" && ts.Name() != "local" && ts.Supports(tracker.CapCreate) {
		created, err := ts.CreateIssue(ctx, tracker.CreateIssueRequest{
			Project:     proj,
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
			Labels:      req.Labels,
		})
		if err != nil {
			trackerTitle := ts.Name()
			if strings.EqualFold(trackerTitle, "github") {
				trackerTitle = "GitHub"
			}
			return nil, fmt.Errorf("%s issue creation failed: %v", trackerTitle, err)
		}
		if created != nil {
			id = ts.FormatTaskID(projID, created.Key, created.ID)
			key = created.Key
			local = false
			extURL = created.ExternalURL
		} else {
			return nil, fmt.Errorf("%s issue creation failed: empty response", ts.Name())
		}
	}

	if req.ExternalURL != nil && *req.ExternalURL != "" {
		extURL = req.ExternalURL
	} else if extURL == nil && req.Source == "github" && githubRepo != "" && strings.HasPrefix(key, "#") {
		cleanNum := strings.TrimPrefix(key, "#")
		url := fmt.Sprintf("https://github.com/%s/issues/%s", models.CleanGithubRepo(githubRepo), cleanNum)
		extURL = &url
	} else if extURL == nil && req.Source == "jira" && jiraUrl != "" && !local {
		// A local key is only known inside the transaction below, which builds
		// this URL itself.
		url := fmt.Sprintf("%s/browse/%s", strings.TrimSuffix(jiraUrl, "/"), key)
		extURL = &url
	}

	labelsJSON, _ := json.Marshal(req.Labels)
	if req.Labels == nil {
		labelsJSON = []byte("[]")
	}
	isPinned := HasPinnedLabel(req.Labels)
	issueType := strings.TrimSpace(req.IssueType)
	parentKey := req.ParentKey
	parentTitle := req.ParentTitle
	parentType := req.ParentType

	// A local key is the project's highest plus one, computed on the locked
	// project row so a creation racing on another instance waits for this one
	// instead of picking the same number. Two tasks may still share a position,
	// which only orders the column.
	var newPos int
	d.mu.Lock()
	defer d.mu.Unlock()
	err := d.conn.WithTx(func(tx *sqlTx) error {
		if local {
			if projID != "" {
				var locked string
				if err := tx.QueryRow("SELECT id FROM projects WHERE id = ?"+d.forUpdate(), projID).Scan(&locked); err != nil && err != sql.ErrNoRows {
					return err
				}
			}
			key, _ = d.getNextTaskKey(tx, projID, prefix)
			if req.Source == "jira" && jiraUrl != "" && extURL == nil && (req.ExternalURL == nil || *req.ExternalURL == "") {
				url := fmt.Sprintf("%s/browse/%s", strings.TrimSuffix(jiraUrl, "/"), key)
				extURL = &url
			}
		}
		var maxPos int
		_ = tx.QueryRow("SELECT COALESCE(MAX(position), -1) FROM tasks WHERE status = ?", req.Status).Scan(&maxPos)
		newPos = maxPos + 1

		if isPinned {
			if _, err := tx.Exec(`
				INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
				ON CONFLICT(task_id) DO UPDATE SET pinned_at = excluded.pinned_at
			`, id, now.Format(time.RFC3339)); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`
			INSERT INTO tasks (id, project_id, key, title, description, status, priority, labels, pinned, assignee, assignee_avatar, creator, creator_avatar, position, due_date, source, external_url, issue_type, parent_key, parent_title, parent_type, sprint, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, projID, key, req.Title, req.Description, string(req.Status), string(req.Priority), string(labelsJSON), boolToInt(isPinned), req.Assignee, req.AssigneeAvatar, req.Creator, req.CreatorAvatar, newPos, req.DueDate, req.Source, extURL, issueType, parentKey, parentTitle, parentType, strings.TrimSpace(req.Sprint), now, now)
		return err
	})
	if err != nil {
		return nil, err
	}

	task := &models.Task{
		ID:             id,
		ProjectID:      projID,
		Key:            key,
		Title:          req.Title,
		Description:    req.Description,
		Status:         req.Status,
		Priority:       req.Priority,
		Labels:         req.Labels,
		Pinned:         isPinned,
		Assignee:       req.Assignee,
		AssigneeAvatar: req.AssigneeAvatar,
		Creator:        req.Creator,
		CreatorAvatar:  req.CreatorAvatar,
		Position:       newPos,
		DueDate:        req.DueDate,
		Sprint:         strings.TrimSpace(req.Sprint),
		Source:         req.Source,
		ExternalURL:    extURL,
		IssueType:      issueType,
		ParentKey:      parentKey,
		ParentTitle:    parentTitle,
		ParentType:     parentType,
		Activities:     []models.TaskActivity{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	return task, nil
}

// CloneTask creates a duplicate/clone of an existing task with customized or preserved parameters.
func (d *DB) CloneTask(taskID string, req models.CloneTaskRequest) (*models.Task, error) {
	d.mu.RLock()
	src, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fmt.Sprintf("%s (Copie)", src.Title)
	}

	targetProjectID := strings.TrimSpace(req.ProjectID)
	if targetProjectID == "" {
		targetProjectID = src.ProjectID
	}

	status := req.Status
	if status == "" {
		status = models.StatusToClarify
	}

	priority := req.Priority
	if priority == "" {
		priority = src.Priority
	}

	desc := ""
	if req.IncludeDescription == nil || *req.IncludeDescription {
		desc = src.Description
	}

	labels := []string{}
	if req.IncludeLabels == nil || *req.IncludeLabels {
		for _, l := range src.Labels {
			if strings.EqualFold(l, "Specified") || strings.EqualFold(l, "Implemented") || strings.EqualFold(l, "Review") || strings.EqualFold(l, "Finished") {
				continue
			}
			labels = append(labels, l)
		}
	}
	labels = SetWorkflowLabel(labels, "#new")

	sprint := ""
	if req.Sprint != "" {
		sprint = req.Sprint
	} else if req.IncludeSprint == nil || *req.IncludeSprint {
		sprint = src.Sprint
	}

	assignee := ""
	assigneeAvatar := ""
	if req.Assignee != "" {
		assignee = req.Assignee
		assigneeAvatar = req.AssigneeAvatar
	} else if req.IncludeAssignee == nil || *req.IncludeAssignee {
		assignee = src.Assignee
		assigneeAvatar = src.AssigneeAvatar
	}

	parentKey := ""
	parentTitle := ""
	parentType := ""
	if req.IncludeParent == nil || *req.IncludeParent {
		parentKey = src.ParentKey
		parentTitle = src.ParentTitle
		parentType = src.ParentType
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = src.Source
	}

	createReq := models.CreateTaskRequest{
		ProjectID:      targetProjectID,
		Title:          title,
		Description:    desc,
		Status:         status,
		Priority:       priority,
		Labels:         labels,
		Assignee:       assignee,
		AssigneeAvatar: assigneeAvatar,
		Sprint:         sprint,
		Source:         source,
		IssueType:      src.IssueType,
		ParentKey:      parentKey,
		ParentTitle:    parentTitle,
		ParentType:     parentType,
	}

	return d.CreateTask(createReq)
}

// UpdateTask edits a work item with no acting user, for callers who have none.
func (d *DB) UpdateTask(id string, req models.UpdateTaskRequest) (*models.Task, error) {
	return d.UpdateTaskBy(Actor{}, id, req)
}

// UpdateTaskBy edits a work item on behalf of whoever asked. The tracker write
// it queues then goes out under their own credential.
func (d *DB) UpdateTaskBy(actor Actor, id string, req models.UpdateTaskRequest) (*models.Task, error) {
	task, err := d.updateTaskBy(actor, id, req)
	if err != nil {
		return task, err
	}
	// Only an edit of the links or the branch can change what the forge reports
	// for this task: a title or label edit must not wait on GitHub or GitLab.
	if req.PrLinks == nil && req.PrURL == nil && req.BranchName == nil {
		return task, nil
	}
	return d.refreshTaskPullRequestStates(tracker.WithActingUser(context.Background(), actor.ID), task), nil
}

func (d *DB) updateTaskBy(actor Actor, id string, req models.UpdateTaskRequest) (*models.Task, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// The row is locked for the whole merge: an edit racing on another server
	// instance waits, then merges into what this one wrote instead of writing
	// back a snapshot that predates it. Queue work waits for the commit.
	tx, err := d.conn.Begin()
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	existing, err := d.lockTaskUnsafe(tx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("task not found")
	}

	oldLabels := existing.Labels
	repoPathToRegister := ""
	oldAssignee := strings.TrimSpace(existing.Assignee)
	var removedLabels []string

	if req.ProjectID != nil && *req.ProjectID != "" {
		existing.ProjectID = *req.ProjectID
	}
	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Status != nil {
		oldStage := GetStageLabelForStatus(existing.Status)
		existing.Status = *req.Status
		if *req.Status == models.StatusFinished || *req.Status == models.StatusDone {
			existing.Status = models.StatusFinished
			existing.Labels = SetWorkflowLabel(existing.Labels, "#finished")
		} else if req.Labels == nil {
			newStage := GetStageLabelForStatus(*req.Status)
			if oldStage != newStage {
				removedLabels = append(removedLabels, oldStage)
			}
			existing.Labels = SetWorkflowLabel(existing.Labels, "#"+newStage)
		}
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.Labels != nil {
		newMap := make(map[string]bool)
		for _, l := range *req.Labels {
			newMap[strings.ToLower(strings.TrimPrefix(l, "#"))] = true
		}
		for _, ol := range oldLabels {
			cleanOl := strings.ToLower(strings.TrimPrefix(ol, "#"))
			if !newMap[cleanOl] {
				removedLabels = append(removedLabels, ol)
			}
		}
		existing.Labels = *req.Labels
	}
	if req.Assignee != nil {
		existing.Assignee = *req.Assignee
	}
	if req.AssigneeAvatar != nil {
		existing.AssigneeAvatar = *req.AssigneeAvatar
	}
	if req.Position != nil {
		existing.Position = *req.Position
	}
	if req.DueDate != nil {
		existing.DueDate = req.DueDate
	}
	if req.BranchName != nil {
		existing.BranchName = req.BranchName
	}
	// PrLinks is the authority; PrURL is the last of the set. A caller that sends
	// only the legacy single URL still records it as a link, and one that sends
	// an empty set detaches every link, which is how a human corrects a task
	// whose recorded PR was wrong.
	if req.PrLinks != nil {
		previousStates := map[string]string{}
		for _, link := range existing.PrLinks {
			previousStates[link.URL] = link.State
		}
		existing.PrLinks = models.NormalizePullRequestLinks(*req.PrLinks)
		for i := range existing.PrLinks {
			existing.PrLinks[i].State = previousStates[existing.PrLinks[i].URL]
		}
		existing.PrURL = pullRequestURLValue(existing.PrLinks)
	}
	if req.PrURL != nil {
		branch := ""
		if existing.BranchName != nil {
			branch = *existing.BranchName
		}
		existing.PrLinks = models.AppendPullRequestLink(existing.PrLinks, *req.PrURL, branch)
		existing.PrURL = pullRequestURLValue(existing.PrLinks)
	}

	// Détection d'une étape agentique explicite dans les labels
	var explicitStage string
	if req.Labels != nil {
		for _, l := range *req.Labels {
			stage := strings.ToLower(strings.TrimPrefix(l, "#"))
			if _, isStage := stageToInternalStatus[stage]; isStage {
				explicitStage = stage
				break
			}
		}
	}

	// Statut du tracker & workflow : alignement bidirectionnel
	if explicitStage != "" {
		existing.Labels = SetWorkflowLabel(existing.Labels, "#"+explicitStage)
		if internal, ok := InternalStatusForStage(explicitStage); ok && req.Status == nil {
			existing.Status = internal
		}
		if proj, _ := d.getProjectByIDUnsafe(existing.ProjectID); proj != nil {
			if target := TrackerStatusForStage(proj, explicitStage); target != "" {
				existing.TrackerStatus = target
			}
		}
	} else if req.TrackerStatus != nil {
		trimmedStatus := strings.TrimSpace(*req.TrackerStatus)
		existing.TrackerStatus = trimmedStatus
		if proj, _ := d.getProjectByIDUnsafe(existing.ProjectID); proj != nil && trimmedStatus != "" {
			if stage := StageForTrackerStatus(proj, trimmedStatus); stage != "" {
				existing.Labels = SetWorkflowLabel(existing.Labels, "#"+stage)
				if internal, ok := InternalStatusForStage(stage); ok && req.Status == nil {
					existing.Status = internal
				}
			}
		}
	} else if req.Labels != nil {
		if proj, _ := d.getProjectByIDUnsafe(existing.ProjectID); proj != nil {
			for _, l := range existing.Labels {
				stage := strings.ToLower(strings.TrimPrefix(l, "#"))
				if _, isStage := stageToInternalStatus[stage]; !isStage {
					continue
				}
				if target := TrackerStatusForStage(proj, stage); target != "" && !strings.EqualFold(target, existing.TrackerStatus) {
					existing.TrackerStatus = target
				}
				if req.Status == nil {
					if internal, ok := InternalStatusForStage(stage); ok {
						existing.Status = internal
					}
				}
				break
			}
		}
	}

	// Forçage de la cohérence de clôture / finition
	if explicitStage == "finished" || explicitStage == "closed" || strings.EqualFold(existing.TrackerStatus, "Done") || strings.EqualFold(existing.TrackerStatus, "Closed") || strings.EqualFold(existing.TrackerStatus, "Terminé") || existing.Status == models.StatusFinished || existing.Status == models.StatusDone || string(existing.Status) == "closed" {
		existing.Status = models.StatusFinished
		existing.Labels = SetWorkflowLabel(existing.Labels, "#finished")
		if proj, _ := d.getProjectByIDUnsafe(existing.ProjectID); proj != nil {
			if target := TrackerStatusForStage(proj, "finished"); target != "" {
				existing.TrackerStatus = target
			}
		}
	}

	if req.RepoPath != nil {
		trimmed := strings.TrimSpace(*req.RepoPath)
		if trimmed == "" {
			// An empty string is an explicit "inherit again", not a stored path.
			existing.RepoPath = nil
		} else {
			existing.RepoPath = &trimmed
			// Feed the project's list so the next ticket picks it from a menu
			// instead of retyping the path, once the task is committed.
			repoPathToRegister = trimmed
		}
	}
	oldSprint := existing.Sprint
	if req.Sprint != nil {
		existing.Sprint = strings.TrimSpace(*req.Sprint)
	}
	if req.Source != nil {
		existing.Source = *req.Source
	}
	if req.ExternalURL != nil {
		existing.ExternalURL = req.ExternalURL
	}
	if req.IssueType != nil {
		existing.IssueType = strings.TrimSpace(*req.IssueType)
	}
	existing.UpdatedAt = time.Now()

	isPinned := HasPinnedLabel(existing.Labels)
	pinnedVal := 0
	if isPinned {
		pinnedVal = 1
		_, _ = tx.Exec(`
			INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
			ON CONFLICT(task_id) DO NOTHING
		`, existing.ID, existing.UpdatedAt.Format(time.RFC3339))
	} else {
		_, _ = tx.Exec(`DELETE FROM pinned_tasks WHERE task_id = ? OR task_id = ?`, existing.ID, existing.Key)
	}
	existing.Pinned = isPinned

	labelsJSON, _ := json.Marshal(existing.Labels)

	_, err = tx.Exec(`
		UPDATE tasks
		SET project_id = ?, title = ?, description = ?, status = ?, priority = ?, labels = ?, pinned = ?, assignee = ?, assignee_avatar = ?, position = ?, due_date = ?, branch_name = ?, pr_url = ?, pr_links = ?, repo_path = ?, tracker_status = ?, source = CASE WHEN source = 'converting' THEN source ELSE ? END, external_url = ?, issue_type = ?, sprint = ?, updated_at = ?
		WHERE id = ?
	`, existing.ProjectID, existing.Title, existing.Description, string(existing.Status), string(existing.Priority), string(labelsJSON), pinnedVal, existing.Assignee, existing.AssigneeAvatar, existing.Position, existing.DueDate, existing.BranchName, existing.PrURL, encodePullRequestLinks(existing.PrLinks), repoPathValue(existing.RepoPath), existing.TrackerStatus, existing.Source, existing.ExternalURL, existing.IssueType, existing.Sprint, existing.UpdatedAt, existing.ID)

	if err != nil {
		return nil, err
	}

	// Detaching every link is an explicit gesture, and the synchronisation
	// remembers it: rediscovery would otherwise put back, a minute later, what
	// a person just removed. Attaching one again clears the flag.
	if req.PrLinks != nil || req.PrURL != nil {
		detached := 0
		if len(existing.PrLinks) == 0 {
			detached = 1
		}
		_, _ = tx.Exec("UPDATE tasks SET pr_links_detached = ? WHERE id = ?", detached, existing.ID)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true
	if repoPathToRegister != "" {
		d.registerProjectRepoPathUnsafe(existing.ProjectID, repoPathToRegister)
	}

	// Enqueue async CLI tracker sync in task activities queue whenever task is modified
	if req.Status != nil || req.Labels != nil || req.Title != nil || req.Description != nil || req.Priority != nil || req.TrackerStatus != nil {
		d.enqueueTrackerUpdateAsUnsafe(actor.ID, existing, req.Status, existing.Labels, removedLabels, TrackerFieldChanges{
			Title:       req.Title != nil,
			Description: req.Description != nil,
			Priority:    req.Priority != nil,
		})
	}

	// Sprint change tracker operation queue
	if req.Sprint != nil && strings.TrimSpace(*req.Sprint) != strings.TrimSpace(oldSprint) {
		sprintID := strings.TrimSpace(*req.Sprint)
		if proj, _ := d.getProjectByIDUnsafe(existing.ProjectID); proj != nil {
			for _, sp := range proj.Sprints {
				if strings.EqualFold(sp.Name, sprintID) || sp.ID == sprintID {
					sprintID = sp.ID
					break
				}
			}
		}
		if _, opErr := d.enqueueTrackerOpUnsafe(tracker.WithActingUser(context.Background(), actor.ID), TrackerOp{
			Kind:       TrackerOpSetSprint,
			ProjectID:  existing.ProjectID,
			TaskID:     existing.ID,
			TaskKey:    existing.Key,
			TaskIDs:    []string{existing.ID},
			SprintID:   sprintID,
			SprintName: strings.TrimSpace(*req.Sprint),
		}); opErr != nil {
			log.Printf("[UpdateTask] sprint de %s non mis en file: %v", existing.Key, opErr)
		}
	}

	// L'assignation ne voyage pas avec la synchro des champs : Jira n'assigne
	// que par identifiant de compte, jamais par nom affiché. Elle part donc
	// comme écriture dédiée, dans la même file d'activités.
	if newAssignee := strings.TrimSpace(existing.Assignee); newAssignee != oldAssignee && existing.Source == "jira" {
		accountID := ""
		if req.AssigneeAccountID != nil {
			accountID = strings.TrimSpace(*req.AssigneeAccountID)
		}
		if _, opErr := d.enqueueTrackerOpUnsafe(tracker.WithActingUser(context.Background(), actor.ID), TrackerOp{
			Kind:         TrackerOpAssign,
			ProjectID:    existing.ProjectID,
			TaskID:       existing.ID,
			TaskKey:      existing.Key,
			AccountID:    accountID,
			AssigneeName: newAssignee,
		}); opErr != nil {
			log.Printf("[UpdateTask] assignation de %s non mise en file: %v", existing.Key, opErr)
		}
	}

	acts, _ := d.getTaskActivitiesUnsafe(existing.ID)
	existing.Activities = acts

	return existing, nil
}

func (d *DB) getSettingsUnsafe() (*models.Settings, error) {
	var s models.Settings
	var aiModel, aiSkillModelsJSON, aiProviderModelsJSON sql.NullString
	var detMode, aiProv, aiCmd, aiCmdAuto, repoP, issTrk, ghRepo, jiraProj, jiraUrl, jiraMail, jiraTok, pClar, pSpec, pImpl, pAdj, pHandoff, pPR, pPick, editCmd, specFw sql.NullString
	var ghURL, ghTok, glURL, glProj, glTok sql.NullString
	var uiScale sql.NullInt64
	var autoSyncEnabled, autoSyncInterval sql.NullInt64

	err := d.conn.QueryRow(`
		SELECT id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar,
		       ai_provider, ai_command_template, ai_command_template_autonomous, ai_model, ai_skill_models, ai_provider_models, repo_path, issue_tracker, github_repo, jira_project, jira_url, jira_email, jira_api_token,
		       github_api_url, github_token, gitlab_url, gitlab_project, gitlab_token,
		       prompt_clarify, prompt_specify, prompt_implement, prompt_adjust, prompt_handoff, prompt_create_pr, prompt_pick, editor_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, updated_at
		FROM settings WHERE id = 1
	`).Scan(
		&s.ID,
		&s.Theme,
		&s.AccentColor,
		&s.Language,
		&s.Density,
		&s.DefaultView,
		&detMode,
		&s.UserName,
		&s.UserEmail,
		&s.UserAvatar,
		&aiProv,
		&aiCmd,
		&aiCmdAuto,
		&aiModel,
		&aiSkillModelsJSON,
		&aiProviderModelsJSON,
		&repoP,
		&issTrk,
		&ghRepo,
		&jiraProj,
		&jiraUrl,
		&jiraMail,
		&jiraTok,
		&ghURL,
		&ghTok,
		&glURL,
		&glProj,
		&glTok,
		&pClar,
		&pSpec,
		&pImpl,
		&pAdj,
		&pHandoff,
		&pPR,
		&pPick,
		&editCmd,
		&specFw,
		&uiScale,
		&autoSyncEnabled,
		&autoSyncInterval,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if detMode.Valid {
		s.DetailMode = detMode.String
	} else {
		s.DetailMode = "panel"
	}
	s.UIScale = NormalizeUIScale(int(uiScale.Int64))
	s.AutoSyncEnabled = autoSyncEnabled.Int64 == 1
	s.AutoSyncIntervalSec = NormalizeAutoSyncInterval(int(autoSyncInterval.Int64))
	if aiProv.Valid {
		s.AIProvider = aiProv.String
	}
	if aiCmd.Valid {
		s.AICommandTemplate = aiCmd.String
	}
	s.AICommandTemplateAutonomous = aiCmdAuto.String
	s.AIModel = aiModel.String
	s.AISkillModels = parseSkillModels(aiSkillModelsJSON.String)
	s.AIProviderModels = parseProviderModels(aiProviderModelsJSON.String)
	if repoP.Valid {
		s.RepoPath = repoP.String
	}
	if issTrk.Valid {
		s.IssueTracker = issTrk.String
	}
	if ghRepo.Valid {
		s.GithubRepo = ghRepo.String
	}
	if jiraProj.Valid {
		s.JiraProject = jiraProj.String
	}
	if jiraUrl.Valid {
		s.JiraUrl = jiraUrl.String
	}
	if jiraMail.Valid {
		s.JiraEmail = jiraMail.String
	}
	if jiraTok.Valid {
		s.JiraAPIToken = jiraTok.String
	}
	s.GithubApiUrl = ghURL.String
	s.GithubToken = ghTok.String
	s.GitlabUrl = glURL.String
	s.GitlabProject = glProj.String
	s.GitlabToken = glTok.String
	s.SpecFramework = models.NormalizeSpecFramework(specFw.String)
	if pClar.Valid {
		s.PromptClarify = pClar.String
	}
	if pSpec.Valid {
		s.PromptSpecify = pSpec.String
	}
	if pImpl.Valid {
		s.PromptImplement = pImpl.String
	}
	if pAdj.Valid {
		s.PromptAdjust = pAdj.String
	}
	if pHandoff.Valid {
		s.PromptHandoff = pHandoff.String
	}
	if pPR.Valid {
		s.PromptCreatePR = pPR.String
	}
	if pPick.Valid {
		s.PromptPick = pPick.String
	}
	if editCmd.Valid && editCmd.String != "" {
		s.EditorCommand = editCmd.String
	} else {
		s.EditorCommand = "code"
	}
	return &s, nil
}

func (d *DB) MoveTask(id string, newStatus models.Status, newPosition int) (*models.Task, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	existing, err := d.getTaskByIDUnsafe(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("task not found")
	}

	// The shift is one statement and stays outside the transaction below:
	// holding the moved task's lock while it locks its neighbours would let two
	// moves on two instances wait on each other.
	now := time.Now()
	_, _ = d.conn.Exec(`
		UPDATE tasks
		SET position = position + 1
		WHERE status = ? AND position >= ? AND id != ?
	`, string(newStatus), newPosition, id)

	var removedLabels []string
	// The labels are derived from the locked row, so a label another instance
	// added meanwhile is kept.
	err = d.conn.WithTx(func(tx *sqlTx) error {
		locked, err := d.lockTaskUnsafe(tx, existing.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return fmt.Errorf("task not found")
		}
		existing = locked
		oldStage := GetStageLabelForStatus(existing.Status)
		newStage := GetStageLabelForStatus(newStatus)
		if oldStage != newStage {
			removedLabels = append(removedLabels, oldStage, "#"+oldStage)
		}

		targetLabel := "#" + strings.TrimPrefix(newStage, "#")
		existing.Status = newStatus
		existing.Position = newPosition
		existing.Labels = SetWorkflowLabel(existing.Labels, targetLabel)
		existing.UpdatedAt = now

		labelsJSON, _ := json.Marshal(existing.Labels)
		_, err = tx.Exec(`
			UPDATE tasks
			SET status = ?, labels = ?, position = ?, updated_at = ?
			WHERE id = ?
		`, string(newStatus), string(labelsJSON), newPosition, now, existing.ID)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Enqueue async CLI tracker sync in task activities queue
	// Déplacement d'étape : seuls le statut et les labels bougent.
	d.enqueueTrackerUpdateUnsafe(existing, &newStatus, existing.Labels, removedLabels, TrackerFieldChanges{})

	acts, _ := d.getTaskActivitiesUnsafe(existing.ID)
	existing.Activities = acts

	return existing, nil
}

func (d *DB) DeleteTask(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	existing, _ := d.getTaskByIDUnsafe(id)
	if existing != nil {
		_, _ = d.conn.Exec("DELETE FROM pinned_tasks WHERE task_id = ?", existing.ID)
		_, _ = d.conn.Exec("DELETE FROM task_activities WHERE task_id = ?", existing.ID)
		_, err := d.conn.Exec("DELETE FROM tasks WHERE id = ?", existing.ID)
		return err
	}

	_, _ = d.conn.Exec("DELETE FROM pinned_tasks WHERE task_id = ?", id)
	_, _ = d.conn.Exec("DELETE FROM task_activities WHERE task_id = ?", id)
	_, err := d.conn.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

func (d *DB) getTaskByIDUnsafe(id string) (*models.Task, error) {
	return d.taskByIDOn(d.conn, id, "")
}

// lockTaskUnsafe reads a task inside a transaction and locks its row until the
// transaction ends, so another server process changing the same task waits for
// this one and then reads what it wrote. Every read-decide-write on a task goes
// through it: labels, pull-request links, stage and branch are then derived
// from the state the previous writer committed, never from a stale snapshot.
// See docs/db-concurrency-audit.md.
//
// Inside the transaction, write through tx only: under SQLite a write on the
// plain connection would wait for the transaction that holds the file.
func (d *DB) lockTaskUnsafe(tx *sqlTx, id string) (*models.Task, error) {
	return d.taskByIDOn(tx, id, d.forUpdate())
}

// rowQuerier is what reading one row needs, from the pool or a transaction.
type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

// taskByIDOn reads a task by id, then by key, through q; lock is appended to
// both SELECTs.
func (d *DB) taskByIDOn(q rowQuerier, id string, lock string) (*models.Task, error) {
	var t models.Task
	var labelsJSON string
	var dueDate, branchName, prURL, repoPath, sprint, team, teamID, trackerStatus, source, extURL, issueType, parentKey, parentTitle, parentType sql.NullString
	var prLinksJSON sql.NullString
	var trackerCreatedAt, trackerUpdatedAt, statusChangedAt sql.NullTime
	var statusStr, priorityStr string

	err := q.QueryRow(`
		SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, creator, creator_avatar, position, due_date, branch_name, pr_url, pr_links, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
		FROM tasks WHERE id = ?`+lock, id).Scan(
		&t.ID,
		&t.ProjectID,
		&t.Key,
		&t.Title,
		&t.Description,
		&statusStr,
		&priorityStr,
		&labelsJSON,
		&t.Assignee,
		&t.AssigneeAvatar,
		&t.Creator,
		&t.CreatorAvatar,
		&t.Position,
		&dueDate,
		&branchName,
		&prURL,
		&prLinksJSON,
		&repoPath,
		&sprint,
		&team,
		&teamID,
		&trackerStatus,
		&source,
		&extURL,
		&issueType,
		&parentKey,
		&parentTitle,
		&parentType,
		&trackerCreatedAt,
		&trackerUpdatedAt,
		&statusChangedAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		err = q.QueryRow(`
			SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, creator, creator_avatar, position, due_date, branch_name, pr_url, pr_links, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
			FROM tasks WHERE key = ? LIMIT 1`+lock, id).Scan(
			&t.ID,
			&t.ProjectID,
			&t.Key,
			&t.Title,
			&t.Description,
			&statusStr,
			&priorityStr,
			&labelsJSON,
			&t.Assignee,
			&t.AssigneeAvatar,
			&t.Creator,
			&t.CreatorAvatar,
			&t.Position,
			&dueDate,
			&branchName,
			&prURL,
			&prLinksJSON,
			&repoPath,
			&sprint,
			&team,
			&teamID,
			&trackerStatus,
			&source,
			&extURL,
			&issueType,
			&parentKey,
			&parentTitle,
			&parentType,
			&trackerCreatedAt,
			&trackerUpdatedAt,
			&statusChangedAt,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	t.Status = models.Status(statusStr)
	t.Priority = models.Priority(priorityStr)
	if dueDate.Valid {
		t.DueDate = &dueDate.String
	}
	if branchName.Valid {
		t.BranchName = &branchName.String
	}
	if prURL.Valid {
		t.PrURL = &prURL.String
	}
	t.PrLinks = decodePullRequestLinks(prLinksJSON.String)
	if repoPath.Valid && repoPath.String != "" {
		p := repoPath.String
		t.RepoPath = &p
	}
	if sprint.Valid {
		t.Sprint = sprint.String
	}
	if team.Valid {
		t.Team = team.String
		t.TeamID = teamID.String
		if trackerCreatedAt.Valid {
			created := trackerCreatedAt.Time
			t.TrackerCreatedAt = &created
		}
		if trackerUpdatedAt.Valid {
			updated := trackerUpdatedAt.Time
			t.TrackerUpdatedAt = &updated
		}
		if statusChangedAt.Valid {
			changed := statusChangedAt.Time
			t.StatusChangedAt = &changed
		}
	}
	if trackerStatus.Valid {
		t.TrackerStatus = trackerStatus.String
	}
	t.Source = taskSource(source, t.Key)

	if extURL.Valid && extURL.String != "" {
		t.ExternalURL = &extURL.String
	} else {
		t.ExternalURL = d.computeExternalURLUnsafe(&t)
	}

	t.IssueType = issueType.String
	t.ParentKey = parentKey.String
	t.ParentTitle = parentTitle.String
	t.ParentType = parentType.String
	_ = json.Unmarshal([]byte(labelsJSON), &t.Labels)
	if t.Labels == nil {
		t.Labels = []string{}
	}
	t.Pinned = HasPinnedLabel(t.Labels)

	return &t, nil
}

func (d *DB) addTaskActivityDirect(act models.TaskActivity) error {
	return insertTaskActivity(d.conn, d.instanceID, act)
}

type activityExecutor interface {
	Exec(string, ...any) (sql.Result, error)
}

// insertTaskActivity records an activity owned by the given server instance.
func insertTaskActivity(conn activityExecutor, instanceID string, act models.TaskActivity) error {
	stepsJSON, _ := json.Marshal(act.Steps)
	if act.Steps == nil {
		stepsJSON = []byte("[]")
	}
	_, err := conn.Exec(`
		INSERT INTO task_activities (id, task_id, project_id, skill_id, skill_name, action, status, summary, output, steps, prompt, started_at, completed_at, error, created_at, waiting_since, user_id, run_provider, run_model, instance_id, concurrent)
		VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, act.ID, act.TaskID, act.ProjectID, act.SkillID, act.SkillName, act.Action, act.Status, act.Summary, act.Output, string(stepsJSON), act.Prompt, act.StartedAt, act.CompletedAt, act.Error, act.CreatedAt, act.WaitingSince, act.UserID, act.Provider, act.Model, instanceID, boolToInt(act.Concurrent))
	return err
}

func (d *DB) AddTaskActivity(act models.TaskActivity) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.addTaskActivityDirect(act)
}

// ownerDisplayName is what an activity shows for its owner: the display name,
// then the e-mail, nothing for a row without owner or for the implicit user,
// who has no identity to show.
func ownerDisplayName(displayName, email string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	return email
}

func (d *DB) getTaskActivitiesUnsafe(taskID string) ([]models.TaskActivity, error) {
	return d.activitiesAttachedTo("a.task_id", taskID)
}

// getProjectActivitiesUnsafe is the project history: the activities attached to
// a project rather than to one of its tickets — its synchronisations, above all.
//
// It is a reader of its own rather than a project identifier smuggled into
// getTaskActivitiesUnsafe, which is exactly the overloading #310 removes from
// the column.
func (d *DB) getProjectActivitiesUnsafe(projectID string) ([]models.TaskActivity, error) {
	return d.activitiesAttachedTo("a.project_id", projectID)
}

// activitiesAttachedTo reads one attachment's history. The column is a literal
// chosen by its two callers, never a value coming from a request.
func (d *DB) activitiesAttachedTo(column, id string) ([]models.TaskActivity, error) {
	rows, err := d.conn.Query(`
		SELECT a.id, COALESCE(a.task_id, ''), COALESCE(a.project_id, ''), a.skill_id, a.skill_name, a.action, a.status, a.summary, a.output, a.steps, a.prompt, a.started_at, a.completed_at, a.error, a.created_at, a.waiting_since,
		       a.user_id, COALESCE(NULLIF(u.chosen_name, ''), u.display_name, ''), COALESCE(u.email, ''), a.run_provider, a.run_model, a.concurrent
		FROM task_activities a LEFT JOIN users u ON u.id = a.user_id WHERE `+column+` = ? ORDER BY a.created_at DESC
	`, id)
	if err != nil {
		return []models.TaskActivity{}, nil
	}
	defer rows.Close()

	var list []models.TaskActivity
	for rows.Next() {
		var a models.TaskActivity
		var stepsJSON string
		var prompt, errStr, runProvider, runModel sql.NullString
		var startedAt, completedAt, waitingSince sql.NullTime
		var ownerName, ownerEmail string

		err := rows.Scan(&a.ID, &a.TaskID, &a.ProjectID, &a.SkillID, &a.SkillName, &a.Action, &a.Status, &a.Summary, &a.Output, &stepsJSON, &prompt, &startedAt, &completedAt, &errStr, &a.CreatedAt, &waitingSince, &a.UserID, &ownerName, &ownerEmail, &runProvider, &runModel, &a.Concurrent)
		if err != nil {
			continue
		}
		a.UserName = ownerDisplayName(ownerName, ownerEmail)
		if waitingSince.Valid {
			a.WaitingSince = &waitingSince.Time
		}
		a.Provider, a.Model = runProvider.String, runModel.String
		_ = json.Unmarshal([]byte(stepsJSON), &a.Steps)
		if a.Steps == nil {
			a.Steps = []string{}
		}
		if prompt.Valid {
			a.Prompt = prompt.String
		}
		if errStr.Valid {
			a.Error = errStr.String
		}
		if startedAt.Valid {
			a.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			a.CompletedAt = &completedAt.Time
		}
		if a.StartedAt != nil && a.CompletedAt != nil {
			dur := a.CompletedAt.Sub(*a.StartedAt)
			if dur.Seconds() < 1 {
				a.Duration = fmt.Sprintf("%dms", dur.Milliseconds())
			} else if dur.Seconds() < 60 {
				a.Duration = fmt.Sprintf("%.1fs", dur.Seconds())
			} else {
				a.Duration = fmt.Sprintf("%dm%ds", int(dur.Minutes()), int(dur.Seconds())%60)
			}
		}
		list = append(list, a)
	}
	if list == nil {
		list = []models.TaskActivity{}
	}
	return list, nil
}

func (d *DB) GetTaskActivities(taskID string) ([]models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getTaskActivitiesUnsafe(taskID)
}

// GetProjectActivities is a project's own activity history.
func (d *DB) GetProjectActivities(projectID string) ([]models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getProjectActivitiesUnsafe(projectID)
}

// TrackerTokenClearSentinel is what the UI sends to delete a stored token, since
// an empty field means "leave it alone". It applies to every tracker credential,
// Jira included, which is why it no longer carries Jira's name.
const TrackerTokenClearSentinel = "__clear__"

func (d *DB) GetSettings() (*models.Settings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var s models.Settings
	var aiModel, aiSkillModelsJSON, aiProviderModelsJSON sql.NullString
	var detMode, aiProv, aiCmd, aiCmdAuto, repoP, issTrk, ghRepo, jiraProj, jiraUrl, jiraMail, jiraTok, pClar, pSpec, pImpl, pAdj, pHandoff, pPR, pPick, specFw, extTerm sql.NullString
	var ghURL, ghTok, glURL, glProj, glTok sql.NullString
	var uiScale sql.NullInt64
	var autoSyncEnabled, autoSyncInterval sql.NullInt64

	err := d.conn.QueryRow(`
		SELECT id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar,
		       ai_provider, ai_command_template, ai_command_template_autonomous, ai_model, ai_skill_models, ai_provider_models, repo_path, issue_tracker, github_repo, jira_project, jira_url, jira_email, jira_api_token,
		       github_api_url, github_token, gitlab_url, gitlab_project, gitlab_token,
		       prompt_clarify, prompt_specify, prompt_implement, prompt_adjust, prompt_handoff, prompt_create_pr, prompt_pick, editor_command, external_terminal_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, updated_at
		FROM settings WHERE id = 1
	`).Scan(
		&s.ID,
		&s.Theme,
		&s.AccentColor,
		&s.Language,
		&s.Density,
		&s.DefaultView,
		&detMode,
		&s.UserName,
		&s.UserEmail,
		&s.UserAvatar,
		&aiProv,
		&aiCmd,
		&aiCmdAuto,
		&aiModel,
		&aiSkillModelsJSON,
		&aiProviderModelsJSON,
		&repoP,
		&issTrk,
		&ghRepo,
		&jiraProj,
		&jiraUrl,
		&jiraMail,
		&jiraTok,
		&ghURL,
		&ghTok,
		&glURL,
		&glProj,
		&glTok,
		&pClar,
		&pSpec,
		&pImpl,
		&pAdj,
		&pHandoff,
		&pPR,
		&pPick,
		&s.EditorCommand,
		&extTerm,
		&specFw,
		&uiScale,
		&autoSyncEnabled,
		&autoSyncInterval,
		&s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return d.withoutTrackerTokens(&models.Settings{
				ID:          1,
				Theme:       "dark",
				AccentColor: "indigo",
				Language:    "fr",
				Density:     "standard",
				DefaultView: "board",
				DetailMode:  "panel",
				UserName:    "Developer",
				UserEmail:   "dev@example.com",
				UserAvatar:  "",
				AIProvider:  "agy",
				// No template: the provider's own command line, in both modes.
				AICommandTemplate: "",
				RepoPath:          ".",
				IssueTracker:      "local",
				GithubRepo:        "",
				PromptClarify:     "",
				PromptSpecify:     "",
				PromptImplement:   "",
				PromptAdjust:      "",
				PromptHandoff:     "",
				PromptCreatePR:    "",
				PromptPick:        "",
				EditorCommand:     "code",
				SpecFramework:     "speckit",
				UpdatedAt:         time.Now(),
			}), nil
		}
		return nil, err
	}

	if detMode.Valid {
		s.DetailMode = detMode.String
	} else {
		s.DetailMode = "panel"
	}
	s.UIScale = NormalizeUIScale(int(uiScale.Int64))
	s.AutoSyncEnabled = autoSyncEnabled.Int64 == 1
	s.AutoSyncIntervalSec = NormalizeAutoSyncInterval(int(autoSyncInterval.Int64))
	if aiProv.Valid {
		s.AIProvider = aiProv.String
	} else {
		s.AIProvider = "agy"
	}
	// An empty template is a value: the launch then runs the command line the
	// agent attests for the provider, in each execution mode. Substituting a
	// default here pinned every project to one mode and could not be undone.
	s.AICommandTemplate = aiCmd.String
	s.AICommandTemplateAutonomous = aiCmdAuto.String
	s.AIModel = aiModel.String
	s.AISkillModels = parseSkillModels(aiSkillModelsJSON.String)
	s.AIProviderModels = parseProviderModels(aiProviderModelsJSON.String)
	if repoP.Valid {
		s.RepoPath = repoP.String
	} else {
		s.RepoPath = "."
	}
	if issTrk.Valid {
		s.IssueTracker = issTrk.String
	} else {
		s.IssueTracker = "local"
	}
	if ghRepo.Valid {
		s.GithubRepo = ghRepo.String
	} else {
		s.GithubRepo = ""
	}
	if jiraProj.Valid {
		s.JiraProject = jiraProj.String
	}
	if jiraUrl.Valid {
		s.JiraUrl = jiraUrl.String
	}
	if jiraMail.Valid {
		s.JiraEmail = jiraMail.String
	}
	if jiraTok.Valid {
		s.JiraAPIToken = jiraTok.String
	}
	s.GithubApiUrl = ghURL.String
	s.GithubToken = ghTok.String
	s.GitlabUrl = glURL.String
	s.GitlabProject = glProj.String
	s.GitlabToken = glTok.String
	if pClar.Valid {
		s.PromptClarify = pClar.String
	}
	if pSpec.Valid {
		s.PromptSpecify = pSpec.String
	}
	if pImpl.Valid {
		s.PromptImplement = pImpl.String
	}
	if pAdj.Valid {
		s.PromptAdjust = pAdj.String
	}
	if pHandoff.Valid {
		s.PromptHandoff = pHandoff.String
	}
	if pPR.Valid {
		s.PromptCreatePR = pPR.String
	}
	if pPick.Valid {
		s.PromptPick = pPick.String
	}
	if extTerm.Valid {
		s.ExternalTerminalCommand = extTerm.String
	}
	s.SpecFramework = models.NormalizeSpecFramework(specFw.String)

	return d.withoutTrackerTokens(&s), nil
}

// UpdateSettings merges a payload over the stored settings: a caller sends only
// the fields it edits, so an empty one keeps its stored value. The names in
// clear are the exception, for fields where empty is a value rather than an
// omission: an empty AI command means "run the provider's own command", which
// is the only way back to a launch that serves both execution modes.
func (d *DB) UpdateSettings(s models.Settings, clear ...string) (*models.Settings, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// The settings row is locked before it is read, so a save racing on another
	// server instance waits, and this one merges into what it committed rather
	// than writing back every field from an older read.
	tx, err := d.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := d.lockSettingsUnsafe(tx); err != nil {
		return nil, err
	}

	cleared := make(map[string]bool, len(clear))
	for _, name := range clear {
		cleared[name] = true
	}

	current, _ := d.getSettingsUnsafe()
	if current != nil {
		if s.Theme == "" {
			s.Theme = current.Theme
		}
		if s.AccentColor == "" {
			s.AccentColor = current.AccentColor
		}
		if s.Language == "" {
			s.Language = current.Language
		}
		if s.Density == "" {
			s.Density = current.Density
		}
		if s.DefaultView == "" {
			s.DefaultView = current.DefaultView
		}
		if s.DetailMode == "" {
			s.DetailMode = current.DetailMode
		}
		if s.UserName == "" {
			s.UserName = current.UserName
		}
		if s.UserEmail == "" {
			s.UserEmail = current.UserEmail
		}
		if s.AIProvider == "" {
			s.AIProvider = current.AIProvider
		}
		if s.AICommandTemplate == "" && !cleared["aiCommandTemplate"] {
			s.AICommandTemplate = current.AICommandTemplate
		}
		if s.AICommandTemplateAutonomous == "" && !cleared["aiCommandTemplateAutonomous"] {
			s.AICommandTemplateAutonomous = current.AICommandTemplateAutonomous
		}
		if s.RepoPath == "" {
			s.RepoPath = current.RepoPath
		}
		if s.IssueTracker == "" {
			s.IssueTracker = current.IssueTracker
		}
		if s.GithubRepo == "" {
			s.GithubRepo = current.GithubRepo
		}
		if s.JiraProject == "" {
			s.JiraProject = current.JiraProject
		}
		if s.JiraUrl == "" {
			s.JiraUrl = current.JiraUrl
		}
		if s.JiraEmail == "" {
			s.JiraEmail = current.JiraEmail
		}
		if s.JiraAPIToken == "" {
			// The UI never receives the token back, so it sends an empty field
			// unless the user typed a new one.
			s.JiraAPIToken = current.JiraAPIToken
		} else if s.JiraAPIToken == TrackerTokenClearSentinel {
			s.JiraAPIToken = ""
		}
		if s.GithubApiUrl == "" {
			s.GithubApiUrl = current.GithubApiUrl
		}
		if s.GitlabUrl == "" {
			s.GitlabUrl = current.GitlabUrl
		}
		if s.GitlabProject == "" {
			s.GitlabProject = current.GitlabProject
		}
		s.GithubToken = keptToken(s.GithubToken, current.GithubToken)
		s.GitlabToken = keptToken(s.GitlabToken, current.GitlabToken)
		if s.PromptClarify == "" {
			s.PromptClarify = current.PromptClarify
		}
		if s.PromptSpecify == "" {
			s.PromptSpecify = current.PromptSpecify
		}
		if s.PromptImplement == "" {
			s.PromptImplement = current.PromptImplement
		}
		if s.PromptAdjust == "" {
			s.PromptAdjust = current.PromptAdjust
		}
		if s.PromptHandoff == "" {
			s.PromptHandoff = current.PromptHandoff
		}
		if s.PromptAdjust != "" {
			s.PromptCreatePR = ""
		} else if s.PromptCreatePR == "" {
			s.PromptCreatePR = current.PromptCreatePR
		}
		if s.PromptPick == "" {
			s.PromptPick = current.PromptPick
		}
		if s.EditorCommand == "" {
			s.EditorCommand = current.EditorCommand
		}
		if s.ExternalTerminalCommand == "" {
			s.ExternalTerminalCommand = current.ExternalTerminalCommand
		}
		if s.SpecFramework == "" {
			s.SpecFramework = current.SpecFramework
		}
	}

	if s.Theme == "" {
		s.Theme = "dark"
	}
	if s.AccentColor == "" {
		s.AccentColor = "indigo"
	}
	if s.Language == "" {
		s.Language = "fr"
	}
	if s.Density == "" {
		s.Density = "standard"
	}
	if s.DefaultView == "" {
		s.DefaultView = "board"
	}
	if s.DetailMode == "" {
		s.DetailMode = "panel"
	}
	if s.UserName == "" {
		s.UserName = "Developer"
	}
	if s.UserEmail == "" {
		s.UserEmail = "dev@example.com"
	}
	if s.AIProvider == "" {
		s.AIProvider = "agy"
	}
	if s.RepoPath == "" {
		s.RepoPath = "."
	}
	if s.IssueTracker == "" {
		s.IssueTracker = "local"
	}
	if s.GithubRepo == "" {
		s.GithubRepo = ""
	}
	if s.EditorCommand == "" {
		s.EditorCommand = "code"
	}
	s.SpecFramework = models.NormalizeSpecFramework(s.SpecFramework)
	s.JiraProject = strings.ToUpper(strings.TrimSpace(s.JiraProject))
	s.UIScale = NormalizeUIScale(s.UIScale)
	s.AutoSyncIntervalSec = NormalizeAutoSyncInterval(s.AutoSyncIntervalSec)
	autoSyncEnabledInt := 0
	if s.AutoSyncEnabled {
		autoSyncEnabledInt = 1
	}

	s.AIModel = strings.TrimSpace(s.AIModel)
	s.AISkillModels = normalizeSkillModels(s.AISkillModels)
	settingsSkillModelsBytes, _ := json.Marshal(s.AISkillModels)
	s.AIProviderModels = agentconfig.NormalizeProviderModels(s.AIProviderModels)
	settingsProviderModelsBytes, _ := json.Marshal(s.AIProviderModels)

	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO settings (id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar, ai_provider, ai_command_template, ai_command_template_autonomous, ai_model, ai_skill_models, ai_provider_models, repo_path, issue_tracker, github_repo, jira_project, jira_url, jira_email, jira_api_token, github_api_url, github_token, gitlab_url, gitlab_project, gitlab_token, prompt_clarify, prompt_specify, prompt_implement, prompt_adjust, prompt_handoff, prompt_create_pr, prompt_pick, editor_command, external_terminal_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			theme = excluded.theme,
			accent_color = excluded.accent_color,
			language = excluded.language,
			density = excluded.density,
			default_view = excluded.default_view,
			detail_mode = excluded.detail_mode,
			user_name = excluded.user_name,
			user_email = excluded.user_email,
			user_avatar = excluded.user_avatar,
			ai_provider = excluded.ai_provider,
			ai_command_template = excluded.ai_command_template,
			ai_command_template_autonomous = excluded.ai_command_template_autonomous,
			ai_model = excluded.ai_model,
			ai_skill_models = excluded.ai_skill_models,
			ai_provider_models = excluded.ai_provider_models,
			repo_path = excluded.repo_path,
			issue_tracker = excluded.issue_tracker,
			github_repo = excluded.github_repo,
			jira_project = excluded.jira_project,
			jira_url = excluded.jira_url,
			jira_email = excluded.jira_email,
			jira_api_token = excluded.jira_api_token,
			github_api_url = excluded.github_api_url,
			github_token = excluded.github_token,
			gitlab_url = excluded.gitlab_url,
			gitlab_project = excluded.gitlab_project,
			gitlab_token = excluded.gitlab_token,
			prompt_clarify = excluded.prompt_clarify,
			prompt_specify = excluded.prompt_specify,
			prompt_implement = excluded.prompt_implement,
			prompt_adjust = excluded.prompt_adjust,
			prompt_handoff = excluded.prompt_handoff,
			prompt_create_pr = excluded.prompt_create_pr,
			prompt_pick = excluded.prompt_pick,
			editor_command = excluded.editor_command,
			external_terminal_command = excluded.external_terminal_command,
			spec_framework = excluded.spec_framework,
			ui_scale = excluded.ui_scale,
			auto_sync_enabled = excluded.auto_sync_enabled,
			auto_sync_interval_sec = excluded.auto_sync_interval_sec,
			updated_at = excluded.updated_at
	`, s.Theme, s.AccentColor, s.Language, s.Density, s.DefaultView, s.DetailMode, s.UserName, s.UserEmail, s.UserAvatar, s.AIProvider, s.AICommandTemplate, s.AICommandTemplateAutonomous, s.AIModel, string(settingsSkillModelsBytes), string(settingsProviderModelsBytes), s.RepoPath, s.IssueTracker, s.GithubRepo, s.JiraProject, s.JiraUrl, s.JiraEmail, s.JiraAPIToken, s.GithubApiUrl, s.GithubToken, s.GitlabUrl, s.GitlabProject, s.GitlabToken, s.PromptClarify, s.PromptSpecify, s.PromptImplement, s.PromptAdjust, s.PromptHandoff, s.PromptCreatePR, s.PromptPick, s.EditorCommand, s.ExternalTerminalCommand, s.SpecFramework, s.UIScale, autoSyncEnabledInt, s.AutoSyncIntervalSec, now)

	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return nil, err
	}

	s.ID = 1
	s.UpdatedAt = now
	return d.withoutTrackerTokens(&s), nil
}

// GetAvailableSkills exposes the workflow catalogue. It derives from the single
// skills.StageSkills table: the skill the UI offers, the file installed in the
// repository and the step the worker runs are by construction the same thing.
// The old pick-issue auto-pilot is gone, the autonomous run button replaced it.
// UIScaleOptions are the interface zoom levels the status bar switches between.
// A ladder rather than a free number: typing a percentage needs a settings
// screen and a keyboard, which is not what "make it bigger, now" asks for.
//
// The four historical levels are kept as they were, 112 included rather than
// rounded to 110: a setting somebody already chose does not move to make a
// prettier sequence. The added steps go down, for whoever wants more tickets on
// screen, and above all up, where stopping at 125 left "it is too small" without
// an answer.
//
// The list must stay identical to UI_SCALE_OPTIONS in web/src/lib/uiScale.ts:
// this one bounds what is stored, that one what is offered.
var UIScaleOptions = []int{80, 90, 100, 112, 125, 150, 175}

// NormalizeAutoSyncInterval floors the background loop's period. Below thirty
// seconds, the tracker is polled faster than it changes, for nothing.
func NormalizeAutoSyncInterval(seconds int) int {
	if seconds <= 0 {
		return 60
	}
	if seconds < 30 {
		return 30
	}
	return seconds
}

// NormalizeUIScale keeps the stored scale on one of the offered steps. A value
// from an older database (zero) becomes 100, and anything off the list snaps to
// the nearest step rather than being refused.
func NormalizeUIScale(scale int) int {
	if scale <= 0 {
		return 100
	}
	best := UIScaleOptions[0]
	bestGap := -1
	for _, option := range UIScaleOptions {
		gap := option - scale
		if gap < 0 {
			gap = -gap
		}
		if bestGap < 0 || gap < bestGap {
			best = option
			bestGap = gap
		}
	}
	return best
}

func (d *DB) GetAvailableSkills() []models.Skill {
	out := make([]models.Skill, 0, len(skills.StageSkills))
	for _, s := range skills.StageSkills {
		name := s.Name
		in, _ := InternalStatusForStage(s.FromStage)
		outStatus, _ := InternalStatusForStage(s.ToStage)
		out = append(out, models.Skill{
			ID:           s.ID,
			Name:         name,
			Command:      s.Command,
			Description:  s.Description,
			InputStatus:  in,
			OutputStatus: outStatus,
			Icon:         s.Icon,
			Color:        s.Color,
			Steps:        s.Steps,
		})
	}
	return out
}

func (d *DB) startQueueWorker() {
	for job := range d.jobQueue {
		go func(j SkillJob) {
			// Counted in from enqueueJob, counted out here whatever happens.
			defer d.jobs.end()
			projID := j.ProjectID
			if projID == "" && j.TaskID != "" {
				d.mu.RLock()
				if t, _ := d.getTaskByIDUnsafe(j.TaskID); t != nil {
					projID = t.ProjectID
				}
				d.mu.RUnlock()
			}
			if projID == "" {
				projID = "default"
			}

			// Tracker operations are quick synchronizations, execute them directly
			if j.SkillID == "tracker_op" {
				d.runJobGuarded(j)
				return
			}

			// One server-side skill worker per project. Execution parallelism is a
			// workstation setting owned by the local agent, not a server-side one.
			// The limiter decides inside this process; the project lock extends
			// the rule to every server instance sharing the database. The job
			// stays queued while it waits for either.
			d.limiter.Acquire(projID, 1)
			defer d.limiter.Release(projID)
			release, err := d.dialect.AcquireProjectWorker(d.conn, projID)
			if err != nil {
				// Degraded to one worker per instance rather than a stuck queue.
				log.Printf("[skill] worker lock of project %s unavailable, running without it: %v", projID, err)
			} else {
				defer release()
			}

			d.runJobGuarded(j)
		}(job)
	}
}

// runJobGuarded isolates one job from the worker goroutine. A panic used to take
// the whole server down with it: the queue runs in its own goroutine, so nothing
// above could recover it.
func (d *DB) runJobGuarded(job SkillJob) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[skill] panique sur l'activité %s (%s): %v\n%s", job.ActivityID, job.SkillID, rec, debug.Stack())
			d.mu.Lock()
			_, _ = d.conn.Exec(`
				UPDATE task_activities
				SET status = 'failed', summary = ?, error = ?, completed_at = ?
				WHERE id = ? AND status != 'canceled'
			`, "Échec interne pendant l'exécution de la skill", fmt.Sprintf("panique: %v", rec), time.Now(), job.ActivityID)
			d.mu.Unlock()
		}
	}()
	d.processSkillJob(job)
}

// branchLabel names a task's branch for a report, and says so when the project
// works without a dedicated branch. Dereferencing it blindly panicked on every
// project with worktrees turned off.
func branchLabel(task *models.Task) string {
	if task != nil && task.BranchName != nil && strings.TrimSpace(*task.BranchName) != "" {
		return *task.BranchName
	}
	return "(sans branche dédiée)"
}

func (d *DB) processSkillJob(job SkillJob) {
	if stage, ok := skills.StageSkillByID(job.SkillID); ok {
		job.SkillID = stage.ID
	}
	// 1. Check if activity was canceled before starting
	d.mu.RLock()
	var currentStatus string
	_ = d.conn.QueryRow("SELECT status FROM task_activities WHERE id = ?", job.ActivityID).Scan(&currentStatus)
	d.mu.RUnlock()

	if currentStatus == string(models.ActivityStatusCanceled) {
		return
	}

	// 2. Mark activity as running
	now := time.Now()
	// Pas de plafond de durée ici : il coupait un run long à cinq minutes et
	// l'enregistrait comme une annulation humaine. Le plafond est désormais un
	// plafond de silence, tenu par la session de terminal.
	ctx, cancel := context.WithCancel(context.Background())
	d.cancelMu.Lock()
	d.cancelMap[job.ActivityID] = cancel
	d.cancelMu.Unlock()

	defer func() {
		cancel()
		d.cancelMu.Lock()
		delete(d.cancelMap, job.ActivityID)
		d.cancelMu.Unlock()
	}()

	// The instance executing the job owns it from here, whoever created it. A
	// cancellation that landed since the check above, possibly through another
	// instance, wins: the job does not start.
	d.mu.Lock()
	_, _ = d.conn.Exec(`
		UPDATE task_activities
		SET status = 'running', started_at = ?, instance_id = ?
		WHERE id = ? AND status != 'canceled'
	`, now, d.instanceID, job.ActivityID)
	_ = d.conn.QueryRow("SELECT status FROM task_activities WHERE id = ?", job.ActivityID).Scan(&currentStatus)
	d.mu.Unlock()
	if currentStatus == string(models.ActivityStatusCanceled) {
		return
	}

	// 3. Special handling for background Sync jobs
	if strings.HasPrefix(job.SkillID, "sync_") {
		d.mu.RLock()
		settings, _ := d.getSettingsUnsafe()
		d.mu.RUnlock()
		if settings == nil {
			settings = &models.Settings{
				AIProvider: "agy",
				RepoPath:   ".",
			}
		}
		d.processSyncJob(ctx, job, settings)
		return
	}

	// 3b. Special handling for background Tracker update jobs
	if job.SkillID == "tracker_update" {
		d.processTrackerUpdateJob(ctx, job)
		return
	}

	// 3c. Single tracker writes: assignment, epic, roadmap horizon labels.
	if job.SkillID == "tracker_op" {
		d.processTrackerOpJob(ctx, job)
		return
	}

	// This activity tracks dispatch, not skill completion. The remote run owns
	// results. The rename also takes the queued row out of the one-run index,
	// which is what lets the remote run below take its place: a rename that
	// failed would make that run collide with its own launch.
	d.mu.Lock()
	_, err := d.conn.Exec("UPDATE task_activities SET skill_id='agent_launch' WHERE id=?", job.ActivityID)
	d.mu.Unlock()

	var task *models.Task
	if err == nil {
		task, err = d.GetTaskByID(job.TaskID)
	}
	if err == nil && task != nil {
		var run *models.TaskActivity
		provider, model := d.ResolveTaskEngine(task.ProjectID, job.SkillID, job.Model)
		run, err = d.StartAgentRun(task.ID, job.SkillID, RunLaunch{Mode: job.Mode, Model: model, Provider: provider, ChainStop: job.ChainStopStage})
		if err == nil {
			err = d.callAgentContext(ctx, agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "execute_skill", SkillID: job.SkillID, Prompt: job.Prompt, RunID: run.ID, Mode: job.Mode, Model: job.Model}, nil)
			if err != nil {
				_, _ = d.FinishRemoteRun(task.ID, run.ID, "failed", err.Error())
			}
		}
	} else if err == nil {
		err = fmt.Errorf("task not found")
	}
	status, summary, errorText := "completed", "Execution launched on the local agent; workflow results are reported through MCP.", ""
	if err != nil {
		status = "failed"
		summary = "Agent launch failed"
		errorText = err.Error()
		// Another run became active on the task while this one waited in the
		// queue, on this server or another.
		if errors.Is(err, ErrTaskBusy) {
			summary = "Agent launch refused: another run is active on this task"
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = d.conn.Exec("UPDATE task_activities SET skill_id='agent_launch',status=?,summary=?,error=?,completed_at=? WHERE id=? AND status != 'canceled'", status, summary, errorText, time.Now(), job.ActivityID)
}

// trackerDisplayName spells a tracker for the activity log.
func trackerDisplayName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "github":
		return "GitHub"
	case "gitlab":
		return "GitLab"
	case "jira":
		return "Jira"
	case "":
		return "tracker"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// syncWindow is how far back one job reads. A tracker that cannot narrow a
// search is asked for the whole project whatever the loop wanted: a full
// paginated read is still one request per hundred work items, where the unit
// re-read it replaces was one per work item.
func syncWindow(ts tracker.TicketingSystem, opts SyncOptions) int {
	if ts == nil || !ts.Supports(tracker.CapIncrementalSync) {
		return 0
	}
	return opts.WindowMin
}

// afterTrackerSync follows an import with what the tracker can tell about the
// project's structure: the teams met on the work items, the board columns and
// sprints when the tracker has them, and the pull requests of what was just
// imported. Neither failure undoes the import.
//
// The first two describe the project rather than its work items, and cost the
// same whether one ticket moved or none. A background pass therefore only asks
// for them on a full read — every half hour — while a synchronisation somebody
// asked for always does. Pull request rediscovery follows the imported work
// items, so an incremental pass pays for exactly what changed.
func (d *DB) afterTrackerSync(ctx context.Context, proj *models.Project, ts tracker.TicketingSystem, tasks []models.Task, opts SyncOptions) []string {
	if proj == nil || ts == nil {
		return nil
	}
	describesProject := !opts.Background || opts.WindowMin == 0
	var steps []string
	if describesProject && ts.Supports(tracker.CapTeam) {
		if note, err := d.RefreshProjectTeamMembers(ctx, proj.ID, tasks); err != nil {
			steps = append(steps, fmt.Sprintf("⚠️ Équipes : %v", err))
		} else {
			steps = append(steps, "4. Équipes : "+note)
		}
	}
	// No board recorded yet is not a reason to skip: SyncProjectBoardColumns
	// resolves the project's board itself and persists the one it retained, so
	// the columns follow the tracker from the very first sync.
	if describesProject && ts.Supports(tracker.CapBoard) {
		if note, err := d.SyncProjectBoardColumns(ctx, proj.ID); err != nil {
			steps = append(steps, fmt.Sprintf("⚠️ Board : %v", err))
		} else {
			steps = append(steps, "5. Board : "+note)
		}
	}
	// The roadmap horizons are carried by the labels of the tracker's epics,
	// and the sync has no other way of seeing them: it imports the work items,
	// never the container that holds them. Without this read, a project whose
	// epics are all classified on the tracker opens with its roadmap entirely
	// unclassified, and nothing on screen says why.
	//
	// The capability is CapEpic and not a macro one because it is a question
	// put to the tracker, in the tracker's own words: Jira has epics, GitLab
	// has epics, GitHub has milestones, and CapEpic is how a tracker says it
	// exposes such a container at all. "Macro" is this product's word for the
	// same thing on its own side of the seam, which is why the answer is
	// written by ImportMacroHorizons into the macros table.
	//
	// The failure is not fatal, as for the teams and the board: a tracker that
	// cannot be reached must not undo an import that succeeded.
	if ts.Supports(tracker.CapEpic) {
		if note, err := d.ImportMacroHorizons(ctx, proj.ID); err != nil {
			steps = append(steps, fmt.Sprintf("⚠️ Roadmap : horizons non importés : %v", err))
		} else {
			steps = append(steps, "6. Roadmap : "+note)
		}
	}
	// Pull-request rediscovery runs here rather than in the import: the import
	// must keep its property of never writing pr_url / pr_links, which is what
	// protects the links a workflow produced.
	steps = append(steps, d.rediscoverProjectPullRequests(ctx, proj, ts, tasks)...)
	steps = append(steps, d.refreshProjectPullRequestStates(ctx, proj.ID)...)
	return steps
}

func (d *DB) processSyncJob(ctx context.Context, job SkillJob, settings *models.Settings) {
	// Whoever asked travels with the job: a personal tracker credential cannot
	// be resolved from a request that ended long before the worker picked it up.
	ctx = tracker.WithActingUser(ctx, job.ActingUser)
	var steps []string
	var summary string
	var outputLines []string
	var hasError bool
	var totalImported int

	switch {
	case job.SkillID == "sync_all":
		steps = append(steps, "1. Starting global multi-tracker synchronization...")
		outputLines = append(outputLines, "### 🌐 Global Synchronization\n")

		projects, _ := d.getProjectsUnsafe()
		if len(projects) == 0 {
			projects = []models.Project{
				{
					ID:           "default",
					GithubRepo:   settings.GithubRepo,
					RepoPath:     settings.RepoPath,
					IssueTracker: settings.IssueTracker,
				},
			}
		}

		for _, p := range projects {
			ts, err := d.TrackerForProject(&p)
			if err != nil || ts == nil || ts.Name() == "local" || !ts.Supports(tracker.CapSync) {
				continue
			}
			tName := ts.Name()
			tRepo := p.GithubRepo
			if tRepo == "" && settings != nil {
				tRepo = settings.GithubRepo
			}
			tPath := p.RepoPath
			if tPath == "" && settings != nil {
				tPath = settings.RepoPath
			}

			syncTasks, syncErr := ts.SyncIssues(ctx, tracker.SyncRequest{
				Project:          &p,
				Repo:             tRepo,
				RepoPath:         tPath,
				UpdatedWithinMin: syncWindow(ts, job.Sync),
			})
			if syncErr != nil {
				hasError = true
				steps = append(steps, fmt.Sprintf("⚠️ %s (%s): %v", tName, p.Name, syncErr))
				outputLines = append(outputLines, fmt.Sprintf("❌ %s (%s): %v", tName, p.Name, syncErr))
			} else {
				for i := range syncTasks {
					syncTasks[i].ProjectID = p.ID
					syncTasks[i].ID = ts.FormatTaskID(p.ID, syncTasks[i].Key, syncTasks[i].ID)
				}
				if impErr := d.ImportOrUpdateTasks(syncTasks); impErr != nil {
					hasError = true
					steps = append(steps, fmt.Sprintf("⚠️ %s: écriture locale échouée: %v", tName, impErr))
				} else {
					steps = append(steps, d.afterTrackerSync(ctx, &p, ts, syncTasks, job.Sync)...)
				}
				steps = append(steps, fmt.Sprintf("✅ %s (%s): %d issues imported", tName, p.Name, len(syncTasks)))
				outputLines = append(outputLines, fmt.Sprintf("✅ %s (%s): %d issues synced", tName, p.Name, len(syncTasks)))
				totalImported += len(syncTasks)
			}
		}

		steps = append(steps, "Global synchronization completed")
		summary = fmt.Sprintf("Global synchronization finished (%d tickets updated)", totalImported)

	case strings.HasPrefix(job.SkillID, "sync_"):
		trackerName := strings.TrimPrefix(job.SkillID, "sync_")
		var proj *models.Project
		if job.ProjectID != "" {
			if p, _ := d.getProjectByIDUnsafe(job.ProjectID); p != nil {
				proj = p
			}
		}

		ts, ok := d.TrackerRegistry().Get(trackerName)
		if (!ok || ts == nil) && proj != nil {
			ts, _ = d.TrackerForProject(proj)
		}
		if ts == nil || !ts.Supports(tracker.CapSync) {
			hasError = true
			summary = fmt.Sprintf("Unsupported tracker for sync: %s", trackerName)
			outputLines = append(outputLines, summary)
			break
		}

		repo := ""
		repoPath := ""
		if proj != nil {
			repo = proj.GithubRepo
			repoPath = proj.RepoPath
		}
		if repo == "" && trackerName == "github" && job.Prompt != "" {
			repo = job.Prompt
		}
		if repo == "" && settings != nil {
			repo = settings.GithubRepo
		}
		if repoPath == "" && settings != nil {
			repoPath = settings.RepoPath
		}

		trackerTitle := ts.Name()
		if strings.EqualFold(trackerTitle, "github") {
			trackerTitle = "GitHub"
		} else if len(trackerTitle) > 0 {
			trackerTitle = strings.ToUpper(trackerTitle[:1]) + trackerTitle[1:]
		}

		targetDesc := repo
		if targetDesc != "" {
			steps = append(steps, fmt.Sprintf("1. Connecting to %s API (%s)...", trackerTitle, targetDesc))
			outputLines = append(outputLines, fmt.Sprintf("### %s Synchronization (%s)\n", trackerTitle, targetDesc))
		} else {
			steps = append(steps, fmt.Sprintf("1. Connecting to %s API...", trackerTitle))
			outputLines = append(outputLines, fmt.Sprintf("### %s Synchronization\n", trackerTitle))
		}

		tasks, err := ts.SyncIssues(ctx, tracker.SyncRequest{
			Project:          proj,
			Repo:             repo,
			RepoPath:         repoPath,
			UpdatedWithinMin: syncWindow(ts, job.Sync),
		})
		if err != nil {
			hasError = true
			errMsg := fmt.Sprintf("%s synchronization failed: %v", trackerTitle, err)
			steps = append(steps, "⚠️ "+errMsg)
			outputLines = append(outputLines, "**Error:** "+errMsg)
			summary = fmt.Sprintf("Error during %s sync", trackerTitle)
		} else {
			steps = append(steps, fmt.Sprintf("2. %d tickets fetched from %s", len(tasks), trackerTitle))
			for i := range tasks {
				if job.ProjectID != "" {
					tasks[i].ProjectID = job.ProjectID
				}
				tasks[i].ID = ts.FormatTaskID(tasks[i].ProjectID, tasks[i].Key, tasks[i].ID)
			}
			if impErr := d.ImportOrUpdateTasks(tasks); impErr != nil {
				hasError = true
				steps = append(steps, "⚠️ 3. Local database write failed: "+impErr.Error())
				outputLines = append(outputLines, "**Error:** "+impErr.Error())
			} else {
				steps = append(steps, "3. Local database updated successfully")
				steps = append(steps, d.afterTrackerSync(ctx, proj, ts, tasks, job.Sync)...)
			}
			totalImported = len(tasks)
			summary = fmt.Sprintf("%d %s issues synchronized successfully", len(tasks), trackerTitle)

			outputLines = append(outputLines, fmt.Sprintf("✅ **%d tickets imported / updated from %s:**\n", len(tasks), trackerTitle))
			for _, t := range tasks {
				outputLines = append(outputLines, fmt.Sprintf("- **[%s]** %s *(Status: %s, Priority: %s)*", t.Key, t.Title, t.Status, t.Priority))
			}
		}
	}

	if job.Sync.Background {
		// The loop counts what its passes actually wrote, rather than what they
		// queued: the job outlives the pass, so this is the only place that
		// knows.
		d.recordAutoSyncPass(job.ProjectID, job.Sync.WindowMin, totalImported, hasError, summary)

		// A pass nobody asked for that brought nothing back has nothing to
		// say, and it runs every few minutes on every project that opted in.
		// A failure is never dropped: a credential the tracker refuses is
		// exactly what these rows are read for.
		if !hasError && totalImported == 0 {
			if delErr := d.DeleteActivity(job.ActivityID); delErr != nil {
				log.Printf("[autosync] activité de synchronisation non supprimée (%s) : %v", job.ActivityID, delErr)
			}
			return
		}
	}

	completedTime := time.Now()
	status := string(models.ActivityStatusCompleted)
	errText := ""
	if hasError {
		status = string(models.ActivityStatusFailed)
		errText = summary
	}

	stepsJSON, _ := json.Marshal(steps)
	d.mu.Lock()
	_, _ = d.conn.Exec(`
		UPDATE task_activities
		SET status = ?, summary = ?, output = ?, steps = ?, error = ?, completed_at = ?
		WHERE id = ? AND status != 'canceled'
	`, status, summary, strings.Join(outputLines, "\n"), string(stepsJSON), errText, completedTime, job.ActivityID)
	d.mu.Unlock()
}

// skillStageLabel donne l'étape atteinte quand une skill se termine, pour savoir
// quel label poser et vers quel statut transitionner.
var skillStageLabel = map[string]string{
	"clarify":   "clarified",
	"specify":   "specified",
	"implement": "implemented",
	"adjust":    "reviewed",
	"review":    "reviewed",
	"pickup":    "reviewed",
	"handoff":   "finished",
}

// enqueueTrackerUpdateUnsafe schedules the tracker sync. changed says which
// editable fields the caller actually touched; anything else is left alone on
// the tracker side.
type TrackerFieldChanges struct {
	Title       bool
	Description bool
	Priority    bool
}

func (d *DB) enqueueTrackerUpdateUnsafe(task *models.Task, status *models.Status, labels []string, removedLabels []string, changed TrackerFieldChanges) {
	d.enqueueTrackerUpdateAsUnsafe("", task, status, labels, removedLabels, changed)
}

// enqueueTrackerUpdateAsUnsafe queues the same write on behalf of whoever asked
// for it. A tracker whose credential belongs to a person can then resolve
// theirs when the worker picks the job up, long after the request ended.
func (d *DB) enqueueTrackerUpdateAsUnsafe(actorID string, task *models.Task, status *models.Status, labels []string, removedLabels []string, changed TrackerFieldChanges) {
	if task == nil {
		return
	}

	proj, _ := d.getProjectByIDUnsafe(task.ProjectID)
	ts, err := d.TrackerRegistry().ForTask(task, proj)
	if err != nil || ts == nil || ts.Name() == "local" || !ts.Supports(tracker.CapUpdate) {
		return
	}

	activityID := uuid.New().String()
	now := time.Now()

	var trackerName string
	var initialSteps []string
	stStr := string(task.Status)
	if status != nil {
		stStr = string(*status)
	}

	var activeStage string
	for _, l := range labels {
		cleanL := strings.TrimPrefix(strings.ToLower(l), "#")
		if cleanL == "new" || cleanL == "clarified" || cleanL == "specified" || cleanL == "implemented" || cleanL == "reviewed" || cleanL == "finished" {
			activeStage = "#" + cleanL
			break
		}
	}

	var changesSummary []string
	if status != nil {
		changesSummary = append(changesSummary, fmt.Sprintf("Statut ➔ %s", *status))
	}
	if len(labels) > 0 {
		changesSummary = append(changesSummary, fmt.Sprintf("Labels : %s", strings.Join(labels, ", ")))
	}
	if len(removedLabels) > 0 {
		changesSummary = append(changesSummary, fmt.Sprintf("Labels retirés : -%s", strings.Join(removedLabels, ", -")))
	}

	trackerName = trackerDisplayName(ts.Name())
	target := ""
	if proj != nil {
		target = firstNonEmpty(proj.GithubRepo, proj.JiraProject, proj.GitlabProject)
	}
	initialSteps = []string{
		fmt.Sprintf("Mise à jour du ticket %s [%s] (%s)", trackerName, task.Key, target),
		fmt.Sprintf("Statut cible : %s | Étape IA : %s", stStr, activeStage),
	}
	if len(changesSummary) > 0 {
		initialSteps = append(initialSteps, strings.Join(changesSummary, " | "))
	}
	initialSteps = append(initialSteps, "Poussée dans la file d'attente d'exécution...")

	actionTitle := fmt.Sprintf("Sync %s : %s", trackerName, task.Key)
	if activeStage != "" {
		actionTitle = fmt.Sprintf("Sync %s : %s (%s)", trackerName, task.Key, activeStage)
	}

	summaryText := fmt.Sprintf("Mise à jour asynchrone sur %s (%s)", trackerName, stStr)
	if len(changesSummary) > 0 {
		summaryText = fmt.Sprintf("Mise à jour sur %s : %s", trackerName, strings.Join(changesSummary, " | "))
	}

	act := models.TaskActivity{
		ID:        activityID,
		TaskID:    task.ID,
		TaskKey:   task.Key,
		TaskTitle: task.Title,
		SkillID:   "tracker_update",
		SkillName: fmt.Sprintf("Sync %s CLI", trackerName),
		Action:    actionTitle,
		Status:    "queued",
		Summary:   summaryText,
		Output:    "",
		Steps:     initialSteps,
		Prompt:    "",
		CreatedAt: now,
		UserID:    strings.TrimSpace(actorID),
	}

	_ = d.addTaskActivityDirect(act)

	job := SkillJob{
		ActivityID:      activityID,
		TaskID:          task.ID,
		SkillID:         "tracker_update",
		ActingUser:      actorID,
		Prompt:          stStr,
		RemovedLabels:   removedLabels,
		TrackerStatus:   strings.TrimSpace(task.TrackerStatus),
		SyncTitle:       changed.Title,
		SyncDescription: changed.Description,
		SyncPriority:    changed.Priority,
	}

	d.enqueueJob(job)
}

func (d *DB) processTrackerUpdateJob(ctx context.Context, job SkillJob) {
	// Whoever asked travels with the job: their credential is what the tracker
	// attributes the write to.
	ctx = tracker.WithActingUser(ctx, job.ActingUser)
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(job.TaskID)
	settings, _ := d.getSettingsUnsafe()
	d.mu.RUnlock()

	if err != nil || task == nil {
		d.mu.Lock()
		_, _ = d.conn.Exec(`
			UPDATE task_activities
			SET status = 'failed', error = 'Tâche introuvable pour la synchronisation tracker', completed_at = CURRENT_TIMESTAMP
			WHERE id = ? AND status != 'canceled'
		`, job.ActivityID)
		d.mu.Unlock()
		return
	}

	proj, _ := d.getProjectByIDUnsafe(task.ProjectID)
	repo := ""
	repoPath := ""
	if proj != nil {
		if proj.GithubRepo != "" {
			repo = proj.GithubRepo
		}
		if proj.RepoPath != "" {
			repoPath = proj.RepoPath
		}
	}
	if repo == "" && settings != nil {
		repo = settings.GithubRepo
	}
	if repoPath == "" && settings != nil {
		repoPath = settings.RepoPath
	}

	steps := []string{
		fmt.Sprintf("Démarrage de la mise à jour CLI pour [%s] %s", task.Key, task.Title),
	}
	var outputText string
	var hasError bool

	// Champs édités uniquement : renvoyer un titre ou une description inchangés
	// écraserait la version du tracker par notre copie aplatie.
	var titleForUpdate, descForUpdate *string
	var priorityForUpdate *models.Priority
	if job.SyncTitle {
		titleForUpdate = &task.Title
	}
	if job.SyncDescription {
		descForUpdate = &task.Description
	}
	if job.SyncPriority {
		priorityForUpdate = &task.Priority
	}

	syncStatus := task.Status
	for _, l := range task.Labels {
		clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(l), "#"))
		if clean == "finished" || clean == "closed" || clean == "done" {
			syncStatus = models.StatusFinished
			break
		}
	}
	if strings.EqualFold(task.TrackerStatus, "Done") || strings.EqualFold(task.TrackerStatus, "Closed") || strings.EqualFold(task.TrackerStatus, "Terminé") {
		syncStatus = models.StatusFinished
	}

	ts, tsErr := d.TrackerForTask(task)
	if tsErr == nil && ts != nil && ts.Supports(tracker.CapUpdate) {
		steps = append(steps, fmt.Sprintf("Exécution: mise à jour %s pour %s (statut: %s, labels: %v)", ts.Name(), task.Key, syncStatus, task.Labels))
		err := ts.UpdateIssue(ctx, tracker.UpdateIssueRequest{
			Project:       proj,
			Task:          task,
			Key:           task.Key,
			Title:         titleForUpdate,
			Description:   descForUpdate,
			Priority:      priorityForUpdate,
			Status:        &syncStatus,
			Labels:        task.Labels,
			RemovedLabels: job.RemovedLabels,
		})
		if err != nil {
			hasError = true
			outputText = fmt.Sprintf("Erreur %s : %v", ts.Name(), err)
			steps = append(steps, fmt.Sprintf("❌ Échec : %v", err))
		} else {
			outputText = fmt.Sprintf("Issue %s %s synchronisée avec succès (Titre: %s, Statut: %s, Labels: %v)", ts.Name(), task.Key, task.Title, syncStatus, task.Labels)
			steps = append(steps, fmt.Sprintf("✅ Issue %s %s mise à jour avec succès", ts.Name(), task.Key))

			if ts.Supports(tracker.CapGet) {
				// Rsync local (two-way unit sync): fetch fresh remote state from tracker and update SQLite
				steps = append(steps, fmt.Sprintf("Synchronisation retour unitaire (rsync local) depuis %s...", ts.Name()))
				if _, syncErr := d.SyncSingleTaskAs(ctx, task.ID); syncErr != nil {
					steps = append(steps, fmt.Sprintf("⚠️ Rsync local partiel : %v", syncErr))
				} else {
					steps = append(steps, "✅ Rsync local terminé : état distant réaligné en base locale")
				}
			}
		}
	} else {
		outputText = "Aucun tracker distant configuré pour cette tâche"
		steps = append(steps, "ℹ️ Tâche locale uniquement, aucune commande distante requise")
	}

	completedTime := time.Now()
	status := string(models.ActivityStatusCompleted)
	errText := ""
	summary := outputText
	if hasError {
		status = string(models.ActivityStatusFailed)
		errText = outputText
		summary = "Échec de la synchronisation tracker"
	}

	stepsJSON, _ := json.Marshal(steps)
	d.mu.Lock()
	_, _ = d.conn.Exec(`
		UPDATE task_activities
		SET status = ?, summary = ?, output = ?, steps = ?, error = ?, completed_at = ?
		WHERE id = ? AND status != 'canceled'
	`, status, summary, outputText, string(stepsJSON), errText, completedTime, job.ActivityID)
	d.mu.Unlock()
}

// SyncSingleTask pulls the single authoritative issue state from the remote tracker and writes it to SQLite.
// SyncSingleTask re-reads one work item with no acting user.
func (d *DB) SyncSingleTask(taskID string) (*models.Task, error) {
	return d.SyncSingleTaskAs(context.Background(), taskID)
}

// SyncSingleTaskAs re-reads it on behalf of whoever asked, so a personal
// tracker credential can be resolved for the read.
func (d *DB) SyncSingleTaskAs(ctx context.Context, taskID string) (*models.Task, error) {
	return d.syncSingleTask(ctx, taskID, false)
}

// ForceSyncSingleTask is the synchronisation a person asked for on one work
// item. It rediscovers that item's pull requests whatever the bounding rule
// says, which is how a task missing its link is repaired without waiting for a
// full pass.
func (d *DB) ForceSyncSingleTask(ctx context.Context, taskID string) (*models.Task, error) {
	return d.syncSingleTask(ctx, taskID, true)
}

func (d *DB) syncSingleTask(ctx context.Context, taskID string, force bool) (*models.Task, error) {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	settings, _ := d.getSettingsUnsafe()
	d.mu.RUnlock()

	if err != nil || task == nil {
		return nil, fmt.Errorf("task not found")
	}

	proj, _ := d.GetProjectByID(task.ProjectID)
	repo := ""
	repoPath := ""
	if proj != nil {
		repo = proj.GithubRepo
		repoPath = proj.RepoPath
	}
	if repo == "" && settings != nil {
		repo = settings.GithubRepo
	}
	if repoPath == "" && settings != nil {
		repoPath = settings.RepoPath
	}

	var syncedTask *models.Task
	ts, tsErr := d.TrackerForTask(task)
	if tsErr == nil && ts != nil && ts.Supports(tracker.CapGet) {
		syncedTask, err = ts.GetIssue(ctx, tracker.GetIssueRequest{
			Project: proj,
			Key:     task.Key,
		})
		if err != nil {
			return nil, err
		}
	}

	if syncedTask != nil {
		syncedTask.ProjectID = task.ProjectID
		if syncedTask.Priority == "" || syncedTask.Priority == models.PriorityMedium {
			if task.Priority != "" {
				syncedTask.Priority = task.Priority
			}
		}
		if syncedTask.Position == 0 && task.Position > 0 {
			syncedTask.Position = task.Position
		}
		if task.BranchName != nil {
			syncedTask.BranchName = task.BranchName
		}
		if task.PrURL != nil {
			syncedTask.PrURL = task.PrURL
		}

		if impErr := d.ImportOrUpdateTasks([]models.Task{*syncedTask}); impErr != nil {
			return nil, impErr
		}
		imported, readErr := d.GetTaskByID(task.ID)
		if readErr != nil || imported == nil {
			return imported, readErr
		}
		// Only a rediscovery the person asked for runs here: the background
		// loop re-reads every unfinished ticket one by one through this path,
		// and discovering on each of them would cost one tracker call per
		// ticket per pass.
		if force {
			if _, _, err := d.discoverAndApply(ctx, proj, ts, imported); err != nil {
				log.Printf("[prdiscovery] %s : %v", imported.Key, err)
			}
		}
		return d.refreshTaskPullRequestStates(ctx, imported), nil
	}

	return d.refreshTaskPullRequestStates(ctx, task), nil
}

// EnqueueSync queues a synchronisation nobody in particular asked for, so it
// runs with the server credential.
func (d *DB) EnqueueSync(syncType string, param string, projectID string) (*models.TaskActivity, error) {
	return d.EnqueueSyncAs("", syncType, param, projectID)
}

// EnqueueSyncAs queues a synchronisation on behalf of whoever asked. On a
// tracker whose credential is personal, this is what lets the work reach the
// site at all: the queue outlives the request, and the token belongs to the
// person rather than to the server.
func (d *DB) EnqueueSyncAs(userID string, syncType string, param string, projectID string) (*models.TaskActivity, error) {
	return d.EnqueueSyncWith(userID, syncType, param, projectID, SyncOptions{})
}

// EnqueueSyncWith is the same queueing with the background loop's two extras:
// the window to read back, and the fact that nobody is watching.
func (d *DB) EnqueueSyncWith(userID string, syncType string, param string, projectID string, opts SyncOptions) (*models.TaskActivity, error) {
	d.mu.RLock()
	settings, _ := d.getSettingsUnsafe()
	var proj *models.Project
	if projectID != "" && projectID != "all" {
		proj, _ = d.getProjectByIDUnsafe(projectID)
	}
	d.mu.RUnlock()

	githubRepo := ""
	if proj != nil {
		githubRepo = proj.GithubRepo
	}
	if githubRepo == "" && settings != nil {
		githubRepo = settings.GithubRepo
	}

	activityID := uuid.New().String()
	now := time.Now()

	var skillName string
	var summary string
	var steps []string

	switch syncType {
	case "github", "sync_github":
		syncType = "sync_github"
		repo := githubRepo
		if param != "" {
			repo = param
		}
		skillName = "Sync GitHub"
		summary = fmt.Sprintf("Synchronisation GitHub (%s) en file d'attente", repo)
		steps = []string{
			fmt.Sprintf("Cible : GitHub Repository (%s)", repo),
			"Poussée dans la file d'attente d'exécution...",
		}
	case "jira", "sync_jira":
		syncType = "sync_jira"
		jKey := jiraProjectKeyFor(proj)
		if param != "" {
			jKey = strings.ToUpper(strings.TrimSpace(param))
		}
		if jKey == "" && settings != nil {
			jKey = settings.JiraProject
		}
		skillName = "Sync Jira"
		summary = fmt.Sprintf("Synchronisation Jira (%s) en file d'attente", jKey)
		steps = []string{
			fmt.Sprintf("Cible : Jira Project (%s)", jKey),
			"Poussée dans la file d'attente d'exécution...",
		}
	default:
		// A tracker that has a name of its own keeps its own job. Falling
		// through to the global synchronisation would silently widen a pass
		// asked for one project into a pass over every project of the
		// deployment, the day a tracker other than GitHub or Jira is
		// registered.
		if name := strings.TrimPrefix(strings.TrimSpace(syncType), "sync_"); name != "" && name != "all" {
			if ts, ok := d.TrackerRegistry().Get(name); ok && ts != nil {
				syncType = "sync_" + name
				skillName = "Sync " + strings.ToUpper(name[:1]) + name[1:]
				summary = fmt.Sprintf("Synchronisation %s en file d'attente", skillName[5:])
				steps = []string{
					fmt.Sprintf("Cible : %s", skillName[5:]),
					"Poussée dans la file d'attente d'exécution...",
				}
				break
			}
		}
		syncType = "sync_all"
		skillName = "Sync Globale"
		summary = "Synchronisation multi-trackers en file d'attente"
		steps = []string{
			"Cibles : Tous les projets et trackers distants configurés",
			"Poussée dans la file d'attente d'exécution...",
		}
	}

	// A synchronisation belongs to a project, or to nothing when it covers every
	// project or a whole tracker. It never belongs to a ticket, and until #310 it
	// said so through a made-up task_id.
	attachedProject := ""
	if proj != nil {
		attachedProject = proj.ID
	}

	if opts.WindowMin > 0 {
		steps = append(steps, fmt.Sprintf("Lecture incrémentale : les tickets modifiés depuis %d minute(s)", opts.WindowMin))
	}

	act := models.TaskActivity{
		ID:        activityID,
		ProjectID: attachedProject,
		SkillID:   syncType,
		SkillName: skillName,
		Action:    "Synchronisation des tickets distants",
		Status:    string(models.ActivityStatusQueued),
		Summary:   summary,
		Output:    "",
		Steps:     steps,
		Prompt:    param,
		CreatedAt: now,
		// The account the read runs under. A background pass borrows the
		// project's owner (ADR 0018), and a refusal has to say whose credential
		// was refused rather than leave it to be guessed.
		UserID: strings.TrimSpace(userID),
	}

	d.mu.Lock()
	_ = d.addTaskActivityDirect(act)
	d.mu.Unlock()

	// Push job to worker queue
	d.enqueueJob(SkillJob{
		ActivityID: activityID,
		ProjectID:  projectID,
		SkillID:    syncType,
		Prompt:     param,
		ActingUser: userID,
		Sync:       opts,
	})

	return &act, nil
}

func (d *DB) EnqueueSkillOnTask(taskID string, skillID string, prompt string) (*models.Task, *models.TaskActivity, error) {
	return d.enqueueSkillOnTask(taskID, skillID, prompt, false, models.SkillModeUnset, "")
}

// EnqueueSkillOnTaskWithOverrides is the launch a user made an explicit choice
// for. An empty mode is not "interactive" and an empty model is not "the CLI
// default": both mean no override, and the precedence still falls through to
// the skill, the project and the global settings.
func (d *DB) EnqueueSkillOnTaskWithOverrides(taskID string, skillID string, prompt string, modeOverride string, modelOverride string) (*models.Task, *models.TaskActivity, error) {
	return d.enqueueSkillOnTask(taskID, skillID, prompt, false, modeOverride, modelOverride)
}

// EnqueueFullChainRun starts the chain: each step enqueues the next until the
// work reaches the project's stop stage. Every step it enqueues runs autonomous,
// whatever mode a single launch of that skill would resolve to.
func (d *DB) EnqueueFullChainRun(taskID string) (*models.Task, *models.TaskActivity, error) {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche non trouvée")
	}

	stopStage := d.FullChainStopStage(task.ProjectID)
	stage := d.StageOfTask(task)
	if stage == stopStage || stage == "finished" || stageAtOrPast(stage, stopStage) {
		return nil, nil, fmt.Errorf("la tâche est déjà à l'étape %s : la suite demande une revue humaine", stage)
	}
	step, ok := NextStep(stage)
	if !ok {
		return nil, nil, fmt.Errorf("aucun pas suivant depuis l'étape %s", stage)
	}
	return d.enqueueSkillOnTask(taskID, step.SkillID, "", true, models.SkillModeAutonomous, "", stopStage)
}

// FullChainStopStage is where a full chain run stops for a project. A project
// with nothing stored keeps the historical stage.
func (d *DB) FullChainStopStage(projectID string) string {
	project, err := d.GetProjectByID(projectID)
	if err != nil || project == nil {
		return DefaultFullChainStopStage
	}
	return models.NormalizeFullChainStopStage(project.FullChainStopStage)
}

// enqueueSkillOnTask files one skill launch. chainStopStage is variadic so the
// ordinary launches, which chain nothing, stay a five-argument call.
func (d *DB) enqueueSkillOnTask(taskID string, skillID string, prompt string, autoChain bool, modeOverride string, modelOverride string, chainStopStage ...string) (*models.Task, *models.TaskActivity, error) {
	skillID = models.NormalizeSkillID(skillID)
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("task not found: %s", taskID)
	}

	if skillID == "adjust" {
		if _, err := d.adjustmentPrerequisite(task, "", false); err != nil {
			return nil, nil, err
		}
	}
	if (skillID == "specify" || skillID == "implement") && d.StageOfTask(task) == "implemented" {
		prompt += "\nPR recovery: preserve all accepted work and the implemented stage. Complete only remaining owner checks and PR creation/reuse/linking. Never advance to reviewed."
	}
	skills := d.GetAvailableSkills()
	var targetSkill *models.Skill
	for _, s := range skills {
		if s.ID == skillID {
			targetSkill = &s
			break
		}
	}
	if targetSkill == nil {
		return nil, nil, fmt.Errorf("skill not found: %s", skillID)
	}

	activityID := uuid.New().String()
	now := time.Now()

	d.mu.RLock()
	skillName := d.resolveSkillNameUnsafe(task.ProjectID, targetSkill.ID, targetSkill.Name)
	d.mu.RUnlock()

	act := models.TaskActivity{
		ID:        activityID,
		TaskID:    task.ID,
		SkillID:   targetSkill.ID,
		SkillName: skillName,
		Action:    fmt.Sprintf("Exécution de la compétence %s", skillName),
		Status:    string(models.ActivityStatusQueued),
		Summary:   fmt.Sprintf("Compétence %s en file d'attente pour la tâche %s", skillName, task.Key),
		Output:    "",
		Steps: []string{
			fmt.Sprintf("Tâche ciblée : %s - %s", task.Key, task.Title),
			"Poussée dans la file d'attente du worker...",
		},
		Prompt:    prompt,
		CreatedAt: now,
	}

	// Every active run makes the task busy, a concurrent one included: a
	// session somebody started by hand is no reason to start a second agent.
	// The insert then settles the race the check cannot: the database refuses a
	// second ordinary active run on the task, from this server or any other.
	// Nothing is queued for a refused run.
	if active, err := d.ActiveRunOnTask(task.ID); err != nil {
		return nil, nil, err
	} else if active != nil {
		return nil, nil, &TaskBusyError{Active: active}
	}
	d.mu.Lock()
	err = d.addTaskActivityDirect(act)
	d.mu.Unlock()
	if err != nil {
		return nil, nil, d.taskBusy(task.ID, err)
	}

	// Push to background channel worker
	stopStage := ""
	if autoChain {
		stopStage = DefaultFullChainStopStage
		if len(chainStopStage) > 0 && chainStopStage[0] != "" {
			stopStage = chainStopStage[0]
		}
	}
	d.enqueueJob(SkillJob{
		ActivityID:     activityID,
		TaskID:         task.ID,
		ProjectID:      task.ProjectID,
		SkillID:        targetSkill.ID,
		Prompt:         prompt,
		Mode:           d.resolveTaskSkillMode(task.ProjectID, targetSkill.ID, modeOverride),
		Model:          strings.TrimSpace(modelOverride),
		ChainStopStage: stopStage,
	})

	return task, &act, nil
}

func (d *DB) RunSkillOnTask(taskID string, skillID string, prompt string) (*models.Task, *models.TaskActivity, error) {
	return d.EnqueueSkillOnTask(taskID, skillID, prompt)
}

// ResolveTaskSkillMode is resolveTaskSkillMode for callers outside the package:
// the handler that dispatches a skill straight to a connected agent resolves the
// mode the same way the job worker does.
func (d *DB) ResolveTaskSkillMode(projectID, skillID, modeOverride string) string {
	return d.resolveTaskSkillMode(projectID, skillID, modeOverride)
}

// ResolveTaskEngine names the provider and the model a launch resolves to, from
// what the server can see: the project over the global settings, with the launch
// override on top. It is what a run record carries until the agent reports the
// engine it really ran, which is the only value that also accounts for the
// workstation override.
func (d *DB) ResolveTaskEngine(projectID, skillID, modelOverride string) (provider string, model string) {
	var settings *models.Settings
	if s, err := d.GetSettings(); err == nil && s != nil {
		settings = s
	}
	var project *models.Project
	if p, err := d.GetProjectByID(projectID); err == nil && p != nil {
		project = p
	}
	levels := agentconfig.ModelConfig{}
	template := ""
	if project != nil {
		provider = strings.TrimSpace(project.AIProvider)
		template = project.AICommandTemplate
		levels = agentconfig.ModelConfig{Model: project.AIModel, SkillModels: project.AISkillModels}
	}
	if settings != nil {
		if provider == "" {
			provider = strings.TrimSpace(settings.AIProvider)
		}
		if strings.TrimSpace(template) == "" {
			template = settings.AICommandTemplate
		}
		levels = agentconfig.MergeModels(levels,
			agentconfig.ModelConfig{Model: settings.AIModel, SkillModels: settings.AISkillModels})
	}
	if provider == "" {
		provider = "agy"
	}
	resolved := agentconfig.ResolveSkillModel(levels, models.NormalizeSkillID(skillID))
	if override := strings.TrimSpace(modelOverride); override != "" {
		// The launch names one run, which is more specific than any per-skill
		// entry, so it wins outright rather than being merged as a bare model.
		resolved = override
	}
	// A run must not claim an engine its command line never carried: a provider
	// without a model flag, or a template with no {model} slot, runs without
	// one. The agent reaches the same conclusion from the configuration it
	// holds; this is the value shown until its report arrives.
	return provider, agentconfig.EffectiveModel(provider, template, resolved)
}

// resolveTaskSkillMode applies the precedence for one launch: the override
// chosen for it, then the skill's setting for this project, then the project
// default, then interactive.
func (d *DB) resolveTaskSkillMode(projectID, skillID, modeOverride string) string {
	projectDefault := ""
	if project, err := d.GetProjectByID(projectID); err == nil && project != nil {
		projectDefault = project.DefaultSkillMode
	}
	return ResolveSkillMode(modeOverride, d.ProjectSkillMode(projectID, skillID), projectDefault)
}

// activityProjectFilter is the "belongs to this project" condition, shared by the
// activity list and the activity statistics so the two cannot answer differently
// about the same project. It returns an empty clause when no project is named.
//
// It matches the recorded attachment and nothing else. Before #310 it also ran
// "a.task_id LIKE '%id%' OR a.prompt LIKE '%id%'", which caught project
// activities by the shape of their made-up identifier — and caught, with them,
// any activity whose prompt happened to mention another project.
//
// The identifier may be a project id or a project slug, as it always could, so
// each side is resolved both ways.
func activityProjectFilter(projectID string) (string, []interface{}) {
	if projectID == "" || projectID == "all" {
		return "", nil
	}
	clause := `(a.project_id = ? OR a.project_id = (SELECT id FROM projects WHERE slug = ?)
		OR t.project_id = ? OR t.project_id = (SELECT slug FROM projects WHERE id = ?)
		OR t.project_id = (SELECT id FROM projects WHERE slug = ?))`
	return clause, []interface{}{projectID, projectID, projectID, projectID, projectID}
}

func (d *DB) GetActivities(projectID, status, skillID, taskID, search string, limit int) ([]models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var conditions []string
	var args []interface{}

	if clause, clauseArgs := activityProjectFilter(projectID); clause != "" {
		conditions = append(conditions, clause)
		args = append(args, clauseArgs...)
	}

	if status != "" && status != "all" {
		if status == "queued" || status == "pending" {
			conditions = append(conditions, "a.status IN ('queued', 'pending')")
		} else {
			conditions = append(conditions, "a.status = ?")
			args = append(args, status)
		}
	}
	if skillID != "" && skillID != "all" {
		conditions = append(conditions, "a.skill_id = ?")
		args = append(args, skillID)
	}
	if taskID != "" {
		conditions = append(conditions, "a.task_id = ?")
		args = append(args, taskID)
	}
	if search != "" {
		cond, searchArgs := d.searchPredicate(search, "a.skill_name", "a.summary", "a.output", "t.key", "t.title")
		conditions = append(conditions, cond)
		args = append(args, searchArgs...)
	}

	sqlQuery := `
		SELECT a.id, COALESCE(a.task_id, ''), COALESCE(a.project_id, ''), COALESCE(t.key, ''), COALESCE(t.title, ''), a.skill_id, a.skill_name,
		       a.action, a.status, a.summary, a.output, a.steps, a.prompt,
		       a.created_at, a.started_at, a.completed_at, a.error, a.waiting_since,
		       a.user_id, COALESCE(NULLIF(u.chosen_name, ''), u.display_name, ''), COALESCE(u.email, ''), a.run_provider, a.run_model
		FROM task_activities a
		LEFT JOIN tasks t ON a.task_id = t.id
		LEFT JOIN users u ON u.id = a.user_id
	`
	if len(conditions) > 0 {
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}
	sqlQuery += " ORDER BY a.created_at DESC"
	if limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := d.conn.Query(sqlQuery, args...)
	if err != nil {
		return []models.TaskActivity{}, err
	}
	defer rows.Close()

	var list []models.TaskActivity
	for rows.Next() {
		var a models.TaskActivity
		var stepsJSON string
		var prompt, errStr, runProvider, runModel sql.NullString
		var startedAt, completedAt, waitingSince sql.NullTime
		var ownerName, ownerEmail string

		err := rows.Scan(
			&a.ID,
			&a.TaskID,
			&a.ProjectID,
			&a.TaskKey,
			&a.TaskTitle,
			&a.SkillID,
			&a.SkillName,
			&a.Action,
			&a.Status,
			&a.Summary,
			&a.Output,
			&stepsJSON,
			&prompt,
			&a.CreatedAt,
			&startedAt,
			&completedAt,
			&errStr,
			&waitingSince,
			&a.UserID,
			&ownerName,
			&ownerEmail,
			&runProvider,
			&runModel,
		)
		if err != nil {
			continue
		}
		a.UserName = ownerDisplayName(ownerName, ownerEmail)
		if waitingSince.Valid {
			a.WaitingSince = &waitingSince.Time
		}
		a.Provider, a.Model = runProvider.String, runModel.String
		_ = json.Unmarshal([]byte(stepsJSON), &a.Steps)
		if a.Steps == nil {
			a.Steps = []string{}
		}
		if prompt.Valid {
			a.Prompt = prompt.String
		}
		if errStr.Valid {
			a.Error = errStr.String
		}
		if startedAt.Valid {
			a.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			a.CompletedAt = &completedAt.Time
		}

		if a.StartedAt != nil && a.CompletedAt != nil {
			dur := a.CompletedAt.Sub(*a.StartedAt)
			if dur.Seconds() < 1 {
				a.Duration = fmt.Sprintf("%dms", dur.Milliseconds())
			} else if dur.Seconds() < 60 {
				a.Duration = fmt.Sprintf("%.1fs", dur.Seconds())
			} else {
				a.Duration = fmt.Sprintf("%dm%ds", int(dur.Minutes()), int(dur.Seconds())%60)
			}
		}

		list = append(list, a)
	}

	if list == nil {
		list = []models.TaskActivity{}
	}
	return list, nil
}

func (d *DB) GetActivityByID(id string) (*models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var a models.TaskActivity
	var stepsJSON string
	var prompt, errStr, runProvider, runModel sql.NullString
	var startedAt, completedAt, waitingSince sql.NullTime
	var ownerName, ownerEmail string

	err := d.conn.QueryRow(`
		SELECT a.id, COALESCE(a.task_id, ''), COALESCE(a.project_id, ''), COALESCE(t.key, ''), COALESCE(t.title, ''), a.skill_id, a.skill_name,
		       a.action, a.status, a.summary, a.output, a.steps, a.prompt,
		       a.created_at, a.started_at, a.completed_at, a.error, a.waiting_since,
		       a.user_id, COALESCE(NULLIF(u.chosen_name, ''), u.display_name, ''), COALESCE(u.email, ''), a.run_provider, a.run_model, a.concurrent
		FROM task_activities a
		LEFT JOIN tasks t ON a.task_id = t.id
		LEFT JOIN users u ON u.id = a.user_id
		WHERE a.id = ?
	`, id).Scan(
		&a.ID,
		&a.TaskID,
		&a.ProjectID,
		&a.TaskKey,
		&a.TaskTitle,
		&a.SkillID,
		&a.SkillName,
		&a.Action,
		&a.Status,
		&a.Summary,
		&a.Output,
		&stepsJSON,
		&prompt,
		&a.CreatedAt,
		&startedAt,
		&completedAt,
		&errStr,
		&waitingSince,
		&a.UserID,
		&ownerName,
		&ownerEmail,
		&runProvider,
		&runModel,
		&a.Concurrent,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.UserName = ownerDisplayName(ownerName, ownerEmail)
	if waitingSince.Valid {
		a.WaitingSince = &waitingSince.Time
	}
	a.Provider, a.Model = runProvider.String, runModel.String

	_ = json.Unmarshal([]byte(stepsJSON), &a.Steps)
	if a.Steps == nil {
		a.Steps = []string{}
	}
	if prompt.Valid {
		a.Prompt = prompt.String
	}
	if errStr.Valid {
		a.Error = errStr.String
	}
	if startedAt.Valid {
		a.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		a.CompletedAt = &completedAt.Time
	}

	if a.StartedAt != nil && a.CompletedAt != nil {
		dur := a.CompletedAt.Sub(*a.StartedAt)
		if dur.Seconds() < 1 {
			a.Duration = fmt.Sprintf("%dms", dur.Milliseconds())
		} else if dur.Seconds() < 60 {
			a.Duration = fmt.Sprintf("%.1fs", dur.Seconds())
		} else {
			a.Duration = fmt.Sprintf("%dm%ds", int(dur.Minutes()), int(dur.Seconds())%60)
		}
	}

	return &a, nil
}

func (d *DB) GetActivityStats(projectID string) (*models.ActivityStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var stats models.ActivityStats
	var query string
	var args []interface{}

	if clause, clauseArgs := activityProjectFilter(projectID); clause != "" {
		query = `
			SELECT a.status, COUNT(*)
			FROM task_activities a
			LEFT JOIN tasks t ON a.task_id = t.id
			WHERE ` + clause + `
			GROUP BY a.status
		`
		args = append(args, clauseArgs...)
	} else {
		query = "SELECT status, COUNT(*) FROM task_activities GROUP BY status"
	}

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return &stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var st string
		var cnt int
		if err := rows.Scan(&st, &cnt); err == nil {
			stats.Total += cnt
			switch st {
			case "queued", "pending":
				stats.Queued += cnt
			case "running":
				stats.Running += cnt
			case "completed":
				stats.Completed += cnt
			case "failed":
				stats.Failed += cnt
			case "canceled":
				stats.Canceled += cnt
			}
		}
	}
	return &stats, nil
}

func (d *DB) RetryActivity(activityID string) (*models.TaskActivity, error) {
	act, err := d.GetActivityByID(activityID)
	if err != nil || act == nil {
		return nil, fmt.Errorf("activity not found")
	}

	_, newAct, err := d.EnqueueSkillOnTask(act.TaskID, act.SkillID, act.Prompt)
	return newAct, err
}

// CancelActivity stops an activity and records it as canceled. The job may be
// running in another server instance sharing the database, so after recording
// the cancellation it is published for whichever instance holds the job.
func (d *DB) CancelActivity(activityID string) error {
	d.cancelLocal(activityID)

	d.mu.Lock()
	_, err := d.conn.Exec(`
		UPDATE task_activities
		SET status = 'canceled', summary = 'Annulée par l''utilisateur', completed_at = CURRENT_TIMESTAMP, waiting_since = NULL
		WHERE id = ? AND status IN ('queued', 'pending', 'running')
	`, activityID)
	d.mu.Unlock()
	if err != nil {
		return err
	}
	d.publish(BusMessage{Kind: busKindCancel, ActivityID: activityID})
	return nil
}

// cancelLocal stops the job this process is running for an activity, if any.
func (d *DB) cancelLocal(activityID string) {
	d.cancelMu.Lock()
	defer d.cancelMu.Unlock()
	if cancel, exists := d.cancelMap[activityID]; exists {
		cancel()
		delete(d.cancelMap, activityID)
	}
}

func (d *DB) DeleteActivity(activityID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM task_activities WHERE id = ?", activityID)
	return err
}

func (d *DB) ClearCompletedActivities() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.conn.Exec("DELETE FROM task_activities WHERE status IN ('completed', 'failed', 'canceled')")
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

// AddTaskComment posts to the tracker with no acting user, for callers who
// have none.
func (d *DB) AddTaskComment(taskID string, body string) error {
	return d.AddTaskCommentAs(context.Background(), taskID, body)
}

// AddTaskCommentAs posts on behalf of whoever the context names. On a tracker
// that attributes a comment to the account behind the token, this is what puts
// the person's own name on what they wrote.
func (d *DB) AddTaskCommentAs(ctx context.Context, taskID string, body string) error {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	var proj *models.Project
	if task != nil && task.ProjectID != "" {
		proj, _ = d.getProjectByIDUnsafe(task.ProjectID)
	}
	d.mu.RUnlock()

	if err != nil || task == nil {
		return fmt.Errorf("task not found")
	}

	ts, err := d.TrackerForTask(task)
	if err != nil {
		return err
	}
	return ts.AddComment(ctx, tracker.AddCommentRequest{
		Project: proj,
		Key:     task.Key,
		Body:    body,
	})
}

func (d *DB) ConvertTaskToRemote(taskID string, target string) (*models.Task, error) {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	settings, _ := d.getSettingsUnsafe()
	var proj *models.Project
	if task != nil && task.ProjectID != "" {
		proj, _ = d.getProjectByIDUnsafe(task.ProjectID)
	}
	d.mu.RUnlock()

	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	if settings == nil {
		settings = &models.Settings{
			RepoPath: ".",
		}
	}

	var newKey string
	var extURL *string
	now := time.Now()

	// Ensure stage label is properly set based on current status
	task.Labels = SetWorkflowLabel(task.Labels, "#"+GetStageLabelForStatus(task.Status))

	ts, ok := d.TrackerRegistry().Get(target)
	if !ok || !ts.Supports(tracker.CapCreate) {
		return nil, fmt.Errorf("tracker distant non supporté: %s", target)
	}

	// Claim the task before creating anything. The claim holds only while the
	// key is still the one read above and no other conversion holds it: a
	// second request, here or on another instance, fails here instead of
	// creating a second issue, including one that read the task before the
	// first conversion changed its key.
	d.mu.Lock()
	var previousSource sql.NullString
	claimErr := d.conn.WithTx(func(tx *sqlTx) error {
		var key string
		var claimedAt time.Time
		if err := tx.QueryRow("SELECT key, source, updated_at FROM tasks WHERE id = ?"+d.forUpdate(), task.ID).Scan(&key, &previousSource, &claimedAt); err != nil {
			return err
		}
		// A claim older than convertClaimExpiry belongs to a conversion that
		// died with its server: it is taken over rather than refusing the task
		// forever. Its previous source is unknown by then; the task was local.
		if previousSource.String == sourceConverting && time.Since(claimedAt) >= convertClaimExpiry {
			previousSource = sql.NullString{String: "local", Valid: true}
		} else if key != task.Key || previousSource.String == sourceConverting {
			return fmt.Errorf("la tâche %s est déjà en cours de conversion ou a déjà été convertie", task.Key)
		}
		_, err := tx.Exec("UPDATE tasks SET source = ?, updated_at = ? WHERE id = ?", sourceConverting, time.Now(), task.ID)
		return err
	})
	d.mu.Unlock()
	if claimErr != nil {
		return nil, claimErr
	}
	// A conversion that does not end with an issue gives the task back as it was.
	release := func() {
		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET source = ? WHERE id = ? AND source = ?", previousSource, task.ID, sourceConverting)
		d.mu.Unlock()
	}

	created, err := ts.CreateIssue(context.Background(), tracker.CreateIssueRequest{
		Project:     proj,
		Title:       task.Title,
		Description: task.Description,
		Priority:    task.Priority,
		Labels:      task.Labels,
	})
	if err != nil {
		release()
		return nil, fmt.Errorf("création %s impossible: %w", ts.Name(), err)
	}
	if created == nil {
		release()
		return nil, fmt.Errorf("création %s impossible: ticket non retourné", ts.Name())
	}
	newKey = created.Key
	extURL = created.ExternalURL

	d.mu.Lock()
	defer d.mu.Unlock()

	// The task is written from its locked row, so an edit made during the
	// tracker call is kept, and only while the claim is still this conversion's.
	// A claim lost meanwhile leaves an issue nobody records: say which one
	// rather than report a success.
	err = d.conn.WithTx(func(tx *sqlTx) error {
		locked, err := d.lockTaskUnsafe(tx, task.ID)
		if err != nil {
			return err
		}
		var source string
		if err := tx.QueryRow("SELECT COALESCE(source, '') FROM tasks WHERE id = ?", task.ID).Scan(&source); err != nil {
			return err
		}
		if locked == nil || source != sourceConverting {
			return fmt.Errorf("conversion de %s interrompue : l'issue %s a été créée sur %s mais n'est pas rattachée à la tâche", task.Key, newKey, ts.Name())
		}
		task = locked
		task.Labels = SetWorkflowLabel(task.Labels, "#"+GetStageLabelForStatus(task.Status))
		task.Key = newKey
		task.Source = target
		task.ExternalURL = extURL
		task.UpdatedAt = now
		labelsJSON, _ := json.Marshal(task.Labels)
		_, err = tx.Exec(`
			UPDATE tasks
			SET key = ?, source = ?, external_url = ?, labels = ?, updated_at = ?
			WHERE id = ?
		`, task.Key, task.Source, task.ExternalURL, string(labelsJSON), now, task.ID)
		return err
	})
	if err != nil {
		return nil, err
	}

	// Creation is confirmed; synchronize its state through the observable queue.
	d.enqueueTrackerUpdateUnsafe(task, &task.Status, task.Labels, nil, TrackerFieldChanges{})

	outputMsg := fmt.Sprintf("Tâche locale convertie vers %s.\nClé distante : %s", strings.ToUpper(target), task.Key)
	if extURL != nil {
		outputMsg += fmt.Sprintf("\nURL : %s", *extURL)
	}

	act := models.TaskActivity{
		ID:        uuid.New().String(),
		TaskID:    task.ID,
		SkillID:   "convert",
		SkillName: "Export Tracker",
		Action:    fmt.Sprintf("Tâche convertie vers %s (%s)", strings.ToUpper(target), task.Key),
		Status:    string(models.ActivityStatusCompleted),
		Summary:   fmt.Sprintf("Issue distante créée avec succès : %s", task.Key),
		Output:    outputMsg,
		Steps:     []string{fmt.Sprintf("Création du ticket distant sur %s", strings.ToUpper(target)), fmt.Sprintf("Mise à jour de la clé (%s) et de la source", task.Key)},
		CreatedAt: now,
	}
	_ = d.addTaskActivityDirect(act)
	acts, _ := d.getTaskActivitiesUnsafe(task.ID)
	task.Activities = acts

	return task, nil
}

// -------------------------------------------------------------
// PROJECTS CRUD & MANAGEMENT
// -------------------------------------------------------------

// resolveSkillNameUnsafe returns the display name of a skill for a given
// project, honouring the project's SkillOverrides map (skillId -> custom label).
// Without this, a renamed skill would keep its default name everywhere outside
// the project settings form.
func (d *DB) resolveSkillNameUnsafe(projectID string, skillID string, defaultName string) string {
	proj, _ := d.getProjectByIDUnsafe(projectID)
	if proj == nil || proj.SkillOverrides == nil {
		return defaultName
	}
	if override := strings.TrimSpace(proj.SkillOverrides[skillID]); override != "" {
		return override
	}
	return defaultName
}

// applySkillCommandOverride rewrites the default prompt of a workflow stage so
// it invokes the slash command the project configured through SkillOverrides.
// An explicit custom prompt in the settings always wins: the user wrote it on
// purpose and it may already name its own command.
func applySkillCommandOverride(settings *models.Settings, proj *models.Project, skillID string) {
	if proj == nil || proj.SkillOverrides == nil || settings == nil {
		return
	}
	override := strings.TrimSpace(proj.SkillOverrides[skillID])
	if override == "" {
		return
	}
	cmd := "/" + strings.TrimPrefix(override, "/")

	switch skillID {
	case "clarify":
		if settings.PromptClarify == "" {
			settings.PromptClarify = cmd + " {issueKey} tracked on {tracker} in {repo}"
		}
	case "specify":
		if settings.PromptSpecify == "" {
			settings.PromptSpecify = cmd + " {issueKey}"
		}
	case "implement":
		if settings.PromptImplement == "" {
			settings.PromptImplement = cmd + " {issueKey}"
		}
	case "adjust", "review":
		if settings.PromptAdjust == "" && settings.PromptCreatePR == "" {
			settings.PromptAdjust = cmd + " {issueKey}"
		}
	case "handoff":
		if settings.PromptHandoff == "" {
			settings.PromptHandoff = cmd + " {issueKey}"
		}
	case "pick":
		if settings.PromptPick == "" {
			settings.PromptPick = cmd + " {issueKey}"
		}
	}
}

// jiraProjectKeyFor resolves the Jira project key of a project. Projects
// created before the dedicated jira_project column existed stored nothing, and
// Taskacao used to pass the slug to acli, so the slug remains the fallback.
func jiraProjectKeyFor(p *models.Project) string {
	if p == nil {
		return ""
	}
	if key := strings.ToUpper(strings.TrimSpace(p.JiraProject)); key != "" {
		return key
	}
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(p.Slug), "-", ""))
}

func ParseGitRepoFromURL(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	raw = strings.TrimSuffix(raw, ".git")
	if strings.HasPrefix(raw, "git@github.com:") {
		return strings.TrimPrefix(raw, "git@github.com:")
	}
	if strings.HasPrefix(raw, "https://github.com/") {
		return strings.TrimPrefix(raw, "https://github.com/")
	}
	if strings.HasPrefix(raw, "http://github.com/") {
		return strings.TrimPrefix(raw, "http://github.com/")
	}
	if strings.HasPrefix(raw, "ssh://git@github.com/") {
		return strings.TrimPrefix(raw, "ssh://git@github.com/")
	}
	return raw
}

// parseRepoPaths decodes the project's known working directories, tolerating an
// empty column on projects created before the field existed.
func parseRepoPaths(raw string) []string {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []string{}
	}
	return normalizeRepoPaths(list)
}

// parseSkillModels reads the per-skill model column. A nil map is the normal
// shape for "no skill departs from the project model", so a malformed or empty
// column yields nil rather than an error: a bad row must not make a project
// unreadable.
func parseSkillModels(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return nil
	}
	var models map[string]string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil
	}
	return normalizeSkillModels(models)
}

// parseProviderModels reads the per-provider model column. Like the per-skill
// one, a malformed column yields nil rather than an error: the lists shipped
// with Sectile then apply, which is better than settings that cannot be read.
func parseProviderModels(raw string) map[string][]string {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return nil
	}
	var models map[string][]string
	if err := json.Unmarshal([]byte(raw), &models); err != nil {
		return nil
	}
	return agentconfig.NormalizeProviderModels(models)
}

// normalizeSkillModels drops the entries that mean nothing, a blank skill key or
// a blank model, so an emptied field in the interface stops overriding instead
// of persisting an empty model.
func normalizeSkillModels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for skill, model := range in {
		skill, model = strings.TrimSpace(skill), strings.TrimSpace(model)
		if skill == "" || model == "" {
			continue
		}
		out[skill] = model
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseSetupProviders decodes the agents a project sets up, tolerating an empty
// column on projects created before the field existed.
func parseSetupProviders(raw string) []string {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []string{}
	}
	return models.NormalizeSetupProviders(list)
}

// normalizeRepoPaths trims, drops blanks and de-duplicates while keeping the
// order the paths were added in, so the list stays predictable in the UI.
func normalizeRepoPaths(list []string) []string {
	seen := make(map[string]bool, len(list))
	out := make([]string, 0, len(list))
	for _, p := range list {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= maxProjectRepoPaths {
			break
		}
	}
	return out
}

// maxProjectRepoPaths caps the auto-fed list so a long-lived project does not
// accumulate an unusable dropdown.
const maxProjectRepoPaths = 25

// registerProjectRepoPathUnsafe records a working directory on the project when
// a ticket pins one, so the next ticket can pick it from the list. Caller must
// hold the write lock.
func (d *DB) registerProjectRepoPathUnsafe(projectID string, repoPath string) {
	repoPath = strings.TrimSpace(repoPath)
	if projectID == "" || repoPath == "" {
		return
	}
	proj, err := d.getProjectByIDUnsafe(projectID)
	if err != nil || proj == nil {
		return
	}
	// The project's own repoPath is always offered, no need to store it twice.
	if strings.TrimSpace(proj.RepoPath) == repoPath {
		return
	}
	for _, existing := range proj.RepoPaths {
		if existing == repoPath {
			return
		}
	}
	updated := normalizeRepoPaths(append(proj.RepoPaths, repoPath))
	payload, err := json.Marshal(updated)
	if err != nil {
		return
	}
	_, _ = d.conn.Exec("UPDATE projects SET repo_paths = ?, updated_at = ? WHERE id = ?", string(payload), time.Now(), proj.ID)
}

// parseTrackerColumns decodes the stored board columns, tolerating an empty
// column on projects that never imported a board.
func parseTrackerColumns(raw string) []models.TrackerColumn {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []models.TrackerColumn{}
	}
	var list []models.TrackerColumn
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []models.TrackerColumn{}
	}
	return list
}

// parseSprints decodes the board's sprints and their state.
func parseSprints(raw string) []models.TrackerSprint {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []models.TrackerSprint{}
	}
	var list []models.TrackerSprint
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []models.TrackerSprint{}
	}
	return list
}

// parseIssueTypes decodes the work item types a project imports. An empty list
// is not a misconfiguration: it means the default types apply.
func parseIssueTypes(raw string) []string {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []string{}
	}
	return list
}

// parseEnabledViews decodes the optional views a project shows, dropping
// anything the current version does not know: a view retired between two
// releases must not leave a dead entry in the sidebar.
func parseEnabledViews(raw string) []string {
	if strings.TrimSpace(raw) == "" || raw == "[]" {
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return []string{}
	}
	return models.NormalizeEnabledViews(list)
}

// parseStageColumns decodes the workflow stage to columns assignment.
func parseStageColumns(raw string) map[string][]string {
	if strings.TrimSpace(raw) == "" || raw == "{}" {
		return map[string][]string{}
	}
	out := map[string][]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string][]string{}
	}
	return out
}

func (d *DB) getProjectsUnsafe() ([]models.Project, error) {
	rows, err := d.conn.Query(`
		SELECT p.id, p.name, p.slug, p.description, p.icon, p.color, p.repo_path, p.repo_paths, p.use_worktrees, p.default_skill_mode, p.full_chain_stop_stage, p.pr_creation_stage, p.board_id, p.tracker_columns, p.stage_columns, p.sprints, p.issue_types, p.enabled_views, p.epic_colors, p.spec_repo_path, p.roadmap_projects, p.mono_repo, p.git_remote_url, p.github_repo, p.github_api_url, p.github_token, p.gitlab_url, p.gitlab_project, p.gitlab_token, p.jira_project, p.issue_tracker, p.tracker_url, p.is_default, p.skill_overrides, p.setup_providers, p.ai_provider, p.ai_command_template, p.ai_command_template_autonomous, p.ai_model, p.ai_skill_models, p.spec_framework, p.tty_mode, p.external_terminal_command, p.auto_sync_enabled, p.auto_sync_interval_min, p.owner_user_id, p.created_at, p.updated_at,
		       COUNT(t.id) as task_count
		FROM projects p
		LEFT JOIN tasks t ON t.project_id = p.id
		GROUP BY p.id
		ORDER BY p.is_default DESC, p.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		var isDefault int
		var autoSyncEnabledInt, autoSyncIntervalMin int
		var skillOverridesJSON, setupProvidersJSON, repoPathsJSON string
		var useWorktrees int
		var defaultSkillMode, fullChainStopStage sql.NullString
		var trackerColumnsJSON, stageColumnsJSON, sprintsJSON, issueTypesJSON, enabledViewsJSON, roadmapProjectsJSON string
		var monoRepo, epicColors int
		var aiProv, aiCmd, aiCmdAuto, specFw, jiraProj, ttyMode, extTerm sql.NullString
		var projModel, projSkillModelsJSON sql.NullString
		var ghURL, ghTok, glURL, glProj, glTok sql.NullString
		var ownerUserID sql.NullString
		err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.Color, &p.RepoPath, &repoPathsJSON, &useWorktrees, &defaultSkillMode, &fullChainStopStage, &p.PRCreationStage, &p.BoardID, &trackerColumnsJSON, &stageColumnsJSON, &sprintsJSON, &issueTypesJSON, &enabledViewsJSON, &epicColors, &p.SpecRepoPath, &roadmapProjectsJSON, &monoRepo, &p.GitRemoteUrl, &p.GithubRepo, &ghURL, &ghTok, &glURL, &glProj, &glTok, &jiraProj, &p.IssueTracker, &p.TrackerUrl, &isDefault, &skillOverridesJSON, &setupProvidersJSON, &aiProv, &aiCmd, &aiCmdAuto, &projModel, &projSkillModelsJSON, &specFw, &ttyMode, &extTerm, &autoSyncEnabledInt, &autoSyncIntervalMin, &ownerUserID, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount,
		)
		if err != nil {
			return nil, err
		}
		p.IsDefault = isDefault == 1
		p.AutoSyncEnabled = autoSyncEnabledInt == 1
		p.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(autoSyncIntervalMin)
		p.SkillOverrides = map[string]string{}
		if skillOverridesJSON != "" && skillOverridesJSON != "{}" {
			_ = json.Unmarshal([]byte(skillOverridesJSON), &p.SkillOverrides)
		}
		p.AIModel = projModel.String
		p.AISkillModels = parseSkillModels(projSkillModelsJSON.String)
		p.SetupProviders = parseSetupProviders(setupProvidersJSON)
		p.RepoPaths = parseRepoPaths(repoPathsJSON)
		p.UseWorktrees = useWorktrees == 1
		p.DefaultSkillMode = models.NormalizeSkillMode(defaultSkillMode.String)
		p.FullChainStopStage = models.NormalizeFullChainStopStage(fullChainStopStage.String)
		p.TrackerColumns = parseTrackerColumns(trackerColumnsJSON)
		p.StageColumns = parseStageColumns(stageColumnsJSON)
		p.Sprints = parseSprints(sprintsJSON)
		p.IssueTypes = parseIssueTypes(issueTypesJSON)
		p.EnabledViews = parseEnabledViews(enabledViewsJSON)
		p.RoadmapProjects = parseRoadmapProjects(roadmapProjectsJSON)
		p.EpicColors = epicColors == 1
		p.MonoRepo = monoRepo == 1
		p.TtyMode = "integrated"
		if ttyMode.Valid && ttyMode.String != "" {
			p.TtyMode = ttyMode.String
		}
		if extTerm.Valid {
			p.ExternalTerminalCommand = extTerm.String
		}
		if aiProv.Valid {
			p.AIProvider = aiProv.String
		}
		if aiCmd.Valid {
			p.AICommandTemplate = aiCmd.String
		}
		p.AICommandTemplateAutonomous = aiCmdAuto.String
		if jiraProj.Valid {
			p.JiraProject = jiraProj.String
		}
		p.GithubApiUrl, p.GithubToken = ghURL.String, ghTok.String
		p.GitlabUrl, p.GitlabProject, p.GitlabToken = glURL.String, glProj.String, glTok.String
		p.SpecFramework = models.NormalizeSpecFramework(specFw.String)
		p.OwnerUserID = strings.TrimSpace(ownerUserID.String)
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []models.Project{}
	}
	return projects, nil
}

func (d *DB) GetProjectsForUser(userID string) ([]models.Project, error) {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		_ = d.EnsureDefaultBookmark(userID)
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	projects, err := d.getProjectsUnsafe()
	if err != nil {
		return nil, err
	}

	var bookmarks map[string]bool
	if userID != "" {
		bookmarks, err = d.getUserProjectBookmarksMapUnsafe(userID)
		if err != nil {
			return nil, err
		}
	}

	for i := range projects {
		if bookmarks != nil && bookmarks[projects[i].ID] {
			projects[i].Bookmarked = true
		} else {
			projects[i].Bookmarked = false
		}
		withoutProjectTokens(&projects[i])
	}
	return projects, nil
}

func (d *DB) GetProjects() ([]models.Project, error) {
	return d.GetProjectsForUser("")
}

func (d *DB) GetProjectByID(id string) (*models.Project, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	proj, err := d.getProjectByIDUnsafe(id)
	return withoutProjectTokens(proj), err
}

func (d *DB) getProjectByIDUnsafe(id string) (*models.Project, error) {
	var p models.Project
	var isDefault int
	var autoSyncEnabledInt, autoSyncIntervalMin int
	var skillOverridesJSON, setupProvidersJSON, repoPathsJSON string
	var useWorktrees int
	var defaultSkillMode, fullChainStopStage sql.NullString
	var trackerColumnsJSON, stageColumnsJSON, sprintsJSON, issueTypesJSON, enabledViewsJSON, roadmapProjectsJSON string
	var monoRepo, epicColors int
	var aiProv, aiCmd, aiCmdAuto, specFw, jiraProj, ttyMode, extTerm sql.NullString
	var projModel, projSkillModelsJSON sql.NullString
	var ghURL, ghTok, glURL, glProj, glTok sql.NullString
	var ownerUserID sql.NullString
	err := d.conn.QueryRow(`
		SELECT p.id, p.name, p.slug, p.description, p.icon, p.color, p.repo_path, p.repo_paths, p.use_worktrees, p.default_skill_mode, p.full_chain_stop_stage, p.pr_creation_stage, p.board_id, p.tracker_columns, p.stage_columns, p.sprints, p.issue_types, p.enabled_views, p.epic_colors, p.spec_repo_path, p.roadmap_projects, p.mono_repo, p.git_remote_url, p.github_repo, p.github_api_url, p.github_token, p.gitlab_url, p.gitlab_project, p.gitlab_token, p.jira_project, p.issue_tracker, p.tracker_url, p.is_default, p.skill_overrides, p.setup_providers, p.ai_provider, p.ai_command_template, p.ai_command_template_autonomous, p.ai_model, p.ai_skill_models, p.spec_framework, p.tty_mode, p.external_terminal_command, p.auto_sync_enabled, p.auto_sync_interval_min, p.owner_user_id, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM tasks WHERE project_id = p.id) as task_count
		FROM projects p
		WHERE p.id = ? OR p.slug = ?
	`, id, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.Color, &p.RepoPath, &repoPathsJSON, &useWorktrees, &defaultSkillMode, &fullChainStopStage, &p.PRCreationStage, &p.BoardID, &trackerColumnsJSON, &stageColumnsJSON, &sprintsJSON, &issueTypesJSON, &enabledViewsJSON, &epicColors, &p.SpecRepoPath, &roadmapProjectsJSON, &monoRepo, &p.GitRemoteUrl, &p.GithubRepo, &ghURL, &ghTok, &glURL, &glProj, &glTok, &jiraProj, &p.IssueTracker, &p.TrackerUrl, &isDefault, &skillOverridesJSON, &setupProvidersJSON, &aiProv, &aiCmd, &aiCmdAuto, &projModel, &projSkillModelsJSON, &specFw, &ttyMode, &extTerm, &autoSyncEnabledInt, &autoSyncIntervalMin, &ownerUserID, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	p.IsDefault = isDefault == 1
	p.AutoSyncEnabled = autoSyncEnabledInt == 1
	p.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(autoSyncIntervalMin)
	p.SkillOverrides = map[string]string{}
	if skillOverridesJSON != "" && skillOverridesJSON != "{}" {
		_ = json.Unmarshal([]byte(skillOverridesJSON), &p.SkillOverrides)
	}
	p.AIModel = projModel.String
	p.AISkillModels = parseSkillModels(projSkillModelsJSON.String)
	p.SetupProviders = parseSetupProviders(setupProvidersJSON)
	p.RepoPaths = parseRepoPaths(repoPathsJSON)
	p.UseWorktrees = useWorktrees == 1
	p.DefaultSkillMode = models.NormalizeSkillMode(defaultSkillMode.String)
	p.FullChainStopStage = models.NormalizeFullChainStopStage(fullChainStopStage.String)
	p.TrackerColumns = parseTrackerColumns(trackerColumnsJSON)
	p.StageColumns = parseStageColumns(stageColumnsJSON)
	p.Sprints = parseSprints(sprintsJSON)
	p.IssueTypes = parseIssueTypes(issueTypesJSON)
	p.EnabledViews = parseEnabledViews(enabledViewsJSON)
	p.RoadmapProjects = parseRoadmapProjects(roadmapProjectsJSON)
	p.EpicColors = epicColors == 1
	p.MonoRepo = monoRepo == 1
	p.TtyMode = "integrated"
	if ttyMode.Valid && ttyMode.String != "" {
		p.TtyMode = ttyMode.String
	}
	if extTerm.Valid {
		p.ExternalTerminalCommand = extTerm.String
	}
	if aiProv.Valid {
		p.AIProvider = aiProv.String
	}
	if aiCmd.Valid {
		p.AICommandTemplate = aiCmd.String
	}
	p.AICommandTemplateAutonomous = aiCmdAuto.String
	if jiraProj.Valid {
		p.JiraProject = jiraProj.String
	}
	p.GithubApiUrl, p.GithubToken = ghURL.String, ghTok.String
	p.GitlabUrl, p.GitlabProject, p.GitlabToken = glURL.String, glProj.String, glTok.String
	p.SpecFramework = models.NormalizeSpecFramework(specFw.String)
	p.OwnerUserID = strings.TrimSpace(ownerUserID.String)
	return &p, nil
}

// CreateProject creates a project nobody signed for: its background
// synchronisation keeps the server credential, which is what an unattended
// deployment has.
func (d *DB) CreateProject(req models.CreateProjectRequest) (*models.Project, error) {
	return d.CreateProjectAs("", req)
}

// CreateProjectAs records who created the project as its owner. On a tracker
// whose credential is personal that is not decoration: the background
// synchronisation has no acting user of its own and borrows the owner's token,
// so a project created by nobody can only reach a tracker the server itself is
// configured for.
func (d *DB) CreateProjectAs(ownerUserID string, req models.CreateProjectRequest) (*models.Project, error) {
	d.mu.Lock()

	name := strings.TrimSpace(req.Name)
	if name == "" {
		d.mu.Unlock()
		return nil, fmt.Errorf("nom du projet obligatoire")
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = strings.ToLower(name)
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "_", "-")
		slug = strings.ReplaceAll(slug, "'", "-")
	}

	id := uuid.New().String()
	icon := req.Icon
	if icon == "" {
		icon = "Folder"
	}
	color := req.Color
	if color == "" {
		color = "indigo"
	}
	issueTracker := req.IssueTracker
	if issueTracker == "" {
		issueTracker = "local"
	}

	gitRemote := strings.TrimSpace(req.GitRemoteUrl)
	githubRepo := strings.TrimSpace(req.GithubRepo)
	if githubRepo == "" && gitRemote != "" {
		githubRepo = ParseGitRepoFromURL(gitRemote)
	}

	// A Jira project key is always uppercase (PE, ENG, OPS…). Fall back to the
	// project slug so an existing Jira-tracked project keeps working.
	jiraProject := strings.ToUpper(strings.TrimSpace(req.JiraProject))
	if jiraProject == "" && issueTracker == "jira" {
		jiraProject = strings.ToUpper(strings.ReplaceAll(slug, "-", ""))
	}

	aiProvider := strings.TrimSpace(req.AIProvider)
	aiCmd := strings.TrimSpace(req.AICommandTemplate)
	aiCmdAutonomous := strings.TrimSpace(req.AICommandTemplateAutonomous)
	specFramework := models.NormalizeSpecFramework(req.SpecFramework)

	now := time.Now()
	isDefInt := 0
	if req.IsDefault {
		isDefInt = 1
	}

	skillOverrides := req.SkillOverrides
	if skillOverrides == nil {
		skillOverrides = map[string]string{}
	}
	skillOverridesBytes, _ := json.Marshal(skillOverrides)
	setupProvidersBytes, _ := json.Marshal(models.NormalizeSetupProviders(req.SetupProviders))
	aiModel := strings.TrimSpace(req.AIModel)
	aiSkillModelsBytes, _ := json.Marshal(normalizeSkillModels(req.AISkillModels))

	// Types importés : vides à la création, ce qui vaut « les types par défaut ».
	// Les réglages du projet les nomment ensuite, à partir des types réels du
	// tracker.
	issueTypes := models.NormalizeIssueTypes(req.IssueTypes)
	if len(req.IssueTypes) == 0 {
		issueTypes = []string{}
	}
	issueTypesBytes, _ := json.Marshal(issueTypes)

	// Vues optionnelles : aucune à la création. Triage, Roadmap et Timeline
	// s'activent depuis les réglages du projet, une fois qu'il en a l'usage.
	enabledViewsBytes, _ := json.Marshal(models.NormalizeEnabledViews(req.EnabledViews))

	// Roadmap projects are read sources only; the project's own key is always
	// read and never listed.
	roadmapProjectsBytes, _ := json.Marshal(NormalizeRoadmapProjects(req.RoadmapProjects, jiraProject))

	// Couleur par épic : désactivée tant que le projet ne la demande pas.
	epicColorsInt := 0
	if req.EpicColors {
		epicColorsInt = 1
	}

	// Mono-dépôt par défaut : c'est le cas courant, et le comportement d'avant.
	monoRepoInt := 1
	if req.MonoRepo != nil && !*req.MonoRepo {
		monoRepoInt = 0
	}

	repoPathsBytes, _ := json.Marshal(normalizeRepoPaths(req.RepoPaths))

	// Worktrees stay on unless the project explicitly opts out, which keeps the
	// behaviour projects had before the option existed.
	useWorktreesInt := 1
	if req.UseWorktrees != nil && !*req.UseWorktrees {
		useWorktreesInt = 0
	}

	autoSyncEnabledInt := 0
	if req.AutoSyncEnabled != nil && *req.AutoSyncEnabled {
		autoSyncEnabledInt = 1
	}
	autoSyncIntervalMin := 5
	if req.AutoSyncIntervalMin != nil {
		autoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(*req.AutoSyncIntervalMin)
	}
	ttyMode := strings.TrimSpace(req.TtyMode)
	if ttyMode == "" {
		ttyMode = "integrated"
	}
	extTermCmd := strings.TrimSpace(req.ExternalTerminalCommand)

	prCreationStage := req.PRCreationStage
	if prCreationStage == "" {
		prCreationStage = "implemented"
	}
	if prCreationStage != "implemented" && prCreationStage != "specified" {
		d.mu.Unlock()
		return nil, fmt.Errorf("prCreationStage must be specified or implemented")
	}

	// The previous default is cleared in the same transaction as the insert,
	// under the default-project lock, so there is never zero or two defaults.
	err := d.conn.WithTx(func(tx *sqlTx) error {
		if req.IsDefault {
			if err := d.lockDefaultProjectUnsafe(tx); err != nil {
				return err
			}
			if _, err := tx.Exec("UPDATE projects SET is_default = 0"); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`
		INSERT INTO projects (id, name, slug, description, icon, color, repo_path, repo_paths, use_worktrees, default_skill_mode, full_chain_stop_stage, pr_creation_stage, board_id, tracker_columns, stage_columns, sprints, issue_types, enabled_views, epic_colors, spec_repo_path, roadmap_projects, mono_repo, git_remote_url, github_repo, github_api_url, github_token, gitlab_url, gitlab_project, gitlab_token, jira_project, issue_tracker, tracker_url, is_default, skill_overrides, setup_providers, ai_provider, ai_command_template, ai_command_template_autonomous, ai_model, ai_skill_models, spec_framework, auto_sync_enabled, auto_sync_interval_min, tty_mode, external_terminal_command, owner_user_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, slug, req.Description, icon, color, req.RepoPath, string(repoPathsBytes), useWorktreesInt, models.NormalizeSkillMode(req.DefaultSkillMode), models.NormalizeFullChainStopStage(req.FullChainStopStage), prCreationStage, req.BoardID, "[]", "{}", "[]", string(issueTypesBytes), string(enabledViewsBytes), epicColorsInt, strings.TrimSpace(req.SpecRepoPath), string(roadmapProjectsBytes), monoRepoInt, gitRemote, githubRepo, strings.TrimSpace(req.GithubApiUrl), strings.TrimSpace(req.GithubToken), strings.TrimSpace(req.GitlabUrl), strings.TrimSpace(req.GitlabProject), strings.TrimSpace(req.GitlabToken), jiraProject, issueTracker, req.TrackerUrl, isDefInt, string(skillOverridesBytes), string(setupProvidersBytes), aiProvider, aiCmd, aiCmdAutonomous, aiModel, string(aiSkillModelsBytes), specFramework, autoSyncEnabledInt, autoSyncIntervalMin, ttyMode, extTermCmd, strings.TrimSpace(ownerUserID), now, now)
		return err
	})
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}

	project, err := d.getProjectByIDUnsafe(id)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return withoutProjectTokens(project), nil
}

// UpdateProject saves a project without naming who saved it, so a project that
// predates the owner column keeps having none.
func (d *DB) UpdateProject(id string, req models.UpdateProjectRequest) (*models.Project, error) {
	return d.UpdateProjectAs("", id, req)
}

// UpdateProjectAs saves it on behalf of whoever asked, and lets an ownerless
// project adopt them. Every project created before the column has no owner, so
// without that adoption their background synchronisation would stay on the
// server credential for good: on a Jira deployment holding only personal
// tokens, that is one failed activity per unfinished work item per pass. An
// owner already recorded is never replaced: saving somebody else's project
// would otherwise hand its synchronisation to the last person who touched it.
func (d *DB) UpdateProjectAs(actingUserID string, id string, req models.UpdateProjectRequest) (*models.Project, error) {
	d.mu.Lock()

	// The project row is locked before it is read, so an edit racing on another
	// server instance waits and this one merges into what it committed, instead
	// of writing back every field from an older snapshot. A change of default
	// takes the default-project lock first. The plain read below then sees the
	// latest committed row, which nobody else can change until the commit.
	tx, err := d.conn.Begin()
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if req.IsDefault != nil {
		if err := d.lockDefaultProjectUnsafe(tx); err != nil {
			d.mu.Unlock()
			return nil, err
		}
	}
	var lockedID string
	if err := tx.QueryRow("SELECT id FROM projects WHERE id = ? OR slug = ?"+d.forUpdate(), id, id).Scan(&lockedID); err != nil && err != sql.ErrNoRows {
		d.mu.Unlock()
		return nil, err
	}

	p, err := d.getProjectByIDUnsafe(id)
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	if p == nil {
		d.mu.Unlock()
		return nil, fmt.Errorf("projet non trouvé")
	}
	if strings.TrimSpace(p.OwnerUserID) == "" {
		p.OwnerUserID = strings.TrimSpace(actingUserID)
	}

	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		p.Name = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		p.Slug = strings.TrimSpace(*req.Slug)
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.Icon != nil && *req.Icon != "" {
		p.Icon = *req.Icon
	}
	if req.Color != nil && *req.Color != "" {
		p.Color = *req.Color
	}
	if req.RepoPath != nil {
		p.RepoPath = *req.RepoPath
	}
	if req.GitRemoteUrl != nil {
		p.GitRemoteUrl = strings.TrimSpace(*req.GitRemoteUrl)
		if p.GithubRepo == "" && p.GitRemoteUrl != "" {
			p.GithubRepo = ParseGitRepoFromURL(p.GitRemoteUrl)
		}
	}
	if req.GithubRepo != nil {
		p.GithubRepo = *req.GithubRepo
	}
	if req.JiraProject != nil {
		p.JiraProject = strings.ToUpper(strings.TrimSpace(*req.JiraProject))
	}
	if req.IssueTracker != nil {
		p.IssueTracker = *req.IssueTracker
	}
	if req.TrackerUrl != nil {
		p.TrackerUrl = *req.TrackerUrl
	}
	if req.SkillOverrides != nil {
		p.SkillOverrides = *req.SkillOverrides
	}
	if req.AIModel != nil {
		p.AIModel = strings.TrimSpace(*req.AIModel)
	}
	if req.AISkillModels != nil {
		p.AISkillModels = normalizeSkillModels(*req.AISkillModels)
	}
	if req.SetupProviders != nil {
		p.SetupProviders = models.NormalizeSetupProviders(*req.SetupProviders)
	}
	if req.RepoPaths != nil {
		p.RepoPaths = normalizeRepoPaths(*req.RepoPaths)
	}
	if req.PRCreationStage != nil {
		if *req.PRCreationStage != "specified" && *req.PRCreationStage != "implemented" {
			d.mu.Unlock()
			return nil, fmt.Errorf("prCreationStage must be specified or implemented")
		}
		p.PRCreationStage = *req.PRCreationStage
	}
	if req.UseWorktrees != nil {
		p.UseWorktrees = *req.UseWorktrees
	}
	if req.DefaultSkillMode != nil {
		p.DefaultSkillMode = models.NormalizeSkillMode(*req.DefaultSkillMode)
	}
	if req.FullChainStopStage != nil {
		p.FullChainStopStage = models.NormalizeFullChainStopStage(*req.FullChainStopStage)
	}
	if req.BoardID != nil {
		p.BoardID = strings.TrimSpace(*req.BoardID)
	}
	if req.TrackerColumns != nil {
		p.TrackerColumns = *req.TrackerColumns
	}
	if req.StageColumns != nil {
		p.StageColumns = *req.StageColumns
	}
	if req.Sprints != nil {
		p.Sprints = *req.Sprints
	}
	if req.IssueTypes != nil {
		p.IssueTypes = models.NormalizeIssueTypes(*req.IssueTypes)
	}
	if req.EnabledViews != nil {
		p.EnabledViews = models.NormalizeEnabledViews(*req.EnabledViews)
	}
	if req.EpicColors != nil {
		p.EpicColors = *req.EpicColors
	}
	if req.SpecRepoPath != nil {
		p.SpecRepoPath = strings.TrimSpace(*req.SpecRepoPath)
	}
	if req.RoadmapProjects != nil {
		p.RoadmapProjects = *req.RoadmapProjects
	}
	if req.MonoRepo != nil {
		p.MonoRepo = *req.MonoRepo
	}
	if req.GithubApiUrl != nil {
		p.GithubApiUrl = strings.TrimSpace(*req.GithubApiUrl)
	}
	if req.GitlabUrl != nil {
		p.GitlabUrl = strings.TrimSpace(*req.GitlabUrl)
	}
	if req.GitlabProject != nil {
		p.GitlabProject = strings.TrimSpace(*req.GitlabProject)
	}
	// The interface never receives a project token back either, so the same
	// empty-means-unchanged rule applies here.
	if req.GithubToken != nil {
		p.GithubToken = keptToken(strings.TrimSpace(*req.GithubToken), p.GithubToken)
	}
	if req.GitlabToken != nil {
		p.GitlabToken = keptToken(strings.TrimSpace(*req.GitlabToken), p.GitlabToken)
	}
	if req.AIProvider != nil {
		p.AIProvider = *req.AIProvider
	}
	// Both commands follow the same rule: absent keeps the stored value, empty
	// clears it. They are trimmed as CreateProject trims them.
	if req.AICommandTemplate != nil {
		p.AICommandTemplate = strings.TrimSpace(*req.AICommandTemplate)
	}
	if req.AICommandTemplateAutonomous != nil {
		p.AICommandTemplateAutonomous = strings.TrimSpace(*req.AICommandTemplateAutonomous)
	}
	if req.SpecFramework != nil {
		p.SpecFramework = models.NormalizeSpecFramework(*req.SpecFramework)
	}
	if req.AutoSyncEnabled != nil {
		p.AutoSyncEnabled = *req.AutoSyncEnabled
	}
	if req.AutoSyncIntervalMin != nil {
		p.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(*req.AutoSyncIntervalMin)
	} else {
		p.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(p.AutoSyncIntervalMin)
	}
	if req.TtyMode != nil && strings.TrimSpace(*req.TtyMode) != "" {
		p.TtyMode = strings.TrimSpace(*req.TtyMode)
	}
	if p.TtyMode == "" {
		p.TtyMode = "integrated"
	}
	if req.ExternalTerminalCommand != nil {
		p.ExternalTerminalCommand = strings.TrimSpace(*req.ExternalTerminalCommand)
	}
	if req.IsDefault != nil {
		p.IsDefault = *req.IsDefault
		if p.IsDefault {
			_, _ = tx.Exec("UPDATE projects SET is_default = 0 WHERE id != ?", p.ID)
		}
	}
	p.UpdatedAt = time.Now()
	isDefInt := 0
	if p.IsDefault {
		isDefInt = 1
	}

	if p.SkillOverrides == nil {
		p.SkillOverrides = map[string]string{}
	}
	skillOverridesBytes, _ := json.Marshal(p.SkillOverrides)
	p.AISkillModels = normalizeSkillModels(p.AISkillModels)
	projSkillModelsBytes, _ := json.Marshal(p.AISkillModels)
	setupProvidersBytes, _ := json.Marshal(models.NormalizeSetupProviders(p.SetupProviders))
	repoPathsBytes, _ := json.Marshal(normalizeRepoPaths(p.RepoPaths))
	useWorktreesInt := 0
	if p.UseWorktrees {
		useWorktreesInt = 1
	}
	if p.TrackerColumns == nil {
		p.TrackerColumns = []models.TrackerColumn{}
	}
	if p.StageColumns == nil {
		p.StageColumns = map[string][]string{}
	}
	trackerColumnsBytes, _ := json.Marshal(p.TrackerColumns)
	stageColumnsBytes, _ := json.Marshal(p.StageColumns)
	if p.Sprints == nil {
		p.Sprints = []models.TrackerSprint{}
	}
	sprintsBytes, _ := json.Marshal(p.Sprints)
	if p.IssueTypes == nil {
		p.IssueTypes = []string{}
	}
	issueTypesBytes, _ := json.Marshal(p.IssueTypes)
	enabledViewsBytes, _ := json.Marshal(models.NormalizeEnabledViews(p.EnabledViews))
	p.RoadmapProjects = NormalizeRoadmapProjects(p.RoadmapProjects, p.JiraProject)
	roadmapProjectsBytes, _ := json.Marshal(p.RoadmapProjects)
	epicColorsInt := 0
	if p.EpicColors {
		epicColorsInt = 1
	}
	monoRepoInt := 0
	if p.MonoRepo {
		monoRepoInt = 1
	}
	autoSyncEnabledInt := 0
	if p.AutoSyncEnabled {
		autoSyncEnabledInt = 1
	}

	_, err = tx.Exec(`
		UPDATE projects
		SET name = ?, slug = ?, description = ?, icon = ?, color = ?, repo_path = ?, repo_paths = ?, use_worktrees = ?, default_skill_mode = ?, full_chain_stop_stage = ?, pr_creation_stage = ?, board_id = ?, tracker_columns = ?, stage_columns = ?, sprints = ?, issue_types = ?, enabled_views = ?, epic_colors = ?, spec_repo_path = ?, roadmap_projects = ?, mono_repo = ?, git_remote_url = ?, github_repo = ?, github_api_url = ?, github_token = ?, gitlab_url = ?, gitlab_project = ?, gitlab_token = ?, jira_project = ?, issue_tracker = ?, tracker_url = ?, is_default = ?, skill_overrides = ?, setup_providers = ?, ai_provider = ?, ai_command_template = ?, ai_command_template_autonomous = ?, ai_model = ?, ai_skill_models = ?, spec_framework = ?, auto_sync_enabled = ?, auto_sync_interval_min = ?, tty_mode = ?, external_terminal_command = ?, owner_user_id = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Slug, p.Description, p.Icon, p.Color, p.RepoPath, string(repoPathsBytes), useWorktreesInt, p.DefaultSkillMode, p.FullChainStopStage, p.PRCreationStage, p.BoardID, string(trackerColumnsBytes), string(stageColumnsBytes), string(sprintsBytes), string(issueTypesBytes), string(enabledViewsBytes), epicColorsInt, strings.TrimSpace(p.SpecRepoPath), string(roadmapProjectsBytes), monoRepoInt, p.GitRemoteUrl, p.GithubRepo, p.GithubApiUrl, p.GithubToken, p.GitlabUrl, p.GitlabProject, p.GitlabToken, p.JiraProject, p.IssueTracker, p.TrackerUrl, isDefInt, string(skillOverridesBytes), string(setupProvidersBytes), p.AIProvider, p.AICommandTemplate, p.AICommandTemplateAutonomous, p.AIModel, string(projSkillModelsBytes), p.SpecFramework, autoSyncEnabledInt, p.AutoSyncIntervalMin, p.TtyMode, p.ExternalTerminalCommand, strings.TrimSpace(p.OwnerUserID), p.UpdatedAt, p.ID)
	if err == nil {
		err = tx.Commit()
		committed = err == nil
	}
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}

	project, err := d.getProjectByIDUnsafe(p.ID)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	project = withoutProjectTokens(project)

	return project, nil
}

func (d *DB) DeleteProject(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	p, err := d.getProjectByIDUnsafe(id)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("projet non trouvé")
	}

	// Under the default-project lock, the project is checked again and its
	// tasks moved to the default in one transaction: another instance making
	// this project the default meanwhile either commits first, and the delete
	// is refused, or waits and finds it gone.
	err = d.conn.WithTx(func(tx *sqlTx) error {
		if err := d.lockDefaultProjectUnsafe(tx); err != nil {
			return err
		}
		var isDefault int
		if err := tx.QueryRow("SELECT is_default FROM projects WHERE id = ?"+d.forUpdate(), p.ID).Scan(&isDefault); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("projet non trouvé")
			}
			return err
		}
		if isDefault != 0 {
			return fmt.Errorf("impossible de supprimer le projet par défaut")
		}

		// Reassign tasks to default project
		var defaultProjID string
		_ = tx.QueryRow("SELECT id FROM projects WHERE is_default = 1 LIMIT 1").Scan(&defaultProjID)
		if defaultProjID == "" {
			defaultProjID = "default"
		}
		if _, err := tx.Exec("UPDATE tasks SET project_id = ? WHERE project_id = ?", defaultProjID, p.ID); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM user_project_bookmarks WHERE project_id = ?", p.ID); err != nil {
			return err
		}
		// Explicit, like DeleteTask's: the ON DELETE CASCADE on project_id only
		// fires under PostgreSQL, because this package never turns SQLite's
		// foreign keys on.
		if _, err := tx.Exec("DELETE FROM task_activities WHERE project_id = ?", p.ID); err != nil {
			return err
		}
		_, err := tx.Exec("DELETE FROM projects WHERE id = ?", p.ID)
		return err
	})
	if err != nil {
		return err
	}
	// A saved view keeps selecting what is left; reading ignores the project
	// anyway, so a failure here costs a stale id in a row, never a ghost.
	_ = d.removeProjectFromBoardViewsUnsafe(p.ID, p.Slug)
	return nil
}

// -------------------------------------------------------------
// PROJECT SKILLS MANAGEMENT & PROVISIONING
// -------------------------------------------------------------

func (d *DB) GetProjectSkillsStatus(projectID string) (*models.ProjectSkillsStatus, error) {
	var result models.ProjectSkillsStatus
	err := d.callAgent(agentprotocol.Operation{ProjectID: projectID, Action: "skills_status"}, &result)
	return &result, err
}

func (d *DB) InstallProjectSkills(projectID string, overrides ...string) (*models.ProjectSkillsStatus, error) {
	op := agentprotocol.Operation{ProjectID: projectID, Action: "sync_config"}
	if len(overrides) > 0 {
		op.Framework = overrides[0]
	}
	if len(overrides) > 1 {
		op.Provider = overrides[1]
	}
	if len(overrides) > 2 {
		op.AICommandTemplate = overrides[2]
	}
	if err := d.callAgent(op, nil); err != nil {
		return nil, err
	}
	return d.GetProjectSkillsStatus(projectID)
}

func (d *DB) InitProjectGit(projectID string) (*models.ProjectGitInitResult, error) {
	var result models.ProjectGitInitResult
	err := d.callAgent(agentprotocol.Operation{ProjectID: projectID, Action: "init_git"}, &result)
	return &result, err
}

// remoteTrackerStatuses asks the project's tracker for its own statuses, for the
// trackers that have them. It runs before DetectTrackerStatuses takes the read
// lock on purpose: this is an HTTP call, and holding the lock across it would
// stall every writer for as long as the instance takes to answer. A tracker
// without the notion, or an unreachable one, simply adds nothing.
func (d *DB) remoteTrackerStatuses(ctx context.Context, projectID, trackerName string) []models.DetectedStatus {
	if trackerName == "github" || trackerName == "local" || trackerName == "" {
		return nil
	}
	proj, _ := d.GetProjectByID(projectID)
	if proj == nil {
		proj = &models.Project{IssueTracker: trackerName}
	}
	ts, err := d.TrackerForProject(proj)
	if err != nil || !ts.Supports(tracker.CapBoard) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, boardAPITimeout)
	defer cancel()
	statuses, err := ts.ListStatuses(ctx, tracker.ProjectRequest{Project: proj})
	if err != nil {
		log.Printf("[statuses] %s n'a pas répondu ses statuts: %v", trackerName, err)
		return nil
	}
	out := make([]models.DetectedStatus, 0, len(statuses))
	for _, st := range statuses {
		sType := "unstarted"
		switch strings.ToLower(st.Category) {
		case "done":
			sType = "completed"
		case "indeterminate":
			sType = "started"
		case "new":
			sType = "backlog"
		}
		out = append(out, models.DetectedStatus{ID: st.Name, Name: st.Name, Type: sType, Source: trackerName})
	}
	return out
}

func (d *DB) DetectTrackerStatuses(ctx context.Context, projectID, trackerName, githubRepo string) ([]models.DetectedStatus, error) {
	// The project row decides which tracker answers, so the name is resolved
	// first when the caller did not give one.
	if trackerName == "" && projectID != "" && projectID != "detect-statuses" {
		if proj, _ := d.GetProjectByID(projectID); proj != nil {
			trackerName = proj.IssueTracker
		}
	}
	remote := d.remoteTrackerStatuses(ctx, projectID, trackerName)

	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []models.DetectedStatus
	seen := make(map[string]bool)

	addStatus := func(name, sType, color, source string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		lower := strings.ToLower(trimmed)
		if seen[lower] {
			return
		}
		seen[lower] = true
		results = append(results, models.DetectedStatus{
			ID:     trimmed,
			Name:   trimmed,
			Type:   sType,
			Color:  color,
			Source: source,
		})
	}

	// 1. If projectID provided, load project info
	if projectID != "" && projectID != "detect-statuses" {
		if proj, _ := d.getProjectByIDUnsafe(projectID); proj != nil {
			if trackerName == "" {
				trackerName = proj.IssueTracker
			}
			if githubRepo == "" {
				githubRepo = proj.GithubRepo
			}
		}
	}

	// 3. Scan existing tasks in SQLite database for project / tracker
	query := "SELECT DISTINCT status FROM tasks WHERE 1=1"
	var args []interface{}
	if projectID != "" && projectID != "detect-statuses" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	if rows, err := d.conn.Query(query, args...); err == nil {
		for rows.Next() {
			var st string
			if sErr := rows.Scan(&st); sErr == nil && st != "" {
				addStatus(st, "db", "", "db")
			}
		}
		rows.Close()
	}

	// 4. If tracker is github, add github states
	if trackerName == "github" {
		addStatus("open", "unstarted", "#3fb950", "github")
		addStatus("closed", "completed", "#8250df", "github")
	}

	// The statuses the tracker itself named, read before the lock was taken.
	for _, st := range remote {
		addStatus(st.Name, st.Type, "", trackerName)
	}

	// 5. Standard fallback presets if list is short or empty
	for _, def := range []struct {
		name  string
		sType string
		color string
	}{
		{"Backlog", "backlog", "#bec2c8"},
		{"Todo", "unstarted", "#e2e2e2"},
		{"In Progress", "started", "#f2c94c"},
		{"In Review", "started", "#5e6ad2"},
		{"Done", "completed", "#27ae60"},
		{"Canceled", "canceled", "#eb5757"},
		{"to_clarify", "backlog", "#06b6d4"},
		{"clarified", "unstarted", "#f59e0b"},
		{"to_implement", "started", "#3b82f6"},
		{"to_test", "started", "#6366f1"},
		{"to_close", "completed", "#10b981"},
	} {
		addStatus(def.name, def.sType, def.color, "preset")
	}

	return results, nil
}

// applyProjectSettings layers a project's own configuration over the global
// settings for one task, the AI engine included.
//
// Elle est partagée par le worker et par le lancement en session TTY : ce
// dernier lisait les réglages globaux et tentait donc de démarrer « agy » sur un
// projet configuré pour Claude, avec un « binaire agy introuvable » à la clé.
func (d *DB) applyProjectSettings(settings *models.Settings, task *models.Task, skillID string) {
	if settings == nil || task == nil || task.ProjectID == "" {
		return
	}
	proj, _ := d.GetProjectByID(task.ProjectID)
	if proj == nil {
		return
	}

	if proj.RepoPath != "" {
		settings.RepoPath = proj.RepoPath
	}
	if proj.GithubRepo != "" {
		settings.GithubRepo = proj.GithubRepo
	}
	if proj.JiraProject != "" {
		settings.JiraProject = proj.JiraProject
	}
	if proj.TrackerUrl != "" {
		settings.JiraUrl = proj.TrackerUrl
	}
	if proj.IssueTracker != "" {
		settings.IssueTracker = proj.IssueTracker
	}
	if proj.AIProvider != "" {
		settings.AIProvider = proj.AIProvider
	}
	// The two commands override independently: a project that only spells out its
	// headless launch keeps the interactive one it inherits.
	if proj.AICommandTemplate != "" {
		settings.AICommandTemplate = proj.AICommandTemplate
	}
	if proj.AICommandTemplateAutonomous != "" {
		settings.AICommandTemplateAutonomous = proj.AICommandTemplateAutonomous
	}
	aiModels := agentconfig.MergeModels(
		agentconfig.ModelConfig{Model: proj.AIModel, SkillModels: proj.AISkillModels},
		agentconfig.ModelConfig{Model: settings.AIModel, SkillModels: settings.AISkillModels},
	)
	settings.AIModel, settings.AISkillModels = aiModels.Model, aiModels.SkillModels
	if proj.SpecFramework != "" {
		settings.SpecFramework = proj.SpecFramework
	}

	// A project may point a workflow stage at a different skill than the
	// scaffolded default (for instance /clarify-workitem instead of
	// /clarify-issue). The executed slash command has to follow the override,
	// otherwise the board shows one command and runs another.
	if models.NormalizeSkillID(skillID) == "adjust" {
		if origin, _ := adjustmentOverrideOrigin(d.projectSkillOverrides(task.ProjectID)); origin == "adjust" {
			// Reconciled project instructions supersede the retained legacy global prompt at execution only.
			settings.PromptCreatePR = ""
		}
	}
	applySkillCommandOverride(settings, proj, skillID)
}

// ProjectSkillCommand returns the slash command of a workflow skill for a
// project: the project's override when it set one, the unified default
// otherwise.
func (d *DB) ProjectSkillCommand(task *models.Task, skillID string) string {
	dirName := models.SkillDirNames[skillID]
	if task != nil && task.ProjectID != "" {
		if proj, _ := d.GetProjectByID(task.ProjectID); proj != nil && proj.SkillOverrides != nil {
			if override := strings.TrimSpace(proj.SkillOverrides[skillID]); override != "" {
				dirName = strings.TrimPrefix(override, "/")
			}
		}
	}
	if dirName == "" {
		dirName = skillID
	}
	return "/" + dirName
}
