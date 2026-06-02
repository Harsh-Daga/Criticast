package analyzer

import "github.com/criticast/criticast/internal/event"

// buildTidToTaskID maps Linux tid → runtime goid from events that carry both.
func buildTidToTaskID(events []event.Event) map[uint32]uint64 {
	m := make(map[uint32]uint64)
	for _, ev := range events {
		if ev.TaskID == 0 || ev.Tid == 0 {
			continue
		}
		m[ev.Tid] = ev.TaskID
	}
	return m
}

func (env scopeEnv) nodeTaskID(n NodeID) (uint64, bool) {
	if g, ok := nodeGoid(n); ok {
		return g, true
	}
	u := uint64(n)
	if u&tidNodeBit == 0 {
		return 0, false
	}
	tid := uint32(u &^ tidNodeBit)
	if env.tidToTaskID == nil {
		return 0, false
	}
	g, ok := env.tidToTaskID[tid]
	return g, ok && g != 0
}

func (env scopeEnv) edgeTouchesRequestGoids(e WaitEdge) bool {
	if g, ok := env.nodeTaskID(e.To); ok {
		if _, ok := env.requestGoids[g]; ok {
			return true
		}
	}
	if g, ok := env.nodeTaskID(e.From); ok {
		if _, ok := env.requestGoids[g]; ok {
			return true
		}
	}
	return false
}
