// Package trace defines the criticast JSONL trace file format.
package trace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/criticast/criticast/internal/event"
)

const Version = 1

// Header is the first line of a trace file (JSON).
type Header struct {
	Version     int    `json:"version"`
	Tgid        uint32 `json:"tgid"`
	MinBlock    uint64 `json:"min_block_ns"`
	SampleMod   uint32 `json:"sample_mod"`
	Started     string `json:"started_utc"`
	KtimeBaseNs uint64 `json:"ktime_base_ns,omitempty"`
	WallBaseUTC string `json:"wall_base_utc,omitempty"`
}

// EventWallTime maps bpf_ktime_get_ns (boot monotonic) to wall clock for GT join.
// Requires KtimeBaseNs and WallBaseUTC from record (first-event anchor).
func (h Header) EventWallTime(tsNs uint64) time.Time {
	if h.KtimeBaseNs == 0 || h.WallBaseUTC == "" {
		return time.Unix(0, int64(tsNs))
	}
	base, err := time.Parse(time.RFC3339Nano, h.WallBaseUTC)
	if err != nil {
		return time.Unix(0, int64(tsNs))
	}
	delta := int64(tsNs) - int64(h.KtimeBaseNs)
	return base.Add(time.Duration(delta))
}

// File holds decoded trace contents.
type File struct {
	Header Header
	Events []event.Event
}

// Write encodes header + one JSON event per line.
func Write(w io.Writer, hdr Header, events []event.Event) error {
	if hdr.Version == 0 {
		hdr.Version = Version
	}
	if hdr.Started == "" {
		if hdr.WallBaseUTC != "" {
			hdr.Started = hdr.WallBaseUTC
		} else {
			hdr.Started = time.Now().UTC().Format(time.RFC3339Nano)
		}
	}
	b, err := json.Marshal(hdr)
	if err != nil {
		return fmt.Errorf("trace header: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	for i := range events {
		if err := enc.Encode(events[i]); err != nil {
			return fmt.Errorf("trace event %d: %w", i, err)
		}
	}
	return nil
}

// Read parses a trace written by Write.
func Read(r io.Reader) (*File, error) {
	sc := bufio.NewScanner(r)
	const maxLine = 4 << 20
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxLine)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("trace: empty file")
	}
	var hdr Header
	if err := json.Unmarshal(sc.Bytes(), &hdr); err != nil {
		return nil, fmt.Errorf("trace header: %w", err)
	}
	if hdr.Version != Version {
		return nil, fmt.Errorf("trace: unsupported version %d", hdr.Version)
	}
	var out File
	out.Header = hdr
	for sc.Scan() {
		var ev event.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return nil, fmt.Errorf("trace event: %w", err)
		}
		out.Events = append(out.Events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReadPath opens path and decodes a trace file.
func ReadPath(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}
