package trace

import (
	"testing"
	"time"
)

func TestHeaderEventWallTime(t *testing.T) {
	wall, err := time.Parse(time.RFC3339Nano, "2026-05-31T12:00:00.5Z")
	if err != nil {
		t.Fatal(err)
	}
	hdr := Header{
		KtimeBaseNs: 10_000,
		WallBaseUTC: wall.Format(time.RFC3339Nano),
	}
	got := hdr.EventWallTime(10_000 + 2_000_000_000)
	want := wall.Add(2 * time.Second)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
