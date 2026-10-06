package trackerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"tasks/internal/models"
)

// A Jira project may map its epic priority and quarter to custom fields of
// its epics (#680). Nothing here knows a field: the candidates are whatever
// closed-list custom fields an epic's edit screen carries, told apart by the
// type their schema declares, never by an id or a name.

// jiraFieldKind tells a single select from a cascading select by the suffix
// of the custom type an edit screen declares, "" for any other field.
func jiraFieldKind(custom string) string {
	switch {
	case strings.HasSuffix(custom, ":select"):
		return models.EpicFieldSelect
	case strings.HasSuffix(custom, ":cascadingselect"):
		return models.EpicFieldCascade
	}
	return ""
}

type jiraFieldOption struct {
	ID       string            `json:"id"`
	Value    string            `json:"value"`
	Disabled bool              `json:"disabled"`
	Children []jiraFieldOption `json:"children"`
}

// epicFieldOptions keeps the options a write may still send: a disabled
// option is shown by Jira on the items that carry it, and refused on any
// other.
func epicFieldOptions(in []jiraFieldOption) []models.EpicFieldOption {
	out := []models.EpicFieldOption{}
	for _, o := range in {
		if o.Disabled || o.ID == "" {
			continue
		}
		option := models.EpicFieldOption{ID: o.ID, Value: o.Value}
		if len(o.Children) > 0 {
			option.Children = epicFieldOptions(o.Children)
		}
		out = append(out, option)
	}
	return out
}

// EpicAxisFieldCandidates lists the single and cascading select custom
// fields of one epic's edit screen, by name.
func (j *JiraAdapter) EpicAxisFieldCandidates(ctx context.Context, project *models.Project, epicKey string) ([]models.EpicFieldCandidate, error) {
	key, err := cleanJiraKey(epicKey)
	if err != nil {
		return nil, err
	}
	c, err := j.forProject(ctx, project)
	if err != nil {
		return nil, err
	}
	var meta struct {
		Fields map[string]struct {
			Name   string `json:"name"`
			Schema struct {
				Custom string `json:"custom"`
			} `json:"schema"`
			AllowedValues []jiraFieldOption `json:"allowedValues"`
		} `json:"fields"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"/editmeta", nil, nil, &meta); err != nil {
		return nil, err
	}
	out := []models.EpicFieldCandidate{}
	for id, field := range meta.Fields {
		kind := jiraFieldKind(field.Schema.Custom)
		if kind == "" {
			continue
		}
		out = append(out, models.EpicFieldCandidate{ID: id, Name: field.Name, Kind: kind, Options: epicFieldOptions(field.AllowedValues)})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Name != out[b].Name {
			return out[a].Name < out[b].Name
		}
		return out[a].ID < out[b].ID
	})
	return out, nil
}

// SetEpicAxisField writes one option of a mapped field on an epic, or clears
// the field for an empty path. Only that field travels.
func (j *JiraAdapter) SetEpicAxisField(ctx context.Context, project *models.Project, epicKey string, field models.EpicAxisField, optionPath string) error {
	key, err := cleanJiraKey(epicKey)
	if err != nil {
		return err
	}
	if strings.TrimSpace(field.ID) == "" {
		return fmt.Errorf("no field to write")
	}
	value, err := jiraAxisFieldValue(field.Kind, optionPath)
	if err != nil {
		return err
	}
	c, err := j.forWrite(ctx, project)
	if err != nil {
		return err
	}
	payload := map[string]any{"fields": map[string]any{field.ID: value}}
	return c.jira(ctx, http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key), nil, payload, nil)
}

// jiraAxisFieldValue is the value an option path is written as: nil clears
// the field.
func jiraAxisFieldValue(kind, path string) (any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if kind != models.EpicFieldCascade {
		return map[string]string{"id": path}, nil
	}
	parent, child, ok := strings.Cut(path, "/")
	if !ok || parent == "" || child == "" {
		return nil, fmt.Errorf("option %q of a cascading field is not \"parent/child\"", path)
	}
	return map[string]any{"id": parent, "child": map[string]string{"id": child}}, nil
}

// epicAxisFieldIDs lists the fields an epic search asks besides its own, the
// project's mapped axis fields; none for a project that maps none, whose
// search therefore stays as it was.
func epicAxisFieldIDs(project *models.Project) []string {
	if project == nil {
		return nil
	}
	return project.EpicAxisFields.IDs()
}

// decodeEpicAxisFieldValues reads the option each asked field carries on a
// raw issue: its id for a select, "parentId/childId" for a cascade, nothing
// for an empty field. A value of another shape is left out: the label then
// decides.
func decodeEpicAxisFieldValues(raw json.RawMessage, ids []string) map[string]string {
	var issue struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if json.Unmarshal(raw, &issue) != nil {
		return nil
	}
	out := map[string]string{}
	for _, id := range ids {
		var option struct {
			ID    string `json:"id"`
			Child *struct {
				ID string `json:"id"`
			} `json:"child"`
		}
		value, ok := issue.Fields[id]
		if !ok || json.Unmarshal(value, &option) != nil || option.ID == "" {
			continue
		}
		path := option.ID
		if option.Child != nil && option.Child.ID != "" {
			path += "/" + option.Child.ID
		}
		out[id] = path
	}
	return out
}
