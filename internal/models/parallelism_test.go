package models

import "testing"

func TestNormalizeParallelismBounds(t *testing.T) {
	for input, want := range map[int]int{-3: 1, 0: 1, 1: 1, 3: 3, MaxParallelism: MaxParallelism, MaxParallelism + 1: MaxParallelism, 42: MaxParallelism} {
		if got := NormalizeParallelism(input); got != want {
			t.Fatalf("NormalizeParallelism(%d) = %d, want %d", input, got, want)
		}
	}
}
