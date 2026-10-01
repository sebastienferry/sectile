package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// The todos of a macro are copied on its tracker, one way (#663, ADR 0046).
// Sectile's list is authoritative: the copy is rewritten after each save and
// never read back, so a hand edit of it on the tracker is replaced by the next
// write. A Jira epic carries it as one comment Sectile owns, a GitHub milestone
// as a block at the end of its description, since a milestone takes no
// comment. A GitLab macro is a pair of labels with a local key (ADR 0030), so
// nothing there could carry it, and its list stays in Sectile.
//
// The framing of a Jira epic is copied the same way, as a second comment
// Sectile owns with its own marker (#636). A milestone carries no framing: its
// description already holds the macro's description and the todo block.

const (
	// todosMirrorMarker is the property that tells Sectile's comment on a Jira
	// epic from the others.
	todosMirrorMarker = "sectile.macroTodos"
	// framingMirrorMarker tells Sectile's framing comment on a Jira epic from
	// the others, the todos comment included.
	framingMirrorMarker = "sectile.macroFraming"
	// todosBlockOpen and todosBlockClose delimit the block Sectile owns at the
	// end of a milestone description.
	todosBlockOpen  = "<!-- sectile:macro-todos -->"
	todosBlockClose = "<!-- /sectile:macro-todos -->"
	// todosMirrorBudget is the size the body of a copy never exceeds, in bytes,
	// kept well under what a Jira comment (32,767 characters, before the ADF
	// overhead) and a milestone description accept.
	todosMirrorBudget = 30000
)

var (
	// todosMirrorDelay is how long the copy waits after the last save of a list
	// before it is queued, so a drag then a rewording make one write. Tests
	// shorten it.
	todosMirrorDelay = 3 * time.Second
	// todosMirrorPauses are the waits between the attempts of one write, which
	// make three attempts in all. Tests shorten them.
	todosMirrorPauses = []time.Duration{time.Second, 3 * time.Second}
)

// macroCopyPart is one field of a macro Sectile copies on its tracker, one
// way: the todos (#663) or the framing (#636). Both go through the same
// eligibility, debounce, queue, retry and status; what is rendered, the marker
// of the comment and the columns holding the state are the part's own.
type macroCopyPart struct {
	// name prefixes the state columns (<name>_mirror_*) and keys the timers.
	// It is one of the two constants below, never input, so the SQL built
	// from it stays safe.
	name   string
	marker string
	opKind TrackerOpKind
	// github is true when a milestone description can carry the part.
	github bool
	// what names the part in the reasons it stays in Sectile.
	what string
	// render is the body of the copy of the macro as it is stored, empty
	// tells there is nothing to copy, written is the note of a success.
	render  func(kind string, m *models.MacroMeta) string
	empty   func(m *models.MacroMeta) bool
	written func(kind string, m *models.MacroMeta) string
	// The runtime messages, each formatted with the macro key first.
	kept, notCopied, nothing, current string
}

var (
	todosCopy = macroCopyPart{
		name:   "todos",
		marker: todosMirrorMarker,
		opKind: TrackerOpEpicTodos,
		github: true,
		what:   "la liste",
		render: func(kind string, m *models.MacroMeta) string { return renderTodosMirror(kind, m.Todos) },
		empty:  func(m *models.MacroMeta) bool { return len(m.Todos) == 0 },
		written: func(kind string, m *models.MacroMeta) string {
			if kind == models.MacroTodosMirrorGithubDescription {
				return fmt.Sprintf("%d todo(s) recopié(s) dans la description du milestone %s", len(m.Todos), m.Key)
			}
			return fmt.Sprintf("%d todo(s) recopié(s) en commentaire sur %s", len(m.Todos), m.Key)
		},
		kept:      "les todos de %s restent dans Sectile : %s",
		notCopied: "todos gardés dans Sectile mais pas recopiés sur %s : %w",
		nothing:   "aucun todo à recopier sur %s",
		current:   "todos de %s déjà à jour",
	}
	framingCopy = macroCopyPart{
		name:   "framing",
		marker: framingMirrorMarker,
		opKind: TrackerOpEpicFraming,
		what:   "le cadrage",
		render: func(_ string, m *models.MacroMeta) string { return renderFramingMirror(m.FramingComment) },
		empty:  func(m *models.MacroMeta) bool { return strings.TrimSpace(m.FramingComment) == "" },
		written: func(_ string, m *models.MacroMeta) string {
			return fmt.Sprintf("cadrage recopié en commentaire sur %s", m.Key)
		},
		kept:      "le cadrage de %s reste dans Sectile : %s",
		notCopied: "cadrage gardé dans Sectile mais pas recopié sur %s : %w",
		nothing:   "aucun cadrage à recopier sur %s",
		current:   "cadrage de %s déjà à jour",
	}
)

