package metrics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeBoard struct {
	users    int
	runs     map[string]int
	failRuns bool
}

func (f fakeBoard) ActiveUserCount(time.Duration) (int, error) { return f.users, nil }

func (f fakeBoard) ActiveRunCounts() (map[string]int, error) {
	if f.failRuns {
		return nil, errors.New("database unavailable")
	}
	return f.runs, nil
}

func scrape(t *testing.T, m *Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, Path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape answered %d", rec.Code)
	}
	return rec.Body.String()
}

func wantLine(t *testing.T, body, line string) {
	t.Helper()
	for _, got := range strings.Split(body, "\n") {
		if got == line {
			return
		}
	}
	t.Errorf("missing %q in the exposition:\n%s", line, body)
}

func TestTheBoardSeriesAreReadAtScrapeTime(t *testing.T) {
	m := New(fakeBoard{users: 3, runs: map[string]int{"running": 2, "queued": 1, "pending": 0}}, 5*time.Minute, Build{Version: "1.2.3", Commit: "abc"})
	body := scrape(t, m)
	wantLine(t, body, "sectile_active_users 3")
	wantLine(t, body, `sectile_active_runs{status="running"} 2`)
	wantLine(t, body, `sectile_active_runs{status="queued"} 1`)
	wantLine(t, body, `sectile_active_runs{status="pending"} 0`)
	wantLine(t, body, `sectile_build_info{commit="abc",version="1.2.3"} 1`)
}

// A read that fails leaves a gap, not a zero that would read as "nothing runs".
func TestAFailedBoardReadLeavesItsSeriesOut(t *testing.T) {
	m := New(fakeBoard{users: 1, failRuns: true}, time.Minute, Build{Version: "dev"})
	body := scrape(t, m)
	if strings.Contains(body, "sectile_active_runs{") {
		t.Fatalf("a failed read still reported runs:\n%s", body)
	}
	wantLine(t, body, "sectile_active_users 1")
}

func TestRequestsAreCountedByControllerAndErrorsByClass(t *testing.T) {
	m := New(nil, time.Minute, Build{Version: "dev"})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/missing") {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/broken") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	route := func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
	server := m.Instrument(mux, route)
	for _, path := range []string{"/api/tasks/1", "/api/tasks/2", "/api/tasks/missing", "/api/tasks/broken"} {
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	body := scrape(t, m)
	// The label is the pattern: two task ids are one series.
	wantLine(t, body, `sectile_http_requests_total{code="200",handler="/api/tasks/",method="GET"} 2`)
	wantLine(t, body, `sectile_http_requests_total{code="404",handler="/api/tasks/",method="GET"} 1`)
	wantLine(t, body, `sectile_http_requests_total{code="500",handler="/api/tasks/",method="GET"} 1`)
	wantLine(t, body, `sectile_http_errors_total{class="client",handler="/api/tasks/",method="GET"} 1`)
	wantLine(t, body, `sectile_http_errors_total{class="server",handler="/api/tasks/",method="GET"} 1`)
	wantLine(t, body, `sectile_http_request_duration_seconds_count{handler="/api/tasks/",method="GET"} 4`)
	if strings.Contains(body, "sectile_active_users") {
		t.Errorf("without a board, the board series must be left out")
	}
}

// The event stream asserts http.Flusher: the wrapper must keep it, and the
// stream is counted but not timed.
func TestAStreamIsCountedButNotTimed(t *testing.T) {
	m := New(nil, time.Minute, Build{Version: "dev"})
	stream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("the instrumented writer hides http.Flusher")
		}
		_, _ = w.Write([]byte("data: x\n\n"))
		flusher.Flush()
	})
	if _, ok := any(&statusRecorder{}).(http.Hijacker); !ok {
		t.Fatal("the instrumented writer hides http.Hijacker, which the WebSocket upgrade needs")
	}
	m.Instrument(stream, func(*http.Request) string { return "/api/events" }).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/events", nil))

	body := scrape(t, m)
	wantLine(t, body, `sectile_http_requests_total{code="200",handler="/api/events",method="GET"} 1`)
	if strings.Contains(body, `sectile_http_request_duration_seconds_count{handler="/api/events"`) {
		t.Errorf("a stream was timed:\n%s", body)
	}
}

func TestAnUnknownMethodDoesNotOpenANewSeries(t *testing.T) {
	if got := methodLabel("BREW"); got != "OTHER" {
		t.Fatalf("methodLabel(BREW) = %q, want OTHER", got)
	}
}
