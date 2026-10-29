package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitLoopbackBinding(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", ""} {
		got, err := listenAddress(host, "8090")
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"127.0.0.1": "127.0.0.1:8090", "::1": "[::1]:8090", "": ":8090"}[host]
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
	for _, pair := range [][2]string{{"localhost", "8090"}, {"127.0.0.1:80", "8090"}, {"127.0.0.1", "0"}, {"", "65536"}} {
		if _, err := listenAddress(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted invalid address %v", pair)
		}
	}
}

func TestIsolatedServerDataDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "standalone")
	t.Setenv("SECTILE_SERVER_DATA_DIR", root)
	if got := appDataDir(); got != root {
		t.Fatalf("got %q, want %q", got, root)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
	got, origin := resolveDBPath("")
	if got != filepath.Join(root, "tasks.db") || origin == "" {
		t.Fatalf("unexpected database location %q (%s)", got, origin)
	}
}
