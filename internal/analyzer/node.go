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

// RequestScope is the analyze/export scope selector from --request.
type RequestScope struct {
	Cookie uint64
	Tid    uint32
	TaskID uint64 // runtime goroutine id (casgstatus task_id), not Linux tid
	Token  string // GT request token (requires --gt-log; joins trace wall time)
}

// Active reports whether any scope field is set.
func (s RequestScope) Active() bool {
	return s.Cookie != 0 || s.Tid != 0 || s.TaskID != 0 || s.Token != ""
}

// ParseRequestScope parses --request: cookie (0x…), tid (decimal or tid=N), goid (goid=N).
func ParseRequestScope(scope string) (RequestScope, bool) {
	if scope == "" {
		return RequestScope{}, false
	}
	scope = strings.TrimSpace(scope)
	lower := strings.ToLower(scope)
	if strings.HasPrefix(lower, "goid=") || strings.HasPrefix(lower, "task_id=") {
		key, val, ok := strings.Cut(scope, "=")
		if !ok {
			return RequestScope{}, false
		}
		_ = key
		v, err := strconv.ParseUint(strings.TrimSpace(val), 10, 64)
		if err != nil || v == 0 {
			return RequestScope{}, false
		}
		return RequestScope{TaskID: v}, true
	}
	if strings.HasPrefix(lower, "token=") {
		_, val, ok := strings.Cut(scope, "=")
		if !ok {
			return RequestScope{}, false
		}
		val = strings.TrimSpace(val)
		if val == "" {
			return RequestScope{}, false
		}
		return RequestScope{Token: val}, true
	}
	if strings.HasPrefix(lower, "tid=") {
		_, val, ok := strings.Cut(scope, "=")
		if !ok {
			return RequestScope{}, false
		}
		v, err := strconv.ParseUint(strings.TrimSpace(val), 10, 32)
		if err != nil || v == 0 {
			return RequestScope{}, false
		}
		return RequestScope{Tid: uint32(v)}, true
	}
	if strings.HasPrefix(scope, "0x") || strings.HasPrefix(scope, "0X") {
		v, err := strconv.ParseUint(scope, 0, 64)
		return RequestScope{Cookie: v}, err == nil
	}
	if v, err := strconv.ParseUint(scope, 10, 32); err == nil {
		return RequestScope{Tid: uint32(v)}, true
	}
	v, err := strconv.ParseUint(scope, 16, 64)
	return RequestScope{Cookie: v}, err == nil && v != 0
}

// EdgeMatchesScope reports whether a wait edge belongs to the request scope.
func EdgeMatchesScope(e WaitEdge, scope RequestScope, scoped bool) bool {
	if !scoped {
		return true
	}
	if scope.Cookie != 0 && e.Cookie == scope.Cookie {
		return true
	}
	if scope.Tid != 0 && e.Tid == scope.Tid {
		return true
	}
	if scope.TaskID != 0 {
		if uint64(e.To) == scope.TaskID || uint64(e.From) == scope.TaskID {
			return true
		}
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