// columns lists the state columns of the part, in the order
// todosMirrorState is scanned.
func (p macroCopyPart) columns() string {
	return fmt.Sprintf("%[1]s_mirror_ref, %[1]s_mirror_hash, %[1]s_mirror_error, %[1]s_mirror_credential, %[1]s_mirror_at", p.name)
}

// todosMirrorState is what the last writes of a copy, todos or framing, left
// on the macro row.
type todosMirrorState struct {
	ref        string
	hash       string
	err        string
	credential string
	at         *time.Time
}

// todosMirrorScope answers, for the macros of one project, where their todos
// are copied. The tracker is resolved once, whatever the number of macros.
type todosMirrorScope struct {
	proj *models.Project
	// milestones are the milestone keys the tracker listed, nil when the list
	// was not read: a key of the right shape is then taken at its word, and the
	// write is what finds out it names nothing.
	milestones map[string]bool
	// writer is the Jira comment writer, nil when the tracker has none, for the
	// reason refusal gives.
	writer  tracker.MarkedCommentWriter
	refusal string
}

func (d *DB) todosMirrorScope(proj *models.Project, milestones map[string]bool) todosMirrorScope {
	scope := todosMirrorScope{proj: proj, milestones: milestones}
	if proj == nil || proj.IssueTracker != "jira" {
		return scope
	}
	ts, err := d.TrackerForProject(proj)
	switch {
	case err != nil:
		scope.refusal = err.Error()
	case !ts.Supports(tracker.CapComment):
		scope.refusal = tracker.Unsupported(ts.Name(), tracker.CapComment).Error()
	default:
		if writer, ok := ts.(tracker.MarkedCommentWriter); ok {
			scope.writer = writer
		} else {
			scope.refusal = tracker.Unsupported(ts.Name(), tracker.CapComment).Error()
		}
	}
	return scope
}

// eligibility tells where the todos of one macro are copied, or why they stay
// in Sectile (FR12).
func (s todosMirrorScope) eligibility(key string) (kind string, reason string) {
	return s.eligibilityOf(todosCopy, key)
}

// eligibilityOf tells where one part of a macro is copied, or why it stays in
// Sectile. Only the todos have a milestone copy (#636, D1).
func (s todosMirrorScope) eligibilityOf(part macroCopyPart, key string) (kind string, reason string) {
	proj := s.proj
	key = strings.TrimSpace(key)
	switch {
	case proj == nil:
		return "", "projet introuvable"
	case strings.EqualFold(proj.IssueTracker, "gitlab"):
		return "", "une macro GitLab n'a pas de ticket qui puisse porter " + part.what
	// Jira first: a Jira project may also name a GitHub repository for its
	// code, and its epics are still Jira epics.
	case proj.IssueTracker == "jira":
		if isForeignMacro(key, proj) {
			return "", "épic d'un autre projet Jira, que Sectile lit sans y écrire"
		}
		if isMilestoneKey(key) {
			return "", "macro locale, sans épic Jira"
		}
		if s.writer == nil {
			return "", s.refusal
		}
		return models.MacroTodosMirrorJiraComment, ""
	case githubMilestoneMacros(proj):
		if !part.github {
			return "", "un milestone GitHub ne prend pas de commentaire"
		}
		if !isMilestoneKey(key) {
			return "", "macro sans milestone GitHub"
		}
		if s.milestones != nil && !s.milestones[strings.ToUpper(key)] {
			return "", fmt.Sprintf("le milestone %s n'existe pas sur GitHub", key)
		}
		return models.MacroTodosMirrorGithubDescription, ""
	default:
		return "", "projet local"
	}
}

