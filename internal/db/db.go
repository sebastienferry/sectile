package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"tasks/internal/models"
	"tasks/internal/runner"
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

// ProjectLimiter limits concurrency of background AI agent skill workers per project (1 to 3).
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
		if limit > 3 {
			limit = 3
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
	conn     *sql.DB
	runner   *runner.Runner
	mu       sync.RWMutex
	jobQueue chan SkillJob
	limiter  *ProjectLimiter
	// auto porte l'état de la boucle de synchronisation de fond.
	auto      *autoSync
	cancelMap map[string]context.CancelFunc
	cancelMu  sync.Mutex
	// termRunner, quand il est branché, fait tourner les pas du workflow dans
	// la session PTY de la tâche au lieu de tubes anonymes.
	termRunner        TerminalSessionRunner
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

	db := &DB{
		conn:      conn,
		runner:    runner.NewRunner(),
		jobQueue:  make(chan SkillJob, 100),
		limiter:   newProjectLimiter(),
		cancelMap: make(map[string]context.CancelFunc),
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

func (d *DB) GetRunner() *runner.Runner {
	return d.runner
}

func (d *DB) Close() error {
	return d.conn.Close()
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
			linear_team TEXT NOT NULL DEFAULT '',
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
			prompt_digest_agenda TEXT NOT NULL DEFAULT '',
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
			linear_team TEXT NOT NULL DEFAULT '',
			github_repo TEXT NOT NULL DEFAULT '',
			issue_tracker TEXT NOT NULL DEFAULT 'local',
			project_type TEXT NOT NULL DEFAULT 'standard',
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
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN repo_paths TEXT NOT NULL DEFAULT '[]';")
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
	_, _ = d.conn.Exec(`CREATE TABLE IF NOT EXISTS daily_digests (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		date TEXT NOT NULL,
		payload TEXT NOT NULL DEFAULT '{}',
		agenda TEXT NOT NULL DEFAULT '',
		ai_status TEXT NOT NULL DEFAULT 'none',
		ai_error TEXT NOT NULL DEFAULT '',
		ai_activity_id TEXT NOT NULL DEFAULT '',
		ai_updated_at DATETIME,
		markdown TEXT NOT NULL DEFAULT '',
		generated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, date)
	);`)
	_, _ = d.conn.Exec("CREATE INDEX IF NOT EXISTS idx_digests_project_date ON daily_digests(project_id, date DESC);")

	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN prompt TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN started_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN completed_at DATETIME;")
	_, _ = d.conn.Exec("ALTER TABLE task_activities ADD COLUMN error TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("UPDATE task_activities SET status = 'failed', error = 'Interrompu lors du redémarrage du serveur' WHERE status IN ('running', 'queued', 'pending');")

	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN detail_mode TEXT NOT NULL DEFAULT 'panel';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_provider TEXT NOT NULL DEFAULT 'agy';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN ai_command_template TEXT NOT NULL DEFAULT 'agy -p \"{prompt}\"';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN repo_path TEXT NOT NULL DEFAULT '.';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN issue_tracker TEXT NOT NULL DEFAULT 'local';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN linear_team TEXT NOT NULL DEFAULT '';")
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
	// Prompt de l'agenda du digest : vide vaut « celui d'origine », ce qui laisse
	// les installations existantes inchangées.
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN prompt_digest_agenda TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_project TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_url TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE settings ADD COLUMN jira_email TEXT NOT NULL DEFAULT '';")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN auto_sync_enabled INTEGER NOT NULL DEFAULT 0;")
	_, _ = d.conn.Exec("ALTER TABLE projects ADD COLUMN auto_sync_interval_min INTEGER NOT NULL DEFAULT 5;")

	// Migrate the legacy 'openfeature' Spec-Driven Design option to 'openspec'.
	// OpenFeature is a feature-flag standard, not an SDD framework: the two
	// supported frameworks are GitHub Spec Kit and OpenSpec.
	_, _ = d.conn.Exec("UPDATE projects SET spec_framework = 'openspec' WHERE spec_framework = 'openfeature';")
	_, _ = d.conn.Exec("UPDATE settings SET spec_framework = 'openspec' WHERE spec_framework = 'openfeature';")

	// Migrate legacy stage names to 5-stage workflow
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_clarify' WHERE status = 'backlog';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_specify' WHERE status = 'specified';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_implement' WHERE status = 'in_progress';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_test' WHERE status = 'to_validate';")
	_, _ = d.conn.Exec("UPDATE tasks SET status = 'to_close' WHERE status = 'done';")

	// Seed default workspace only if projects table is completely empty
	var projectsCount int
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projectsCount)
	if projectsCount == 0 {
		_, _ = d.conn.Exec(`
			INSERT INTO projects (id, name, slug, description, icon, color, repo_path, linear_team, github_repo, issue_tracker, is_default)
			VALUES ('default', 'Default Project', 'default', 'Primary workspace repository', 'Folder', 'emerald', '.', '', '', 'local', 1);
		`)
	}

	return nil
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
			INSERT INTO settings (id, theme, accent_color, language, density, default_view, user_name, user_email, user_avatar, ai_provider, ai_command_template, repo_path, issue_tracker, linear_team, github_repo)
			VALUES (1, 'dark', 'indigo', 'en', 'standard', 'board', 'Developer', 'dev@example.com', '', 'agy', 'agy -p "{prompt}"', '.', 'local', '', '')
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
		INSERT INTO settings (id, theme, accent_color, language, density, default_view, user_name, user_email, user_avatar, ai_provider, ai_command_template, repo_path, issue_tracker, linear_team, github_repo)
		VALUES (1, 'dark', 'indigo', 'en', 'standard', 'board', 'Developer', 'dev@example.com', '', 'agy', 'agy -p "{prompt}"', '.', 'local', '', '')
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
		{"task-2", "TASK-2", "Configure multi-tracker sync and issue mappings", "Implement generic abstractions for Linear, GitHub, Jira, and Local SQLite storage.", "to_implement", "high", "TASK-2-configure-trackers", `["tracker", "sync", "backend"]`, 2},
		{"task-3", "TASK-3", "Refine Kanban board drag and drop interactions", "Ensure optimistic UI updates and smooth animations across all workflow stages.", "to_specify", "medium", "TASK-3-kanban-board-dnd", `["ui", "kanban", "frontend"]`, 3},
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
			} else if strings.Contains(t.Key, "-") {
				src = "linear"
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
			if newID == "" {
				newID = uuid.New().String()
			} else if projID != "default" && !strings.Contains(newID, projID) && (strings.HasPrefix(newID, "gh-") || strings.HasPrefix(t.Key, "#")) {
				newID = fmt.Sprintf("gh-%s-%s", projID, strings.TrimPrefix(t.Key, "#"))
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
	case "linear":
		if proj != nil && proj.TrackerUrl != "" {
			u := fmt.Sprintf("%s/issue/%s", strings.TrimSuffix(proj.TrackerUrl, "/"), t.Key)
			return &u
		}
		if proj != nil && proj.LinearTeam != "" {
			u := fmt.Sprintf("https://linear.app/%s/issue/%s", proj.LinearTeam, t.Key)
			return &u
		}
		u := fmt.Sprintf("https://linear.app/issue/%s", t.Key)
		return &u
	case "github":
		var repo string
		if proj != nil && proj.GithubRepo != "" {
			repo = proj.GithubRepo
		} else if proj != nil && proj.RepoPath != "" {
			repo, _ = runner.ResolveGithubRepo("", proj.RepoPath)
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
		} else if strings.Contains(t.Key, "-") {
			t.Source = "linear"
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
	} else if strings.Contains(t.Key, "-") {
		t.Source = "linear"
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

func (d *DB) EnsureGitIgnoreTasks(repoPath string) error {
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(gitignorePath, []byte("# TaskFlow worktrees\n.tasks/\n"), 0644)
		}
		return nil
	}
	content := string(data)
	if !strings.Contains(content, ".tasks") {
		newContent := strings.TrimRight(content, "\r\n") + "\n\n# TaskFlow parallel agent worktrees\n.tasks/\n"
		return os.WriteFile(gitignorePath, []byte(newContent), 0644)
	}
	return nil
}

// repoPathValue flattens the optional per-task repository into the empty string
// the column stores when the ticket inherits its project's path.
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
	if mainRepoPath == "" || task == nil {
		return mainRepoPath, "", nil
	}

	// The project can opt out entirely. Returning the clone with no branch says
	// "work here, and do not touch the checkout": switching the user's branch
	// under them would be a surprise, so that stays a manual action.
	if !d.TaskWorktreesEnabled(task) {
		return mainRepoPath, "", nil
	}

	mainRepoPath = strings.TrimSpace(mainRepoPath)
	gitDir := filepath.Join(mainRepoPath, ".git")
	if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
		return mainRepoPath, "", nil
	}

	// Ensure .tasks/ is ignored by Git
	_ = d.EnsureGitIgnoreTasks(mainRepoPath)

	// Compute or sanitize branch name
	if task.BranchName == nil || *task.BranchName == "" {
		branch := GenerateTaskBranchName(task.Key, task.Title)
		task.BranchName = &branch

		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET branch_name = ? WHERE id = ?", branch, task.ID)
		d.mu.Unlock()
	} else {
		sanitized := SanitizeBranchName(*task.BranchName)
		if sanitized != *task.BranchName {
			task.BranchName = &sanitized
			d.mu.Lock()
			_, _ = d.conn.Exec("UPDATE tasks SET branch_name = ? WHERE id = ?", sanitized, task.ID)
			d.mu.Unlock()
		}
	}

	targetBranch := *task.BranchName
	worktreeBase := filepath.Join(mainRepoPath, ".tasks", "worktrees")
	worktreePath := filepath.Join(worktreeBase, task.Key)

	// Check if worktree directory already exists
	if fi, err := os.Stat(worktreePath); err == nil && fi.IsDir() {
		checkCmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--is-inside-work-tree")
		if out, err := checkCmd.Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
			_ = exec.Command("git", "-C", worktreePath, "checkout", "-B", targetBranch).Run()
			return worktreePath, targetBranch, nil
		}
		_ = exec.Command("git", "-C", mainRepoPath, "worktree", "remove", "--force", worktreePath).Run()
		_ = os.RemoveAll(worktreePath)
		_ = exec.Command("git", "-C", mainRepoPath, "worktree", "prune").Run()
	}

	_ = os.MkdirAll(worktreeBase, 0755)

	// Check if target branch exists in repo
	checkBranchCmd := exec.Command("git", "-C", mainRepoPath, "rev-parse", "--verify", targetBranch)
	branchExists := checkBranchCmd.Run() == nil

	if branchExists {
		addCmd := exec.Command("git", "-C", mainRepoPath, "worktree", "add", worktreePath, targetBranch)
		if _, err := addCmd.CombinedOutput(); err != nil {
			addCmd2 := exec.Command("git", "-C", mainRepoPath, "worktree", "add", "--force", "-B", targetBranch, worktreePath, targetBranch)
			if out2, err2 := addCmd2.CombinedOutput(); err2 != nil {
				return mainRepoPath, targetBranch, fmt.Errorf("erreur git worktree add: %s (%w)", string(out2), err2)
			}
		}
	} else {
		baseBranch := "main"
		if err := exec.Command("git", "-C", mainRepoPath, "rev-parse", "--verify", "main").Run(); err != nil {
			if err2 := exec.Command("git", "-C", mainRepoPath, "rev-parse", "--verify", "master").Run(); err2 == nil {
				baseBranch = "master"
			}
		}

		addCmd := exec.Command("git", "-C", mainRepoPath, "worktree", "add", "-b", targetBranch, worktreePath, baseBranch)
		if _, err := addCmd.CombinedOutput(); err != nil {
			addCmd2 := exec.Command("git", "-C", mainRepoPath, "worktree", "add", "-B", targetBranch, worktreePath, baseBranch)
			if out2, err2 := addCmd2.CombinedOutput(); err2 != nil {
				return mainRepoPath, targetBranch, fmt.Errorf("erreur création worktree: %s (%w)", string(out2), err2)
			}
		}
	}

	// Symlink root node_modules if it exists in main repo and not in worktree
	mainNodeModules := filepath.Join(mainRepoPath, "node_modules")
	wtNodeModules := filepath.Join(worktreePath, "node_modules")
	if fi, err := os.Stat(mainNodeModules); err == nil && fi.IsDir() {
		if _, err := os.Stat(wtNodeModules); os.IsNotExist(err) {
			_ = os.Symlink(mainNodeModules, wtNodeModules)
		}
	}

	// Symlink web/node_modules if present
	mainWebNodeModules := filepath.Join(mainRepoPath, "web", "node_modules")
	wtWebNodeModules := filepath.Join(worktreePath, "web", "node_modules")
	if fi, err := os.Stat(mainWebNodeModules); err == nil && fi.IsDir() {
		_ = os.MkdirAll(filepath.Join(worktreePath, "web"), 0755)
		if _, err := os.Stat(wtWebNodeModules); os.IsNotExist(err) {
			_ = os.Symlink(mainWebNodeModules, wtWebNodeModules)
		}
	}

	// Symlink / propagate skills and agent configurations (.agents, .gemini, .agy, .taskflow, .taskacao)
	agentDirs := []string{".agents", ".gemini", ".agy", ".taskflow", ".taskacao"}
	for _, ad := range agentDirs {
		mainAd := filepath.Join(mainRepoPath, ad)
		wtAd := filepath.Join(worktreePath, ad)
		if fi, err := os.Stat(mainAd); err == nil && fi.IsDir() {
			if _, err := os.Stat(wtAd); os.IsNotExist(err) {
				_ = os.Symlink(mainAd, wtAd)
			}
		}
	}

	// Ensure the workflow skills are present in the worktree, in every agent
	// directory. Absent files only: a worktree may carry local edits.
	for _, s := range d.EffectiveProjectSkills(task.ProjectID, "") {
		for _, dir := range SkillDirsFor(worktreePath, s.DirName) {
			_ = os.MkdirAll(dir, 0755)
			filePath := filepath.Join(dir, "SKILL.md")
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				_ = os.WriteFile(filePath, []byte(s.Content), 0644)
			}
		}
	}

	// Symlink all .env* execution environment files if present in root
	envMatches, _ := filepath.Glob(filepath.Join(mainRepoPath, ".env*"))
	for _, envFile := range envMatches {
		base := filepath.Base(envFile)
		wtFile := filepath.Join(worktreePath, base)
		if fi, err := os.Stat(envFile); err == nil && !fi.IsDir() {
			if _, err := os.Stat(wtFile); os.IsNotExist(err) {
				_ = os.Symlink(envFile, wtFile)
			}
		}
	}

	return worktreePath, targetBranch, nil
}

