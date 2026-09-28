package httpgzip

import (
	"bufio"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var largeJSON = `{"tasks":[` + strings.Repeat(`{"id":"t","title":"a task","status":"todo"},`, 200) + `{}]}`

func serve(t *testing.T, handler http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler(handler).ServeHTTP(rec, req)
	return rec
}

func gzipRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	return req
}

func gunzip(t *testing.T, body io.Reader) string {
	t.Helper()
	reader, err := gzip.NewReader(body)
	if err != nil {
		t.Fatalf("body is not gzip: %v", err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("gzip body does not decode: %v", err)
	}
	return string(plain)
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, body)
	}
}

func TestCompressesLargeJSON(t *testing.T) {
	rec := serve(t, jsonHandler(largeJSON), gzipRequest("/api/tasks"))

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want Accept-Encoding", got)
	}
	if rec.Body.Len() >= len(largeJSON) {
		t.Errorf("compressed body is %d bytes, plain is %d", rec.Body.Len(), len(largeJSON))
	}
	if got := gunzip(t, rec.Body); got != largeJSON {
		t.Errorf("decoded body differs from what the handler wrote")
	}
}

func TestCompressesAcrossManySmallWrites(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		for _, c := range largeJSON {
			_, _ = io.WriteString(w, string(c))
		}
	}
	rec := serve(t, handler, gzipRequest("/"))

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := gunzip(t, rec.Body); got != largeJSON {
		t.Errorf("decoded body differs from what the handler wrote")
	}
}

func TestLeavesResponsesAlone(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		req     func() *http.Request
	}{
		{
			name:    "client does not accept gzip",
			handler: jsonHandler(largeJSON),
			req:     func() *http.Request { return httptest.NewRequest(http.MethodGet, "/", nil) },
		},
		{
			name:    "client refuses gzip explicitly",
			handler: jsonHandler(largeJSON),
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Accept-Encoding", "gzip;q=0, *")
				return req
			},
		},
		{
			name:    "body below the threshold",
			handler: jsonHandler(`{"ok":true}`),
			req:     func() *http.Request { return gzipRequest("/") },
		},
		{
			name: "already encoded",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("Content-Encoding", "br")
				_, _ = io.WriteString(w, largeJSON)
			},
			req: func() *http.Request { return gzipRequest("/metrics") },
		},
		{
			name: "binary type",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "image/png")
				_, _ = io.WriteString(w, largeJSON)
			},
			req: func() *http.Request { return gzipRequest("/logo.png") },
		},
		{
			name:    "byte range",
			handler: jsonHandler(largeJSON),
			req: func() *http.Request {
				req := gzipRequest("/assets/index.js")
				req.Header.Set("Range", "bytes=0-99")
				return req
			},
		},
		{
			name:    "websocket upgrade",
			handler: jsonHandler(largeJSON),
			req: func() *http.Request {
				req := gzipRequest("/ws/agent-connect")
				req.Header.Set("Upgrade", "websocket")
				return req
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, tc.handler, tc.req())
			if got := rec.Header().Get("Content-Encoding"); got == "gzip" {
				t.Fatalf("response was compressed")
			}
			if !strings.Contains(largeJSON+`{"ok":true}`, rec.Body.String()) || rec.Body.Len() == 0 {
				t.Errorf("body was altered: %q", rec.Body.String())
			}
		})
	}
}

func TestEventStreamIsFlushedPlain(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: task_updated\ndata: {}\n\n")
		w.(http.Flusher).Flush()
	}
	rec := serve(t, handler, gzipRequest("/api/events"))

	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, an event stream must stay plain", got)
	}
	if !rec.Flushed {
		t.Errorf("the flush did not reach the client")
	}
	if got := rec.Body.String(); got != "event: task_updated\ndata: {}\n\n" {
		t.Errorf("body = %q", got)
	}
}

func TestFlushSendsCompressedBytesAtOnce(t *testing.T) {
	flushedBytes := -1
	var rec *httptest.ResponseRecorder
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"line":1}`)
		w.(http.Flusher).Flush()
		flushedBytes = rec.Body.Len()
		_, _ = io.WriteString(w, `{"line":2}`)
	}
	rec = httptest.NewRecorder()
	Handler(http.HandlerFunc(handler)).ServeHTTP(rec, gzipRequest("/api/v1/agent/run-output"))

	if flushedBytes <= 0 {
		t.Fatalf("nothing reached the client at the flush")
	}
	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := gunzip(t, rec.Body); got != `{"line":1}{"line":2}` {
		t.Errorf("decoded body = %q", got)
	}
}

func TestSniffsTheTypeFromThePlainBody(t *testing.T) {
	page := "<!DOCTYPE html><html><body>" + strings.Repeat("<p>sectile</p>", 200) + "</body></html>"
	handler := func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, page) }
	rec := serve(t, handler, gzipRequest("/"))

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", got)
	}
	if got := gunzip(t, rec.Body); got != page {
		t.Errorf("decoded body differs from what the handler wrote")
	}
}

func TestRewritesHeadersThatDescribeThePlainBody(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Content-Length", "99999")
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"abc"`)
		_, _ = io.WriteString(w, largeJSON)
	}
	rec := serve(t, handler, gzipRequest("/assets/index.js"))

	for _, name := range []string{"Content-Length", "Accept-Ranges"} {
		if got := rec.Header().Get(name); got != "" {
			t.Errorf("%s = %q, want it dropped", name, got)
		}
	}
	if got := rec.Header().Get("ETag"); got != `W/"abc"` {
		t.Errorf("ETag = %q, want the weak form", got)
	}
}

func TestKeepsStatusAndBodilessAnswers(t *testing.T) {
	rec := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}, gzipRequest("/"))
	if rec.Code != http.StatusNotModified || rec.Header().Get("Content-Encoding") != "" || rec.Body.Len() != 0 {
		t.Errorf("304: code %d, encoding %q, %d body bytes", rec.Code, rec.Header().Get("Content-Encoding"), rec.Body.Len())
	}

	rec = serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, largeJSON)
	}, gzipRequest("/api/nope"))
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("404: code %d, encoding %q", rec.Code, rec.Header().Get("Content-Encoding"))
	}
}

type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestHijackReachesTheConnection(t *testing.T) {
	under := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler := func(w http.ResponseWriter, r *http.Request) {
		if _, _, err := w.(http.Hijacker).Hijack(); err != nil {
			t.Errorf("Hijack: %v", err)
		}
	}
	Handler(http.HandlerFunc(handler)).ServeHTTP(under, gzipRequest("/"))

	if !under.hijacked {
		t.Fatalf("the hijack did not reach the underlying writer")
	}
	if under.Body.Len() != 0 {
		t.Errorf("bytes were written after the hijack")
	}
}

func TestAcceptsGzip(t *testing.T) {
	cases := map[string]bool{
		"":                     false,
		"gzip":                 true,
		"GZIP":                 true,
		"deflate, br":          false,
		"br;q=1.0, gzip;q=0.8": true,
		"gzip;q=0":             false,
		"*":                    true,
		"*;q=0":                false,
		"gzip;q=0, *":          false,
		"identity, x-gzip":     true,
	}
	for header, want := range cases {
		if got := acceptsGzip(header); got != want {
			t.Errorf("acceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}
