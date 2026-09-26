package main

import (
	"testing"
	"time"
)

func envOf(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestTheShutdownGraceIsReadFromTheEnvironment(t *testing.T) {
	for raw, want := range map[string]time.Duration{"": defaultShutdownGrace, "0": 0, "0s": 0, "12s": 12 * time.Second} {
		got, err := shutdownGraceFromEnv(envOf(map[string]string{"SECTILE_SHUTDOWN_GRACE": raw}))
		if err != nil || got != want {
			t.Fatalf("%q: %v %v, want %v", raw, got, err, want)
		}
	}
	for _, raw := range []string{"five", "-1s", "5"} {
		if _, err := shutdownGraceFromEnv(envOf(map[string]string{"SECTILE_SHUTDOWN_GRACE": raw})); err == nil {
			t.Fatalf("%q must be refused", raw)
		}
	}
}

func TestTheInstanceBoundsAreReadFromTheEnvironment(t *testing.T) {
	heartbeat, deadAfter, reclaim, err := instanceTimingFromEnv(envOf(map[string]string{
		"SECTILE_INSTANCE_HEARTBEAT":  "1s",
		"SECTILE_INSTANCE_DEAD_AFTER": "4s",
	}))
	if err != nil || heartbeat != time.Second || deadAfter != 4*time.Second || reclaim != 0 {
		t.Fatalf("got %v %v %v %v", heartbeat, deadAfter, reclaim, err)
	}
	for _, raw := range []string{"soon", "0s", "-2s"} {
		if _, _, _, err := instanceTimingFromEnv(envOf(map[string]string{"SECTILE_INSTANCE_RECLAIM": raw})); err == nil {
			t.Fatalf("%q must be refused", raw)
		}
	}
}
