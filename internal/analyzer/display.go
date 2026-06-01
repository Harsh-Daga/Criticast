package analyzer

import (
	"fmt"

	"github.com/criticast/criticast/internal/event"
	"github.com/criticast/criticast/internal/mechanism"
)

// DisplayWaitClass is the user-facing wait class (GT mechanism may refine BPF class).
func DisplayWaitClass(e WaitEdge) string {
	switch e.Meta.Mechanism {
	case mechanism.ConnPool:
		return "WC_CONN_POOL"
	case mechanism.ChanWorkHandoff:
		if e.WaitClass == event.WCChan {
			return "WC_CHAN"
		}
	case mechanism.Mutex:
		if e.WaitClass == event.WCMutex {
			return "WC_MUTEX"
		}
	}
	return waitClassName(e.WaitClass)
}

func waitClassName(wc event.WaitClass) string {
	names := []string{
		"WC_UNKNOWN", "WC_FUTEX", "WC_EPOLL", "WC_IOURING", "WC_NET", "WC_DISK",
		"WC_RUNQ", "WC_SLEEP", "WC_GC", "WC_CHAN", "WC_MUTEX", "WC_SELECT", "WC_SEMA", "WC_COND",
	}
	if int(wc) < len(names) {
		return names[wc]
	}
	return fmt.Sprintf("WC_%d", wc)
}
