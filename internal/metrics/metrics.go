// Package metrics exposes the server's Prometheus metrics: how the HTTP
// controllers are used (hits, latency, errors) and what the board is doing
// (active users, active runs).
//
// The HTTP series are this instance's own: each replica counts the requests it
// served, and a dashboard sums them. The board series are read from the shared
// database at scrape time, so every replica reports the same value and a
// dashboard takes max() of them, never sum(). See docs/adrs/0027.
package metrics

import (
	"bufio"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Path is where the metrics are served, on the server's own port.
const Path = "/metrics"

// Board is what the board series are read from. *db.DB satisfies it.
type Board interface {
	ActiveUserCount(window time.Duration) (int, error)
	ActiveRunCounts() (map[string]int, error)
}

// Build is the identity sectile_build_info reports.
type Build struct {
	Version string
	Commit  string
}

// Metrics owns one registry, so a test builds its own and nothing leaks
// through the process-wide default one.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	errors   *prometheus.CounterVec
	latency  *prometheus.HistogramVec
}

// New registers every series. board may be nil, in which case the board
// series are left out rather than reported as zero.
func New(board Board, activeWindow time.Duration, build Build) *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sectile_http_requests_total",
			Help: "HTTP requests served, by controller, method and status code.",
		}, []string{"handler", "method", "code"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "sectile_http_errors_total",
			Help: "HTTP requests answered with an error, by controller, method and class (client for 4xx, server for 5xx).",
		}, []string{"handler", "method", "class"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "sectile_http_request_duration_seconds",
			Help:    "Time to answer an HTTP request, by controller and method. Streamed responses (event streams, WebSockets) are left out.",
			Buckets: prometheus.DefBuckets,
		}, []string{"handler", "method"}),
	}
	m.registry.MustRegister(
		m.requests, m.errors, m.latency,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "sectile_build_info",
			Help:        "The running build; the value is always 1.",
			ConstLabels: prometheus.Labels{"version": build.Version, "commit": build.Commit},
		}, func() float64 { return 1 }),
	)
	if board != nil {
		m.registry.MustRegister(&boardCollector{board: board, window: activeWindow})
	}
	return m
}

// Handler serves the registry in the Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{Registry: m.registry})
}

// Instrument counts and times every request next serves. route names the
// controller a request reached; it must answer from a bounded set, the mux
// pattern, never the raw path, or every task id would become its own series.
func (m *Metrics) Instrument(next http.Handler, route func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler := route(r)
		if handler == "" {
			handler = "unmatched"
		}
		method := methodLabel(r.Method)
		recorder := &statusRecorder{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(recorder, r)
		status := recorder.statusCode()
		m.requests.WithLabelValues(handler, method, strconv.Itoa(status)).Inc()
		if class := errorClass(status); class != "" {
			m.errors.WithLabelValues(handler, method, class).Inc()
		}
		// A stream lasts as long as its client stays: its duration says nothing
		// about how fast the controller answers, and would drown the histogram.
		if !recorder.streamed {
			m.latency.WithLabelValues(handler, method).Observe(time.Since(start).Seconds())
		}
	})
}

// methodLabel keeps the method label bounded: a client may send any token.
func methodLabel(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	}
	return "OTHER"
}

func errorClass(status int) string {
	switch {
	case status >= 500:
		return "server"
	case status >= 400:
		return "client"
	}
	return ""
}

// statusRecorder remembers the status a handler answered, and whether it
// streamed. It keeps Flush and Hijack reachable: the event stream asserts
// http.Flusher and the WebSocket upgrade asserts http.Hijacker, and a wrapper
// hiding either would break them.
type statusRecorder struct {
	http.ResponseWriter
	status   int
	streamed bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	s.streamed = true
	if s.status == 0 {
		s.status = http.StatusOK
	}
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("the response writer cannot be hijacked")
	}
	s.streamed = true
	if s.status == 0 {
		s.status = http.StatusSwitchingProtocols
	}
	return hijacker.Hijack()
}

// Unwrap lets http.ResponseController reach the writer underneath.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// statusCode is what the client received: a handler that wrote nothing
// answered 200.
func (s *statusRecorder) statusCode() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}

var (
	activeUsersDesc = prometheus.NewDesc("sectile_active_users",
		"Accounts whose browser session reached the server within the active window. Read from the shared database: every replica reports the same value.",
		nil, nil)
	activeRunsDesc = prometheus.NewDesc("sectile_active_runs",
		"Runs that are not over, by status. Read from the shared database: every replica reports the same value.",
		[]string{"status"}, nil)
)

// boardCollector reads the board series at scrape time. A failed read leaves
// its series out of that scrape, which Prometheus reads as a gap rather than
// as a drop to zero.
type boardCollector struct {
	board  Board
	window time.Duration
}

func (c *boardCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- activeUsersDesc
	ch <- activeRunsDesc
}

func (c *boardCollector) Collect(ch chan<- prometheus.Metric) {
	if users, err := c.board.ActiveUserCount(c.window); err == nil {
		ch <- prometheus.MustNewConstMetric(activeUsersDesc, prometheus.GaugeValue, float64(users))
	} else {
		log.Printf("metrics: counting the active users: %v", err)
	}
	if runs, err := c.board.ActiveRunCounts(); err == nil {
		for status, count := range runs {
			ch <- prometheus.MustNewConstMetric(activeRunsDesc, prometheus.GaugeValue, float64(count), status)
		}
	} else {
		log.Printf("metrics: counting the active runs: %v", err)
	}
}
