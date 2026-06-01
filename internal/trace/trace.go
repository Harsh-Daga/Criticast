// Package trace defines the criticast trace file format (JSONL v1 and v2).
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

// Header is the v1 first-line JSON (also embedded in File for wall-time helpers).
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

// File holds decoded trace contents (v1 or v2).
type File struct {
	Header Header
	Events []event.Event
	Stacks map[int32][]uint64
	Footer *Footer
	Format int // Version1 or Version2
}

// WriteOptions configures v2 trace output.
type WriteOptions struct {
	Stacks map[int32][]uint64
	Footer *Footer
}

// Write encodes a v2 trace: header, stacks section, events, footer.
func Write(w io.Writer, hdr Header, events []event.Event, opts *WriteOptions) error {
	if hdr.Version == 0 {
		hdr.Version = LatestVersion
	}
	if hdr.Started == "" {
		if hdr.WallBaseUTC != "" {
			hdr.Started = hdr.WallBaseUTC
		} else {
			hdr.Started = time.Now().UTC().Format(time.RFC3339Nano)
		}
	}

	v2 := V2Header{
		Magic:              Magic,
		Version:            LatestVersion,
		Endianness:         hostEndian(),
		Tgid:               hdr.Tgid,
		MinBlockNs:         hdr.MinBlock,
		SampleMod:          hdr.SampleMod,
		StartedUTC:         hdr.Started,
		KtimeBaseNs:        hdr.KtimeBaseNs,
		WallBaseUTC:        hdr.WallBaseUTC,
		StructEventVersion: structEventVersion,
	}
	b, err := json.Marshal(v2)
	if err != nil {
		return fmt.Errorf("trace header: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
		return err
	}

	stacks := map[int32][]uint64{}
	if opts != nil && opts.Stacks != nil {
		stacks = opts.Stacks
	}
	stkLine, err := json.Marshal(StacksSection{Section: sectionStacks, Stacks: stacks})
	if err != nil {
		return fmt.Errorf("trace stacks section: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", stkLine); err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	for i := range events {
		if err := enc.Encode(events[i]); err != nil {
			return fmt.Errorf("trace event %d: %w", i, err)
		}
	}

	footer := Footer{Section: sectionFooter}
	if opts != nil && opts.Footer != nil {
		footer = *opts.Footer
		footer.Section = sectionFooter
	}
	ftLine, err := json.Marshal(footer)
	if err != nil {
		return fmt.Errorf("trace footer: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", ftLine); err != nil {
		return err
	}
	return nil
}

// Read parses v1 (header + events) or v2 (header + stacks + events + footer).
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
	first := sc.Bytes()

	var probe struct {
		Magic   string `json:"magic"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(first, &probe); err != nil {
		return nil, fmt.Errorf("trace header: %w", err)
	}

	if probe.Magic == Magic && probe.Version == Version2 {
		return readV2(sc, first)
	}
	return readV1(sc, first)
}

func readV1(sc *bufio.Scanner, first []byte) (*File, error) {
	var hdr Header
	if err := json.Unmarshal(first, &hdr); err != nil {
		return nil, fmt.Errorf("trace header: %w", err)
	}
	if hdr.Version != Version1 && hdr.Version != 0 {
		return nil, fmt.Errorf("trace: unsupported version %d (expected %d)", hdr.Version, Version1)
	}
	if hdr.Version == 0 {
		hdr.Version = Version1
	}
	out := &File{Header: hdr, Format: Version1}
	for sc.Scan() {
		line := sc.Bytes()
		if sec, ok := isSectionLine(line); ok {
			return nil, fmt.Errorf("trace: unexpected section %q in v1 trace", sec)
		}
		var ev event.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("trace event: %w", err)
		}
		out.Events = append(out.Events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func readV2(sc *bufio.Scanner, first []byte) (*File, error) {
	var v2 V2Header
	if err := json.Unmarshal(first, &v2); err != nil {
		return nil, fmt.Errorf("trace v2 header: %w", err)
	}
	out := &File{
		Header: headerFromV2(v2),
		Format: Version2,
		Stacks: make(map[int32][]uint64),
	}
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, errors.New("trace v2: missing stacks section")
	}
	var stk StacksSection
	if err := json.Unmarshal(sc.Bytes(), &stk); err != nil {
		return nil, fmt.Errorf("trace stacks section: %w", err)
	}
	if stk.Section != sectionStacks {
		return nil, fmt.Errorf("trace v2: expected stacks section, got %q", stk.Section)
	}
	if stk.Stacks != nil {
		out.Stacks = stk.Stacks
	}

	for sc.Scan() {
		line := sc.Bytes()
		sec, isSec := isSectionLine(line)
		if isSec {
			switch sec {
			case sectionFooter:
				var ft Footer
				if err := json.Unmarshal(line, &ft); err != nil {
					return nil, fmt.Errorf("trace footer: %w", err)
				}
				out.Footer = &ft
				continue
			case sectionStacks:
				return nil, errors.New("trace v2: duplicate stacks section")
			default:
				return nil, fmt.Errorf("trace v2: unknown section %q", sec)
			}
		}
		var ev event.Event
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("trace event: %w", err)
		}
		out.Events = append(out.Events, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
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
