package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// On GitLab a board column is a board list, and a list groups the issues
// carrying its label. Every board also has the implicit Open list (open issues
// with no list label) and Closed. Moving a card between columns therefore
// swaps list labels, or closes and reopens the issue; the `#<stage>` label is
// never part of that arithmetic (D7).

const (
	gitlabOpenStatus   = "opened"
	gitlabClosedStatus = "closed"
)

type gitlabBoard struct {
	ID    int64             `json:"id"`
	Name  string            `json:"name"`
	Lists []gitlabBoardList `json:"lists"`
}

// gitlabBoardList is one list of a board. Only label lists become columns: the
// assignee, milestone and iteration lists of Premium group nothing a status
// can name.
type gitlabBoardList struct {
	ID       int64 `json:"id"`
	Position int   `json:"position"`
	Label    *struct {
		Name string `json:"name"`
	} `json:"label"`
}

// gitlabBoards reads the project's boards with their lists, which the boards
// endpoint embeds.
func (c *Client) gitlabBoards(ctx context.Context, projectPath string) ([]gitlabBoard, error) {
	items, err := c.gitlabPages(ctx, "/projects/"+gitlabProjectSegment(projectPath)+"/boards", nil)
	if err != nil {
		return nil, err
	}
	boards := make([]gitlabBoard, 0, len(items))
	for _, raw := range items {
		var b gitlabBoard
		if json.Unmarshal(raw, &b) == nil && b.ID > 0 {
			boards = append(boards, b)
		}
	}
	return boards, nil
}

// labelLists returns a board's label lists in board order.
func (b gitlabBoard) labelLists() []string {
	lists := append([]gitlabBoardList{}, b.Lists...)
	sort.SliceStable(lists, func(i, j int) bool { return lists[i].Position < lists[j].Position })
	var names []string
	for _, l := range lists {
		if l.Label != nil && strings.TrimSpace(l.Label.Name) != "" {
			names = append(names, l.Label.Name)
		}
	}
	return names
}

// boardListLabels is the set of list labels of every board of the project.
func boardListLabels(boards []gitlabBoard) map[string]bool {
	set := map[string]bool{}
	for _, b := range boards {
		for _, name := range b.labelLists() {
			set[name] = true
		}
	}
	return set
}

func (g *GitlabAdapter) ListBoards(ctx context.Context, req tracker.BoardsRequest) ([]models.TrackerBoard, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	boards, err := c.gitlabBoards(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	out := make([]models.TrackerBoard, 0, len(boards))
	for _, b := range boards {
		name := b.Name
		if strings.TrimSpace(name) == "" {
			name = "Board " + strconv.FormatInt(b.ID, 10)
		}
		out = append(out, models.TrackerBoard{ID: strconv.FormatInt(b.ID, 10), Name: name, Type: "gitlab"})
	}
	return out, nil
}

// ListBoardColumns returns Open, the board's label lists in order, then
// Closed. The board is the one asked for, else the project's, else its first.
func (g *GitlabAdapter) ListBoardColumns(ctx context.Context, req tracker.BoardRequest) ([]models.TrackerColumn, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	boards, err := c.gitlabBoards(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	boardID := strings.TrimSpace(req.BoardID)
	if boardID == "" && req.Project != nil {
		boardID = strings.TrimSpace(req.Project.BoardID)
	}
	var board *gitlabBoard
	for i := range boards {
		if boardID == "" || strconv.FormatInt(boards[i].ID, 10) == boardID {
			board = &boards[i]
			break
		}
	}
	if board == nil {
		if boardID == "" {
			return nil, fmt.Errorf("le projet GitLab %s n'a aucun board", projectPath)
		}
		return nil, fmt.Errorf("board GitLab %s introuvable dans le projet %s", boardID, projectPath)
	}
	columns := []models.TrackerColumn{{Name: "Open", Statuses: []string{gitlabOpenStatus}}}
	for _, name := range board.labelLists() {
		columns = append(columns, models.TrackerColumn{Name: name, Statuses: []string{name}})
	}
	columns = append(columns, models.TrackerColumn{Name: "Closed", Statuses: []string{gitlabClosedStatus}})
	return columns, nil
}

// ListStatuses is opened, every list label of the project's boards, and
// closed, which is everything a card's column can be read from.
func (g *GitlabAdapter) ListStatuses(ctx context.Context, req tracker.ProjectRequest) ([]tracker.TrackerStatus, error) {
	c, projectPath, err := g.forProject(ctx, req.Project)
	if err != nil {
		return nil, err
	}
	boards, err := c.gitlabBoards(ctx, projectPath)
	if err != nil {
		return nil, err
	}
	out := []tracker.TrackerStatus{{ID: gitlabOpenStatus, Name: gitlabOpenStatus, Category: "new"}}
	seen := map[string]bool{}
	for _, b := range boards {
		for _, name := range b.labelLists() {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, tracker.TrackerStatus{ID: name, Name: name, Category: "indeterminate"})
		}
	}
	return append(out, tracker.TrackerStatus{ID: gitlabClosedStatus, Name: gitlabClosedStatus, Category: "done"}), nil
}

// gitlabColumnMove is the label and state change that puts an issue into one
// column.
type gitlabColumnMove struct {
	add, remove []string
	stateEvent  string
}

// columnMove computes the move into a column named by its status: closed
// closes, opened drops every list label, a list label replaces the others.
func (g *GitlabAdapter) columnMove(ctx context.Context, c *Client, projectPath, status string) (gitlabColumnMove, error) {
	status = strings.TrimSpace(status)
	if strings.EqualFold(status, gitlabClosedStatus) {
		return gitlabColumnMove{stateEvent: "close"}, nil
	}
	boards, err := c.gitlabBoards(ctx, projectPath)
	if err != nil {
		return gitlabColumnMove{}, err
	}
	lists := boardListLabels(boards)
	move := gitlabColumnMove{stateEvent: "reopen"}
	target := ""
	if !strings.EqualFold(status, gitlabOpenStatus) && !strings.EqualFold(status, "open") {
		for name := range lists {
			if strings.EqualFold(name, status) {
				target = name
			}
		}
		if target == "" {
			return gitlabColumnMove{}, fmt.Errorf("colonne GitLab inconnue : %q", status)
		}
		move.add = []string{target}
	}
	for name := range lists {
		if name != target {
			move.remove = append(move.remove, name)
		}
	}
	sort.Strings(move.remove)
	return move, nil
}

// Transition moves an issue to the column its status names.
func (g *GitlabAdapter) Transition(ctx context.Context, key string, status string) error {
	iid, err := gitlabIssueIID(key)
	if err != nil {
		return err
	}
	c, projectPath, err := g.forWrite(ctx, nil)
	if err != nil {
		return err
	}
	move, err := g.columnMove(ctx, c, projectPath, status)
	if err != nil {
		return err
	}
	payload := map[string]any{"state_event": move.stateEvent}
	if len(move.add) > 0 {
		payload["add_labels"] = strings.Join(move.add, ",")
	}
	if len(move.remove) > 0 {
		payload["remove_labels"] = strings.Join(move.remove, ",")
	}
	return c.gitlab(ctx, http.MethodPut, gitlabIssuePath(projectPath, iid), nil, payload, nil)
}
