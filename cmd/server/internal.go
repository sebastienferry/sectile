package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"tasks/internal/handlers"
)

// defaultInternalPort is where the instances sharing a database reach each
// other when SECTILE_INTERNAL_PORT says nothing. It must be declared on the
// container and never routed by the ingress.
const defaultInternalPort = "8092"

func internalPortFromEnv(getenv func(string) string) string {
	if port := strings.TrimSpace(getenv("SECTILE_INTERNAL_PORT")); port != "" {
		return port
	}
	return defaultInternalPort
}

// internalURL is the address this instance advertises to the others:
// SECTILE_INTERNAL_URL when set, otherwise its first non-loopback IPv4, which
// in a pod is the pod address.
func internalURL(getenv func(string) string, port string) string {
	if url := strings.TrimSpace(getenv("SECTILE_INTERNAL_URL")); url != "" {
		return strings.TrimRight(url, "/")
	}
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
				continue
			}
			return fmt.Sprintf("http://%s:%s", ipNet.IP.To4(), port)
		}
	}
	log.Printf("⚠️  Aucune adresse IPv4 à annoncer aux autres instances ; définissez SECTILE_INTERNAL_URL.")
	return ""
}

// startInternalListener serves the endpoints other instances forward agent
// work and MCP requests to, on their own port. A failure to authenticate the
// instances, or to listen, leaves this instance serving its own agents and
// MCP sessions and says why.
func startInternalListener(h *handlers.Handler, port string) {
	if err := h.EnableAgentCluster(); err != nil {
		log.Printf("⚠️  Relais d'agents et de sessions MCP entre instances désactivé : %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/internal/agent/", h.InternalHandler())
	mux.Handle("/internal/mcp", h.InternalMCPHandler())
	mux.Handle("/internal/mcp/sessions", h.InternalMCPSessionsHandler())
	mux.Handle("/internal/credentials/keys", h.InternalCredentialsHandler())
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Printf("⚠️  Port interne %s indisponible, relais d'agents entre instances impossible : %v", port, err)
		return
	}
	log.Printf("   interne : :%s", port)
	go func() {
		if err := http.Serve(listener, mux); err != nil {
			log.Printf("⚠️  Serveur interne arrêté : %v", err)
		}
	}()
	// The instance is registered and now reachable: a key unlocked elsewhere
	// from here on is pushed to it, and one unlocked before is pulled (#409).
	go func() {
		if adopted := h.PullUnlockedKeys(); adopted > 0 {
			log.Printf("%d clé(s) descellée(s) reprise(s) des autres instances", adopted)
		}
	}()
}
