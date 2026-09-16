package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

type SkillJob struct {
	ActivityID    string
	TaskID        string
	ProjectID     string
	SkillID       string
	Prompt        string
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
	// AutoChain enchaîne le pas suivant du workflow à la fin de celui-ci, jusqu'à
	// l'étape de revue. Porté par le job et non par l'interface : la chaîne doit
	// survivre à la fermeture de l'onglet.
	AutoChain bool
	// Op porte l'écriture tracker à effectuer quand SkillID vaut "tracker_op" :
	// assignation, rattachement à un épic, découpe d'épic, labels d'horizon.
	Op *TrackerOp
}

// ProjectLimiter limits concurrency of background AI agent skill workers per project (1 to models.MaxParallelism).
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
		if limit > models.MaxParallelism {
			limit = models.MaxParallelism
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
	agentOperations  AgentOperations
	trackers         *trackerapi.Client
	trackerRegistry  *tracker.Registry
	prEvidenceLookup func(string, string) (trackerapi.PullRequest, error)
	conn             *sql.DB
	mu               sync.RWMutex
	jobQueue         chan SkillJob
	limiter          *ProjectLimiter
	// auto porte l'état de la boucle de synchronisation de fond.
	auto              *autoSync
	cancelMap         map[string]context.CancelFunc
	cancelMu          sync.Mutex
	postBackListeners []PostBackListener
	postBackMu        sync.RWMutex
}

func NewDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)

	trackerClient := trackerapi.NewClient()
	db := &DB{
		conn:            conn,
		trackers:        trackerClient,
		trackerRegistry: trackerapi.NewDefaultRegistry(trackerClient),
		jobQueue:        make(chan SkillJob, 100),
		limiter:         newProjectLimiter(),
		cancelMap:       make(map[string]context.CancelFunc),
	}
	if err := db.initIdentitySchema(); err != nil {
		return nil, err
	}
	if err := db.initSessionSchema(); err != nil {
		return nil, err
	}
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Start background queue worker
	go db.startQueueWorker()

	if err := db.seedIfEmpty(); err != nil {
		log.Printf("Warning: error seeding default data: %v", err)
	}

	return db, nil
}