// renderTodosMirror renders the body of the copy of a list, within the size
// budget (FR14, FR15). The Jira comment shows a ballot box per line, since ADF
// has no task item inside an ordered list; the milestone description uses
// GitHub's own checkboxes and sits between the block markers. An empty list
// says so on Jira and removes the block on GitHub.
func renderTodosMirror(kind string, todos []models.MacroTodo) string {
	const (
		heading = "### 📋 [Sectile] Todos"
		footer  = "_Liste tenue dans Sectile : une modification faite ici est remplacée à la prochaine mise à jour._"
	)
	github := kind == models.MacroTodosMirrorGithubDescription
	if len(todos) == 0 {
		if github {
			return ""
		}
		return heading + "\n\n_Aucun todo pour l'instant._\n\n" + footer
	}

	lines := make([]string, 0, len(todos))
	for i, todo := range todos {
		box := "☐"
		if todo.Done {
			box = "☑"
		}
		if github {
			box = "[ ]"
			if todo.Done {
				box = "[x]"
			}
		}
		text := strings.Join(strings.Fields(todo.Text), " ")
		if github {
			// A todo quoting a block marker would end the block early, and the
			// next split would keep the rest as a person's text. GitHub shows
			// the escaped form as typed.
			text = strings.ReplaceAll(text, "<!--", "&lt;!--")
		}
		line := fmt.Sprintf("%d. %s %s", i+1, box, text)
		if key := strings.TrimSpace(todo.StoryKey); key != "" {
			line += " - " + key
		}
		lines = append(lines, line)
	}

	wrap := func(list []string, more int) string {
		var b strings.Builder
		if github {
			b.WriteString(todosBlockOpen + "\n")
		}
		b.WriteString(heading + "\n\n")
		b.WriteString(strings.Join(list, "\n"))
		if more > 0 {
			fmt.Fprintf(&b, "\n\n… et %d autres todos dans Sectile.", more)
		}
		b.WriteString("\n\n" + footer)
		if github {
			b.WriteString("\n" + todosBlockClose)
		}
		return b.String()
	}
	body := wrap(lines, 0)
	for kept := len(lines) - 1; len(body) > todosMirrorBudget && kept >= 0; kept-- {
		body = wrap(lines[:kept], len(lines)-kept)
	}
	return body
}

// renderFramingMirror renders the body of the framing comment of a Jira epic,
// within the size budget (#636, FR8, FR9). It carries no date, so two equal
// saves render the same body and make one write. An empty framing says so; it
// is only ever written over a comment that exists (FR11).
func renderFramingMirror(framing string) string {
	const (
		heading   = "### 🧭 [Sectile] Cadrage"
		footer    = "_Cadrage tenu dans Sectile : une modification faite ici est remplacée à la prochaine mise à jour._"
		truncated = "_… cadrage tronqué, texte complet dans Sectile._"
	)
	text := strings.TrimSpace(framing)
	if text == "" {
		return heading + "\n\n_Aucun cadrage pour l'instant._\n\n" + footer
	}
	body := heading + "\n\n" + text + "\n\n" + footer
	if len(body) <= todosMirrorBudget {
		return body
	}
	room := todosMirrorBudget - len(heading) - len(truncated) - len(footer) - 3*len("\n\n")
	cut := text[:room]
	if line := strings.LastIndex(cut, "\n"); line > 0 {
		cut = cut[:line]
	} else {
		// One line longer than the budget: cut on a character, never inside one.
		for len(cut) > 0 && !utf8.RuneStart(text[len(cut)]) {
			cut = cut[:len(cut)-1]
		}
	}
	return heading + "\n\n" + strings.TrimRight(cut, " \t\r\n") + "\n\n" + truncated + "\n\n" + footer
}

