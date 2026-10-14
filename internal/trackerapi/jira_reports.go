package trackerapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

func (j *JiraAdapter) UploadReport(ctx context.Context, req tracker.AddCommentRequest, filename string) (*models.ReportAttachment, error) {
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return nil, err
	}
	if filename != filepath.Base(filename) || strings.ContainsAny(filename, "\r\n\"\\") || !strings.HasSuffix(filename, ".md") {
		return nil, fmt.Errorf("nom de pièce jointe invalide")
	}
	c, err := j.forWrite(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	if err := c.jiraConfigured(); err != nil {
		return nil, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	header.Set("Content-Type", "text/markdown; charset=utf-8")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write([]byte(req.Body)); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	raw, _, err := c.requestBody(ctx, http.MethodPost, c.jiraEndpoint()+"/rest/api/3/issue/"+url.PathEscape(key)+"/attachments", c.jiraAuthorization(), writer.FormDataContentType(), &body, map[string]string{"X-Atlassian-Token": "no-check"})
	if err != nil {
		return nil, jiraError(err)
	}
	var attachments []models.ReportAttachment
	if err := json.Unmarshal(raw, &attachments); err != nil {
		return nil, fmt.Errorf("réponse de pièce jointe Jira illisible : %w", err)
	}
	if len(attachments) != 1 || attachments[0].ID == "" || attachments[0].Name != filename || attachments[0].URL == "" {
		return nil, fmt.Errorf("Jira n'a pas confirmé la pièce jointe")
	}
	return &attachments[0], nil
}

func (j *JiraAdapter) FindReportAttachment(ctx context.Context, req tracker.AddCommentRequest, filename string) (*models.ReportAttachment, error) {
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return nil, err
	}
	c, err := j.forWrite(ctx, req.Tracker)
	if err != nil {
		return nil, err
	}
	var result struct {
		Fields struct {
			Attachments []models.ReportAttachment `json:"attachment"`
		} `json:"fields"`
	}
	if err := c.jira(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key), url.Values{"fields": {"attachment"}}, nil, &result); err != nil {
		return nil, err
	}
	var found *models.ReportAttachment
	for _, a := range result.Fields.Attachments {
		if a.Name == filename {
			if found != nil {
				return nil, fmt.Errorf("plusieurs pièces jointes portent le nom du rapport")
			}
			copy := a
			found = &copy
		}
	}
	return found, nil
}

// Plain summary text is never interpreted as Markdown, including literal marks.
func plainReportADF(body string) map[string]any {
	paragraphs := []any{}
	for _, line := range strings.Split(body, "\n") {
		content := []any{}
		if line != "" {
			content = append(content, map[string]any{"type": "text", "text": line})
		}
		paragraphs = append(paragraphs, map[string]any{"type": "paragraph", "content": content})
	}
	return map[string]any{"type": "doc", "version": 1, "content": paragraphs}
}

func (j *JiraAdapter) AddReportSummary(ctx context.Context, req tracker.AddCommentRequest, marker string) (string, error) {
	key, err := cleanJiraKey(req.Key)
	if err != nil {
		return "", err
	}
	c, err := j.forWrite(ctx, req.Tracker)
	if err != nil {
		return "", err
	}
	base := "/rest/api/3/issue/" + url.PathEscape(key) + "/comment"
	// A property permits reconciliation when a response was lost after creation.
	found, err := c.jiraMarkedComment(ctx, base, marker)
	if err != nil {
		return "", err
	}
	if found != "" {
		return found, nil
	}
	var result struct {
		ID string `json:"id"`
	}
	err = c.jira(ctx, http.MethodPost, base, nil, map[string]any{"body": plainReportADF(req.Body), "properties": []map[string]any{{"key": marker, "value": map[string]any{"report": true}}}}, &result)
	if err != nil {
		return "", err
	}
	if result.ID == "" {
		return "", fmt.Errorf("Jira n'a pas confirmé le commentaire")
	}
	return result.ID, nil
}