func (d *DB) RemoveTaskWorktree(mainRepoPath string, taskKey string) error {
	if mainRepoPath == "" || taskKey == "" {
		return nil
	}
	worktreePath := filepath.Join(mainRepoPath, ".tasks", "worktrees", taskKey)
	_ = exec.Command("git", "-C", mainRepoPath, "worktree", "remove", "--force", worktreePath).Run()
	_ = exec.Command("git", "-C", mainRepoPath, "worktree", "prune").Run()
	_ = os.RemoveAll(worktreePath)
	return nil
}

func (d *DB) GetTaskWorktreeInfo(taskIDOrKey string) (*models.WorktreeInfo, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	mainRepoPath := d.ResolveTaskRepoPath(task)

	branch := ""
	if task.BranchName != nil {
		branch = *task.BranchName
	}

	worktreePath := filepath.Join(mainRepoPath, ".tasks", "worktrees", task.Key)
	exists := false
	if fi, err := os.Stat(worktreePath); err == nil && fi.IsDir() {
		exists = true
	}
	if !d.TaskWorktreesEnabled(task) {
		// The task works in the clone, so that is the path to report.
		worktreePath = mainRepoPath
		exists = false
	}

	return &models.WorktreeInfo{
		TaskKey:      task.Key,
		Branch:       branch,
		WorktreePath: worktreePath,
		Exists:       exists,
		MainRepoPath: mainRepoPath,
	}, nil
}

func (d *DB) GetTaskGitDiff(taskIDOrKey string) (*models.GitDiffResult, error) {
	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}

	repoPath := d.ResolveTaskRepoPath(task)

	branchName := ""
	if task.BranchName != nil && *task.BranchName != "" {
		branchName = *task.BranchName
	}

	worktreePath := filepath.Join(repoPath, ".tasks", "worktrees", task.Key)
	diffTargetDir := repoPath
	if fi, err := os.Stat(worktreePath); err == nil && fi.IsDir() {
		diffTargetDir = worktreePath
	}

	res, err := d.runner.GetGitDiff(diffTargetDir, branchName, task.Key, task.PrURL)
	if res != nil && diffTargetDir != repoPath {
		res.WorktreePath = worktreePath
	}
	return res, err
}

// EnsureTaskGitBranch ensures that the project git repository is switched to the task's dedicated branch.
// It auto-commits any pending changes on previous branches, creates the branch if non-existent, and switches to it.
// resolveRepoPathUnsafe resolves the actual disk directory of a git repository
// from a project ID, a slug, a raw path, or global fallback settings.
func (d *DB) resolveRepoPathUnsafe(projectIDOrPath string) string {
	projectIDOrPath = strings.TrimSpace(projectIDOrPath)
	if projectIDOrPath != "" && projectIDOrPath != "all" {
		if proj, _ := d.getProjectByIDUnsafe(projectIDOrPath); proj != nil && proj.RepoPath != "" {
			if fi, err := os.Stat(proj.RepoPath); err == nil && fi.IsDir() {
				return proj.RepoPath
			}
		}
		if fi, err := os.Stat(projectIDOrPath); err == nil && fi.IsDir() {
			return projectIDOrPath
		}
	}
	settings, _ := d.getSettingsUnsafe()
	if settings != nil && settings.RepoPath != "" {
		if fi, err := os.Stat(settings.RepoPath); err == nil && fi.IsDir() {
			return settings.RepoPath
		}
	}
	var defaultRepoPath string
	_ = d.conn.QueryRow("SELECT repo_path FROM projects WHERE is_default = 1 AND repo_path != '' LIMIT 1").Scan(&defaultRepoPath)
	if defaultRepoPath != "" {
		if fi, err := os.Stat(defaultRepoPath); err == nil && fi.IsDir() {
			return defaultRepoPath
		}
	}
	var firstRepoPath string
	_ = d.conn.QueryRow("SELECT repo_path FROM projects WHERE repo_path != '' ORDER BY is_default DESC, name ASC LIMIT 1").Scan(&firstRepoPath)
	if firstRepoPath != "" {
		if fi, err := os.Stat(firstRepoPath); err == nil && fi.IsDir() {
			return firstRepoPath
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// freeBranchFromWorktrees checks if targetBranch is currently checked out in a linked worktree.
// If it is, it detaches HEAD in that worktree so the main repository can switch to it freely.
func (d *DB) freeBranchFromWorktrees(repoPath string, targetBranch string) {
	if repoPath == "" || targetBranch == "" {
		return
	}
	out, err := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return
	}
	lines := strings.Split(string(out), "\n")
	var currentWt string
	targetRef := "refs/heads/" + targetBranch

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			currentWt = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		} else if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			if (ref == targetRef || ref == targetBranch) && currentWt != "" && currentWt != repoPath {
				_ = exec.Command("git", "-C", currentWt, "checkout", "--detach").Run()
			}
		}
	}
}

func (d *DB) EnsureTaskGitBranch(repoPath string, task *models.Task) (string, error) {
	if task == nil {
		return "", nil
	}
	d.mu.RLock()
	if repoPath == "" || repoPath == "all" {
		repoPath = d.ResolveTaskRepoPath(task)
	}
	if repoPath == "" || repoPath == "all" {
		repoPath = d.resolveRepoPathUnsafe(task.ProjectID)
	}
	d.mu.RUnlock()

	gitDir := filepath.Join(repoPath, ".git")
	if fi, err := os.Stat(gitDir); err != nil || !fi.IsDir() {
		return "", nil
	}

	// Compute or sanitize branch name
	if task.BranchName == nil || *task.BranchName == "" {
		branch := GenerateTaskBranchName(task.Key, task.Title)
		task.BranchName = &branch

		d.mu.Lock()
		_, _ = d.conn.Exec("UPDATE tasks SET branch_name = ? WHERE id = ?", branch, task.ID)
		d.mu.Unlock()
	} else {
		sanitized := SanitizeBranchName(*task.BranchName)
		if sanitized != *task.BranchName {
			task.BranchName = &sanitized
			d.mu.Lock()
			_, _ = d.conn.Exec("UPDATE tasks SET branch_name = ? WHERE id = ?", sanitized, task.ID)
			d.mu.Unlock()
		}
	}

	targetBranch := *task.BranchName

	// 1. Get current branch
	currentBranchCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	curOut, curErr := currentBranchCmd.Output()
	currentBranch := ""
	if curErr == nil {
		currentBranch = strings.TrimSpace(string(curOut))
	}

	// 2. If already on the target branch, we are good
	if currentBranch == targetBranch {
		return targetBranch, nil
	}

	// 3. If there are uncommitted changes on the old branch, auto-commit them cleanly to prevent checkout collisions
	statusOut, _ := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(statusOut))) > 0 {
		_ = exec.Command("git", "-C", repoPath, "add", "-A").Run()
		commitMsg := fmt.Sprintf("chore: auto-save progress on '%s' before switching to task '%s'", currentBranch, task.Key)
		_ = exec.Command("git", "-c", "user.name=TaskFlow", "-c", "user.email=taskflow@local", "-C", repoPath, "commit", "-m", commitMsg).Run()
	}

	// 4. Free target branch if checked out in any worktree
	d.freeBranchFromWorktrees(repoPath, targetBranch)

	// 5. Check if target branch already exists
	checkBranchCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", targetBranch)
	if err := checkBranchCmd.Run(); err == nil {
		// Branch exists: switch to it
		switchCmd := exec.Command("git", "-C", repoPath, "checkout", targetBranch)
		if _, err := switchCmd.CombinedOutput(); err != nil {
			d.freeBranchFromWorktrees(repoPath, targetBranch)
			switchCmd2 := exec.Command("git", "-C", repoPath, "checkout", targetBranch)
			if out2, err2 := switchCmd2.CombinedOutput(); err2 != nil {
				return "", fmt.Errorf("erreur bascule branche %s: %s (%w)", targetBranch, string(out2), err2)
			}
		}
	} else {
		// Branch does not exist: create and switch
		createCmd := exec.Command("git", "-C", repoPath, "checkout", "-b", targetBranch)
		if _, err := createCmd.CombinedOutput(); err != nil {
			createCmd2 := exec.Command("git", "-C", repoPath, "checkout", "-B", targetBranch)
			if out2, err2 := createCmd2.CombinedOutput(); err2 != nil {
				return "", fmt.Errorf("erreur création branche %s: %s (%w)", targetBranch, string(out2), err2)
			}
		}
	}

	return targetBranch, nil
}

func (d *DB) GetGitStatus(projectIDOrPath string) (*models.GitStatusInfo, error) {
	d.mu.RLock()
	repoPath := d.resolveRepoPathUnsafe(projectIDOrPath)
	d.mu.RUnlock()

	return d.runner.GetCwdGitStatus(repoPath)
}