// todosMirrorHash is the hash of a body, "" for the empty one: a milestone
// with no block is what a macro never copied already shows.
func todosMirrorHash(body string) string {
	if body == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// splitTodosBlock separates a milestone description into the text Sectile does
// not own and its todo block. A description without a block returns whole,
// with an empty block. An opening marker without its closing one gives Sectile
// everything from the marker on; text a person added after the block stays
// theirs.
func splitTodosBlock(description string) (outside string, block string) {
	start := strings.Index(description, todosBlockOpen)
	if start < 0 {
		return description, ""
	}
	before := strings.TrimRight(description[:start], " \t\r\n")
	rest := description[start:]
	end := strings.Index(rest, todosBlockClose)
	if end < 0 {
		return before, rest
	}
	block = rest[:end+len(todosBlockClose)]
	after := strings.TrimSpace(rest[end+len(todosBlockClose):])
	switch {
	case after == "":
		return before, block
	case before == "":
		return after, block
	default:
		return before + "\n\n" + after, block
	}
}

// joinTodosBlock writes the text outside the block, a blank line, then the
// block; the text alone when there is no block.
func joinTodosBlock(outside, block string) string {
	if block == "" {
		return outside
	}
	trimmed := strings.TrimRight(outside, " \t\r\n")
	if trimmed == "" {
		return block
	}
	return trimmed + "\n\n" + block
}

// milestoneNumber reads the number of an "M-<n>" key, 0 for any other.
func milestoneNumber(key string) int {
	if !isMilestoneKey(key) {
		return 0
	}
	var n int
	_, _ = fmt.Sscanf(strings.ToUpper(strings.TrimSpace(key)), "M-%d", &n)
	return n
}

// todosMirrorStatus is the status of the copy of a macro, from where it goes
// and what the last writes left (FR17).
func todosMirrorStatus(m *models.MacroMeta, kind, reason string, state todosMirrorState) *models.MacroTodosMirror {
	return macroCopyStatus(todosCopy, m, kind, reason, state)
}

// macroCopyStatus is the status of the copy of one part of a macro.
func macroCopyStatus(part macroCopyPart, m *models.MacroMeta, kind, reason string, state todosMirrorState) *models.MacroTodosMirror {
	status := &models.MacroTodosMirror{Kind: kind, Reason: reason}
	if kind == "" {
		return status
	}
	// A macro whose part was never copied and holds nothing has nothing to
	// show: it is not waiting for a write.
	never := state.hash == "" && state.ref == ""
	status.UpToDate = (never && part.empty(m)) || (!never && state.hash == todosMirrorHash(part.render(kind, m)))
	status.Error = state.err
	status.CredentialMissing = state.credential
	status.WrittenAt = state.at
	status.URL = m.ExternalURL
	if kind == models.MacroTodosMirrorJiraComment && m.ExternalURL != "" && state.ref != "" {
		sep := "?"
		if strings.Contains(m.ExternalURL, "?") {
			sep = "&"
		}
		status.URL = m.ExternalURL + sep + "focusedCommentId=" + state.ref
	}
	return status
}

// readTodosMirrorState reads what the last writes of the todos copy left.
func (d *DB) readTodosMirrorState(projectID, key string) (todosMirrorState, error) {
	return d.readMacroCopyState(todosCopy, projectID, key)
}

// readMacroCopyState reads what the last writes of the copy of a part left.
func (d *DB) readMacroCopyState(part macroCopyPart, projectID, key string) (todosMirrorState, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var state todosMirrorState
	var at sql.NullTime
	err := d.conn.QueryRow(`SELECT `+part.columns()+`
		FROM macros WHERE project_id = ? AND key = ?`, projectID, key).Scan(&state.ref, &state.hash, &state.err, &state.credential, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if at.Valid {
		state.at = &at.Time
	}
	return state, err
}

// fillMacroCopies sets the copy statuses of a macro a handler returns, the
// ones GetProjectMacros fills on a list: its todos and its framing.
func (d *DB) fillMacroCopies(proj *models.Project, m *models.MacroMeta) {
	if m == nil || proj == nil {
		return
	}
	state, err := d.readTodosMirrorState(proj.ID, m.Key)
	if err != nil {
		return
	}
	framing, err := d.readMacroCopyState(framingCopy, proj.ID, m.Key)
	if err != nil {
		return
	}
	scope := d.todosMirrorScope(proj, nil)
	kind, reason := scope.eligibility(m.Key)
	if m.ExternalURL == "" {
		// A macro a save returns has no address yet: the list read computes
		// it, and the copy's link needs it.
		one := []models.MacroMeta{*m}
		d.fillMacroURLsFromTasks(proj.ID, one)
		m.ExternalURL = one[0].ExternalURL
	}
	if m.ExternalURL == "" && kind == models.MacroTodosMirrorGithubDescription {
		if repo := trackerapi.CleanGithubRepo(proj.GithubRepo); repo != "" {
			m.ExternalURL = fmt.Sprintf("https://github.com/%s/milestone/%d", repo, milestoneNumber(m.Key))
		}
	}
	m.TodosMirror = todosMirrorStatus(m, kind, reason, state)
	kind, reason = scope.eligibilityOf(framingCopy, m.Key)
	m.FramingMirror = macroCopyStatus(framingCopy, m, kind, reason, framing)
}

// TodosMirrorRefusal is why a macro's todos are not copied on its tracker, ""
// when they are. The republish route asks before queuing anything (FR18).
func (d *DB) TodosMirrorRefusal(projectID, key string) string {
	return d.macroCopyRefusal(todosCopy, projectID, key)
}

// FramingMirrorRefusal is why a macro's framing is not copied on its tracker,
// "" when it is (#636, FR14).
func (d *DB) FramingMirrorRefusal(projectID, key string) string {
	return d.macroCopyRefusal(framingCopy, projectID, key)
}

func (d *DB) macroCopyRefusal(part macroCopyPart, projectID, key string) string {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return "projet non trouvé"
	}
	if _, reason := d.todosMirrorScope(proj, nil).eligibilityOf(part, key); reason != "" {
		return fmt.Sprintf(part.kept, strings.TrimSpace(key), reason)
	}
	return ""
}

// todosMirrorTimer is the pending copy of one macro on this instance, with the
// person whose last save scheduled it.
type todosMirrorTimer struct {
	mu         sync.Mutex
	timer      *time.Timer
	userID     string
	unattended bool
}

// scheduleTodosMirror queues the copy of a macro's todos a moment after the
// last save of its list on this instance, as the person ctx names (FR6, FR7).
func (d *DB) scheduleTodosMirror(ctx context.Context, projectID, key string) {
	d.scheduleMacroCopy(ctx, todosCopy, projectID, key)
}

// scheduleMacroCopy queues the copy of one part of a macro a moment after its
// last save on this instance, as the person ctx names. A macro whose part
// stays in Sectile schedules nothing, so no tracker call is ever made for it.
// Each part has its own timer: a todos save never cancels a pending framing
// write. Several instances may each queue a write; the hash makes the second
// one write nothing.
func (d *DB) scheduleMacroCopy(ctx context.Context, part macroCopyPart, projectID, key string) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return
	}
	if kind, _ := d.todosMirrorScope(proj, nil).eligibilityOf(part, key); kind == "" {
		return
	}
	value, _ := d.todosMirrorTimers.LoadOrStore(projectID+"\x00"+key+"\x00"+part.name, &todosMirrorTimer{})
	pending := value.(*todosMirrorTimer)
	pending.mu.Lock()
	defer pending.mu.Unlock()
	pending.userID = tracker.ActingUser(ctx)
	pending.unattended = tracker.Unattended(ctx)
	if pending.timer != nil {
		pending.timer.Stop()
	}
	pending.timer = time.AfterFunc(todosMirrorDelay, func() {
		pending.mu.Lock()
		user, unattended := pending.userID, pending.unattended
		pending.timer = nil
		pending.mu.Unlock()
		// The context of the save is long gone: the write is signed by the
		// person it named, as #482 requires of every queued write.
		opCtx := tracker.WithActingUser(context.Background(), user)
		if unattended {
			opCtx = tracker.WithUnattended(opCtx)
		}
		if _, err := d.EnqueueTrackerOp(opCtx, TrackerOp{
			Kind:      part.opKind,
			ProjectID: projectID,
			TaskKey:   key,
			EpicKey:   key,
		}); err != nil {
			log.Printf("[macros] recopie (%s) de %s non mise en file : %v", part.name, key, err)
		}
	})
}

