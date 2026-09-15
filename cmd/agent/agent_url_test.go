package main

import "testing"

func TestResolveServerURL(t *testing.T) {
	t.Run("prefers command-line flag over environment variables", func(t *testing.T) {
		t.Setenv("REMOTE_URL", "https://remote.example.com")
		got := resolveServerURL("https://flag.example.com")
		if got != "https://flag.example.com" {
			t.Fatalf("expected flag URL, got %q", got)
		}
	})

	t.Run("uses REMOTE_URL when flag is empty", func(t *testing.T) {
		t.Setenv("REMOTE_URL", "https://remote.example.com")
		got := resolveServerURL("")
		if got != "https://remote.example.com" {
			t.Fatalf("expected REMOTE_URL, got %q", got)
		}
	})

	t.Run("returns empty string when nothing is set", func(t *testing.T) {
		t.Setenv("REMOTE_URL", "")
		got := resolveServerURL("")
		if got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})
}
