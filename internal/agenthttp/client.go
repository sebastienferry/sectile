// Package agenthttp builds the HTTP client every agent-side caller uses to
// reach the server. The credential travels on a transport rather than on each
// request, so no call site can forget it, and redirects are never followed:
// the paired device token must not leak to whatever a redirect points at.
package agenthttp

import (
	"net/http"
	"time"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

// Client returns a client that presents token on every request it makes.
func Client(token string) *http.Client {
	return &http.Client{Transport: bearerTransport{token: token, base: http.DefaultTransport}, Timeout: 2 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
