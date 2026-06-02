package attribution

import (
	"strconv"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
	"github.com/criticast/criticast/internal/mechanism"
)

// LineageStore holds per-goid request tokens (lineage-first, CHARTER §C.3).
type LineageStore struct {
	cookie         map[uint64]string
	parent         map[uint64]uint64
	handlerByToken map[string]uint64 // handler goid per request token (spawn propagation)
	expire         map[uint64]time.Time
	ttl            time.Duration
	sudog          map[uint64]string // sudog.elem → token (E2)
}

// NewLineageStore creates an empty lineage table.
func NewLineageStore(cookieTTL time.Duration) *LineageStore {
	return &LineageStore{
		cookie:         make(map[uint64]string),
		parent:         make(map[uint64]uint64),
		handlerByToken: make(map[string]uint64),
		expire:         make(map[uint64]time.Time),
		ttl:            cookieTTL,
		sudog:          make(map[uint64]string),
	}
}

// ApplyRecord updates lineage from a ground-truth site (offline replay).
func (s *LineageStore) ApplyRecord(rec groundtruth.Record) {
	switch rec.Site {
	case groundtruth.SiteHandlerEntry:
		s.setCookie(rec.Goid, rec.Token, rec.TS)
		if rec.Token != "" {
			s.handlerByToken[rec.Token] = rec.Goid
		}
	case groundtruth.SiteSpawn, groundtruth.SiteSpawnWork:
		if h, ok := s.handlerByToken[rec.Token]; ok && h != 0 {
			if tok := s.Cookie(h, rec.TS); tok != "" {
				s.setCookie(rec.Goid, tok, rec.TS)
				s.parent[rec.Goid] = h
				return
			}
		}
		if p, ok := s.parent[rec.Goid]; ok {
			if tok := s.Cookie(p, rec.TS); tok != "" {
				s.setCookie(rec.Goid, tok, rec.TS)
				return
			}
		}
		s.setCookie(rec.Goid, rec.Token, rec.TS)
	case groundtruth.SiteWorkerPoolSend:
		s.setCookie(rec.Goid, rec.Token, rec.TS)
		if elem := parseElem(rec.Extra); elem != 0 {
			s.NoteSudogElem(elem, rec.Token)
		}
	default:
		if rec.Token != "" && s.Cookie(rec.Goid, rec.TS) == "" {
			s.setCookie(rec.Goid, rec.Token, rec.TS)
		}
	}
}

// NoteSudogElem records chan item pointer → token for E2 handoff matching.
func (s *LineageStore) NoteSudogElem(elem uint64, token string) {
	if elem != 0 && token != "" {
		s.sudog[elem] = token
	}
}

// SudogToken looks up a token by sudog.elem pointer.
func (s *LineageStore) SudogToken(elem uint64) string {
	if elem == 0 {
		return ""
	}
	return s.sudog[elem]
}

func (s *LineageStore) setCookie(goid uint64, token string, ts time.Time) {
	if token == "" {
		return
	}
	s.cookie[goid] = token
	if s.ttl > 0 {
		s.expire[goid] = ts.Add(s.ttl)
	}
}

// Cookie returns the lineage token for goid at ts.
func (s *LineageStore) Cookie(goid uint64, ts time.Time) string {
	if s.ttl > 0 {
		if exp, ok := s.expire[goid]; ok && ts.After(exp) {
			delete(s.cookie, goid)
			delete(s.expire, goid)
			return ""
		}
	}
	return s.cookie[goid]
}

// IsResourceMechanism reports mutex/pool — do not inherit waker cookie (E3).
func IsResourceMechanism(m string) bool {
	return m == mechanism.Mutex || m == mechanism.ConnPool
}

// parseElem decodes the decimal sudog-element id emitted by the P0-B fixture.
func parseElem(s string) uint64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
