//go:build sectile_headless

package webui

import "testing"

func TestHeadlessBuildOmitsWebAssets(t *testing.T) {
	if !Headless() {
		t.Fatal("Headless() = false, want true")
	}
	if filesystem, embedded := FS(); embedded || filesystem != nil {
		t.Fatalf("FS() = (%v, %t), want (nil, false)", filesystem, embedded)
	}
}
