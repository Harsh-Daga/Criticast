package analyzer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/criticast/criticast/internal/event"
)

// nodeIDSpace separates thread-only ids from goroutine task_ids.
const tidNodeBit = uint64(1) << 40

// NodeID is a stable graph vertex (prefer task_id / goid, else tid).
type NodeID uint64

// NodeFromTask returns the graph node for a task_id or tid.
func NodeFromTask(taskID uint64, tid uint32) NodeID {
	if taskID != 0 {
		return NodeID(taskID)
	}
	return NodeID(uint64(tid) | tidNodeBit)
}

// NodeFromEvent derives the wakee node for an event.
func NodeFromEvent(ev event.Event) NodeID {
	return NodeFromTask(ev.TaskID, ev.Tid)
}

// ParseRequestScope matches --request against cookie (hex) or tid (decimal).
func ParseRequestScope(scope string) (cookie uint64, tid uint32, ok bool) {
	if scope == "" {
		return 0, 0, false
	}
	scope = strings.TrimSpace(scope)
	if strings.HasPrefix(scope, "0x") || strings.HasPrefix(scope, "0X") {
		v, err := strconv.ParseUint(scope, 0, 64)
		return v, 0, err == nil
	}
	if v, err := strconv.ParseUint(scope, 10, 32); err == nil {
		return 0, uint32(v), true
	}
	v, err := strconv.ParseUint(scope, 16, 64)
	return v, 0, err == nil
}

// MatchesScope reports whether an edge belongs to the request scope.
func MatchesScope(cookie uint64, tid uint32, scopeCookie uint64, scopeTid uint32, scoped bool) bool {
	if !scoped {
		return true
	}
	if scopeCookie != 0 && cookie == scopeCookie {
		return true
	}
	if scopeTid != 0 && tid == scopeTid {
		return true
	}
	return false
}

// TaskKeyFromNode maps a graph node back to segment grouping key.
func TaskKeyFromNode(id NodeID, fallbackTid uint32) TaskKey {
	u := uint64(id)
	if u&tidNodeBit != 0 {
		return TaskKey{Tid: uint32(u &^ tidNodeBit)}
	}
	if u != 0 {
		return TaskKey{TaskID: u}
	}
	return TaskKey{Tid: fallbackTid}
}

// FormatNode renders a node id for CLI output.
func FormatNode(id NodeID) string {
	u := uint64(id)
	if u&tidNodeBit != 0 {
		return fmt.Sprintf("tid=%d", uint32(u&^tidNodeBit))
	}
	return fmt.Sprintf("task_id=%d", u)
}
