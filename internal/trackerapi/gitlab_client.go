package trackerapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// GitLab is reached through the same guarded request as GitHub and Jira: the
// REST API v4 of the configured instance for nearly everything, its GraphQL
// endpoint for the iterations REST cannot write. What is specific here is the
// pagination (X-Next-Page), the project addressing (one URL-encoded path
// segment) and the error shape, which is worth quoting.

const (
	// gitlabPageSize is what the REST API serves at most per page.
	gitlabPageSize = 100
	// gitlabMaxPages caps a runaway pagination at 10 000 items. Reaching it is
	// an error rather than a silent truncation: an import that comes back short
	// would look complete.
	gitlabMaxPages = 100
	// gitlabErrorMessageLimit bounds the quoted refusal.
	gitlabErrorMessageLimit = 300
)

// gitlabConfigured says what is missing before any network call is attempted.
func (c *Client) gitlabConfigured() error {
	if c == nil || strings.TrimSpace(c.GitlabURL) == "" {
		return fmt.Errorf("configurez l'URL de l'API GitLab")
	}
	if c.GitlabToken == "" {
		return c.missingCredential("GitLab")
	}
	return nil
}

// gitlabProjectPath is the project the client addresses: the project's own
// setting, else the default of the settings, both already resolved by For.
func (c *Client) gitlabProjectPath() (string, error) {
	p := strings.Trim(strings.TrimSpace(c.GitlabProject), "/")
	if p == "" {
		return "", fmt.Errorf("configurez le projet GitLab (groupe/projet) du projet Sectile")
	}
	return p, nil
}

// gitlabProjectSegment is the project as one path segment of a REST URL:
// acme/platform/app becomes acme%2Fplatform%2Fapp, a numeric id stays as is.
func gitlabProjectSegment(projectPath string) string {
	return url.PathEscape(strings.Trim(strings.TrimSpace(projectPath), "/"))
}

// gitlab runs one request against the instance. path is relative to the API
// root ("/projects/acme%2Fapp/issues"), query is optional, payload is
// JSON-encoded when not nil, result is decoded when not nil.
func (c *Client) gitlab(ctx context.Context, method, path string, query url.Values, payload, result any) error {
	if err := c.gitlabConfigured(); err != nil {
		return err
	}
	endpoint := strings.TrimRight(c.GitlabURL, "/") + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	raw, _, err := c.request(ctx, method, endpoint, "Bearer "+c.GitlabToken, payload)
	if err != nil {
		return gitlabError(err)
	}
	if result != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, result); err != nil {
			return fmt.Errorf("GitLab a renvoyé une réponse illisible : %w", err)
		}
	}
	return nil
}

// gitlabPages reads every page of a list, following X-Next-Page.
func (c *Client) gitlabPages(ctx context.Context, path string, query url.Values) ([]json.RawMessage, error) {
	if err := c.gitlabConfigured(); err != nil {
		return nil, err
	}
	base := strings.TrimRight(c.GitlabURL, "/") + "/" + strings.TrimLeft(path, "/")
	q := url.Values{}
	for k, v := range query {
		q[k] = append([]string(nil), v...)
	}
	q.Set("per_page", fmt.Sprint(gitlabPageSize))
	var items []json.RawMessage
	page := "1"
	for n := 0; page != ""; n++ {
		if n >= gitlabMaxPages {
			return nil, fmt.Errorf("GitLab renvoie plus de %d pages pour %s : lecture interrompue", gitlabMaxPages, path)
		}
		q.Set("page", page)
		raw, header, err := c.request(ctx, http.MethodGet, base+"?"+q.Encode(), "Bearer "+c.GitlabToken, nil)
		if err != nil {
			return nil, gitlabError(err)
		}
		var chunk []json.RawMessage
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil, fmt.Errorf("GitLab a renvoyé une page illisible : %w", err)
		}
		items = append(items, chunk...)
		next := strings.TrimSpace(header.Get("X-Next-Page"))
		if next == page {
			return nil, fmt.Errorf("la pagination GitLab a répété une page")
		}
		page = next
	}
	return items, nil
}

// gitlabGraphQLEndpoint is the GraphQL entry point of the instance: the REST
// root with its /api/v4 replaced by /api/graphql.
func (c *Client) gitlabGraphQLEndpoint() string {
	base := strings.TrimRight(c.GitlabURL, "/")
	return strings.TrimSuffix(base, "/api/v4") + "/api/graphql"
}

