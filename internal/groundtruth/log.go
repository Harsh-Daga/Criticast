package groundtruth

import (
	"bufio"
	"os"
	"sort"
	"time"

	"github.com/criticast/criticast/internal/mechanism"
)

// ParseLogFile reads lines containing CRITICAST_GT records from path or stdin ("-").
func ParseLogFile(path string) ([]Record, error) {
	var sc *bufio.Scanner
	if path == "-" {
		sc = bufio.NewScanner(os.Stdin)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sc = bufio.NewScanner(f)
	}
	sc.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var out []Record
	for sc.Scan() {
		rec, err := ParseLine(sc.Text())
		if err != nil {
			continue
		}
		out = append(out, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TS.Before(out[j].TS)
	})
	return out, nil
}

// Timeline indexes ground-truth records for attribution join.
type Timeline struct {
	records []Record
	byGoid  map[uint64][]Record
}

// NewTimeline builds an index from sorted records.
func NewTimeline(records []Record) *Timeline {
	t := &Timeline{
		records: records,
		byGoid:  make(map[uint64][]Record),
	}
	for _, rec := range records {
		t.byGoid[rec.Goid] = append(t.byGoid[rec.Goid], rec)
	}
	return t
}

// MechanismAt returns the P0-B mechanism for goid at or before ts (last labeled site).
func (t *Timeline) MechanismAt(goid uint64, ts time.Time) string {
	list := t.byGoid[goid]
	m := ""
	for _, rec := range list {
		if rec.TS.After(ts) {
			break
		}
		if mm := Mechanism(rec.Site); mm != mechanism.Unknown {
			m = mm
		}
	}
	return m
}

// TokenAt returns the request token for goid at or before ts (last known).
func (t *Timeline) TokenAt(goid uint64, ts time.Time) string {
	list := t.byGoid[goid]
	if len(list) == 0 {
		return ""
	}
	token := ""
	for _, rec := range list {
		if rec.TS.After(ts) {
			break
		}
		if rec.Token != "" {
			token = rec.Token
		}
	}
	return token
}

// EdgesFromSites builds evaluation edges from GT site transitions (P0-B offline).
func (t *Timeline) EdgesFromSites() []SiteEdge {
	var edges []SiteEdge
	for _, rec := range t.records {
		m := Mechanism(rec.Site)
		if m == mechanism.Unknown || rec.Site == SiteHandlerEntry || rec.Site == SiteHandlerExit {
			continue
		}
		edges = append(edges, SiteEdge{
			Goid:      rec.Goid,
			Token:     rec.Token,
			Site:      rec.Site,
			Mechanism: m,
			TS:        rec.TS,
			Aux:       rec.Extra,
		})
	}
	return edges
}

// SiteEdge is one mechanism-labeled ground-truth observation.
type SiteEdge struct {
	Goid      uint64
	Token     string
	Site      string
	Mechanism string
	TS        time.Time
	Aux       string
}
