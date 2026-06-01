package attribution

import (
	"testing"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
)

func TestLineageSpawnFromHandler(t *testing.T) {
	ts := time.Now()
	s := NewLineageStore(0)
	s.ApplyRecord(groundtruth.Record{Token: "A", Site: groundtruth.SiteHandlerEntry, Goid: 10, TS: ts})
	s.ApplyRecord(groundtruth.Record{Token: "A", Site: groundtruth.SiteSpawn, Goid: 20, TS: ts.Add(time.Millisecond)})
	if got := s.Cookie(20, ts.Add(2*time.Millisecond)); got != "A" {
		t.Fatalf("spawn goid cookie = %q", got)
	}
}
