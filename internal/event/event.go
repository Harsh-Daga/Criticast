// Package event defines the L2→L4 wire contract (CHARTER §B.2).
package event

import "unsafe"

const Size = 80

type Type uint8

const (
	EVBlockBegin Type = iota
	EVBlockEnd
	EVRunQ
	EVSyscallBoundary
	EVIOSubmit
	EVIOComplete
	EVTaskState
	EVSpanOpen
	EVSpanClose
	EVSpawn
)

type WaitClass uint8

const (
	WCUnknown WaitClass = iota
	WCFutex
	WCEpoll
	WCIoUring
	WCNet
	WCDisk
	WCRunQ
	WCSleep
	WCGC
	WCChan
	WCMutex
	WCSelect
	WCSema
	WCCond
)

// Event is the fixed 80-byte kernel record (must match bpf/event.h).
type Event struct {
	TsNs         uint64
	CPU          uint32
	Tgid         uint32
	Tid          uint32
	WakerTid     uint32
	BlockedNs    uint64
	StackID      int32
	WakerStackID int32
	Cookie       uint64
	TaskID       uint64
	WakerTaskID  uint64
	Aux          uint64
	Type         Type
	WaitClass    WaitClass
	PrevState    uint8
	Confidence   uint8
}

func init() {
	if unsafe.Sizeof(Event{}) != Size {
		panic("event.Event size mismatch with kernel struct")
	}
}

// Stat indices match bpf/event.h stat_idx.
const (
	StatRingbufDrops = iota
	StatEventsEmitted
	StatBlocksSeen
	StatPreempts
	StatRunQClosed
	StatShortFiltered
	StatSampledOut
	StatStackFail
	StatSwitchSeen
	StatTargetPrev
	StatRunningEmitted
	StatMax
)
