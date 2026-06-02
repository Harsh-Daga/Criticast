/* SPDX-License-Identifier: GPL-2.0-only */
/*
 * runtime.waitReason values for Go 1.21+ (runtime2.go iota).
 * Regenerate when CI adds a new Go release row in offsets.json.
 */
#ifndef CRITICAST_GO_WAITREASON_H
#define CRITICAST_GO_WAITREASON_H

#define GO_WAIT_REASON_CHAN_RECEIVE  8
#define GO_WAIT_REASON_CHAN_SEND     9
#define GO_WAIT_REASON_SELECT        10
#define GO_WAIT_REASON_SEMACQUIRE      11
#define GO_WAIT_REASON_SLEEP           12
#define GO_WAIT_REASON_SYNC_MUTEX_LOCK 13
#define GO_WAIT_REASON_SYNC_RW_MUTEX_RLOCK 14
#define GO_WAIT_REASON_SYNC_RW_MUTEX_LOCK  15
#define GO_WAIT_REASON_SYNC_WAIT_GROUP 16
#define GO_WAIT_REASON_IO_WAIT         21

static __always_inline __u8 go_reason_to_wait_class(__u32 reason)
{
	switch (reason) {
	case GO_WAIT_REASON_CHAN_RECEIVE:
	case GO_WAIT_REASON_CHAN_SEND:
		return WC_CHAN;
	case GO_WAIT_REASON_SYNC_MUTEX_LOCK:
	case GO_WAIT_REASON_SYNC_RW_MUTEX_RLOCK:
	case GO_WAIT_REASON_SYNC_RW_MUTEX_LOCK:
		return WC_MUTEX;
	case GO_WAIT_REASON_SELECT:
		return WC_SELECT;
	case GO_WAIT_REASON_SEMACQUIRE:
		return WC_SEMA;
	case GO_WAIT_REASON_IO_WAIT:
		return WC_NET;
	case GO_WAIT_REASON_SLEEP:
		return WC_SLEEP;
	case 1:
	case 2:
	case 3:
	case 4:
	case 5:
		return WC_GC;
	default:
		return WC_UNKNOWN;
	}
}

#endif