// stopTodosMirrorTimers drops the copies still waiting for their delay, which a
// closed database could not queue.
func (d *DB) stopTodosMirrorTimers() {
	d.todosMirrorTimers.Range(func(_, value any) bool {
		pending := value.(*todosMirrorTimer)
		pending.mu.Lock()
		if pending.timer != nil {
			pending.timer.Stop()
			pending.timer = nil
		}
		pending.mu.Unlock()
		return true
	})
}

// retryTransient runs a tracker write up to three times while it fails for a
// reason that may pass, waiting longer each time (FR13). Any other failure is
// returned at once.
func retryTransient(ctx context.Context, write func(context.Context) error) error {
	var err error
	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, macroWriteTimeout)
		err = write(attemptCtx)
		cancel()
		if err == nil || !trackerapi.IsTransient(err) || attempt >= len(todosMirrorPauses) || ctx.Err() != nil {
			return err
		}
		select {
		case <-time.After(todosMirrorPauses[attempt]):
		case <-ctx.Done():
			return err
		}
	}
}

// PushMacroTodosMirror writes the current todos of a macro on its tracker copy.
// It performs the tracker calls itself, so it is only ever run from a queued
// activity (TrackerOpEpicTodos). The list is read when the write runs, not when
// it was scheduled; a body equal to the last one written makes no call unless
// force asks for it, which is what republishing does.
func (d *DB) PushMacroTodosMirror(ctx context.Context, projectID, key string, force bool) (string, error) {
	return d.pushMacroCopy(ctx, todosCopy, projectID, key, force)
}

