//go:build !sectile_headless

package webui

import "testing"

func TestDefaultBuildRetainsWebUIBehavior(t *testing.T) {
	if Headless() {
		t.Fatal("Headless() = true, want false for the default build")
	}
}