// gitlabGraphQLError is a GraphQL refusal: GitLab answers 200 with the reason
// in "errors", and the messages are what the activity has to show.
type gitlabGraphQLError struct {
	Messages []string
}

func (e *gitlabGraphQLError) Error() string {
	return "GitLab a refusé la requête : " + truncateText(strings.Join(e.Messages, " | "), gitlabErrorMessageLimit)
}

// gitlabGraphQL runs one GraphQL query or mutation and decodes its data.
func (c *Client) gitlabGraphQL(ctx context.Context, query string, variables map[string]any, result any) error {
	if err := c.gitlabConfigured(); err != nil {
		return err
	}
	raw, _, err := c.request(ctx, http.MethodPost, c.gitlabGraphQLEndpoint(), "Bearer "+c.GitlabToken, map[string]any{"query": query, "variables": variables})
	if err != nil {
		return gitlabError(err)
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("GitLab a renvoyé une réponse GraphQL illisible : %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, e := range envelope.Errors {
			messages = append(messages, strings.TrimSpace(e.Message))
		}
		return &gitlabGraphQLError{Messages: messages}
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("GitLab n'a renvoyé aucune donnée GraphQL")
	}
	if result != nil {
		if err := json.Unmarshal(envelope.Data, result); err != nil {
			return fmt.Errorf("GitLab a renvoyé une réponse GraphQL illisible : %w", err)
		}
	}
	return nil
}

// gitlabMutationErrors turns the "errors" list a GitLab mutation payload
// carries into an error, nil when it is empty.
func gitlabMutationErrors(action string, errs []string) error {
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s : %s", action, truncateText(strings.Join(errs, " | "), gitlabErrorMessageLimit))
}

// gitlabError says in French what GitLab refused, quoting its own message when
// it wrote one and never the body itself. A rate limit keeps the *HTTPError in
// the chain, so IsRateLimited still recognises it and the loop backs off.
func gitlabError(err error) error {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}
	text := gitlabErrorText(httpErr.Body)
	switch httpErr.Status {
	case http.StatusUnauthorized:
		return fmt.Errorf("%w : jeton GitLab refusé, vérifiez qu'il est valide et porte le scope api", err)
	case http.StatusForbidden:
		if text != "" {
			return fmt.Errorf("%w : accès refusé par GitLab : %s", err, text)
		}
		return fmt.Errorf("%w : accès refusé par GitLab", err)
	case http.StatusNotFound:
		if text != "" {
			return fmt.Errorf("%w : projet ou ticket GitLab introuvable : %s", err, text)
		}
		return fmt.Errorf("%w : projet ou ticket GitLab introuvable", err)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w : GitLab limite le nombre de requêtes, nouvel essai plus tard", err)
	}
	if text != "" {
		return fmt.Errorf("%w : GitLab a refusé la requête (HTTP %d) : %s", err, httpErr.Status, text)
	}
	return fmt.Errorf("%w : GitLab a refusé la requête (HTTP %d)", err, httpErr.Status)
}

// gitlabErrorText pulls out of a refusal the sentence GitLab wrote for a
// human. GitLab puts it in "message" or "error"; "message" is a sentence, a
// list, or an object keyed by the offending field.
func gitlabErrorText(body []byte) string {
	var payload struct {
		Message          json.RawMessage `json:"message"`
		Error            string          `json:"error"`
		ErrorDescription string          `json:"error_description"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return ""
	}
	text := gitlabMessageText(payload.Message)
	if text == "" {
		text = strings.TrimSpace(payload.ErrorDescription)
	}
	if text == "" {
		text = strings.TrimSpace(payload.Error)
	}
	return truncateText(text, gitlabErrorMessageLimit)
}

// gitlabMessageText unfolds the three shapes of "message".
func gitlabMessageText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var phrase string
	if json.Unmarshal(raw, &phrase) == nil {
		return strings.TrimSpace(phrase)
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil {
		return strings.Join(list, " | ")
	}
	var byField map[string][]string
	if json.Unmarshal(raw, &byField) == nil {
		// Sorted, so the same refusal always reads the same.
		fields := make([]string, 0, len(byField))
		for field := range byField {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			parts = append(parts, fmt.Sprintf("%s : %s", field, strings.Join(byField[field], ", ")))
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

func truncateText(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}