// PushMacroFramingMirror writes the current framing of a Jira epic on the
// comment Sectile owns for it (#636). Like the todos copy it only ever runs
// from a queued activity (TrackerOpEpicFraming). An empty framing never
// creates a comment; it rewrites one that exists, never deletes it.
func (d *DB) PushMacroFramingMirror(ctx context.Context, projectID, key string, force bool) (string, error) {
	return d.pushMacroCopy(ctx, framingCopy, projectID, key, force)
}

func (d *DB) pushMacroCopy(ctx context.Context, part macroCopyPart, projectID, key string, force bool) (string, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return "", fmt.Errorf("projet non trouvé")
	}
	// Checked again: an epic may have become foreign, a project may have
	// changed tracker since the save.
	scope := d.todosMirrorScope(proj, nil)
	kind, reason := scope.eligibilityOf(part, key)
	if kind == "" {
		return "", fmt.Errorf(part.kept, key, reason)
	}
	meta, err := d.readMacroRow(projectID, key)
	if err != nil {
		return "", err
	}
	state, err := d.readMacroCopyState(part, projectID, key)
	if err != nil {
		return "", err
	}
	if part.empty(meta) && state.hash == "" && state.ref == "" {
		return fmt.Sprintf(part.nothing, key), nil
	}
	body := part.render(kind, meta)
	hash := todosMirrorHash(body)
	if !force && hash == state.hash && (kind != models.MacroTodosMirrorJiraComment || state.ref != "") {
		return fmt.Sprintf(part.current, key), nil
	}

	ref := state.ref
	switch kind {
	case models.MacroTodosMirrorJiraComment:
		err = retryTransient(ctx, func(ctx context.Context) error {
			id, err := scope.writer.UpsertMarkedComment(ctx, tracker.UpsertMarkedCommentRequest{
				Project:   proj,
				Key:       key,
				CommentID: ref,
				Marker:    part.marker,
				Value:     map[string]any{"macroKey": key},
				Body:      body,
			})
			if err == nil {
				ref = id
			}
			return err
		})
	case models.MacroTodosMirrorGithubDescription:
		ref = ""
		err = d.writeMilestoneTodosBlock(ctx, proj, key, body)
	}
	if err != nil {
		d.recordMacroCopyFailure(part, projectID, key, err)
		return "", fmt.Errorf(part.notCopied, key, err)
	}
	d.recordMacroCopySuccess(part, projectID, key, ref, hash)
	return part.written(kind, meta), nil
}

