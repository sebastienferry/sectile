package main

import (
	"strings"
	"testing"
)

func TestInternalPortAndURLFromTheEnvironment(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }

	if got := internalPortFromEnv(getenv); got != defaultInternalPort {
		t.Errorf("default port = %q, want %q", got, defaultInternalPort)
	}
	env["SECTILE_INTERNAL_PORT"] = "9100"
	if got := internalPortFromEnv(getenv); got != "9100" {
		t.Errorf("port = %q, want 9100", got)
	}

	env["SECTILE_INTERNAL_URL"] = "http://sectile-0.sectile:9100/"
	if got := internalURL(getenv, "9100"); got != "http://sectile-0.sectile:9100" {
		t.Errorf("url = %q", got)
	}
	delete(env, "SECTILE_INTERNAL_URL")
	if got := internalURL(getenv, "9100"); got != "" && (!strings.HasPrefix(got, "http://") || !strings.HasSuffix(got, ":9100")) {
		t.Errorf("detected url = %q", got)
	}
}
