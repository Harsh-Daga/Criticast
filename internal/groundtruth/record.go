// Package groundtruth defines the P0-B dual-source log line format (context token + site + goid).
package groundtruth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/criticast/criticast/internal/mechanism"
)

// Prefix is the log marker for machine join with eBPF traces.
const Prefix = "CRITICAST_GT "

// Record is one ground-truth observation at a synchronization or spawn site.
type Record struct {
	TS    time.Time `json:"ts"`
	Goid  uint64    `json:"goid"`
	Token string    `json:"token"`
	Site  string    `json:"site"`
	Span  string    `json:"span,omitempty"`
	Extra string    `json:"extra,omitempty"`
}

// FormatLine returns a single log line for log.Printf.
func (r Record) FormatLine() (string, error) {
	if r.Token == "" || r.Site == "" {
		return "", errors.New("groundtruth: token and site required")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("groundtruth: marshal: %w", err)
	}
	return Prefix + string(b), nil
}

// ParseLine decodes a log line containing CRITICAST_GT JSON.
func ParseLine(line string) (Record, error) {
	idx := strings.Index(line, Prefix)
	if idx < 0 {
		return Record{}, errors.New("groundtruth: missing prefix")
	}
	payload := strings.TrimSpace(line[idx+len(Prefix):])
	var rec Record
	if err := json.Unmarshal([]byte(payload), &rec); err != nil {
		return Record{}, fmt.Errorf("groundtruth: json: %w", err)
	}
	if rec.Token == "" || rec.Site == "" {
		return Record{}, errors.New("groundtruth: token and site required")
	}
	return rec, nil
}

// Sites used by the adversarial workload (see internal/mechanism).
const (
	SiteHandlerEntry    = "handler-entry"
	SiteHandlerExit     = "handler-exit"
	SiteSpawn           = "spawn"
	SiteSpawnWork       = "spawn-work"
	SiteWorkerPoolSend  = "worker-pool-send"
	SiteWorkerRecv      = "worker-recv"
	SiteWorkerDone      = "worker-done"
	SiteConnPoolAcquire = "conn-pool-acquire"
	SiteConnPoolRelease = "conn-pool-release"
	SiteMutexLock       = "mutex-lock"
	SiteMutexUnlock     = "mutex-unlock"
)

// Mechanism maps a site to the P0-B per-mechanism matrix row.
func Mechanism(site string) string {
	switch site {
	case SiteSpawn, SiteSpawnWork:
		return mechanism.SpawnLineage
	case SiteWorkerPoolSend, SiteWorkerRecv, SiteWorkerDone:
		return mechanism.ChanWorkHandoff
	case SiteConnPoolAcquire, SiteConnPoolRelease:
		return mechanism.ConnPool
	case SiteMutexLock, SiteMutexUnlock:
		return mechanism.Mutex
	default:
		return mechanism.Unknown
	}
}