// writeMilestoneTodosBlock replaces the todo block of a milestone description,
// leaving the text outside it as GitHub has it (FR11).
func (d *DB) writeMilestoneTodosBlock(ctx context.Context, proj *models.Project, key, block string) error {
	number := milestoneNumber(key)
	if number == 0 {
		return fmt.Errorf("%s n'est pas un milestone GitHub", key)
	}
	client, err := d.trackerForWrite(ctx, "github", proj.ID)
	if err != nil {
		return err
	}
	var current *trackerapi.GithubMilestoneItem
	if err := retryTransient(ctx, func(context.Context) error {
		var err error
		current, err = client.GetGithubMilestone(proj.GithubRepo, proj.RepoPath, number)
		return err
	}); err != nil {
		var httpErr *trackerapi.HTTPError
		if errors.As(err, &httpErr) && httpErr.Status == http.StatusNotFound {
			return fmt.Errorf("le milestone %s n'existe pas sur GitHub", key)
		}
		return err
	}
	outside, _ := splitTodosBlock(current.Description)
	next := joinTodosBlock(outside, block)
	if next == current.Description {
		return nil
	}
	return retryTransient(ctx, func(context.Context) error {
		return client.SetGithubMilestoneDescription(proj.GithubRepo, proj.RepoPath, number, next)
	})
}

// readMacroRow reads the stored list, description and framing of one macro, without
// the tracker read GetProjectMacros makes.
func (d *DB) readMacroRow(projectID, key string) (*models.MacroMeta, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	meta := &models.MacroMeta{ProjectID: projectID, Key: key, Todos: []models.MacroTodo{}}
	var todosJSON string
	err := d.conn.QueryRow(`SELECT description, framing_comment, todos FROM macros WHERE project_id = ? AND key = ?`, projectID, key).Scan(&meta.Description, &meta.FramingComment, &todosJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return meta, nil
	}
	if err != nil {
		return nil, err
	}
	meta.Todos = parseMacroTodos(todosJSON)
	return meta, nil
}

// recordTodosMirrorSuccess stores what a successful write left, and clears the
// last failure. Only the copy writes these columns: a list save never touches
// them, so it cannot clobber them.
func (d *DB) recordTodosMirrorSuccess(projectID, key, ref, hash string) {
	d.recordMacroCopySuccess(todosCopy, projectID, key, ref, hash)
}

