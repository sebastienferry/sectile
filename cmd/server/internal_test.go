package main

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/handlers"
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

func TestInternalListenerReleasesItsPortBeforeDatabaseClose(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(reservation.Addr().(*net.TCPAddr).Port)
	_ = reservation.Close()
	h := handlers.NewHandler(database)
	stop := startInternalListener(h, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := stop(ctx); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("internal listener retained port: %v", err)
	}
	_ = listener.Close()
}
