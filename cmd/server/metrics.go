package main

import (
	"log"
	"net"
	"net/http"
	"strings"

	"tasks/internal/metrics"
)

// defaultMetricsAddr is where Prometheus scrapes when SECTILE_METRICS_ADDR says
// nothing. Like the internal port, it must be declared on the container and
// never routed by the ingress: the metrics carry no secret, but they describe
// who uses the board and how, which is nobody else's business.
const defaultMetricsAddr = ":8093"

// metricsAddrFromEnv is the metrics listener's address, or "" when
// SECTILE_METRICS_ADDR turns it off. A bare port is accepted as ":port".
func metricsAddrFromEnv(getenv func(string) string) string {
	addr := strings.TrimSpace(getenv("SECTILE_METRICS_ADDR"))
	switch strings.ToLower(addr) {
	case "":
		return defaultMetricsAddr
	case "off", "false", "0", "disabled":
		return ""
	}
	if !strings.Contains(addr, ":") {
		return ":" + addr
	}
	return addr
}

// startMetricsListener serves the metrics on their own address. A port already
// taken costs the metrics, never the board: the server logs why and carries on.
func startMetricsListener(m *metrics.Metrics, addr string) {
	if addr == "" {
		log.Printf("   métriques : désactivées (SECTILE_METRICS_ADDR)")
		return
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("⚠️  Adresse des métriques %s indisponible, métriques Prometheus non exposées : %v", addr, err)
		return
	}
	mux := http.NewServeMux()
	mux.Handle(metrics.Path, m.Handler())
	log.Printf("   métriques : %s%s", addr, metrics.Path)
	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Printf("⚠️  Serveur de métriques arrêté : %v", err)
		}
	}()
}

// routeOf names the controller a request reaches: the mux pattern, which is a
// bounded set, rather than the path, which carries ids.
func routeOf(mux *http.ServeMux) func(*http.Request) string {
	return func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
}