func (d *DB) recordMacroCopySuccess(part macroCopyPart, projectID, key, ref, hash string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.conn.Exec(fmt.Sprintf(`UPDATE macros SET %[1]s_mirror_ref = ?, %[1]s_mirror_hash = ?, %[1]s_mirror_error = '', %[1]s_mirror_credential = '', %[1]s_mirror_at = ?
		WHERE project_id = ? AND key = ?`, part.name), ref, hash, time.Now().UTC(), projectID, key); err != nil {
		log.Printf("[macros] état de la recopie (%s) de %s non enregistré : %v", part.name, key, err)
	}
}

// recordMacroCopyFailure keeps the last failure, and the tracker whose token
// it lacked, until a write succeeds.
func (d *DB) recordMacroCopyFailure(part macroCopyPart, projectID, key string, cause error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, err := d.conn.Exec(fmt.Sprintf(`UPDATE macros SET %[1]s_mirror_error = ?, %[1]s_mirror_credential = ? WHERE project_id = ? AND key = ?`, part.name),
		cause.Error(), trackerapi.MissingCredentialTracker(cause), projectID, key); err != nil {
		log.Printf("[macros] échec de la recopie (%s) de %s non enregistré : %v", part.name, key, err)
	}
}

// GetMacro reads one macro as a client sees it: its fields, its computed flags
// and the status of the copies of its todos and its framing on the tracker.
func (d *DB) GetMacro(projectID, key string) (*models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	key = strings.TrimSpace(key)
	if projectID == "" || key == "" {
		return nil, fmt.Errorf("projet et clé de macro obligatoires")
	}
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}
	macros, err := d.GetProjectMacros(projectID)
	if err != nil {
		return nil, err
	}
	for i := range macros {
		if strings.EqualFold(macros[i].Key, key) {
			return &macros[i], nil
		}
	}
	return nil, fmt.Errorf("macro %s introuvable dans le projet", key)
}

// MacroTodoInput is one line of a full ordered list an agent saves. It carries
// no story key: only story creation attaches a line.
type MacroTodoInput struct {
	ID                   string
	Text                 string
	Done                 bool
	TargetProjectID      string
	TargetTrackerProject string
}

// ReplaceMacroTodos saves a full ordered list given by an agent (FR20). It
// refuses a blank text, an unknown id or a repeated id before saving anything,
// keeps the story key and the origin of every known line, removes the stored
// lines the list omits, and saves through the panel's path, which schedules the
// copy on the tracker as the person ctx names.
func (d *DB) ReplaceMacroTodos(ctx context.Context, projectID, key string, items []MacroTodoInput) (*models.MacroMeta, error) {
	current, err := d.GetMacro(projectID, key)
	if err != nil {
		return nil, err
	}
	key = current.Key
	stored := make(map[string]models.MacroTodo, len(current.Todos))
	for _, todo := range current.Todos {
		stored[todo.ID] = todo
	}

	seen := make(map[string]bool, len(items))
	todos := make([]models.MacroTodo, 0, len(items))
	for i, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			return nil, fmt.Errorf("le todo n°%d n'a pas de texte : rien n'a été enregistré", i+1)
		}
		todo := models.MacroTodo{}
		if id := strings.TrimSpace(item.ID); id != "" {
			if seen[id] {
				return nil, fmt.Errorf("le todo %s figure deux fois dans la liste : rien n'a été enregistré", id)
			}
			known, ok := stored[id]
			if !ok {
				return nil, fmt.Errorf("le todo %s n'existe pas sur %s : rien n'a été enregistré", id, key)
			}
			seen[id] = true
			todo = known
		}
		todo.Text = text
		todo.Done = item.Done
		todo.TargetProjectID = item.TargetProjectID
		todo.TargetTrackerProject = item.TargetTrackerProject
		todos = append(todos, todo)
	}

	if _, err := d.UpdateMacro(ctx, current.ProjectID, key, nil, nil, nil, nil, &todos, nil); err != nil {
		return nil, err
	}
	return d.GetMacro(current.ProjectID, key)
}
