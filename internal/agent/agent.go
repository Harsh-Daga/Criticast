// Package agent drains the BPF ring buffer into a bounded channel (L1).
package agent

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"

	"github.com/criticast/criticast/internal/event"
)

// DefaultEventChanCap is the bounded channel size for ringbuf → userspace drain.
const DefaultEventChanCap = 4096

// Stats tracks userspace-side drops and events received.
type Stats struct {
	Received   uint64
	ChanDrops  uint64
	ReadErrors uint64
	Malformed  uint64
}

// Recorder drains kernel ringbuf events.
type Recorder struct {
	reader *ringbuf.Reader
	out    chan event.Event
	stats  Stats
}

// NewRecorder creates a ringbuf reader on the given map.
func NewRecorder(eventsMap *ebpf.Map, chanCap int) (*Recorder, error) {
	if eventsMap == nil {
		return nil, errors.New("events map is nil")
	}
	if chanCap <= 0 {
		chanCap = DefaultEventChanCap
	}
	rd, err := ringbuf.NewReader(eventsMap)
	if err != nil {
		return nil, fmt.Errorf("ringbuf reader: %w", err)
	}
	return &Recorder{
		reader: rd,
		out:    make(chan event.Event, chanCap),
	}, nil
}

// Events returns the bounded event channel.
func (r *Recorder) Events() <-chan event.Event {
	return r.out
}

// StatsSnapshot returns atomic counters.
func (r *Recorder) StatsSnapshot() Stats {
	return Stats{
		Received:   atomic.LoadUint64(&r.stats.Received),
		ChanDrops:  atomic.LoadUint64(&r.stats.ChanDrops),
		ReadErrors: atomic.LoadUint64(&r.stats.ReadErrors),
		Malformed:  atomic.LoadUint64(&r.stats.Malformed),
	}
}

// Run drains until ctx is cancelled. Never blocks the kernel — drops if channel full.
func (r *Recorder) Run(ctx context.Context) error {
	defer r.reader.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		rec, err := r.reader.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return nil
			}
			if errors.Is(err, context.Canceled) {
				return err
			}
			atomic.AddUint64(&r.stats.ReadErrors, 1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				continue
			}
		}

		ev, ok := decodeEvent(rec.RawSample)
		if !ok {
			atomic.AddUint64(&r.stats.Malformed, 1)
			continue
		}
		atomic.AddUint64(&r.stats.Received, 1)

		select {
		case r.out <- ev:
		case <-ctx.Done():
			return ctx.Err()
		default:
			atomic.AddUint64(&r.stats.ChanDrops, 1)
		}
	}
}

func decodeEvent(raw []byte) (event.Event, bool) {
	if len(raw) < event.Size {
		return event.Event{}, false
	}
	var ev event.Event
	copy((*[event.Size]byte)(unsafe.Pointer(&ev))[:], raw[:event.Size])
	return ev, true
}