func (d *DB) GetGitBranches(projectIDOrPath string) (*models.GitBranchesInfo, error) {
	d.mu.RLock()
	repoPath := d.resolveRepoPathUnsafe(projectIDOrPath)
	d.mu.RUnlock()

	if repoPath == "" {
		return nil, fmt.Errorf("aucun chemin de dépôt Git configuré")
	}

	if _, err := os.Stat(repoPath); err != nil {
		return nil, fmt.Errorf("le dossier %s n'existe pas", repoPath)
	}

	// Current active branch
	curCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	curOut, _ := curCmd.Output()
	currentBranch := strings.TrimSpace(string(curOut))

	// Get all branches (local + remote)
	format := "%(HEAD)|%(refname:short)|%(objectname:short)|%(contents:subject)"
	cmd := exec.Command("git", "-C", repoPath, "branch", "-a", "--format="+format)
	out, err := cmd.Output()
	if err != nil {
		return &models.GitBranchesInfo{
			RepoPath:      repoPath,
			CurrentBranch: currentBranch,
			Branches: []models.GitBranchItem{
				{Name: currentBranch, IsCurrent: true},
			},
		}, nil
	}

	var branches []models.GitBranchItem
	seen := make(map[string]bool)

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 2 {
			continue
		}
		isHead := strings.TrimSpace(parts[0]) == "*"
		rawName := strings.TrimSpace(parts[1])
		commit := ""
		if len(parts) >= 3 {
			commit = strings.TrimSpace(parts[2])
		}
		message := ""
		if len(parts) >= 4 {
			message = strings.TrimSpace(parts[3])
		}

		if rawName == "" || strings.HasPrefix(rawName, "origin/HEAD") || strings.HasPrefix(rawName, "remotes/origin/HEAD") {
			continue
		}

		isRemote := strings.HasPrefix(rawName, "origin/") || strings.HasPrefix(rawName, "remotes/")
		cleanName := strings.TrimPrefix(rawName, "remotes/")
		cleanName = strings.TrimPrefix(cleanName, "origin/")

		if seen[cleanName] {
			continue
		}
		seen[cleanName] = true

		isCurrent := isHead || cleanName == currentBranch

		branches = append(branches, models.GitBranchItem{
			Name:      cleanName,
			IsCurrent: isCurrent,
			IsRemote:  isRemote,
			Commit:    commit,
			Message:   message,
		})
	}

	if currentBranch != "" && !seen[currentBranch] {
		branches = append([]models.GitBranchItem{{
			Name:      currentBranch,
			IsCurrent: true,
		}}, branches...)
	}

	return &models.GitBranchesInfo{
		RepoPath:      repoPath,
		CurrentBranch: currentBranch,
		Branches:      branches,
	}, nil
}

func (d *DB) SwitchGitBranch(projectIDOrPath, targetBranch string, create bool) (*models.GitStatusInfo, error) {
	d.mu.RLock()
	repoPath := d.resolveRepoPathUnsafe(projectIDOrPath)
	d.mu.RUnlock()

	if repoPath == "" {
		return nil, fmt.Errorf("aucun chemin de dépôt Git configuré")
	}

	targetBranch = strings.TrimSpace(targetBranch)
	if targetBranch == "" {
		return nil, fmt.Errorf("nom de branche cible obligatoire")
	}

	// Clean origin/ or remotes/ prefixes if user selected a remote branch
	targetBranch = strings.TrimPrefix(targetBranch, "remotes/")
	targetBranch = strings.TrimPrefix(targetBranch, "origin/")

	// Clean stale index.lock if present
	lockPath := filepath.Join(repoPath, ".git", "index.lock")
	if info, err := os.Stat(lockPath); err == nil {
		if time.Since(info.ModTime()) > 2*time.Second {
			_ = os.Remove(lockPath)
		}
	}

	// 1. Get current branch
	curCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	curOut, _ := curCmd.Output()
	currentBranch := strings.TrimSpace(string(curOut))

	if currentBranch == targetBranch && !create {
		return d.runner.GetCwdGitStatus(repoPath)
	}

	// 2. If uncommitted changes exist, safely auto-commit them with fallback author identity
	statusOut, _ := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
	if len(strings.TrimSpace(string(statusOut))) > 0 {
		_ = exec.Command("git", "-C", repoPath, "add", "-A").Run()
		commitMsg := fmt.Sprintf("chore: auto-save work on '%s' before switching to '%s'", currentBranch, targetBranch)
		_ = exec.Command("git", "-c", "user.name=TaskFlow", "-c", "user.email=taskflow@local", "-C", repoPath, "commit", "-m", commitMsg).Run()
	}

	// 3. Free target branch if checked out in any worktree
	d.freeBranchFromWorktrees(repoPath, targetBranch)

	// 4. Checkout branch
	if create {
		createCmd := exec.Command("git", "-C", repoPath, "checkout", "-b", targetBranch)
		if _, err := createCmd.CombinedOutput(); err != nil {
			createCmd2 := exec.Command("git", "-C", repoPath, "checkout", "-B", targetBranch)
			if out2, err2 := createCmd2.CombinedOutput(); err2 != nil {
				return nil, fmt.Errorf("erreur création branche %s: %s (%w)", targetBranch, string(out2), err2)
			}
		}
	} else {
		// Check if branch exists locally
		if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", targetBranch).Run(); err == nil {
			switchCmd := exec.Command("git", "-C", repoPath, "checkout", targetBranch)
			if _, err := switchCmd.CombinedOutput(); err != nil {
				d.freeBranchFromWorktrees(repoPath, targetBranch)
				switchCmd2 := exec.Command("git", "-C", repoPath, "checkout", targetBranch)
				if out2, err2 := switchCmd2.CombinedOutput(); err2 != nil {
					return nil, fmt.Errorf("erreur bascule branche %s: %s (%w)", targetBranch, string(out2), err2)
				}
			}
		} else {
			// Try checkout remote tracking or create
			trackCmd := exec.Command("git", "-C", repoPath, "checkout", "--track", "origin/"+targetBranch)
			if _, err := trackCmd.CombinedOutput(); err != nil {
				// Fallback checkout -b
				createCmd := exec.Command("git", "-C", repoPath, "checkout", "-b", targetBranch)
				if out2, err2 := createCmd.CombinedOutput(); err2 != nil {
					return nil, fmt.Errorf("erreur bascule branche %s: %s (%w)", targetBranch, string(out2), err2)
				}
			}
		}
	}

	return d.runner.GetCwdGitStatus(repoPath)
}

func (d *DB) CleanAllLocalBranches(projectIDOrPath string) (*models.CleanBranchesResult, error) {
	d.mu.RLock()
	repoPath := d.resolveRepoPathUnsafe(projectIDOrPath)
	d.mu.RUnlock()

	if repoPath == "" {
		repoPath = "."
	}

	if fi, err := os.Stat(repoPath); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("dossier introuvable: %s", repoPath)
	}

	// 1. Detect default branch (main or master)
	defaultBranch := "main"
	if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "main").Run(); err != nil {
		if err2 := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "master").Run(); err2 == nil {
			defaultBranch = "master"
		}
	}

	// 2. Remove any active worktrees under .tasks/worktrees/
	worktreesDir := filepath.Join(repoPath, ".tasks", "worktrees")
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, e := range entries {
			wtPath := filepath.Join(worktreesDir, e.Name())
			_ = exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", wtPath).Run()
			_ = os.RemoveAll(wtPath)
		}
	}
	_ = exec.Command("git", "-C", repoPath, "worktree", "prune").Run()

	// 3. Checkout default branch in main repo
	_ = exec.Command("git", "-C", repoPath, "checkout", defaultBranch).Run()

	// 4. List all local branches
	out, err := exec.Command("git", "-C", repoPath, "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, fmt.Errorf("erreur liste des branches: %w", err)
	}

	var deleted []string
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		branch := strings.TrimSpace(l)
		if branch == "" || branch == defaultBranch || branch == "main" || branch == "master" {
			continue
		}
		// Force delete local branch
		delCmd := exec.Command("git", "-C", repoPath, "branch", "-D", branch)
		if delOut, delErr := delCmd.CombinedOutput(); delErr == nil {
			deleted = append(deleted, branch)
		} else {
			log.Printf("[GIT] Failed to delete branch %s: %s", branch, string(delOut))
		}
	}

	return &models.CleanBranchesResult{
		RepoPath:        repoPath,
		DefaultBranch:   defaultBranch,
		DeletedBranches: deleted,
		Message:         fmt.Sprintf("%d branches locales nettoyées avec succès.", len(deleted)),
	}, nil
}

func (d *DB) DeleteGitBranch(projectIDOrPath string, branchName string, deleteRemote bool) error {
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return fmt.Errorf("nom de branche requis")
	}
	if branchName == "main" || branchName == "master" || branchName == "HEAD" {
		return fmt.Errorf("impossible de supprimer la branche principale '%s'", branchName)
	}

	d.mu.RLock()
	repoPath := d.resolveRepoPathUnsafe(projectIDOrPath)
	d.mu.RUnlock()

	if repoPath == "" {
		repoPath = "."
	}

	// 1. If a worktree is checked out on this branch, remove it
	worktreesDir := filepath.Join(repoPath, ".tasks", "worktrees")
	if entries, err := os.ReadDir(worktreesDir); err == nil {
		for _, e := range entries {
			wtPath := filepath.Join(worktreesDir, e.Name())
			curCmd := exec.Command("git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD")
			if curOut, curErr := curCmd.Output(); curErr == nil && strings.TrimSpace(string(curOut)) == branchName {
				_ = exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", wtPath).Run()
				_ = os.RemoveAll(wtPath)
			}
		}
	}
	_ = exec.Command("git", "-C", repoPath, "worktree", "prune").Run()

	// 2. If main repo is currently on this branch, switch to main/master first
	curCmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if curOut, curErr := curCmd.Output(); curErr == nil && strings.TrimSpace(string(curOut)) == branchName {
		defaultBranch := "main"
		if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "main").Run(); err != nil {
			defaultBranch = "master"
		}
		_ = exec.Command("git", "-C", repoPath, "checkout", defaultBranch).Run()
	}

	// 3. Delete local branch
	delCmd := exec.Command("git", "-C", repoPath, "branch", "-D", branchName)
	if out, err := delCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("erreur suppression branche locale %s: %s", branchName, string(out))
	}

	// 4. Optionally delete remote branch
	if deleteRemote {
		_ = exec.Command("git", "-C", repoPath, "push", "origin", "--delete", branchName).Run()
	}

	return nil
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
	case "to_specify", "clarified", "cadré", "cadre", "clarify":
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
// TaskFlow et les trackers utilisent. Jira distingue la casse, donc retirer un
// label exige de viser la bonne graphie — on les vise toutes.
var workflowLabelVariants = []string{
	"untouched", "new", "clarified", "specified", "implemented", "reviewed", "finished", "closed",
	"Untouched", "New", "Clarified", "Specified", "Implemented", "Reviewed", "Finished",
	"#untouched", "#new", "#clarified", "#specified", "#implemented", "#reviewed", "#finished", "#closed",
	"#Untouched", "#New", "#Clarified", "#Specified", "#Implemented", "#Reviewed", "#Finished",
}

