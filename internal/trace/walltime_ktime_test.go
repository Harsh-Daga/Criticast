package trace

import (
	"testing"
	"time"
)

func TestWallToKtimeRoundTrip(t *testing.T) {
	wall, err := time.Parse(time.RFC3339Nano, "2026-06-01T20:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	hdr := Header{
		KtimeBaseNs: 5_000_000_000,
		WallBaseUTC: wall.Format(time.RFC3339Nano),
	}
	k, ok := hdr.WallToKtime(wall.Add(14 * time.Millisecond))
	if !ok {
		t.Fatal("WallToKtime failed")
	}
	got, ok := hdr.KtimeToWall(k)
	if !ok || !got.Equal(wall.Add(14*time.Millisecond)) {
		t.Fatalf("round trip got %v ok=%v", got, ok)
	}
}
