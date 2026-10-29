package main

import (
	"fmt"
	"net"
	"strconv"
)

// listenAddress accepts only literal IPs, avoiding DNS-dependent bind behavior.
// An absent host preserves the existing server deployment default.
func listenAddress(host, port string) (string, error) {
	if host != "" && net.ParseIP(host) == nil {
		return "", fmt.Errorf("SECTILE_SERVER_HOST must be a literal IP address")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("PORT must be between 1 and 65535")
	}
	return net.JoinHostPort(host, port), nil
}