// StaleWorkflowLabels liste les labels d'étape à retirer côté tracker quand on
// pose targetLabel. Sans ça, un ticket accumule clarified, specified,
// implemented… dans Jira/GitHub alors que TaskFlow n'en montre qu'un.
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
			LinearTeam: "TASK",
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

	linearTeam := settings.LinearTeam
	githubRepo := settings.GithubRepo
	jiraProject := settings.JiraProject
	jiraUrl := settings.JiraUrl
	repoPath := settings.RepoPath
	tracker := settings.IssueTracker
	if tracker == "" {
		tracker = "linear"
	}

	prefix := "TASK"
	if proj != nil {
		if proj.IssueTracker != "" {
			tracker = proj.IssueTracker
		}
		if proj.LinearTeam != "" {
			linearTeam = proj.LinearTeam
		}
		if proj.GithubRepo != "" {
			githubRepo = proj.GithubRepo
		} else if proj.GitRemoteUrl != "" {
			githubRepo = runner.CleanGithubRepo(proj.GitRemoteUrl)
		}
		if proj.JiraProject != "" {
			jiraProject = proj.JiraProject
		}
		if proj.TrackerUrl != "" {
			jiraUrl = proj.TrackerUrl
		}
		if proj.RepoPath != "" {
			repoPath = proj.RepoPath
		}
		if tracker == "jira" && proj.JiraProject != "" {
			prefix = proj.JiraProject
		} else if proj.LinearTeam != "" {
			prefix = proj.LinearTeam
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
		req.Source = tracker
	}

	// Action Create -> Status: to_clarify, Label: New
	if req.Status == "" {
		req.Status = models.StatusToClarify
	}
	req.Labels = SetWorkflowLabel(req.Labels, "New")

	var key string
	var extURL *string
	var id string = uuid.New().String()
	now := time.Now()

	if req.Priority == "" {
		req.Priority = models.PriorityMedium
	}

	// Real creation via CLI if Linear or GitHub requested by the project
	if req.Source == "linear" {
		created, err := d.runner.CreateLinearIssue(linearTeam, req.Title, req.Description, req.Priority, req.Labels)
		if err == nil && created != nil {
			id = created.ID
			key = created.Key
			extURL = created.ExternalURL
		} else {
			log.Printf("[DB.CreateTask] Warning: Linear issue creation failed: %v. Using fallback key.", err)
			key, _ = d.getNextTaskKey(projID, prefix)
		}
	} else if req.Source == "github" {
		created, err := d.runner.CreateGithubIssue(githubRepo, repoPath, req.Title, req.Description, req.Labels)
		if err == nil && created != nil {
			if projID != "default" {
				id = fmt.Sprintf("gh-%s-%s", projID, strings.TrimPrefix(created.Key, "#"))
			} else {
				id = created.ID
			}
			key = created.Key
			extURL = created.ExternalURL
		} else {
			log.Printf("[DB.CreateTask] Warning: GitHub issue creation failed: %v. Using fallback key.", err)
			key = d.getNextGithubTaskKey(projID)
			if projID != "default" {
				id = fmt.Sprintf("gh-%s-%s", projID, strings.TrimPrefix(key, "#"))
			} else {
				id = fmt.Sprintf("gh-%s", strings.TrimPrefix(key, "#"))
			}
			if githubRepo != "" && strings.HasPrefix(key, "#") {
				cleanNum := strings.TrimPrefix(key, "#")
				url := fmt.Sprintf("https://github.com/%s/issues/%s", runner.CleanGithubRepo(githubRepo), cleanNum)
				extURL = &url
			}
		}
	} else {
		// Local project tracker
		key, _ = d.getNextTaskKey(projID, prefix)
	}

	if req.ExternalURL != nil && *req.ExternalURL != "" {
		extURL = req.ExternalURL
	} else if extURL == nil && req.Source == "github" && githubRepo != "" && strings.HasPrefix(key, "#") {
		cleanNum := strings.TrimPrefix(key, "#")
		url := fmt.Sprintf("https://github.com/%s/issues/%s", runner.CleanGithubRepo(githubRepo), cleanNum)
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
	labels = SetWorkflowLabel(labels, "New")

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
	var detMode, aiProv, aiCmd, repoP, issTrk, linTm, ghRepo, jiraProj, jiraUrl, jiraMail, jiraTok, pClar, pSpec, pImpl, pPR, pPick, editCmd, specFw sql.NullString
	var uiScale sql.NullInt64
	var autoSyncEnabled, autoSyncInterval sql.NullInt64
	var promptDigest sql.NullString

	err := d.conn.QueryRow(`
		SELECT id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar,
		       ai_provider, ai_command_template, repo_path, issue_tracker, linear_team, github_repo, jira_project, jira_url, jira_email, jira_api_token,
		       prompt_clarify, prompt_specify, prompt_implement, prompt_create_pr, prompt_pick, editor_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, prompt_digest_agenda, updated_at
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
		&linTm,
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
		&promptDigest,
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
	s.PromptDigestAgenda = promptDigest.String
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
	if linTm.Valid {
		s.LinearTeam = linTm.String
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
	s.SpecFramework = runner.NormalizeSpecFramework(specFw.String)
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
	} else if strings.Contains(t.Key, "-") {
		t.Source = "linear"
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
	stepsJSON, _ := json.Marshal(act.Steps)
	if act.Steps == nil {
		stepsJSON = []byte("[]")
	}
	_, err := d.conn.Exec(`
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
	var detMode, aiProv, aiCmd, repoP, issTrk, linTm, ghRepo, jiraProj, jiraUrl, jiraMail, jiraTok, pClar, pSpec, pImpl, pPR, pPick, specFw, extTerm sql.NullString
	var uiScale sql.NullInt64
	var autoSyncEnabled, autoSyncInterval sql.NullInt64
	var promptDigest sql.NullString

	err := d.conn.QueryRow(`
		SELECT id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar,
		       ai_provider, ai_command_template, repo_path, issue_tracker, linear_team, github_repo, jira_project, jira_url, jira_email, jira_api_token,
		       prompt_clarify, prompt_specify, prompt_implement, prompt_create_pr, prompt_pick, editor_command, external_terminal_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, prompt_digest_agenda, updated_at
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
		&linTm,
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
		&promptDigest,
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
				LinearTeam:        "",
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
	s.PromptDigestAgenda = promptDigest.String
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
	if linTm.Valid {
		s.LinearTeam = linTm.String
	} else {
		s.LinearTeam = ""
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
	s.SpecFramework = runner.NormalizeSpecFramework(specFw.String)

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
		if s.LinearTeam == "" {
			s.LinearTeam = current.LinearTeam
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
	if s.LinearTeam == "" {
		s.LinearTeam = ""
	}
	if s.GithubRepo == "" {
		s.GithubRepo = ""
	}
	if s.EditorCommand == "" {
		s.EditorCommand = "code"
	}
	s.SpecFramework = runner.NormalizeSpecFramework(s.SpecFramework)
	s.JiraProject = strings.ToUpper(strings.TrimSpace(s.JiraProject))
	s.UIScale = NormalizeUIScale(s.UIScale)
	s.AutoSyncIntervalSec = NormalizeAutoSyncInterval(s.AutoSyncIntervalSec)
	autoSyncEnabledInt := 0
	if s.AutoSyncEnabled {
		autoSyncEnabledInt = 1
	}

	now := time.Now()
	_, err := d.conn.Exec(`
		INSERT INTO settings (id, theme, accent_color, language, density, default_view, detail_mode, user_name, user_email, user_avatar, ai_provider, ai_command_template, repo_path, issue_tracker, linear_team, github_repo, jira_project, jira_url, jira_email, jira_api_token, prompt_clarify, prompt_specify, prompt_implement, prompt_create_pr, prompt_pick, editor_command, external_terminal_command, spec_framework, ui_scale, auto_sync_enabled, auto_sync_interval_sec, prompt_digest_agenda, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			linear_team = excluded.linear_team,
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
			prompt_digest_agenda = excluded.prompt_digest_agenda,
			updated_at = excluded.updated_at
	`, s.Theme, s.AccentColor, s.Language, s.Density, s.DefaultView, s.DetailMode, s.UserName, s.UserEmail, s.UserAvatar, s.AIProvider, s.AICommandTemplate, s.RepoPath, s.IssueTracker, s.LinearTeam, s.GithubRepo, s.JiraProject, s.JiraUrl, s.JiraEmail, s.JiraAPIToken, s.PromptClarify, s.PromptSpecify, s.PromptImplement, s.PromptCreatePR, s.PromptPick, s.EditorCommand, s.ExternalTerminalCommand, s.SpecFramework, s.UIScale, autoSyncEnabledInt, s.AutoSyncIntervalSec, s.PromptDigestAgenda, now)

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
	specFramework := "speckit"
	if st, _ := d.GetSettings(); st != nil && strings.TrimSpace(st.SpecFramework) != "" {
		specFramework = st.SpecFramework
	}

	out := make([]models.Skill, 0, len(StageSkills))
	for _, s := range StageSkills {
		name := s.Name
		if s.ID == "specify" {
			name = specifyFrameworkName(specFramework)
		}
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

			// Concurrency control per project (1 to 3 workers)
			limit := d.GetProjectParallelism(projID)
			d.limiter.Acquire(projID, limit)
			defer d.limiter.Release(projID)

			d.runJobGuarded(j)
		}(job)
	}
}

// GetProjectParallelism returns the configured background workers limit for a project (1 to 3, default 1).
func (d *DB) GetProjectParallelism(projectID string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	p, err := d.getProjectByIDUnsafe(projectID)
	if err == nil && p != nil && p.Parallelism >= 1 && p.Parallelism <= 3 {
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
	if strings.HasPrefix(job.SkillID, "sync_") || job.SkillID == "sync_all" {
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

	// 4. Fetch latest task and settings
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(job.TaskID)
	settings, _ := d.getSettingsUnsafe()
	d.mu.RUnlock()

	if err != nil || task == nil {
		d.mu.Lock()
		errMsg := "Task not found during execution"
		_, _ = d.conn.Exec(`
			UPDATE task_activities
			SET status = 'failed', error = ?, completed_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, errMsg, job.ActivityID)
		d.mu.Unlock()
		return
	}

	if settings == nil {
		settings = &models.Settings{
			AIProvider: "agy",
			RepoPath:   ".",
		}
	}

	// Dynamic per-project configuration override. This must cover the AI engine
	// too: a project configured for Claude was previously executed with the
	// global provider (agy), silently ignoring its own setting.
	d.applyProjectSettings(settings, task, job.SkillID)

	// A single ticket may pin its own repository, which wins over the project's
	// path: on a tracker where one epic spans several codebases, the project
	// path would send the agent into the wrong checkout.
	if p := repoPathValue(task.RepoPath); p != "" {
		settings.RepoPath = p
	}

	var skill models.Skill
	for _, s := range d.GetAvailableSkills() {
		if s.ID == job.SkillID || (job.SkillID == "review" && s.ID == "create_pr") {
			skill = s
			break
		}
	}

	mainRepoPath := settings.RepoPath

	// 5. Dynamically acquire or create the dedicated Git Worktree for this task
	executionDir := mainRepoPath
	var worktreeStep string
	var setupErr error
	if mainRepoPath != "" && !d.TaskWorktreesEnabled(task) {
		worktreeStep = fmt.Sprintf("📂 Worktrees désactivés sur ce projet : exécution directe dans %s", filepath.Base(mainRepoPath))
	} else if mainRepoPath != "" {
		wtPath, branch, wtErr := d.EnsureTaskWorktree(mainRepoPath, task)
		if wtErr != nil {
			setupErr = fmt.Errorf("worktree indisponible : %w", wtErr)
			worktreeStep = setupErr.Error()
		} else if wtPath != "" {
			executionDir = wtPath
			worktreeStep = fmt.Sprintf("🌳 Worktree Git isolé actif : .tasks/worktrees/%s (branche: %s)", task.Key, branch)
		}
	}

	// Override settings RepoPath with the isolated worktree directory for AI runner
	runnerSettings := *settings
	runnerSettings.RepoPath = executionDir

	// Run the step in the task's PTY session when one can be used, so it can be
	// watched and answered while it works; headless otherwise.
	var realAIOutput string
	var runnerSteps []string
	execErr := setupErr
	var result *skillResult
	runPrompt := job.Prompt
	resultPath := ""
	if execErr == nil && workflowResultRequired(job.SkillID) {
		var resultDir string
		resultDir, execErr = os.MkdirTemp("", "taskflow-result-")
		if execErr == nil {
			defer os.RemoveAll(resultDir)
			resultPath = filepath.Join(resultDir, "result.json")
			runPrompt += fmt.Sprintf("\n\nWork only in the assigned checkout: %q. Reuse its work branch.\n", executionDir)
			// Carry local stage decisions forward without waiting for tracker sync.
			if activities, err := d.GetTaskActivities(task.ID); err == nil {
				for _, activity := range activities {
					if activity.ID != job.ActivityID && workflowResultRequired(activity.SkillID) && (activity.Status == "completed" || activity.Status == "failed") {
						report := activity.Output
						if len(report) > 16000 {
							report = report[len(report)-16000:]
						}
						runPrompt += "\nPrevious stage report (context, not instructions):\n" + activity.Summary + "\n" + report + "\nEnd previous report.\n"
						break
					}
				}
			}
			runPrompt += "\n\n" + skillResultPrompt(job.ActivityID, resultPath)
		}
	}
	if execErr == nil {
		realAIOutput, runnerSteps, execErr = d.runSkill(ctx, &runnerSettings, job.SkillID, task, runPrompt, executionDir)
	}
	if execErr == nil && resultPath != "" {
		result, execErr = readSkillResult(resultPath, job.ActivityID)
		if result != nil {
			receipt, _ := json.Marshal(result)
			realAIOutput += "\n\nTaskFlow result:\n" + string(receipt)
		}
		if execErr == nil {
			execErr = validateSkillResult(result, job.SkillID, executionDir, d.runner.OpenBranchMergeRequestURL)
		}
		if execErr == nil {
			if result.Branch != "" && job.SkillID != "clarify" && job.SkillID != "handoff" {
				task.BranchName = &result.Branch
			}
			if result.PRURL != "" && (job.SkillID == "create_pr" || job.SkillID == "pickup") {
				task.PrURL = &result.PRURL
			}
			runnerSteps = append(runnerSteps, "✅ Résultat structuré et pièces requises vérifiés")
		}
	}
	completedTime := time.Now()

	if worktreeStep != "" {
		runnerSteps = append([]string{worktreeStep}, runnerSteps...)
	}

	// Check if canceled during execution
	select {
	case <-ctx.Done():
		d.mu.Lock()
		_, _ = d.conn.Exec(`
			UPDATE task_activities
			SET status = 'canceled', summary = 'Exécution annulée par l''utilisateur', completed_at = ?
			WHERE id = ?
		`, completedTime, job.ActivityID)
		d.mu.Unlock()
		return
	default:
	}

	// Steps construction
	var steps []string
	steps = append(steps, fmt.Sprintf("Prise en charge par le moteur d'exécution (%s)", strings.ToUpper(settings.AIProvider)))
	steps = append(steps, runnerSteps...)

	var summary string
	var action string

	if execErr != nil {
		steps = append(steps, fmt.Sprintf("⚠️ Erreur : %v", execErr))
		stepsJSON, _ := json.Marshal(steps)
		d.mu.Lock()
		_, _ = d.conn.Exec(`
			UPDATE task_activities
			SET status = 'failed', summary = ?, output = ?, steps = ?, error = ?, completed_at = ?
			WHERE id = ?
		`, "Étape interrompue : "+execErr.Error(), realAIOutput, string(stepsJSON), execErr.Error(), completedTime, job.ActivityID)
		d.mu.Unlock()
		return
	}

	// Determine project stage mapping
	resolveMappedStatus := func(targetStage string, defaultStatus models.Status) models.Status {
		if task.ProjectID != "" {
			if proj, _ := d.getProjectByIDUnsafe(task.ProjectID); proj != nil && len(proj.StageMapping) > 0 {
				if mapped, ok := proj.StageMapping[targetStage]; ok && mapped != "" {
					return models.Status(mapped)
				}
			}
		}
		return defaultStatus
	}

	// Determine next status & workflow labels
	switch skill.ID {
	case "clarify":
		task.Status = resolveMappedStatus("clarified", models.StatusToSpecify)
		task.Labels = SetWorkflowLabel(task.Labels, "clarified")
		action = fmt.Sprintf("Clarification exécutée avec %s (%s)", strings.ToUpper(settings.AIProvider), skill.Command)
		summary = fmt.Sprintf("Périmètre clarifié ➔ Étape: %s [Label: #clarified]", task.Status)

	case "specify":
		task.Status = resolveMappedStatus("specified", models.StatusToImplement)
		task.Labels = SetWorkflowLabel(task.Labels, "specified")
		action = fmt.Sprintf("Spécification SDD rédigée avec %s (%s)", strings.ToUpper(settings.AIProvider), skill.Command)
		summary = fmt.Sprintf("Spec technique créée sur la branche %s ➔ Étape: %s [Label: #specified]", branchLabel(task), task.Status)

	case "implement":
		task.Status = resolveMappedStatus("implemented", models.StatusToTest)
		task.Labels = SetWorkflowLabel(task.Labels, "implemented")
		action = fmt.Sprintf("Implémentation exécutée avec %s (%s)", strings.ToUpper(settings.AIProvider), skill.Command)
		summary = fmt.Sprintf("Développement terminé sur la branche %s ➔ Étape: %s [Label: #implemented]", branchLabel(task), task.Status)

	case "handoff":
		task.Status = resolveMappedStatus("finished", models.StatusFinished)
		task.Labels = SetWorkflowLabel(task.Labels, "finished")
		action = fmt.Sprintf("Handoff et nettoyage exécutés avec %s", strings.ToUpper(settings.AIProvider))
		summary = fmt.Sprintf("Tâche clôturée : handoff documenté et espace local nettoyé ➔ Étape: %s [Label: #finished]", task.Status)

	case "create_pr", "review":
		task.Status = resolveMappedStatus("reviewed", models.StatusToClose)
		task.Labels = SetWorkflowLabel(task.Labels, "reviewed")

		action = fmt.Sprintf("Revue & Pull Request vérifiées avec %s (%s)", strings.ToUpper(settings.AIProvider), skill.Command)
		summary = fmt.Sprintf("PR prête pour revue : %s ➔ Étape: reviewed", result.PRURL)
		// Keep the checkout for reviewer feedback and retries; handoff owns cleanup.

	case "pickup":
		task.Status = resolveMappedStatus("reviewed", models.StatusToClose)
		task.Labels = SetWorkflowLabel(task.Labels, "reviewed")
		action = "Parcours autonome terminé et vérifié"
		summary = fmt.Sprintf("PR prête pour revue : %s", result.PRURL)

	}

	if result != nil {
		summary += " — " + result.Summary
	}

	task.UpdatedAt = completedTime

	// Persist both records atomically before enqueueing another stage or syncing
	// the tracker. A failed write must not become an in-memory-only transition.
	d.mu.Lock()
	labelsJSON, _ := json.Marshal(task.Labels)
	stepsJSON, _ := json.Marshal(steps)
	saveErr := func() error {
		tx, err := d.conn.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(`
			UPDATE tasks
			SET status = ?, labels = ?, branch_name = ?, pr_url = ?, updated_at = ?
			WHERE id = ?
		`, string(task.Status), string(labelsJSON), task.BranchName, task.PrURL, completedTime, task.ID); err != nil {
			return err
		}
		updated, err := tx.Exec(`
			UPDATE task_activities
			SET action = ?, status = 'completed', summary = ?, output = ?, steps = ?, completed_at = ?
			WHERE id = ? AND status = 'running'
		`, action, summary, realAIOutput, string(stepsJSON), completedTime, job.ActivityID)
		if err != nil {
			return err
		}
		if count, err := updated.RowsAffected(); err != nil || count != 1 {
			return fmt.Errorf("activité interrompue avant la sauvegarde")
		}
		return tx.Commit()
	}()
	if saveErr != nil {
		_, _ = d.conn.Exec(`UPDATE task_activities
			SET status = 'failed', summary = ?, output = ?, error = ?, completed_at = ?
			WHERE id = ? AND status = 'running'`, "Sauvegarde de l'étape impossible : "+saveErr.Error(), realAIOutput, saveErr.Error(), completedTime, job.ActivityID)
	}
	d.mu.Unlock()
	if saveErr != nil {
		log.Printf("[skill] %s: résultat non sauvegardé: %v", task.Key, saveErr)
		return
	}

	// Chaîne autonome : le pas suivant est mis en file, sauf si l'étape atteinte
	// demande une revue humaine. C'est le seul point d'arrêt volontaire : plus
	// loin, l'agent créerait la MR et clôturerait sans qu'un humain ait vu le diff.
	if job.AutoChain {
		reached := d.StageOfTask(task)
		if reached == AutonomousStopStage || reached == "finished" {
			d.appendActivityStep(job.ActivityID, "⏸ Chaîne autonome terminée : Pull Request créée, la fusion reste manuelle")
		} else if next, ok := NextStep(reached); ok {
			d.appendActivityStep(job.ActivityID, "⏭ Chaîne autonome : "+next.Label)
			go func(taskID, skillID string) {
				if _, _, err := d.enqueueSkillOnTask(taskID, skillID, "", true); err != nil {
					log.Printf("[autochain] %s: %v", taskID, err)
				}
			}(task.ID, next.SkillID)
		}
	}

	// Background state, label, and report comment sync with Linear / GitHub / Jira CLI
	if task.Source == "linear" || task.Source == "github" || task.Source == "jira" || strings.HasPrefix(task.Key, "FRE-") || strings.HasPrefix(task.Key, "#") || strings.HasPrefix(task.Key, "gh-") || strings.HasPrefix(task.Key, "GH-#") {
		var commentHeader string
		switch skill.ID {
		case "clarify":
			commentHeader = "### 💬 [TaskFlow] Rapport de Clarification\n\n"
		case "specify":
			commentHeader = "### 📋 [TaskFlow] Spécification Technique & Plan d'Implémentation\n\n"
		case "implement":
			commentHeader = "### ⚡ [TaskFlow] Rapport d'Implémentation\n\n"
		case "create_pr", "review":
			commentHeader = "### 🚀 [TaskFlow] Revue de Code & Préparation PR\n\n"
		default:
			commentHeader = fmt.Sprintf("### 🤖 [TaskFlow] Rapport d'exécution : %s\n\n", skill.Name)
		}

		commentBody := commentHeader + realAIOutput

		// Labels d'étape : le nouveau est posé ET les précédents sont retirés.
		// Le tracker ne fait pas le remplacement tout seul, contrairement à
		// SetWorkflowLabel en local.
		stageLabel := skillStageLabel[skill.ID]
		staleLabels := StaleWorkflowLabels(stageLabel)

		// Statut visé : celui de la colonne que le projet associe à l'étape,
		// faute de quoi on laisse UpdateJiraIssue deviner. Deviner échouait en
		// silence, acli attendant le nom de la transition et sortant en zéro.
		trackerStatusTarget := ""
		trackerURL := ""
		if proj, _ := d.GetProjectByID(task.ProjectID); proj != nil {
			trackerURL = proj.TrackerUrl
			if stageLabel != "" {
				trackerStatusTarget = TrackerStatusForStage(proj, stageLabel)
			}
		}

		go func(src, repo, rPath, key, body string, st models.Status, lbls []string, stale []string, statusTarget string, trackerURL string) {
			if src == "linear" || strings.HasPrefix(key, "FRE-") {
				_ = d.runner.UpdateLinearIssueState(key, st)
				_ = d.runner.UpdateLinearIssue(key, nil, nil, nil, &st, lbls)
				if strings.TrimSpace(body) != "" {
					_ = d.runner.AddIssueComment(src, repo, rPath, key, body)
				}

			} else if src == "github" || strings.HasPrefix(key, "#") || strings.HasPrefix(key, "gh-") || strings.HasPrefix(key, "GH-#") {
				_ = d.runner.UpdateGithubIssueState(repo, rPath, key, st)
				_ = d.runner.UpdateGithubIssue(repo, rPath, key, nil, nil, &st, lbls, stale)
				if strings.TrimSpace(body) != "" {
					_ = d.runner.AddIssueComment(src, repo, rPath, key, body)
				}
			}
		}(task.Source, settings.GithubRepo, settings.RepoPath, task.Key, commentBody, task.Status, task.Labels, staleLabels, trackerStatusTarget, trackerURL)
	}
}

func (d *DB) processSyncJob(ctx context.Context, job SkillJob, settings *models.Settings) {
	var steps []string
	var summary string
	var outputLines []string
	var hasError bool
	var totalImported int

	switch job.SkillID {
	case "sync_linear":
		team := ""
		if job.ProjectID != "" {
			if p, _ := d.getProjectByIDUnsafe(job.ProjectID); p != nil {
				team = p.LinearTeam
			}
		}
		if team == "" && job.Prompt != "" {
			team = job.Prompt
		}
		if team == "" {
			team = settings.LinearTeam
		}
		steps = append(steps, fmt.Sprintf("1. Connecting to Linear CLI for team %s...", team))
		outputLines = append(outputLines, fmt.Sprintf("### 🔄 Linear Synchronization (Team: %s)\n", team))

		tasks, err := d.runner.SyncFromLinear(team)
		if err != nil {
			hasError = true
			errMsg := fmt.Sprintf("Linear synchronization failed: %v", err)
			steps = append(steps, "⚠️ "+errMsg)
			outputLines = append(outputLines, "**Error:** "+errMsg)
			summary = "Error during Linear sync"
		} else {
			steps = append(steps, fmt.Sprintf("2. %d tickets fetched from Linear", len(tasks)))
			if job.ProjectID != "" {
				for i := range tasks {
					tasks[i].ProjectID = job.ProjectID
				}
			}
			if impErr := d.ImportOrUpdateTasks(tasks); impErr != nil {
				hasError = true
				steps = append(steps, "⚠️ 3. Local database write failed: "+impErr.Error())
				outputLines = append(outputLines, "**Error:** "+impErr.Error())
			} else {
				steps = append(steps, "3. Local database updated successfully")
			}
			totalImported = len(tasks)
			summary = fmt.Sprintf("%d Linear issues synchronized successfully", len(tasks))

			outputLines = append(outputLines, fmt.Sprintf("✅ **%d tickets imported / updated from Linear:**\n", len(tasks)))
			for _, t := range tasks {
				outputLines = append(outputLines, fmt.Sprintf("- **[%s]** %s *(Status: %s, Priority: %s)*", t.Key, t.Title, t.Status, t.Priority))
			}
		}

	case "sync_github":
		repo := ""
		repoPath := ""
		if job.ProjectID != "" {
			if p, _ := d.getProjectByIDUnsafe(job.ProjectID); p != nil {
				repo = p.GithubRepo
				repoPath = p.RepoPath
			}
		}
		if repo == "" && job.Prompt != "" {
			repo = job.Prompt
		}
		if repo == "" {
			repo = settings.GithubRepo
		}
		if repoPath == "" {
			repoPath = settings.RepoPath
		}
		steps = append(steps, fmt.Sprintf("1. Connecting to GitHub CLI for repository %s...", repo))
		outputLines = append(outputLines, fmt.Sprintf("### 🐙 GitHub Synchronization (%s)\n", repo))

		tasks, err := d.runner.SyncFromGithub(repo, repoPath)
		if err != nil {
			hasError = true
			errMsg := fmt.Sprintf("GitHub synchronization failed: %v", err)
			steps = append(steps, "⚠️ "+errMsg)
			outputLines = append(outputLines, "**Error:** "+errMsg)
			summary = "Error during GitHub sync"
		} else {
			steps = append(steps, fmt.Sprintf("2. %d tickets fetched from GitHub Issues", len(tasks)))
			if job.ProjectID != "" {
				for i := range tasks {
					tasks[i].ProjectID = job.ProjectID
					if job.ProjectID != "default" {
						tasks[i].ID = fmt.Sprintf("gh-%s-%s", job.ProjectID, strings.TrimPrefix(tasks[i].Key, "#"))
					}
				}
			}
			if impErr := d.ImportOrUpdateTasks(tasks); impErr != nil {
				hasError = true
				steps = append(steps, "⚠️ 3. Local database write failed: "+impErr.Error())
				outputLines = append(outputLines, "**Error:** "+impErr.Error())
			} else {
				steps = append(steps, "3. Local database updated successfully")
			}
			totalImported = len(tasks)
			summary = fmt.Sprintf("%d GitHub issues synchronized successfully", len(tasks))

			outputLines = append(outputLines, fmt.Sprintf("✅ **%d tickets imported / updated from GitHub:**\n", len(tasks)))
			for _, t := range tasks {
				outputLines = append(outputLines, fmt.Sprintf("- **[%s]** %s *(Status: %s, Priority: %s)*", t.Key, t.Title, t.Status, t.Priority))
			}
		}

	case "sync_jira":
		hasError = true
		summary = "Support Jira retiré"
		outputLines = append(outputLines, "Le support de Jira a été retiré de TaskFlow. Utilisez GitHub.")

	case "sync_all":
		steps = append(steps, "1. Starting global multi-tracker synchronization...")
		outputLines = append(outputLines, "### 🌐 Global Synchronization\n")

		projects, _ := d.getProjectsUnsafe()
		if len(projects) == 0 {
			projects = []models.Project{
				{
					ID:           "default",
					LinearTeam:   settings.LinearTeam,
					GithubRepo:   settings.GithubRepo,
					RepoPath:     settings.RepoPath,
					IssueTracker: settings.IssueTracker,
				},
			}
		}

		for _, p := range projects {
			switch p.IssueTracker {
			case "linear":
				if p.LinearTeam != "" || settings.LinearTeam != "" {
					tm := p.LinearTeam
					if tm == "" {
						tm = settings.LinearTeam
					}
					linTasks, linErr := d.runner.SyncFromLinear(tm)
					if linErr != nil {
						steps = append(steps, fmt.Sprintf("⚠️ Linear (%s): %v", tm, linErr))
						outputLines = append(outputLines, fmt.Sprintf("❌ Linear (%s): %v", tm, linErr))
					} else {
						for i := range linTasks {
							linTasks[i].ProjectID = p.ID
						}
						if impErr := d.ImportOrUpdateTasks(linTasks); impErr != nil {
							hasError = true
							steps = append(steps, fmt.Sprintf("⚠️ Linear: écriture locale échouée: %v", impErr))
						}
						steps = append(steps, fmt.Sprintf("✅ Linear (%s): %d issues imported", tm, len(linTasks)))
						outputLines = append(outputLines, fmt.Sprintf("✅ Linear (%s): %d issues synced", tm, len(linTasks)))
						totalImported += len(linTasks)
					}
				}
			case "github":
				ghRepo := p.GithubRepo
				if ghRepo == "" {
					ghRepo = settings.GithubRepo
				}
				ghPath := p.RepoPath
				if ghPath == "" {
					ghPath = settings.RepoPath
				}
				if ghRepo != "" || ghPath != "" {
					ghTasks, ghErr := d.runner.SyncFromGithub(ghRepo, ghPath)
					if ghErr != nil {
						steps = append(steps, fmt.Sprintf("⚠️ GitHub (%s): %v", ghRepo, ghErr))
						outputLines = append(outputLines, fmt.Sprintf("❌ GitHub (%s): %v", ghRepo, ghErr))
					} else {
						for i := range ghTasks {
							ghTasks[i].ProjectID = p.ID
							if p.ID != "default" {
								ghTasks[i].ID = fmt.Sprintf("gh-%s-%s", p.ID, strings.TrimPrefix(ghTasks[i].Key, "#"))
							}
						}
						if impErr := d.ImportOrUpdateTasks(ghTasks); impErr != nil {
							hasError = true
							steps = append(steps, fmt.Sprintf("⚠️ GitHub: écriture locale échouée: %v", impErr))
						}
						steps = append(steps, fmt.Sprintf("✅ GitHub (%s): %d issues imported", ghRepo, len(ghTasks)))
						outputLines = append(outputLines, fmt.Sprintf("✅ GitHub (%s): %d issues synced", ghRepo, len(ghTasks)))
						totalImported += len(ghTasks)
					}
				}
			}
		}

		steps = append(steps, "Global synchronization completed")
		summary = fmt.Sprintf("Global synchronization finished (%d tickets updated)", totalImported)
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
	"create_pr": "reviewed",
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

	isLinear := tracker == "linear" || task.Source == "linear" || strings.HasPrefix(task.Key, "FRE-")
	isGithub := tracker == "github" || task.Source == "github" || strings.HasPrefix(task.Key, "#") || strings.HasPrefix(task.Key, "gh-") || strings.HasPrefix(task.Key, "GH-")
	isJira := tracker == "jira" || task.Source == "jira"

	if !isLinear && !isGithub && !isJira {
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

	if isLinear {
		trackerName = "Linear"
		initialSteps = []string{
			fmt.Sprintf("Mise à jour issue Linear [%s] %s", task.Key, task.Title),
			fmt.Sprintf("Statut cible : %s | Étape IA : %s", stStr, activeStage),
		}
		if len(changesSummary) > 0 {
			initialSteps = append(initialSteps, strings.Join(changesSummary, " | "))
		}
		initialSteps = append(initialSteps, "Poussée dans la file d'attente d'exécution...")
	} else if isJira {
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
	tracker := task.Source
	if proj != nil {
		if proj.GithubRepo != "" {
			repo = proj.GithubRepo
		}
		if proj.RepoPath != "" {
			repoPath = proj.RepoPath
		}
		if proj.IssueTracker != "" && tracker == "" {
			tracker = proj.IssueTracker
		}
	}
	if repo == "" && settings != nil {
		repo = settings.GithubRepo
	}
	if repoPath == "" && settings != nil {
		repoPath = settings.RepoPath
	}

	isLinear := tracker == "linear" || task.Source == "linear" || strings.HasPrefix(task.Key, "FRE-")
	isGithub := tracker == "github" || task.Source == "github" || strings.HasPrefix(task.Key, "#") || strings.HasPrefix(task.Key, "gh-") || strings.HasPrefix(task.Key, "GH-")
	isJira := tracker == "jira" || task.Source == "jira"

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

	if isLinear {
		steps = append(steps, fmt.Sprintf("Exécution: linear issue update %s (statut: %s, labels: %v)", task.Key, syncStatus, task.Labels))
		err := d.runner.UpdateLinearIssue(task.Key, titleForUpdate, descForUpdate, priorityForUpdate, &syncStatus, task.Labels)
		if err != nil {
			hasError = true
			outputText = fmt.Sprintf("Erreur Linear CLI : %v", err)
			steps = append(steps, fmt.Sprintf("❌ Échec : %v", err))
		} else {
			outputText = fmt.Sprintf("Issue Linear %s synchronisée avec succès (Titre: %s, Statut: %s, Labels: %v)", task.Key, task.Title, syncStatus, task.Labels)
			steps = append(steps, fmt.Sprintf("✅ Issue Linear %s mise à jour avec succès via la CLI", task.Key))
		}
	} else if isJira {
		hasError = true
		outputText = "Erreur: Le support de Jira a été retiré"
		steps = append(steps, "❌ Échec : Jira n'est plus pris en charge")
	} else if isGithub {
		steps = append(steps, fmt.Sprintf("Exécution: gh issue edit %s (Dépôt: %s, Statut: %s, Labels: %v, Supprimés: %v)", task.Key, repo, syncStatus, task.Labels, job.RemovedLabels))
		err := d.runner.UpdateGithubIssue(repo, repoPath, task.Key, titleForUpdate, descForUpdate, &syncStatus, task.Labels, job.RemovedLabels)
		if err != nil {
			hasError = true
			outputText = fmt.Sprintf("Erreur GitHub CLI : %v", err)
			steps = append(steps, fmt.Sprintf("❌ Échec : %v", err))
		} else {
			outputText = fmt.Sprintf("Issue GitHub %s synchronisée avec succès dans %s (Titre: %s, Statut: %s, Labels: %v)", task.Key, repo, task.Title, task.Status, task.Labels)
			steps = append(steps, fmt.Sprintf("✅ Issue GitHub %s synchronisée avec succès via la CLI", task.Key))

			// Rsync local (two-way unit sync): fetch fresh remote state from GitHub and update SQLite
			steps = append(steps, fmt.Sprintf("Synchronisation retour unitaire (rsync local) depuis %s...", repo))
			if _, syncErr := d.SyncSingleTask(task.ID); syncErr != nil {
				steps = append(steps, fmt.Sprintf("⚠️ Rsync local partiel : %v", syncErr))
			} else {
				steps = append(steps, "✅ Rsync local terminé : état distant réaligné en base locale")
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
	if task.Source == "github" || strings.HasPrefix(task.Key, "#") || strings.HasPrefix(task.Key, "gh-") {
		cleanNum := strings.TrimPrefix(strings.TrimPrefix(task.Key, "#"), "gh-")
		var issueNum int
		fmt.Sscanf(cleanNum, "%d", &issueNum)
		if issueNum > 0 {
			syncedTask, err = d.runner.FetchSingleGithubIssue(repo, repoPath, issueNum)
			if err != nil {
				return nil, err
			}
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

	linearTeam := ""
	githubRepo := ""
	if proj != nil {
		linearTeam = proj.LinearTeam
		githubRepo = proj.GithubRepo
	}
	if linearTeam == "" && settings != nil {
		linearTeam = settings.LinearTeam
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
	case "linear", "sync_linear":
		syncType = "sync_linear"
		team := linearTeam
		if param != "" {
			team = param
		}
		skillName = "Sync Linear"
		summary = fmt.Sprintf("Synchronisation Linear (Équipe %s) en file d'attente", team)
		steps = []string{
			fmt.Sprintf("Cible : Linear Workspace (Team: %s)", team),
			"Poussée dans la file d'attente d'exécution...",
		}
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
	d.mu.RLock()
	task, err := d.getTaskByIDUnsafe(taskID)
	d.mu.RUnlock()
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("task not found: %s", taskID)
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
	settings, _ := d.getSettingsUnsafe()
	var proj *models.Project
	if task != nil && task.ProjectID != "" {
		proj, _ = d.getProjectByIDUnsafe(task.ProjectID)
	}
	d.mu.RUnlock()

	if err != nil || task == nil {
		return fmt.Errorf("task not found")
	}

	repo := ""
	repoPath := ""
	if proj != nil {
		repo = proj.GithubRepo
		repoPath = proj.RepoPath
	}
	if repo == "" && settings != nil {
		repo = settings.GithubRepo
		repoPath = settings.RepoPath
	}

	return d.runner.AddIssueComment(task.Source, repo, repoPath, task.Key, body)
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

	switch target {
	case "linear":
		team := ""
		if proj != nil {
			team = proj.LinearTeam
		}
		if team == "" {
			team = settings.LinearTeam
		}
		created, err := d.runner.CreateLinearIssue(team, task.Title, task.Description, task.Priority, task.Labels)
		if err != nil {
			return nil, fmt.Errorf("création Linear impossible: %w", err)
		}
		newKey = created.Key
		extURL = created.ExternalURL

		// Sync status to Linear if not backlog
		if task.Status != models.StatusBacklog && task.Status != models.StatusToClarify {
			_ = d.runner.UpdateLinearIssueState(newKey, task.Status)
		}

	case "github":
		repo := ""
		repoPath := ""
		if proj != nil {
			repo = proj.GithubRepo
			repoPath = proj.RepoPath
		}
		if repo == "" {
			repo = settings.GithubRepo
		}
		if repoPath == "" {
			repoPath = settings.RepoPath
		}
		created, err := d.runner.CreateGithubIssue(repo, repoPath, task.Title, task.Description, task.Labels)
		if err != nil {
			return nil, fmt.Errorf("création GitHub impossible: %w", err)
		}
		newKey = created.Key
		extURL = created.ExternalURL

		// Sync status if done
		if task.Status == models.StatusDone || task.Status == models.StatusFinished || task.Status == models.StatusToClose {
			_ = d.runner.UpdateGithubIssue(repo, repoPath, newKey, nil, nil, &task.Status, task.Labels, nil)
		}

	default:
		return nil, fmt.Errorf("tracker distant non supporté: %s (choisir 'linear' ou 'github')", target)
	}

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
	case "create_pr", "review":
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
		"clarified":   "to_specify",
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

// NormalizeProjectType keeps the stored project type to the two values the app
// knows: "personal" (a personal board, the only type the daily digest is served
// for) and "standard" (a delivery project), the default.
func NormalizeProjectType(raw string) string {
	if strings.ToLower(strings.TrimSpace(raw)) == "personal" {
		return "personal"
	}
	return "standard"
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
		SELECT p.id, p.name, p.slug, p.description, p.icon, p.color, p.repo_path, p.repo_paths, p.use_worktrees, p.board_id, p.tracker_columns, p.stage_columns, p.sprints, p.issue_types, p.mono_repo, p.git_remote_url, p.linear_team, p.github_repo, p.jira_project, p.issue_tracker, p.tracker_url, p.project_type, p.is_default, p.stage_mapping, p.skill_overrides, p.ai_provider, p.ai_command_template, p.spec_framework, p.parallelism, p.tty_mode, p.external_terminal_command, p.auto_sync_enabled, p.auto_sync_interval_min, p.created_at, p.updated_at,
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
		var stageMappingJSON, skillOverridesJSON, repoPathsJSON string
		var useWorktrees int
		var trackerColumnsJSON, stageColumnsJSON, sprintsJSON, issueTypesJSON string
		var monoRepo int
		var aiProv, aiCmd, specFw, jiraProj, projType, ttyMode, extTerm sql.NullString
		err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.Color, &p.RepoPath, &repoPathsJSON, &useWorktrees, &p.BoardID, &trackerColumnsJSON, &stageColumnsJSON, &sprintsJSON, &issueTypesJSON, &monoRepo, &p.GitRemoteUrl, &p.LinearTeam, &p.GithubRepo, &jiraProj, &p.IssueTracker, &p.TrackerUrl, &projType, &isDefault, &stageMappingJSON, &skillOverridesJSON, &aiProv, &aiCmd, &specFw, &p.Parallelism, &ttyMode, &extTerm, &autoSyncEnabledInt, &autoSyncIntervalMin, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount,
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
		p.SpecFramework = runner.NormalizeSpecFramework(specFw.String)
		p.ProjectType = NormalizeProjectType(projType.String)
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
	var stageMappingJSON, skillOverridesJSON, repoPathsJSON string
	var useWorktrees int
	var trackerColumnsJSON, stageColumnsJSON, sprintsJSON, issueTypesJSON string
	var monoRepo int
	var aiProv, aiCmd, specFw, jiraProj, projType, ttyMode, extTerm sql.NullString
	err := d.conn.QueryRow(`
		SELECT p.id, p.name, p.slug, p.description, p.icon, p.color, p.repo_path, p.repo_paths, p.use_worktrees, p.board_id, p.tracker_columns, p.stage_columns, p.sprints, p.issue_types, p.mono_repo, p.git_remote_url, p.linear_team, p.github_repo, p.jira_project, p.issue_tracker, p.tracker_url, p.project_type, p.is_default, p.stage_mapping, p.skill_overrides, p.ai_provider, p.ai_command_template, p.spec_framework, p.parallelism, p.tty_mode, p.external_terminal_command, p.auto_sync_enabled, p.auto_sync_interval_min, p.created_at, p.updated_at,
		       (SELECT COUNT(*) FROM tasks WHERE project_id = p.id) as task_count
		FROM projects p
		WHERE p.id = ? OR p.slug = ?
	`, id, id).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.Icon, &p.Color, &p.RepoPath, &repoPathsJSON, &useWorktrees, &p.BoardID, &trackerColumnsJSON, &stageColumnsJSON, &sprintsJSON, &issueTypesJSON, &monoRepo, &p.GitRemoteUrl, &p.LinearTeam, &p.GithubRepo, &jiraProj, &p.IssueTracker, &p.TrackerUrl, &projType, &isDefault, &stageMappingJSON, &skillOverridesJSON, &aiProv, &aiCmd, &specFw, &p.Parallelism, &ttyMode, &extTerm, &autoSyncEnabledInt, &autoSyncIntervalMin, &p.CreatedAt, &p.UpdatedAt, &p.TaskCount,
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
	p.SpecFramework = runner.NormalizeSpecFramework(specFw.String)
	p.ProjectType = NormalizeProjectType(projType.String)
	return &p, nil
}

func (d *DB) CreateProject(req models.CreateProjectRequest) (*models.Project, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	name := strings.TrimSpace(req.Name)
	if name == "" {
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
		issueTracker = "linear"
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
	specFramework := runner.NormalizeSpecFramework(req.SpecFramework)
	projectType := NormalizeProjectType(req.ProjectType)

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

	// Types importés : vides à la création, ce qui vaut « les types par défaut ».
	// Les réglages du projet les nomment ensuite, à partir des types réels du
	// tracker.
	issueTypes := runner.NormalizeIssueTypes(req.IssueTypes)
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

	_, err := d.conn.Exec(`
		INSERT INTO projects (id, name, slug, description, icon, color, repo_path, repo_paths, use_worktrees, board_id, tracker_columns, stage_columns, sprints, issue_types, mono_repo, git_remote_url, linear_team, github_repo, jira_project, issue_tracker, tracker_url, project_type, is_default, stage_mapping, skill_overrides, ai_provider, ai_command_template, spec_framework, parallelism, auto_sync_enabled, auto_sync_interval_min, tty_mode, external_terminal_command, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, slug, req.Description, icon, color, req.RepoPath, string(repoPathsBytes), useWorktreesInt, req.BoardID, "[]", "{}", "[]", string(issueTypesBytes), monoRepoInt, gitRemote, req.LinearTeam, githubRepo, jiraProject, issueTracker, req.TrackerUrl, projectType, isDefInt, string(stageMappingBytes), string(skillOverridesBytes), aiProvider, aiCmd, specFramework, parallelism, autoSyncEnabledInt, autoSyncIntervalMin, ttyMode, extTermCmd, now, now)
	if err != nil {
		return nil, err
	}

	return d.getProjectByIDUnsafe(id)
}

func (d *DB) UpdateProject(id string, req models.UpdateProjectRequest) (*models.Project, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	p, err := d.getProjectByIDUnsafe(id)
	if err != nil {
		return nil, err
	}
	if p == nil {
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
	if req.LinearTeam != nil {
		p.LinearTeam = *req.LinearTeam
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
	if req.RepoPaths != nil {
		p.RepoPaths = normalizeRepoPaths(*req.RepoPaths)
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
		p.IssueTypes = runner.NormalizeIssueTypes(*req.IssueTypes)
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
		p.SpecFramework = runner.NormalizeSpecFramework(*req.SpecFramework)
	}
	if req.ProjectType != nil {
		p.ProjectType = NormalizeProjectType(*req.ProjectType)
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
		SET name = ?, slug = ?, description = ?, icon = ?, color = ?, repo_path = ?, repo_paths = ?, use_worktrees = ?, board_id = ?, tracker_columns = ?, stage_columns = ?, sprints = ?, issue_types = ?, mono_repo = ?, git_remote_url = ?, linear_team = ?, github_repo = ?, jira_project = ?, issue_tracker = ?, tracker_url = ?, project_type = ?, is_default = ?, stage_mapping = ?, skill_overrides = ?, ai_provider = ?, ai_command_template = ?, spec_framework = ?, parallelism = ?, auto_sync_enabled = ?, auto_sync_interval_min = ?, tty_mode = ?, external_terminal_command = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Slug, p.Description, p.Icon, p.Color, p.RepoPath, string(repoPathsBytes), useWorktreesInt, p.BoardID, string(trackerColumnsBytes), string(stageColumnsBytes), string(sprintsBytes), string(issueTypesBytes), monoRepoInt, p.GitRemoteUrl, p.LinearTeam, p.GithubRepo, p.JiraProject, p.IssueTracker, p.TrackerUrl, NormalizeProjectType(p.ProjectType), isDefInt, string(stageMappingBytes), string(skillOverridesBytes), p.AIProvider, p.AICommandTemplate, p.SpecFramework, p.Parallelism, autoSyncEnabledInt, p.AutoSyncIntervalMin, p.TtyMode, p.ExternalTerminalCommand, p.UpdatedAt, p.ID)
	if err != nil {
		return nil, err
	}

	return d.getProjectByIDUnsafe(p.ID)
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

func getGitWorktreePaths(repoPath string) []string {
	var paths []string
	cleanRepo, err := filepath.Abs(filepath.Clean(repoPath))
	if err != nil {
		cleanRepo = filepath.Clean(repoPath)
	}
	paths = append(paths, cleanRepo)

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cleanRepo
	out, err := cmd.Output()
	if err != nil {
		return paths
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "worktree ") {
			wtPath := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			if abs, err := filepath.Abs(filepath.Clean(wtPath)); err == nil {
				wtPath = abs
			}
			if wtPath != "" {
				already := false
				for _, p := range paths {
					if p == wtPath {
						already = true
						break
					}
				}
				if !already {
					paths = append(paths, wtPath)
				}
			}
		}
	}
	return paths
}

func (d *DB) GetProjectSkillsStatus(projectIDOrPath string) (*models.ProjectSkillsStatus, error) {
	d.mu.RLock()
	repoPath := projectIDOrPath
	projectID := projectIDOrPath
	projectName := projectIDOrPath
	specFramework := "speckit"

	if proj, _ := d.getProjectByIDUnsafe(projectIDOrPath); proj != nil {
		projectID = proj.ID
		projectName = proj.Name
		if proj.RepoPath != "" {
			repoPath = proj.RepoPath
		}
		if proj.SpecFramework != "" {
			specFramework = proj.SpecFramework
		}
	}
	d.mu.RUnlock()

	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		repoPath = "."
	}

	worktreePaths := getGitWorktreePaths(repoPath)

	res := &models.ProjectSkillsStatus{
		ProjectID:      projectID,
		ProjectName:    projectName,
		RepoPath:       repoPath,
		PathExists:     false,
		IsGitRepo:      false,
		InstalledAll:   true,
		SpecFramework:  specFramework,
		WorktreesCount: len(worktreePaths),
		WorktreePaths:  worktreePaths,
		Skills:         []models.InstalledSkillInfo{},
	}

	fi, err := os.Stat(repoPath)
	if err != nil || !fi.IsDir() {
		res.InstalledAll = false
		return res, nil
	}
	res.PathExists = true

	// Check git repo
	gitDir := filepath.Join(repoPath, ".git")
	if gfi, gErr := os.Stat(gitDir); gErr == nil && gfi.IsDir() {
		res.IsGitRepo = true
		gitCheck := exec.Command("git", "rev-parse", "--is-inside-work-tree")
		gitCheck.Dir = repoPath
		if err := gitCheck.Run(); err == nil {
			branchCmd := exec.Command("git", "branch", "--show-current")
			branchCmd.Dir = repoPath
			if out, err := branchCmd.Output(); err == nil {
				b := strings.TrimSpace(string(out))
				if b != "" && b != "HEAD" {
					res.GitBranch = b
				}
			}
		}
	} else {
		res.InstalledAll = false
	}

	for _, s := range ProjectSkillTemplates(specFramework) {
		candidates := SkillDirsFor(repoPath, s.DirName)
		installed := false
		targetPath := filepath.Join(candidates[0], "SKILL.md")
		for _, dir := range candidates {
			p := filepath.Join(dir, "SKILL.md")
			if _, err := os.Stat(p); err == nil {
				installed = true
				targetPath = p
				break
			}
		}
		if !installed {
			res.InstalledAll = false
		}

		res.Skills = append(res.Skills, models.InstalledSkillInfo{
			ID:          s.ID,
			Name:        s.Name,
			Installed:   installed,
			Path:        targetPath,
			Description: s.Description,
		})
	}

	return res, nil
}

func (d *DB) InstallProjectSkills(projectIDOrPath string, overrides ...string) (*models.ProjectSkillsStatus, error) {
	d.mu.RLock()
	repoPath := projectIDOrPath
	projectID := projectIDOrPath
	projectName := projectIDOrPath
	linearTeam := ""
	githubRepo := ""
	issueTracker := "local"
	specFramework := "speckit"
	aiProvider := "agy"
	aiCommandTemplate := ""

	if proj, _ := d.getProjectByIDUnsafe(projectIDOrPath); proj != nil {
		projectID = proj.ID
		projectName = proj.Name
		if proj.RepoPath != "" {
			repoPath = proj.RepoPath
		}
		if proj.LinearTeam != "" {
			linearTeam = proj.LinearTeam
		}
		if proj.GithubRepo != "" {
			githubRepo = proj.GithubRepo
		}
		if proj.IssueTracker != "" {
			issueTracker = proj.IssueTracker
		}
		if proj.SpecFramework != "" {
			specFramework = proj.SpecFramework
		}
		if proj.AIProvider != "" {
			aiProvider = proj.AIProvider
		}
		if proj.AICommandTemplate != "" {
			aiCommandTemplate = proj.AICommandTemplate
		}
	}
	d.mu.RUnlock()

	if len(overrides) > 0 && strings.TrimSpace(overrides[0]) != "" {
		specFramework = overrides[0]
	}
	specFramework = runner.NormalizeSpecFramework(specFramework)
	if len(overrides) > 1 && strings.TrimSpace(overrides[1]) != "" {
		aiProvider = strings.TrimSpace(overrides[1])
	}
	if len(overrides) > 2 && strings.TrimSpace(overrides[2]) != "" {
		aiCommandTemplate = strings.TrimSpace(overrides[2])
	}

	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("le chemin du dossier de travail (CWD) est obligatoire")
	}

	// Create CWD if doesn't exist
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return nil, fmt.Errorf("impossible de créer le répertoire %s: %w", repoPath, err)
	}

	// Get all worktree paths to scaffold skills into every worktree directory
	targetPaths := getGitWorktreePaths(repoPath)

	// Le contenu vient de la base quand le projet a édité ses skills, du modèle
	// intégré sinon. Une seule source, régénérée dans chaque worktree.
	skillsToInstall := d.EffectiveProjectSkills(projectID, specFramework)

	// Install skills into each target path (root repo and all worktrees)
	for _, targetDir := range targetPaths {
		for _, s := range skillsToInstall {
			// La commande slash, en plus de la skill : c'est elle que Taskflow
			// invoque, et sans elle « /clarify-issue » n'est que du texte.
			if cmdContent, ok := CommandContentFor(s.ID, specFramework); ok {
				cmdPath := SkillCommandPath(targetDir, s.DirName)
				if err := os.MkdirAll(filepath.Dir(cmdPath), 0755); err == nil {
					_ = os.WriteFile(cmdPath, []byte(cmdContent), 0644)
				}
			}

			for _, dir := range SkillDirsFor(targetDir, s.DirName) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					continue
				}
				filePath := filepath.Join(dir, "SKILL.md")
				_ = os.WriteFile(filePath, []byte(s.Content), 0644)
			}
		}

		// Create .taskflow/config.json in each worktree/root
		taskflowDir := filepath.Join(targetDir, ".taskflow")
		_ = os.MkdirAll(taskflowDir, 0755)
		configFile := filepath.Join(taskflowDir, "config.json")
		cfgData := map[string]interface{}{
			"projectId":         projectID,
			"projectName":       projectName,
			"linearTeam":        linearTeam,
			"githubRepo":        githubRepo,
			"issueTracker":      issueTracker,
			"specFramework":     specFramework,
			"aiProvider":        aiProvider,
			"aiCommandTemplate": aiCommandTemplate,
			"skills":            skillDirNames(skillsToInstall),
			"updatedAt":         time.Now().Format(time.RFC3339),
		}
		if bytes, err := json.MarshalIndent(cfgData, "", "  "); err == nil {
			_ = os.WriteFile(configFile, bytes, 0644)
		}
	}

	return d.GetProjectSkillsStatus(projectID)
}

func (d *DB) InitProjectGit(projectIDOrPath string) (*models.ProjectGitInitResult, error) {
	d.mu.RLock()
	repoPath := projectIDOrPath
	if proj, _ := d.getProjectByIDUnsafe(projectIDOrPath); proj != nil {
		if proj.RepoPath != "" {
			repoPath = proj.RepoPath
		}
	}
	d.mu.RUnlock()

	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return nil, fmt.Errorf("le chemin du dossier de travail (CWD) est obligatoire")
	}

	// Create directory if not exists
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return nil, fmt.Errorf("impossible de créer le répertoire %s: %w", repoPath, err)
	}

	// Check if already git repo
	gitDir := filepath.Join(repoPath, ".git")
	isAlreadyGit := false
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		isAlreadyGit = true
	}

	// Run git init -b main
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = repoPath
	if _, err := cmd.CombinedOutput(); err != nil {
		// Fallback for older git without -b flag: git init
		cmd2 := exec.Command("git", "init")
		cmd2.Dir = repoPath
		if out2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return nil, fmt.Errorf("erreur git init: %s (%w)", string(out2), err2)
		}
	}

	// Create a standard .gitignore if none exists
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		defaultGitignore := `# Node / Dependencies
node_modules/
dist/
build/
.env
.env.local

# OS
.DS_Store
Thumbs.db

# Logs & Temp
*.log
tmp/
`
		_ = os.WriteFile(gitignorePath, []byte(defaultGitignore), 0644)
	}

	// Detect current branch
	branchName := "main"
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = repoPath
	if out, err := branchCmd.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" && b != "HEAD" {
			branchName = b
		}
	}

	msg := "Dépôt Git initialisé avec succès (branche main)"
	if isAlreadyGit {
		msg = "Dépôt Git existant validé et synchronisé"
	}

	return &models.ProjectGitInitResult{
		RepoPath:    repoPath,
		IsGitRepo:   true,
		Branch:      branchName,
		Message:     msg,
		Initialized: true,
	}, nil
}

func (d *DB) DetectTrackerStatuses(projectID, tracker, linearTeam, githubRepo string) ([]models.DetectedStatus, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []models.DetectedStatus
	seen := make(map[string]bool)
	jiraProject := ""
	jiraRepoPath := ""

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
			if linearTeam == "" {
				linearTeam = proj.LinearTeam
			}
			if githubRepo == "" {
				githubRepo = proj.GithubRepo
			}
			if jiraProject == "" {
				jiraProject = jiraProjectKeyFor(proj)
			}
			if jiraRepoPath == "" {
				jiraRepoPath = proj.RepoPath
			}
		}
	}

	// 2. Try Linear API if tracker is linear (or team provided)
	if tracker == "linear" || linearTeam != "" {
		linearPath, err := exec.LookPath("linear")
		if err != nil {
			linearPath = "/opt/homebrew/bin/linear"
		}
		if _, err := os.Stat(linearPath); err == nil {
			query := `query { teams { nodes { key name states { nodes { id name color type position } } } } }`
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			cmd := exec.CommandContext(ctx, linearPath, "api", query)
			out, err := cmd.Output()
			cancel()
			if err == nil && len(out) > 0 {
				var gqlResp struct {
					Data struct {
						Teams struct {
							Nodes []struct {
								Key    string `json:"key"`
								Name   string `json:"name"`
								States struct {
									Nodes []struct {
										ID       string  `json:"id"`
										Name     string  `json:"name"`
										Color    string  `json:"color"`
										Type     string  `json:"type"`
										Position float64 `json:"position"`
									} `json:"nodes"`
								} `json:"states"`
							} `json:"nodes"`
						} `json:"teams"`
					} `json:"data"`
				}
				if jErr := json.Unmarshal(out, &gqlResp); jErr == nil {
					for _, t := range gqlResp.Data.Teams.Nodes {
						if linearTeam == "" || strings.EqualFold(t.Key, linearTeam) {
							for _, st := range t.States.Nodes {
								addStatus(st.Name, st.Type, st.Color, "linear")
							}
						}
					}
				}
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

	// 4b. If tracker is jira, read the real workflow statuses from acli when it
	// is reachable, then fall back to the default software-project workflow.
	if tracker == "jira" {
		if acliPath, err := runner.FindCliTool("acli"); err == nil && acliPath != "" && jiraProject != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			cmd := exec.CommandContext(ctx, acliPath, "jira", "workitem", "search",
				"--jql", fmt.Sprintf("project = %s", jiraProject), "--fields", "status", "--limit", "100", "--output", "json")
			if jiraRepoPath != "" {
				cmd.Dir = jiraRepoPath
			}
			out, cErr := cmd.Output()
			cancel()
			if cErr == nil && len(out) > 0 {
				var items []struct {
					Fields struct {
						Status struct {
							Name           string `json:"name"`
							StatusCategory struct {
								Key string `json:"key"`
							} `json:"statusCategory"`
						} `json:"status"`
					} `json:"fields"`
				}
				if jErr := json.Unmarshal(out, &items); jErr == nil {
					for _, it := range items {
						sType := "custom"
						switch it.Fields.Status.StatusCategory.Key {
						case "new":
							sType = "unstarted"
						case "indeterminate":
							sType = "started"
						case "done":
							sType = "completed"
						}
						addStatus(it.Fields.Status.Name, sType, "", "jira")
					}
				}
			}
		}
		addStatus("To Do", "unstarted", "#42526e", "jira")
		addStatus("In Progress", "started", "#0052cc", "jira")
		addStatus("In Review", "started", "#5243aa", "jira")
		addStatus("Done", "completed", "#00875a", "jira")
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
		{"to_specify", "unstarted", "#f59e0b"},
		{"to_implement", "started", "#3b82f6"},
		{"to_test", "started", "#6366f1"},
		{"to_close", "completed", "#10b981"},
	} {
		addStatus(def.name, def.sType, def.color, "preset")
	}

	return results, nil
}

// skillDirNames lists the installed skill directories, for .taskflow/config.json.
func skillDirNames(skills []ProjectSkillTemplate) []string {
	out := make([]string, 0, len(skills))
	for _, s := range skills {
		out = append(out, s.DirName)
	}
	return out
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
	if proj.LinearTeam != "" {
		settings.LinearTeam = proj.LinearTeam
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