func (d *DB) Close() error {
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
			repo_path TEXT NOT NULL DEFAULT '.',
			issue_tracker TEXT NOT NULL DEFAULT 'local',
			github_repo TEXT NOT NULL DEFAULT '',
			prompt_clarify TEXT NOT NULL DEFAULT '',
			prompt_specify TEXT NOT NULL DEFAULT '',
			prompt_implement TEXT NOT NULL DEFAULT '',
			prompt_create_pr TEXT NOT NULL DEFAULT '',
			prompt_pick TEXT NOT NULL DEFAULT '',
			editor_command TEXT NOT NULL DEFAULT 'code',
			ui_scale INTEGER NOT NULL DEFAULT 100,
			auto_sync_enabled INTEGER NOT NULL DEFAULT 0,
			auto_sync_interval_sec INTEGER NOT NULL DEFAULT 60,
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
			board_id TEXT NOT NULL DEFAULT '',
			tracker_columns TEXT NOT NULL DEFAULT '[]',
			sprints TEXT NOT NULL DEFAULT '[]',
			issue_types TEXT NOT NULL DEFAULT '[]',
			mono_repo INTEGER NOT NULL DEFAULT 1,
			stage_columns TEXT NOT NULL DEFAULT '{}',
			github_repo TEXT NOT NULL DEFAULT '',
			issue_tracker TEXT NOT NULL DEFAULT 'local',
			is_default INTEGER NOT NULL DEFAULT 0,
			stage_mapping TEXT NOT NULL DEFAULT '{}',
			parallelism INTEGER NOT NULL DEFAULT 1,
			auto_sync_enabled INTEGER NOT NULL DEFAULT 0,
			auto_sync_interval_min INTEGER NOT NULL DEFAULT 5,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
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
			UNIQUE(project_id, key)
		);`,
		`CREATE TABLE IF NOT EXISTS task_activities (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			skill_id TEXT NOT NULL,
			skill_name TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'completed',
			summary TEXT NOT NULL DEFAULT '',
			output TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '[]',
			prompt TEXT NOT NULL DEFAULT '',
			started_at DATETIME,
			completed_at DATETIME,
			error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_position ON tasks(status, position);`,
		`CREATE INDEX IF NOT EXISTS idx_activities_task ON task_activities(task_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_activities_status ON task_activities(status);`,
		`CREATE INDEX IF NOT EXISTS idx_activities_created ON task_activities(created_at DESC);`,
	}

	for _, query := range queries {
		if _, err := d.conn.Exec(query); err != nil {
			return err
		}
	}

	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN stage_mapping TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN git_remote_url TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN tracker_url TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN skill_overrides TEXT NOT NULL DEFAULT '{}';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN setup_providers TEXT NOT NULL DEFAULT '[]';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN repo_paths TEXT NOT NULL DEFAULT '[]';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN pr_creation_stage TEXT NOT NULL DEFAULT 'implemented';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN use_worktrees INTEGER NOT NULL DEFAULT 1;")
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
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN spec_framework TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN jira_project TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN parallelism INTEGER NOT NULL DEFAULT 1;")
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
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN prompt TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN started_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN completed_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN error TEXT NOT NULL DEFAULT '';")
	// Remote invocations outlive the server process and report their own outcome.
	_, _ = d.conn.Exec("UPDATE task_activities SET status = 'failed', error = 'Interrupted by server restart' WHERE status IN ('running', 'queued', 'pending') AND skill_id != 'remote_run';")

	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN detail_mode TEXT NOT NULL DEFAULT 'panel';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_provider TEXT NOT NULL DEFAULT 'agy';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_command_template TEXT NOT NULL DEFAULT 'agy -p \"{prompt}\"';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN repo_path TEXT NOT NULL DEFAULT '.';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN issue_tracker TEXT NOT NULL DEFAULT 'local';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN github_repo TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_clarify TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_specify TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_implement TEXT NOT NULL DEFAULT '';")
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
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN auto_sync_enabled INTEGER NOT NULL DEFAULT 0;")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN auto_sync_interval_min INTEGER NOT NULL DEFAULT 5;")

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
				INSERT INTO tasks (id, project_id, key, title, description, status, priority, labels, pinned, assignee, assignee_avatar, position, due_date, source, external_url, issue_type, parent_key, parent_title, parent_type, sprint, team, team_id, tracker_status, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, newID, projID, t.Key, t.Title, t.Description, string(t.Status), string(t.Priority), string(labelsJSON), pinnedVal, t.Assignee, t.AssigneeAvatar, t.Position, t.DueDate, src, t.ExternalURL, t.IssueType, t.ParentKey, t.ParentTitle, t.ParentType, t.Sprint, t.Team, t.TeamID, t.TrackerStatus, t.TrackerCreatedAt, t.TrackerUpdatedAt, t.StatusChangedAt, t.CreatedAt, now); insErr != nil {
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

// GetTaskFacets returns the sprints and teams found on the tasks of a project,
// or of the whole board when projectID is empty. The values must come from a
// dedicated query rather than from the filtered task list, otherwise selecting
// a sprint would empty the very dropdown it was picked from.
func (d *DB) GetTaskFacets(projectID string) (*TaskFacets, error) {
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

	for _, column := range []string{"sprint", "team"} {
		query := fmt.Sprintf("SELECT DISTINCT %s FROM tasks WHERE %s != ''", column, column)
		args := []interface{}{}
		if projectID != "" {
			query += " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
			args = append(args, projectID, projectID, projectID)
		}
		query += fmt.Sprintf(" ORDER BY %s DESC", column)

		rows, err := d.conn.Query(query, args...)
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
	assigneeQuery := "SELECT assignee, COUNT(*) FROM tasks WHERE TRIM(assignee) != ''"
	unassignedQuery := "SELECT COUNT(*) FROM tasks WHERE TRIM(assignee) = ''"
	scopeArgs := []interface{}{}
	if projectID != "" {
		scope := " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
		assigneeQuery += scope
		unassignedQuery += scope
		scopeArgs = append(scopeArgs, projectID, projectID, projectID)
	}
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
	macroQuery := "SELECT parent_key, parent_title, COUNT(*) FROM tasks WHERE TRIM(parent_key) != '' OR TRIM(parent_title) != ''"
	noMacroQuery := "SELECT COUNT(*) FROM tasks WHERE TRIM(parent_key) = '' AND TRIM(parent_title) = ''"
	if projectID != "" {
		scope := " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
		macroQuery += scope
		noMacroQuery += scope
	}
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
	if projectID != "" {
		macroTableQuery += " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
		macroArgs = append(macroArgs, projectID, projectID, projectID)
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

	statusQuery := "SELECT tracker_status, COUNT(*) FROM tasks WHERE TRIM(tracker_status) != ''"
	if projectID != "" {
		statusQuery += " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
	}
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
		countQuery := fmt.Sprintf("SELECT %s, COUNT(*) FROM tasks WHERE TRIM(%s) != ''", column, column)
		if projectID != "" {
			countQuery += " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
		}
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
	labelQuery := "SELECT labels FROM tasks WHERE labels != '' AND labels != '[]'"
	if projectID != "" {
		labelQuery += " AND (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
	}
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
	if projectID != "" {
		totalQuery += " WHERE (project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))"
	}
	_ = d.conn.QueryRow(totalQuery, scopeArgs...).Scan(&facets.Total)

	return facets, nil
}

// GetTasks lists the tasks matching the filters. pinnedOnly restricts to the
// pinned tickets, which is the fastest way back to the two or three chantiers in
// flight when the board carries three hundred.
func (d *DB) GetTasks(query, status, priority, label, projectID, sprint, team, assignee, macro string, trackerStatuses, issueTypes []string, pinnedOnly bool) ([]models.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var conditions []string
	var args []interface{}

	if projectID != "" && projectID != "all" {
		conditions = append(conditions, "(project_id = ? OR project_id = (SELECT slug FROM projects WHERE id = ?) OR project_id = (SELECT id FROM projects WHERE slug = ?))")
		args = append(args, projectID, projectID, projectID)
	}

	if pinnedOnly {
		conditions = append(conditions, "pinned = 1")
	}

	if query != "" {
		// Le parent compte dans la recherche : chercher une clé d'épic ou son
		// titre doit ramener ses enfants, c'est la façon naturelle d'isoler un
		// chantier alors qu'aucun ticket ne porte l'épic dans son propre titre.
		conditions = append(conditions, "(key LIKE ? OR title LIKE ? OR description LIKE ? OR labels LIKE ? OR assignee LIKE ? OR parent_key LIKE ? OR parent_title LIKE ?)")
		pattern := "%" + query + "%"
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
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

	sqlQuery := "SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at FROM tasks"
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
			&t.Position,
			&dueDate,
			&branchName,
			&prURL,
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
		if source.Valid && source.String != "" {
			t.Source = source.String
		} else if strings.HasPrefix(t.Key, "#") || strings.HasPrefix(t.Key, "GH-#") || strings.HasPrefix(t.Key, "gh-") {
			t.Source = "github"
		} else {
			t.Source = "local"
		}

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
	var trackerCreatedAt, trackerUpdatedAt, statusChangedAt sql.NullTime
	var statusStr, priorityStr string

	err := d.conn.QueryRow(`
		SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
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
		&t.Position,
		&dueDate,
		&branchName,
		&prURL,
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
			SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
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
			&t.Position,
			&dueDate,
			&branchName,
			&prURL,
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
	if source.Valid && source.String != "" {
		t.Source = source.String
	} else if strings.HasPrefix(t.Key, "#") || strings.HasPrefix(t.Key, "GH-#") || strings.HasPrefix(t.Key, "gh-") {
		t.Source = "github"
	} else {
		t.Source = "local"
	}

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

// SanitizeBranchName removes characters illegal in git branch names.
func SanitizeBranchName(branch string) string {
	branch = strings.TrimSpace(branch)
	var b strings.Builder
	for _, r := range branch {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '/' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	res := b.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	res = strings.Trim(res, "-")
	return res
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

func (d *DB) getNextTaskKey(projectID string, prefix string) (string, error) {
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

	rows, err := d.conn.Query(query, args...)
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

// Workflow labels following the AI lifecycle:
// new -> clarified -> specified -> implemented -> reviewed -> finished
var WorkflowLabels = []string{"new", "clarified", "specified", "implemented", "reviewed", "finished", "untouched", "New", "Clarified", "Specified", "Implemented", "Reviewed", "Finished", "Untouched"}

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
		if strings.HasPrefix(targetLabel, "#") {
			result = append(result, "#"+cleanTarget)
		} else {
			result = append(result, cleanTarget)
		}
	}
	return result
}

func (d *DB) CreateTask(req models.CreateTaskRequest) (*models.Task, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

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
	req.Labels = SetWorkflowLabel(req.Labels, "new")

	var key string
	var extURL *string
	var id string = uuid.New().String()
	now := time.Now()

	if req.Priority == "" {
		req.Priority = models.PriorityMedium
	}

	// Remote creation requires confirmation from the server HTTP adapter.
	ts, tsErr := d.TrackerForProject(proj)
	if tsErr == nil && ts != nil && req.Source != "local" && ts.Name() != "local" && ts.Supports(tracker.CapCreate) {
		created, err := ts.CreateIssue(context.Background(), tracker.CreateIssueRequest{
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
			extURL = created.ExternalURL
		} else {
			return nil, fmt.Errorf("%s issue creation failed: empty response", ts.Name())
		}
	} else {
		// Local project tracker
		key, _ = d.getNextTaskKey(projID, prefix)
	}

	if req.ExternalURL != nil && *req.ExternalURL != "" {
		extURL = req.ExternalURL
	} else if extURL == nil && req.Source == "github" && githubRepo != "" && strings.HasPrefix(key, "#") {
		cleanNum := strings.TrimPrefix(key, "#")
		url := fmt.Sprintf("https://github.com/%s/issues/%s", models.CleanGithubRepo(githubRepo), cleanNum)
		extURL = &url
	} else if extURL == nil && req.Source == "jira" && jiraUrl != "" {
		url := fmt.Sprintf("%s/browse/%s", strings.TrimSuffix(jiraUrl, "/"), key)
		extURL = &url
	}

	var maxPos int
	_ = d.conn.QueryRow("SELECT COALESCE(MAX(position), -1) FROM tasks WHERE status = ?", req.Status).Scan(&maxPos)
	newPos := maxPos + 1

	labelsJSON, _ := json.Marshal(req.Labels)
	if req.Labels == nil {
		labelsJSON = []byte("[]")
	}

	isPinned := HasPinnedLabel(req.Labels)
	pinnedVal := 0
	if isPinned {
		pinnedVal = 1
		_, _ = d.conn.Exec(`
			INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
			ON CONFLICT(task_id) DO UPDATE SET pinned_at = excluded.pinned_at
		`, id, now.Format(time.RFC3339))
	}

	issueType := strings.TrimSpace(req.IssueType)
	parentKey := req.ParentKey
	parentTitle := req.ParentTitle
	parentType := req.ParentType

	_, err := d.conn.Exec(`
		INSERT INTO tasks (id, project_id, key, title, description, status, priority, labels, pinned, assignee, assignee_avatar, position, due_date, source, external_url, issue_type, parent_key, parent_title, parent_type, sprint, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, projID, key, req.Title, req.Description, string(req.Status), string(req.Priority), string(labelsJSON), pinnedVal, req.Assignee, req.AssigneeAvatar, newPos, req.DueDate, req.Source, extURL, issueType, parentKey, parentTitle, parentType, strings.TrimSpace(req.Sprint), now, now)

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
	labels = SetWorkflowLabel(labels, "new")

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

func (d *DB) UpdateTask(id string, req models.UpdateTaskRequest) (*models.Task, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	existing, err := d.getTaskByIDUnsafe(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("task not found")
	}

	oldLabels := existing.Labels
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
			existing.Labels = SetWorkflowLabel(existing.Labels, newStage)
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
	if req.PrURL != nil {
		existing.PrURL = req.PrURL
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
			// instead of retyping the path.
			d.registerProjectRepoPathUnsafe(existing.ProjectID, trimmed)
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
		_, _ = d.conn.Exec(`
			INSERT INTO pinned_tasks (task_id, pinned_at) VALUES (?, ?)
			ON CONFLICT(task_id) DO NOTHING
		`, existing.ID, existing.UpdatedAt.Format(time.RFC3339))
	} else {
		_, _ = d.conn.Exec(`DELETE FROM pinned_tasks WHERE task_id = ? OR task_id = ?`, existing.ID, existing.Key)
	}
	existing.Pinned = isPinned

	labelsJSON, _ := json.Marshal(existing.Labels)

	_, err = d.conn.Exec(`
		UPDATE tasks
		SET project_id = ?, title = ?, description = ?, status = ?, priority = ?, labels = ?, pinned = ?, assignee = ?, assignee_avatar = ?, position = ?, due_date = ?, branch_name = ?, pr_url = ?, repo_path = ?, tracker_status = ?, source = ?, external_url = ?, issue_type = ?, sprint = ?, updated_at = ?
		WHERE id = ?
	`, existing.ProjectID, existing.Title, existing.Description, string(existing.Status), string(existing.Priority), string(labelsJSON), pinnedVal, existing.Assignee, existing.AssigneeAvatar, existing.Position, existing.DueDate, existing.BranchName, existing.PrURL, repoPathValue(existing.RepoPath), existing.TrackerStatus, existing.Source, existing.ExternalURL, existing.IssueType, existing.Sprint, existing.UpdatedAt, existing.ID)

	if err != nil {
		return nil, err
	}

	// Enqueue async CLI tracker sync in task activities queue whenever task is modified
	if req.Status != nil || req.Labels != nil || req.Title != nil || req.Description != nil || req.Priority != nil || req.TrackerStatus != nil {
		d.enqueueTrackerUpdateUnsafe(existing, req.Status, existing.Labels, removedLabels, TrackerFieldChanges{
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
		if _, opErr := d.enqueueTrackerOpUnsafe(TrackerOp{
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

	// L'assignation ne peut pas voyager avec la synchro des champs : acli n'a pas
	// de --assignee, et Jira n'assigne que par identifiant de compte. Elle part
	// donc comme écriture dédiée, dans la même file d'activités.
	if newAssignee := strings.TrimSpace(existing.Assignee); newAssignee != oldAssignee && existing.Source == "jira" {
		accountID := ""
		if req.AssigneeAccountID != nil {
			accountID = strings.TrimSpace(*req.AssigneeAccountID)
		}
		if _, opErr := d.enqueueTrackerOpUnsafe(TrackerOp{
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
	var detMode, aiProv, aiCmd, repoP, issTrk, ghRepo, jiraProj, jiraUrl, jiraMail, jiraTok, pClar, pSpec, pImpl, pPR, pPick, editCmd, specFw sql.NullString
	var uiScale sql.NullInt64
	var autoSyncEnabled, autoSyncInterval sql.NullInt64

	err := d.conn.QueryRow(`
		SELECT id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar,
		       ai_provider, ai_command_template, repo_path, issue_tracker, github_repo, jira_project, jira_url, jira_email, jira_api_token,
		       prompt_clarify, prompt_specify, prompt_implement, prompt_create_pr, prompt_pick, editor_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, updated_at
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
		&repoP,
		&issTrk,
		&ghRepo,
		&jiraProj,
		&jiraUrl,
		&jiraMail,
		&jiraTok,
		&pClar,
		&pSpec,
		&pImpl,
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

	now := time.Now()
	_, _ = d.conn.Exec(`
		UPDATE tasks
		SET position = position + 1
		WHERE status = ? AND position >= ? AND id != ?
	`, string(newStatus), newPosition, id)

	oldStage := GetStageLabelForStatus(existing.Status)
	newStage := GetStageLabelForStatus(newStatus)
	var removedLabels []string
	if oldStage != newStage {
		removedLabels = append(removedLabels, oldStage, "#"+oldStage)
	}

	targetLabel := "#" + strings.TrimPrefix(newStage, "#")
	existing.Status = newStatus
	existing.Position = newPosition
	existing.Labels = SetWorkflowLabel(existing.Labels, targetLabel)
	existing.UpdatedAt = now

	labelsJSON, _ := json.Marshal(existing.Labels)
	_, err = d.conn.Exec(`
		UPDATE tasks
		SET status = ?, labels = ?, position = ?, updated_at = ?
		WHERE id = ?
	`, string(newStatus), string(labelsJSON), newPosition, now, existing.ID)
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
	var t models.Task
	var labelsJSON string
	var dueDate, branchName, prURL, repoPath, sprint, team, teamID, trackerStatus, source, extURL, issueType, parentKey, parentTitle, parentType sql.NullString
	var trackerCreatedAt, trackerUpdatedAt, statusChangedAt sql.NullTime
	var statusStr, priorityStr string

	err := d.conn.QueryRow(`
		SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
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
		&t.Position,
		&dueDate,
		&branchName,
		&prURL,
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
			SELECT id, project_id, key, title, description, status, priority, labels, assignee, assignee_avatar, position, due_date, branch_name, pr_url, repo_path, sprint, team, team_id, tracker_status, source, external_url, issue_type, parent_key, parent_title, parent_type, tracker_created_at, tracker_updated_at, status_changed_at, created_at, updated_at
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
			&t.Position,
			&dueDate,
			&branchName,
			&prURL,
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
	if source.Valid && source.String != "" {
		t.Source = source.String
	} else if strings.HasPrefix(t.Key, "#") || strings.HasPrefix(t.Key, "GH-#") || strings.HasPrefix(t.Key, "gh-") {
		t.Source = "github"
	} else {
		t.Source = "local"
	}

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
	return insertTaskActivity(d.conn, act)
}

type activityExecutor interface {
	Exec(string, ...any) (sql.Result, error)
}

func insertTaskActivity(conn activityExecutor, act models.TaskActivity) error {
	stepsJSON, _ := json.Marshal(act.Steps)
	if act.Steps == nil {
		stepsJSON = []byte("[]")
	}
	_, err := conn.Exec(`
		INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, summary, output, steps, prompt, started_at, completed_at, error, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, act.ID, act.TaskID, act.SkillID, act.SkillName, act.Action, act.Status, act.Summary, act.Output, string(stepsJSON), act.Prompt, act.StartedAt, act.CompletedAt, act.Error, act.CreatedAt)
	return err
}

func (d *DB) AddTaskActivity(act models.TaskActivity) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.addTaskActivityDirect(act)
}

func (d *DB) getTaskActivitiesUnsafe(taskID string) ([]models.TaskActivity, error) {
	rows, err := d.conn.Query(`
		SELECT id, task_id, skill_id, skill_name, action, status, summary, output, steps, prompt, started_at, completed_at, error, created_at
		FROM task_activities WHERE task_id = ? ORDER BY created_at DESC
	`, taskID)
	if err != nil {
		return []models.TaskActivity{}, nil
	}
	defer rows.Close()

	var list []models.TaskActivity
	for rows.Next() {
		var a models.TaskActivity
		var stepsJSON string
		var prompt, errStr sql.NullString
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(&a.ID, &a.TaskID, &a.SkillID, &a.SkillName, &a.Action, &a.Status, &a.Summary, &a.Output, &stepsJSON, &prompt, &startedAt, &completedAt, &errStr, &a.CreatedAt)
		if err != nil {
			continue
		}
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

// JiraTokenClearSentinel is what the UI sends to delete a stored token, since an
// empty field means "leave it alone".
const JiraTokenClearSentinel = "__clear__"

func (d *DB) GetSettings() (*models.Settings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var s models.Settings
	var detMode, aiProv, aiCmd, repoP, issTrk, ghRepo, jiraProj, jiraUrl, jiraMail, jiraTok, pClar, pSpec, pImpl, pPR, pPick, specFw, extTerm sql.NullString
	var uiScale sql.NullInt64
	var autoSyncEnabled, autoSyncInterval sql.NullInt64

	err := d.conn.QueryRow(`
		SELECT id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar,
		       ai_provider, ai_command_template, repo_path, issue_tracker, github_repo, jira_project, jira_url, jira_email, jira_api_token,
		       prompt_clarify, prompt_specify, prompt_implement, prompt_create_pr, prompt_pick, editor_command, external_terminal_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, updated_at
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
		&repoP,
		&issTrk,
		&ghRepo,
		&jiraProj,
		&jiraUrl,
		&jiraMail,
		&jiraTok,
		&pClar,
		&pSpec,
		&pImpl,
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
			return &models.Settings{
				ID:                1,
				Theme:             "dark",
				AccentColor:       "indigo",
				Language:          "fr",
				Density:           "standard",
				DefaultView:       "board",
				DetailMode:        "panel",
				UserName:          "Developer",
				UserEmail:         "dev@example.com",
				UserAvatar:        "",
				AIProvider:        "agy",
				AICommandTemplate: "agy --dangerously-skip-permissions -p \"{prompt}\"",
				RepoPath:          ".",
				IssueTracker:      "local",
				GithubRepo:        "",
				PromptClarify:     "",
				PromptSpecify:     "",
				PromptImplement:   "",
				PromptCreatePR:    "",
				PromptPick:        "",
				EditorCommand:     "code",
				SpecFramework:     "speckit",
				UpdatedAt:         time.Now(),
			}, nil
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
	if aiCmd.Valid && aiCmd.String != "" {
		s.AICommandTemplate = aiCmd.String
	} else {
		s.AICommandTemplate = "agy --dangerously-skip-permissions -p \"{prompt}\""
	}
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
	if pClar.Valid {
		s.PromptClarify = pClar.String
	}
	if pSpec.Valid {
		s.PromptSpecify = pSpec.String
	}
	if pImpl.Valid {
		s.PromptImplement = pImpl.String
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

	return &s, nil
}

func (d *DB) UpdateSettings(s models.Settings) (*models.Settings, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

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
		if s.AICommandTemplate == "" {
			s.AICommandTemplate = current.AICommandTemplate
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
		} else if s.JiraAPIToken == JiraTokenClearSentinel {
			s.JiraAPIToken = ""
		}
		if s.PromptClarify == "" {
			s.PromptClarify = current.PromptClarify
		}
		if s.PromptSpecify == "" {
			s.PromptSpecify = current.PromptSpecify
		}
		if s.PromptImplement == "" {
			s.PromptImplement = current.PromptImplement
		}
		if s.PromptCreatePR == "" {
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
	if s.AICommandTemplate == "" {
		s.AICommandTemplate = "agy -p \"{prompt}\""
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

	now := time.Now()
	_, err := d.conn.Exec(`
		INSERT INTO settings (id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar, ai_provider, ai_command_template, repo_path, issue_tracker, github_repo, jira_project, jira_url, jira_email, jira_api_token, prompt_clarify, prompt_specify, prompt_implement, prompt_create_pr, prompt_pick, editor_command, external_terminal_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			repo_path = excluded.repo_path,
			issue_tracker = excluded.issue_tracker,
			github_repo = excluded.github_repo,
			jira_project = excluded.jira_project,
			jira_url = excluded.jira_url,
			jira_email = excluded.jira_email,
			jira_api_token = excluded.jira_api_token,
			prompt_clarify = excluded.prompt_clarify,
			prompt_specify = excluded.prompt_specify,
			prompt_implement = excluded.prompt_implement,
			prompt_create_pr = excluded.prompt_create_pr,
			prompt_pick = excluded.prompt_pick,
			editor_command = excluded.editor_command,
			external_terminal_command = excluded.external_terminal_command,
			spec_framework = excluded.spec_framework,
			ui_scale = excluded.ui_scale,
			auto_sync_enabled = excluded.auto_sync_enabled,
			auto_sync_interval_sec = excluded.auto_sync_interval_sec,
			updated_at = excluded.updated_at
	`, s.Theme, s.AccentColor, s.Language, s.Density, s.DefaultView, s.DetailMode, s.UserName, s.UserEmail, s.UserAvatar, s.AIProvider, s.AICommandTemplate, s.RepoPath, s.IssueTracker, s.GithubRepo, s.JiraProject, s.JiraUrl, s.JiraEmail, s.JiraAPIToken, s.PromptClarify, s.PromptSpecify, s.PromptImplement, s.PromptCreatePR, s.PromptPick, s.EditorCommand, s.ExternalTerminalCommand, s.SpecFramework, s.UIScale, autoSyncEnabledInt, s.AutoSyncIntervalSec, now)

	if err != nil {
		return nil, err
	}

	s.ID = 1
	s.UpdatedAt = now
	return &s, nil
}

// GetAvailableSkills exposes the workflow catalogue. It derives from the single
// StageSkills table: the skill the UI offers, the file installed in the
// repository and the step the worker runs are by construction the same thing.
// The old pick-issue auto-pilot is gone, the autonomous run button replaced it.
// UIScaleOptions are the four interface zoom levels the status bar switches
// between. Four steps is what a quick switch can hold: a free number would need
// a settings screen and a keyboard, which is not what "make it bigger, now" asks
// for.
var UIScaleOptions = []int{90, 100, 112, 125}

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
	out := make([]models.Skill, 0, len(StageSkills))
	for _, s := range StageSkills {
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

			// Concurrency control per project (1 to models.MaxParallelism workers)
			limit := d.GetProjectParallelism(projID)
			d.limiter.Acquire(projID, limit)
			defer d.limiter.Release(projID)

			d.runJobGuarded(j)
		}(job)
	}
}

// GetProjectParallelism returns the configured background workers limit for a project (1 to models.MaxParallelism, default 1).
func (d *DB) GetProjectParallelism(projectID string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	p, err := d.getProjectByIDUnsafe(projectID)
	if err == nil && p != nil && p.Parallelism >= 1 && p.Parallelism <= models.MaxParallelism {
		return p.Parallelism
	}
	return 1
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
				WHERE id = ?
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
	if stage, ok := StageSkillByID(job.SkillID); ok {
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

	d.mu.Lock()
	_, _ = d.conn.Exec(`
		UPDATE task_activities
		SET status = 'running', started_at = ?
		WHERE id = ?
	`, now, job.ActivityID)
	d.mu.Unlock()

	// 3. Special handling for background Sync jobs
	if job.SkillID != "sync_task" && (strings.HasPrefix(job.SkillID, "sync_") || job.SkillID == "sync_all") {
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

	// 3c. Synchronisation unitaire d'un ticket en arrière-plan
	if job.SkillID == "sync_task" {
		d.processSyncTaskJob(ctx, job)
		return
	}

	// 3d. Écritures tracker unitaires : assignation, épic, labels d'horizon.
	if job.SkillID == "tracker_op" {
		d.processTrackerOpJob(ctx, job)
		return
	}

	// This activity tracks dispatch, not skill completion. The remote run owns results.
	d.mu.Lock()
	_, _ = d.conn.Exec("UPDATE task_activities SET skill_id='agent_launch' WHERE id=?", job.ActivityID)
	d.mu.Unlock()

	task, err := d.GetTaskByID(job.TaskID)
	if err == nil && task != nil {
		var run *models.TaskActivity
		run, err = d.StartAgentRemoteRun(task.ID, job.SkillID)
		if err == nil {
			err = d.callAgentContext(ctx, agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "execute_skill", SkillID: job.SkillID, Prompt: job.Prompt, RunID: run.ID}, nil)
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
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = d.conn.Exec("UPDATE task_activities SET skill_id='agent_launch',status=?,summary=?,error=?,completed_at=? WHERE id=?", status, summary, errorText, time.Now(), job.ActivityID)
}

func (d *DB) processSyncJob(ctx context.Context, job SkillJob, settings *models.Settings) {
	var steps []string
	var summary string
	var outputLines []string
	var hasError bool
	var totalImported int

	switch {
	case job.SkillID == "sync_jira":
		hasError = true
		summary = "Support Jira retiré"
		outputLines = append(outputLines, "Le support de Jira a été retiré de Sectile. Utilisez GitHub.")

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
				Project:  &p,
				Repo:     tRepo,
				RepoPath: tPath,
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
			Project:  proj,
			Repo:     repo,
			RepoPath: repoPath,
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
			}
			totalImported = len(tasks)
			summary = fmt.Sprintf("%d %s issues synchronized successfully", len(tasks), trackerTitle)

			outputLines = append(outputLines, fmt.Sprintf("✅ **%d tickets imported / updated from %s:**\n", len(tasks), trackerTitle))
			for _, t := range tasks {
				outputLines = append(outputLines, fmt.Sprintf("- **[%s]** %s *(Status: %s, Priority: %s)*", t.Key, t.Title, t.Status, t.Priority))
			}
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
		WHERE id = ?
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
	if task == nil {
		return
	}

	proj, _ := d.getProjectByIDUnsafe(task.ProjectID)
	tracker := task.Source
	if proj != nil && proj.IssueTracker != "" && tracker == "" {
		tracker = proj.IssueTracker
	}

	isGithub := tracker == "github" || task.Source == "github" || strings.HasPrefix(task.Key, "#") || strings.HasPrefix(task.Key, "gh-") || strings.HasPrefix(task.Key, "GH-")
	isJira := tracker == "jira" || task.Source == "jira"

	if !isGithub && !isJira {
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

	if isJira {
		trackerName = "Jira"
		jiraKey := ""
		if proj != nil && proj.JiraProject != "" {
			jiraKey = proj.JiraProject
		}
		initialSteps = []string{
			fmt.Sprintf("Mise à jour du ticket Jira [%s] (projet %s)", task.Key, jiraKey),
			fmt.Sprintf("Statut cible : %s | Étape IA : %s", stStr, activeStage),
		}
		if len(changesSummary) > 0 {
			initialSteps = append(initialSteps, strings.Join(changesSummary, " | "))
		}
		initialSteps = append(initialSteps, "Poussée dans la file d'attente d'exécution...")
	} else {
		trackerName = "GitHub"
		repo := ""
		if proj != nil && proj.GithubRepo != "" {
			repo = proj.GithubRepo
		}
		initialSteps = []string{
			fmt.Sprintf("Mise à jour issue GitHub [%s] (%s)", task.Key, repo),
			fmt.Sprintf("Statut cible : %s | Étape IA : %s", stStr, activeStage),
		}
		if len(changesSummary) > 0 {
			initialSteps = append(initialSteps, strings.Join(changesSummary, " | "))
		}
		initialSteps = append(initialSteps, "Poussée dans la file d'attente d'exécution...")
	}

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
	}

	_ = d.addTaskActivityDirect(act)

	job := SkillJob{
		ActivityID:      activityID,
		TaskID:          task.ID,
		SkillID:         "tracker_update",
		Prompt:          stStr,
		RemovedLabels:   removedLabels,
		TrackerStatus:   strings.TrimSpace(task.TrackerStatus),
		SyncTitle:       changed.Title,
		SyncDescription: changed.Description,
		SyncPriority:    changed.Priority,
	}

	select {
	case d.jobQueue <- job:
	default:
		go func() {
			d.jobQueue <- job
		}()
	}
}

func (d *DB) processTrackerUpdateJob(ctx context.Context, job SkillJob) {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(job.TaskID)
	settings, _ := d.getSettingsUnsafe()
	d.mu.RUnlock()

	if err != nil || task == nil {
		d.mu.Lock()
		_, _ = d.conn.Exec(`
			UPDATE task_activities
			SET status = 'failed', error = 'Tâche introuvable pour la synchronisation tracker', completed_at = CURRENT_TIMESTAMP
			WHERE id = ?
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
		err := ts.UpdateIssue(context.Background(), tracker.UpdateIssueRequest{
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
				if _, syncErr := d.SyncSingleTask(task.ID); syncErr != nil {
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
		WHERE id = ?
	`, status, summary, outputText, string(stepsJSON), errText, completedTime, job.ActivityID)
	d.mu.Unlock()
}

// SyncSingleTask pulls the single authoritative issue state from the remote tracker and writes it to SQLite.
func (d *DB) SyncSingleTask(taskID string) (*models.Task, error) {
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
		syncedTask, err = ts.GetIssue(context.Background(), tracker.GetIssueRequest{
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
		return d.GetTaskByID(task.ID)
	}

	return task, nil
}

// EnqueueSingleTaskSync enqueues a background sync activity for a single task.
func (d *DB) EnqueueSingleTaskSync(task *models.Task) (*models.TaskActivity, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	activityID := uuid.New().String()
	now := time.Now()
	act := models.TaskActivity{
		ID:        activityID,
		TaskID:    task.ID,
		TaskKey:   task.Key,
		SkillID:   "sync_task",
		SkillName: "Sync Ticket",
		Action:    fmt.Sprintf("Synchronisation de %s (arrière-plan)", task.Key),
		Status:    string(models.ActivityStatusQueued),
		Summary:   fmt.Sprintf("Synchronisation de %s en file d'attente", task.Key),
		Steps: []string{
			fmt.Sprintf("Cible : Ticket %s", task.Key),
			"Poussée dans la file d'attente d'exécution...",
		},
		CreatedAt: now,
	}

	d.mu.Lock()
	err := d.addTaskActivityDirect(act)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	job := SkillJob{
		ActivityID: activityID,
		TaskID:     task.ID,
		SkillID:    "sync_task",
		ProjectID:  task.ProjectID,
	}

	d.pushTrackerOpJob(job)
	return &act, nil
}

func (d *DB) processSyncTaskJob(ctx context.Context, job SkillJob) {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(job.TaskID)
	d.mu.RUnlock()

	if err != nil || task == nil {
		d.finishTrackerOp(job.ActivityID, []string{"❌ Ticket introuvable"}, "Ticket introuvable pour la synchronisation unitaire", fmt.Errorf("ticket introuvable"))
		return
	}

	syncedTask, syncErr := d.SyncSingleTask(task.ID)
	if syncErr != nil {
		d.finishTrackerOp(job.ActivityID, []string{fmt.Sprintf("❌ Échec : %v", syncErr)}, fmt.Sprintf("Échec de la synchronisation de %s", task.Key), syncErr)
		return
	}

	d.finishTrackerOp(job.ActivityID, []string{fmt.Sprintf("✅ Ticket %s synchronisé avec succès", syncedTask.Key)}, fmt.Sprintf("Synchronisation de %s effectuée avec succès", syncedTask.Key), nil)
}

func (d *DB) EnqueueSync(syncType string, param string, projectID string) (*models.TaskActivity, error) {
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
	targetTaskID := "sync-" + syncType
	if proj != nil {
		targetTaskID = "sync-" + proj.ID
	}

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
		syncType = "sync_all"
		skillName = "Sync Globale"
		summary = "Synchronisation multi-trackers en file d'attente"
		steps = []string{
			"Cibles : Tous les projets et trackers distants configurés",
			"Poussée dans la file d'attente d'exécution...",
		}
	}

	act := models.TaskActivity{
		ID:        activityID,
		TaskID:    targetTaskID,
		SkillID:   syncType,
		SkillName: skillName,
		Action:    "Synchronisation des tickets distants",
		Status:    string(models.ActivityStatusQueued),
		Summary:   summary,
		Output:    "",
		Steps:     steps,
		Prompt:    param,
		CreatedAt: now,
	}

	d.mu.Lock()
	_ = d.addTaskActivityDirect(act)
	d.mu.Unlock()

	// Push job to worker queue
	d.jobQueue <- SkillJob{
		ActivityID: activityID,
		TaskID:     targetTaskID,
		ProjectID:  projectID,
		SkillID:    syncType,
		Prompt:     param,
	}

	return &act, nil
}

func (d *DB) EnqueueSkillOnTask(taskID string, skillID string, prompt string) (*models.Task, *models.TaskActivity, error) {
	return d.enqueueSkillOnTask(taskID, skillID, prompt, false)
}

// EnqueueAutonomousRun starts the chain: each step enqueues the next until the
// work reaches the review stage.
func (d *DB) EnqueueAutonomousRun(taskID string) (*models.Task, *models.TaskActivity, error) {
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche non trouvée")
	}

	stage := d.StageOfTask(task)
	if stage == AutonomousStopStage || stage == "finished" {
		return nil, nil, fmt.Errorf("la tâche est déjà à l'étape %s : la suite demande une revue humaine", stage)
	}
	step, ok := NextStep(stage)
	if !ok {
		return nil, nil, fmt.Errorf("aucun pas suivant depuis l'étape %s", stage)
	}
	return d.enqueueSkillOnTask(taskID, step.SkillID, "", true)
}

func (d *DB) enqueueSkillOnTask(taskID string, skillID string, prompt string, autoChain bool) (*models.Task, *models.TaskActivity, error) {
	skillID = models.NormalizeSkillID(skillID)
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("task not found: %s", taskID)
	}

	if skillID == "adjust" {
		if _, err := d.adjustmentPrerequisite(task, false); err != nil {
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

	d.mu.Lock()
	_ = d.addTaskActivityDirect(act)
	d.mu.Unlock()

	// Push to background channel worker
	d.jobQueue <- SkillJob{
		ActivityID: activityID,
		TaskID:     task.ID,
		ProjectID:  task.ProjectID,
		SkillID:    targetSkill.ID,
		Prompt:     prompt,
		AutoChain:  autoChain,
	}

	return task, &act, nil
}

func (d *DB) RunSkillOnTask(taskID string, skillID string, prompt string) (*models.Task, *models.TaskActivity, error) {
	return d.EnqueueSkillOnTask(taskID, skillID, prompt)
}

func (d *DB) GetActivities(projectID, status, skillID, taskID, search string, limit int) ([]models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var conditions []string
	var args []interface{}

	if projectID != "" && projectID != "all" {
		conditions = append(conditions, "((t.project_id = ? OR t.project_id = (SELECT slug FROM projects WHERE id = ?) OR t.project_id = (SELECT id FROM projects WHERE slug = ?)) OR a.task_id LIKE ? OR a.prompt LIKE ?)")
		args = append(args, projectID, projectID, projectID, "%"+projectID+"%", "%"+projectID+"%")
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
		conditions = append(conditions, "(a.skill_name LIKE ? OR a.summary LIKE ? OR a.output LIKE ? OR t.key LIKE ? OR t.title LIKE ?)")
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}

	sqlQuery := `
		SELECT a.id, a.task_id, COALESCE(t.key, ''), COALESCE(t.title, ''), a.skill_id, a.skill_name,
		       a.action, a.status, a.summary, a.output, a.steps, a.prompt,
		       a.created_at, a.started_at, a.completed_at, a.error
		FROM task_activities a
		LEFT JOIN tasks t ON a.task_id = t.id
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
		var prompt, errStr sql.NullString
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&a.ID,
			&a.TaskID,
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
		)
		if err != nil {
			continue
		}
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
	var prompt, errStr sql.NullString
	var startedAt, completedAt sql.NullTime

	err := d.conn.QueryRow(`
		SELECT a.id, a.task_id, COALESCE(t.key, ''), COALESCE(t.title, ''), a.skill_id, a.skill_name,
		       a.action, a.status, a.summary, a.output, a.steps, a.prompt,
		       a.created_at, a.started_at, a.completed_at, a.error
		FROM task_activities a
		LEFT JOIN tasks t ON a.task_id = t.id
		WHERE a.id = ?
	`, id).Scan(
		&a.ID,
		&a.TaskID,
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
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

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

	if projectID != "" && projectID != "all" {
		query = `
			SELECT a.status, COUNT(*) 
			FROM task_activities a
			LEFT JOIN tasks t ON a.task_id = t.id
			WHERE ((t.project_id = ? OR t.project_id = (SELECT slug FROM projects WHERE id = ?) OR t.project_id = (SELECT id FROM projects WHERE slug = ?)) OR a.task_id LIKE ? OR a.prompt LIKE ?)
			GROUP BY a.status
		`
		args = append(args, projectID, projectID, projectID, "%"+projectID+"%", "%"+projectID+"%")
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

func (d *DB) CancelActivity(activityID string) error {
	d.cancelMu.Lock()
	if cancel, exists := d.cancelMap[activityID]; exists {
		cancel()
		delete(d.cancelMap, activityID)
	}
	d.cancelMu.Unlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec(`
		UPDATE task_activities
		SET status = 'canceled', summary = 'Annulée par l''utilisateur', completed_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status IN ('queued', 'pending', 'running')
	`, activityID)
	return err
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

func (d *DB) AddTaskComment(taskID string, body string) error {
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
	return ts.AddComment(context.Background(), tracker.AddCommentRequest{
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
	task.Labels = SetWorkflowLabel(task.Labels, GetStageLabelForStatus(task.Status))

	ts, ok := d.TrackerRegistry().Get(target)
	if !ok || !ts.Supports(tracker.CapCreate) {
		return nil, fmt.Errorf("tracker distant non supporté: %s", target)
	}

	created, err := ts.CreateIssue(context.Background(), tracker.CreateIssueRequest{
		Project:     proj,
		Title:       task.Title,
		Description: task.Description,
		Priority:    task.Priority,
		Labels:      task.Labels,
	})
	if err != nil {
		return nil, fmt.Errorf("création %s impossible: %w", ts.Name(), err)
	}
	if created == nil {
		return nil, fmt.Errorf("création %s impossible: ticket non retourné", ts.Name())
	}
	newKey = created.Key
	extURL = created.ExternalURL

	d.mu.Lock()
	defer d.mu.Unlock()

	task.Key = newKey
	task.Source = target
	task.ExternalURL = extURL
	task.UpdatedAt = now

	labelsJSON, _ := json.Marshal(task.Labels)
	_, err = d.conn.Exec(`
		UPDATE tasks
		SET key = ?, source = ?, external_url = ?, labels = ?, updated_at = ?
		WHERE id = ?
	`, task.Key, task.Source, task.ExternalURL, string(labelsJSON), now, task.ID)
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
		if settings.PromptCreatePR == "" {
			settings.PromptCreatePR = cmd + " {issueKey}"
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

func defaultStageMapping() map[string]string {
	return map[string]string{
		"new":         "to_clarify",
		"untouched":   "to_clarify",
		"clarified":   "clarified",
		"specified":   "to_implement",
		"implemented": "to_test",
		"reviewed":    "to_test",
		"finished":    "to_close",
	}
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
		SELECT p.id, p.name, p.slug, p.description, p.icon, p.color, p.repo_path, p.repo_paths, p.use_worktrees, p.pr_creation_stage, p.board_id, p.tracker_columns, p.stage_columns, p.sprints, p.issue_types, p.mono_repo, p.git_remote_url, p.github_repo, p.jira_project, p.issue_tracker, p.tracker_url, p.is_default, p.stage_mapping, p.skill_overrides, p.setup_providers, p.ai_provider, p.ai_command_template, p.spec_framework, p.parallelism, p.tty_mode, p.external_terminal_command, p.auto_sync_enabled, p.auto_sync_interval_min, p.created_at, p.updated_at,
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
		var stageMappingJSON, skillOverridesJSON, setupProvidersJSON, repoPathsJSON string
		var useWorktrees int
		var trackerColumnsJSON, stageColumnsJSON, sprintsJSON, issueTypesJSON string
		var monoRepo int
		var aiProv, aiCmd, specFw, jiraProj, ttyMode, extTerm sql.NullString
		err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.Color, &p.RepoPath, &repoPathsJSON, &useWorktrees, &p.PRCreationStage, &p.BoardID, &trackerColumnsJSON, &stageColumnsJSON, &sprintsJSON, &issueTypesJSON, &monoRepo, &p.GitRemoteUrl, &p.GithubRepo, &jiraProj, &p.IssueTracker, &p.TrackerUrl, &isDefault, &stageMappingJSON, &skillOverridesJSON, &setupProvidersJSON, &aiProv, &aiCmd, &specFw, &p.Parallelism, &ttyMode, &extTerm, &autoSyncEnabledInt, &autoSyncIntervalMin, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount,
		)
		if err != nil {
			return nil, err
		}
		p.IsDefault = isDefault == 1
		p.AutoSyncEnabled = autoSyncEnabledInt == 1
		p.AutoSyncIntervalMin = models.NormalizeAutoSyncIntervalMin(autoSyncIntervalMin)
		p.StageMapping = defaultStageMapping()
		if stageMappingJSON != "" && stageMappingJSON != "{}" {
			_ = json.Unmarshal([]byte(stageMappingJSON), &p.StageMapping)
		}
		p.SkillOverrides = map[string]string{}
		if skillOverridesJSON != "" && skillOverridesJSON != "{}" {
			_ = json.Unmarshal([]byte(skillOverridesJSON), &p.SkillOverrides)
		}
		p.SetupProviders = parseSetupProviders(setupProvidersJSON)
		p.RepoPaths = parseRepoPaths(repoPathsJSON)
		p.UseWorktrees = useWorktrees == 1
		p.TrackerColumns = parseTrackerColumns(trackerColumnsJSON)
		p.StageColumns = parseStageColumns(stageColumnsJSON)
		p.Sprints = parseSprints(sprintsJSON)
		p.IssueTypes = parseIssueTypes(issueTypesJSON)
		p.MonoRepo = monoRepo == 1
		p.Parallelism = models.NormalizeParallelism(p.Parallelism)
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
		if jiraProj.Valid {
			p.JiraProject = jiraProj.String
		}
		p.SpecFramework = models.NormalizeSpecFramework(specFw.String)
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []models.Project{}
	}
	return projects, nil
}

func (d *DB) GetProjects() ([]models.Project, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getProjectsUnsafe()
}

func (d *DB) GetProjectByID(id string) (*models.Project, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.getProjectByIDUnsafe(id)
}

func (d *DB) getProjectByIDUnsafe(id string) (*models.Project, error) {
	var p models.Project
	var isDefault int
	var autoSyncEnabledInt, autoSyncIntervalMin int
	var stageMappingJSON, skillOverridesJSON, setupProvidersJSON, repoPathsJSON string
	var useWorktrees int
	var trackerColumnsJSON, stageColumnsJSON, sprintsJSON, issueTypesJSON string
	var monoRepo int
	var aiProv, aiCmd, specFw, jiraProj, ttyMode, extTerm sql.NullString
	err := d.conn.QueryRow(`
		SELECT p.id, p.name, p.slug, p.description, p.icon, p.color, p.repo_path, p.repo_paths, p.use_worktrees, p.pr_creation_stage, p.board_id, p.tracker_columns, p.stage_columns, p.sprints, p.issue_types, p.mono_repo, p.git_remote_url, p.github_repo, p.jira_project, p.issue_tracker, p.tracker_url, p.is_default, p.stage_mapping, p.skill_overrides, p.setup_providers, p.ai_provider, p.ai_command_template, p.spec_framework, p.parallelism, p.tty_mode, p.external_terminal_command, p.auto_sync_enabled, p.auto_sync_interval_min, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM tasks WHERE project_id = p.id) as task_count
		FROM projects p
		WHERE p.id = ? OR p.slug = ?
	`, id, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.Color, &p.RepoPath, &repoPathsJSON, &useWorktrees, &p.PRCreationStage, &p.BoardID, &trackerColumnsJSON, &stageColumnsJSON, &sprintsJSON, &issueTypesJSON, &monoRepo, &p.GitRemoteUrl, &p.GithubRepo, &jiraProj, &p.IssueTracker, &p.TrackerUrl, &isDefault, &stageMappingJSON, &skillOverridesJSON, &setupProvidersJSON, &aiProv, &aiCmd, &specFw, &p.Parallelism, &ttyMode, &extTerm, &autoSyncEnabledInt, &autoSyncIntervalMin, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount,
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
	p.StageMapping = defaultStageMapping()
	if stageMappingJSON != "" && stageMappingJSON != "{}" {
		_ = json.Unmarshal([]byte(stageMappingJSON), &p.StageMapping)
	}
	p.SkillOverrides = map[string]string{}
	if skillOverridesJSON != "" && skillOverridesJSON != "{}" {
		_ = json.Unmarshal([]byte(skillOverridesJSON), &p.SkillOverrides)
	}
	p.SetupProviders = parseSetupProviders(setupProvidersJSON)
	p.RepoPaths = parseRepoPaths(repoPathsJSON)
	p.UseWorktrees = useWorktrees == 1
	p.TrackerColumns = parseTrackerColumns(trackerColumnsJSON)
	p.StageColumns = parseStageColumns(stageColumnsJSON)
	p.Sprints = parseSprints(sprintsJSON)
	p.IssueTypes = parseIssueTypes(issueTypesJSON)
	p.MonoRepo = monoRepo == 1
	p.Parallelism = models.NormalizeParallelism(p.Parallelism)
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
	if jiraProj.Valid {
		p.JiraProject = jiraProj.String
	}
	p.SpecFramework = models.NormalizeSpecFramework(specFw.String)
	return &p, nil
}

func (d *DB) CreateProject(req models.CreateProjectRequest) (*models.Project, error) {
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
	specFramework := models.NormalizeSpecFramework(req.SpecFramework)

	now := time.Now()
	isDefInt := 0
	if req.IsDefault {
		isDefInt = 1
		_, _ = d.conn.Exec("UPDATE projects SET is_default = 0")
	}

	stageMapping := req.StageMapping
	if stageMapping == nil || len(stageMapping) == 0 {
		stageMapping = defaultStageMapping()
	}
	stageMappingBytes, _ := json.Marshal(stageMapping)

	skillOverrides := req.SkillOverrides
	if skillOverrides == nil {
		skillOverrides = map[string]string{}
	}
	skillOverridesBytes, _ := json.Marshal(skillOverrides)
	setupProvidersBytes, _ := json.Marshal(models.NormalizeSetupProviders(req.SetupProviders))

	// Types importés : vides à la création, ce qui vaut « les types par défaut ».
	// Les réglages du projet les nomment ensuite, à partir des types réels du
	// tracker.
	issueTypes := models.NormalizeIssueTypes(req.IssueTypes)
	if len(req.IssueTypes) == 0 {
		issueTypes = []string{}
	}
	issueTypesBytes, _ := json.Marshal(issueTypes)

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

	parallelism := models.NormalizeParallelism(req.Parallelism)
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

	_, err := d.conn.Exec(`
		INSERT INTO projects (id, name, slug, description, icon, color, repo_path, repo_paths, use_worktrees, pr_creation_stage, board_id, tracker_columns, stage_columns, sprints, issue_types, mono_repo, git_remote_url, github_repo, jira_project, issue_tracker, tracker_url, is_default, stage_mapping, skill_overrides, setup_providers, ai_provider, ai_command_template, spec_framework, parallelism, auto_sync_enabled, auto_sync_interval_min, tty_mode, external_terminal_command, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, slug, req.Description, icon, color, req.RepoPath, string(repoPathsBytes), useWorktreesInt, prCreationStage, req.BoardID, "[]", "{}", "[]", string(issueTypesBytes), monoRepoInt, gitRemote, githubRepo, jiraProject, issueTracker, req.TrackerUrl, isDefInt, string(stageMappingBytes), string(skillOverridesBytes), string(setupProvidersBytes), aiProvider, aiCmd, specFramework, parallelism, autoSyncEnabledInt, autoSyncIntervalMin, ttyMode, extTermCmd, now, now)
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}

	project, err := d.getProjectByIDUnsafe(id)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (d *DB) UpdateProject(id string, req models.UpdateProjectRequest) (*models.Project, error) {
	d.mu.Lock()

	p, err := d.getProjectByIDUnsafe(id)
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	if p == nil {
		d.mu.Unlock()
		return nil, fmt.Errorf("projet non trouvé")
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
	if req.StageMapping != nil {
		p.StageMapping = *req.StageMapping
	}
	if req.SkillOverrides != nil {
		p.SkillOverrides = *req.SkillOverrides
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
	if req.MonoRepo != nil {
		p.MonoRepo = *req.MonoRepo
	}
	if req.AIProvider != nil {
		p.AIProvider = *req.AIProvider
	}
	if req.AICommandTemplate != nil {
		p.AICommandTemplate = *req.AICommandTemplate
	}
	if req.SpecFramework != nil {
		p.SpecFramework = models.NormalizeSpecFramework(*req.SpecFramework)
	}
	if req.Parallelism != nil {
		p.Parallelism = models.NormalizeParallelism(*req.Parallelism)
	} else {
		p.Parallelism = models.NormalizeParallelism(p.Parallelism)
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
			_, _ = d.conn.Exec("UPDATE projects SET is_default = 0 WHERE id != ?", p.ID)
		}
	}
	p.UpdatedAt = time.Now()
	isDefInt := 0
	if p.IsDefault {
		isDefInt = 1
	}

	if p.StageMapping == nil || len(p.StageMapping) == 0 {
		p.StageMapping = defaultStageMapping()
	}
	stageMappingBytes, _ := json.Marshal(p.StageMapping)

	if p.SkillOverrides == nil {
		p.SkillOverrides = map[string]string{}
	}
	skillOverridesBytes, _ := json.Marshal(p.SkillOverrides)
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
	monoRepoInt := 0
	if p.MonoRepo {
		monoRepoInt = 1
	}
	autoSyncEnabledInt := 0
	if p.AutoSyncEnabled {
		autoSyncEnabledInt = 1
	}

	_, err = d.conn.Exec(`
		UPDATE projects
		SET name = ?, slug = ?, description = ?, icon = ?, color = ?, repo_path = ?, repo_paths = ?, use_worktrees = ?, pr_creation_stage = ?, board_id = ?, tracker_columns = ?, stage_columns = ?, sprints = ?, issue_types = ?, mono_repo = ?, git_remote_url = ?, github_repo = ?, jira_project = ?, issue_tracker = ?, tracker_url = ?, is_default = ?, stage_mapping = ?, skill_overrides = ?, setup_providers = ?, ai_provider = ?, ai_command_template = ?, spec_framework = ?, parallelism = ?, auto_sync_enabled = ?, auto_sync_interval_min = ?, tty_mode = ?, external_terminal_command = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Slug, p.Description, p.Icon, p.Color, p.RepoPath, string(repoPathsBytes), useWorktreesInt, p.PRCreationStage, p.BoardID, string(trackerColumnsBytes), string(stageColumnsBytes), string(sprintsBytes), string(issueTypesBytes), monoRepoInt, p.GitRemoteUrl, p.GithubRepo, p.JiraProject, p.IssueTracker, p.TrackerUrl, isDefInt, string(stageMappingBytes), string(skillOverridesBytes), string(setupProvidersBytes), p.AIProvider, p.AICommandTemplate, p.SpecFramework, p.Parallelism, autoSyncEnabledInt, p.AutoSyncIntervalMin, p.TtyMode, p.ExternalTerminalCommand, p.UpdatedAt, p.ID)
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}

	project, err := d.getProjectByIDUnsafe(p.ID)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}

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
	if p.IsDefault {
		return fmt.Errorf("impossible de supprimer le projet par défaut")
	}

	// Reassign tasks to default project
	var defaultProjID string
	_ = d.conn.QueryRow("SELECT id FROM projects WHERE is_default = 1 LIMIT 1").Scan(&defaultProjID)
	if defaultProjID == "" {
		defaultProjID = "default"
	}
	_, _ = d.conn.Exec("UPDATE tasks SET project_id = ? WHERE project_id = ?", defaultProjID, p.ID)
	_, err = d.conn.Exec("DELETE FROM projects WHERE id = ?", p.ID)
	return err
}

// -------------------------------------------------------------
// PROJECT SKILLS MANAGEMENT & PROVISIONING
// -------------------------------------------------------------

type ProjectSkillTemplate struct {
	ID          string
	Name        string
	DirName     string
	Description string
	Content     string
}

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

func (d *DB) DetectTrackerStatuses(projectID, tracker, githubRepo string) ([]models.DetectedStatus, error) {
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
			if tracker == "" {
				tracker = proj.IssueTracker
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
	if tracker == "github" {
		addStatus("open", "unstarted", "#3fb950", "github")
		addStatus("closed", "completed", "#8250df", "github")
	}

	if tracker == "jira" {
		return nil, fmt.Errorf("Jira synchronization is not supported by this version")
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
	if proj.AICommandTemplate != "" {
		settings.AICommandTemplate = proj.AICommandTemplate
	}
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
