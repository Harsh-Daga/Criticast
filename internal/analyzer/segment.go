package analyzer

import (
	"sort"

	"github.com/criticast/criticast/internal/event"
)

// SegKind is RUNNING, RUNNABLE, or BLOCKED (charter E.1).
type SegKind uint8

const (
	Running SegKind = iota
	Runnable
	Blocked
)

// Segment is a contiguous interval for one task (tid or task_id).
type Segment struct {
	Start, End uint64
	Kind       SegKind
	WaitClass  event.WaitClass
	WakerID    uint64 // task_id of waker when Kind == Blocked
	Cookie     uint64
	Confidence uint8
	StackID    int32
	WakerStkID int32
	Tgid       uint32
	Tid        uint32
	TaskID     uint64
}

// TaskKey identifies a thread/goroutine for segment grouping.
type TaskKey struct {
	TaskID uint64
	Tid    uint32
}

func taskKeyFromEvent(ev event.Event) TaskKey {
	if ev.TaskID != 0 {
		return TaskKey{TaskID: ev.TaskID, Tid: ev.Tid}
	}
	return TaskKey{Tid: ev.Tid}
}

// BuildSegments folds events into per-task segments (block-end, runq, measured running).
func BuildSegments(events []event.Event) map[TaskKey][]Segment {
	byTask := make(map[TaskKey][]event.Event)
	for _, ev := range events {
		k := taskKeyFromEvent(ev)
		byTask[k] = append(byTask[k], ev)
	}
	out := make(map[TaskKey][]Segment, len(byTask))
	for k, evs := range byTask {
		sort.Slice(evs, func(i, j int) bool { return evs[i].TsNs < evs[j].TsNs })
		out[k] = segmentsForTask(k, evs)
	}
	return out
}

func segmentsForTask(k TaskKey, evs []event.Event) []Segment {
	var segs []Segment
	for _, ev := range evs {
		switch ev.Type {
		case event.EVBlockEnd:
			if ev.BlockedNs == 0 {
				continue
			}
			start := ev.TsNs - ev.BlockedNs
			waker := ev.WakerTaskID
			if waker == 0 {
				waker = uint64(ev.WakerTid)
			}
			segs = append(segs, Segment{
				Start:      start,
				End:        ev.TsNs,
				Kind:       Blocked,
				WaitClass:  ev.WaitClass,
				WakerID:    waker,
				Cookie:     ev.Cookie,
				Confidence: ev.Confidence,
				StackID:    ev.StackID,
				WakerStkID: ev.WakerStackID,
				Tgid:       ev.Tgid,
				Tid:        ev.Tid,
				TaskID:     k.TaskID,
			})
		case event.EVRunQ:
			if ev.BlockedNs == 0 {
				continue
			}
			start := ev.TsNs - ev.BlockedNs
			segs = append(segs, Segment{
				Start:     start,
				End:       ev.TsNs,
				Kind:      Runnable,
				WaitClass: event.WCRunQ,
				Tgid:      ev.Tgid,
				Tid:       ev.Tid,
				TaskID:    k.TaskID,
			})
		case event.EVTaskState:
			if ev.BlockedNs == 0 {
				continue
			}
			start := ev.TsNs - ev.BlockedNs
			segs = append(segs, Segment{
				Start:  start,
				End:    ev.TsNs,
				Kind:   Running,
				Tgid:   ev.Tgid,
				Tid:    ev.Tid,
				TaskID: k.TaskID,
			})
		}
	}
	return segs
}

// Summary holds aggregate segment stats for text output.
type Summary struct {
	Tasks          int
	BlockedSegs    int
	RunnableSegs   int
	TotalBlockedNs uint64
}

// SummarizeSegments counts segments across all tasks.
func SummarizeSegments(byTask map[TaskKey][]Segment) Summary {
	var s Summary
	s.Tasks = len(byTask)
	for _, segs := range byTask {
		for _, seg := range segs {
			switch seg.Kind {
			case Blocked:
				s.BlockedSegs++
				s.TotalBlockedNs += seg.End - seg.Start
			case Runnable:
				s.RunnableSegs++
			case Running:
				// counted in TotalBlockedNs only for Blocked; running tracked via segments
			}
		}
	}
	return s
}
