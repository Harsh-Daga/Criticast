package analyzer

import "github.com/criticast/criticast/internal/event"

// expandEpochMembershipFromHandlerWakeups adds the handler and every waker that unblocked
// the handler during [kFrom, kTo] (observed wait-for graph; no send-side GT window).
func expandEpochMembershipFromHandlerWakeups(
	events []event.Event,
	handlerTask uint64,
	kFrom, kTo uint64,
	out map[uint64]struct{},
) map[uint64]struct{} {
	if out == nil {
		out = make(map[uint64]struct{})
	}
	if handlerTask != 0 {
		out[handlerTask] = struct{}{}
	}
	if kTo <= kFrom || handlerTask == 0 {
		return out
	}
	for _, ev := range events {
		if ev.Type != event.EVBlockEnd || ev.BlockedNs == 0 {
			continue
		}
		wakee := ev.TaskID
		if wakee == 0 {
			wakee = uint64(ev.Tid)
		}
		if wakee != handlerTask {
			continue
		}
		start := ev.TsNs - ev.BlockedNs
		end := ev.TsNs
		if end < kFrom || start > kTo {
			continue
		}
		waker := ev.WakerTaskID
		if waker == 0 {
			waker = uint64(ev.WakerTid)
		}
		if waker != 0 {
			out[waker] = struct{}{}
		}
	}
	return out
}
