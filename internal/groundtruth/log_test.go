package groundtruth

import (
	"testing"
	"time"
)

func TestParseLineWithLogPrefix(t *testing.T) {
	line, err := Record{Token: "X", Site: SiteMutexLock, Goid: 5}.FormatLine()
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ParseLine("app: " + line)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Token != "X" || rec.Site != SiteMutexLock {
		t.Fatalf("got %+v", rec)
	}
}

func TestTimelineTokenAt(t *testing.T) {
	t0, err := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	t1, err := time.Parse(time.RFC3339, "2020-01-01T00:00:01Z")
	if err != nil {
		t.Fatal(err)
	}
	recs := []Record{
		{Token: "A", Site: SiteHandlerEntry, Goid: 1, TS: t0},
		{Token: "B", Site: SiteMutexLock, Goid: 1, TS: t1},
	}
	tl := NewTimeline(recs)
	tok := tl.TokenAt(1, t1)
	if tok != "B" {
		t.Fatalf("token %q", tok)
	}
}
