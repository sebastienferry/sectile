package trackerapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/tracker"
)

func TestJiraReportUploadPreservesBytesAndAuthentication(t *testing.T) {
	for _, oauth := range []bool{false, true} {
		t.Run(map[bool]string{false: "token", true: "OAuth"}[oauth], func(t *testing.T) {
			body := "\r\n **bold `code`**\r\n| table |\n  日本語 é  \n"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				prefix := ""
				auth := jiraBasicAuth("ada@example.com", "secret")
				if oauth {
					prefix = "/ex/jira/cloud"
					auth = "Bearer grant"
				}
				if r.URL.Path != prefix+"/rest/api/3/issue/PE-1/attachments" || r.Method != "POST" || r.Header.Get("Authorization") != auth || r.Header.Get("X-Atlassian-Token") != "no-check" {
					t.Errorf("incorrect attachment request: %s", r.URL.Path)
				}
				reader, err := r.MultipartReader()
				if err != nil {
					t.Fatal(err)
				}
				part, err := reader.NextPart()
				if err != nil {
					t.Fatal(err)
				}
				raw, _ := io.ReadAll(part)
				if part.FormName() != "file" || part.FileName() != "PE-1-specification.md" || string(raw) != body || part.Header.Get("Content-Type") != "text/markdown; charset=utf-8" {
					t.Errorf("multipart did not preserve report: %q", raw)
				}
				if _, err := reader.NextPart(); err != io.EOF {
					t.Errorf("unexpected extra part: %v", err)
				}
				io.WriteString(w, `[{"id":42,"filename":"PE-1-specification.md","content":"https://jira.example/attachment/42"}]`)
			}))
			defer server.Close()
			c := &Client{HTTP: server.Client(), JiraURL: server.URL, JiraEmail: "ada@example.com", JiraToken: "secret"}
			if oauth {
				c.JiraAPIBase = server.URL + "/ex/jira/cloud"
				c.JiraBearer = "grant"
			}
			attachment, err := NewJiraAdapter(c).UploadReport(tracker.WithUnattended(context.Background()), tracker.AddCommentRequest{Key: "PE-1", Body: body}, "PE-1-specification.md")
			if err != nil || attachment.ID != "42" {
				t.Fatalf("upload: %#v %v", attachment, err)
			}
		})
	}
}

func TestJiraReportSummaryBypassesMarkdown(t *testing.T) {
	text := "Decision **`code`** stays literal.\nQuestion two.\nQuestion three.\nOpen question?\nhttps://github.com/acme/app/pull/2"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			io.WriteString(w, `{"comments":[],"total":0}`)
			return
		}
		var payload struct {
			Body       json.RawMessage `json:"body"`
			Properties []any           `json:"properties"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(payload.Body), `"marks"`) || strings.Contains(string(payload.Body), `"code"`) || !strings.Contains(string(payload.Body), "Decision **`code`** stays literal.") || len(payload.Properties) != 1 {
			t.Errorf("summary interpreted Markdown: %s", payload.Body)
		}
		io.WriteString(w, `{"id":"7"}`)
	}))
	defer server.Close()
	c := &Client{HTTP: server.Client(), JiraURL: server.URL, JiraEmail: "ada@example.com", JiraToken: "secret"}
	id, err := NewJiraAdapter(c).AddReportSummary(tracker.WithUnattended(context.Background()), tracker.AddCommentRequest{Key: "PE-1", Body: text}, "sectile.report.id")
	if err != nil || id != "7" {
		t.Fatalf("summary: %s %v", id, err)
	}
}

func TestJiraAttachmentResponseAndFilenameFailures(t *testing.T) {
	for _, reply := range []string{`[]`, `{}`, `[{"id":"1"}]`, `not json`} {
		t.Run(reply, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, reply) }))
			defer server.Close()
			j := NewJiraAdapter(&Client{HTTP: server.Client(), JiraURL: server.URL, JiraEmail: "ada@example.com", JiraToken: "secret"})
			if _, err := j.UploadReport(tracker.WithUnattended(context.Background()), tracker.AddCommentRequest{Key: "PE-1", Body: "report"}, "PE-1.md"); err == nil {
				t.Fatal("invalid response accepted")
			}
			if _, err := j.UploadReport(tracker.WithUnattended(context.Background()), tracker.AddCommentRequest{Key: "PE-1", Body: "report"}, "../PE-1.md"); err == nil {
				t.Fatal("unsafe filename accepted")
			}
		})
	}
}

var _ tracker.ReportWriter = (*JiraAdapter)(nil)
