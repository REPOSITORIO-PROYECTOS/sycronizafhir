package db

import (
	"testing"
	"time"
)

func TestERPWallClock(t *testing.T) {
	got := ERPWallClock(time.Date(2026, 11, 4, 1, 30, 0, 0, time.UTC))
	want := time.Date(2026, 11, 3, 22, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
