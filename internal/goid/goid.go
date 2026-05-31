// Package goid reads the current goroutine ID from the runtime stack header.
// Ground-truth logging helper — production attribution uses DWARF/casgstatus.
package goid

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
)

// Current returns the goid of the calling goroutine.
// Parses the "goroutine N [" prefix from runtime.Stack(false).
func Current() (uint64, error) {
	var buf [128]byte
	n := runtime.Stack(buf[:], false)
	id, err := ParseStackHeader(buf[:n])
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ParseStackHeader extracts the goroutine id from a runtime.Stack snippet.
func ParseStackHeader(stack []byte) (uint64, error) {
	// "goroutine 123 [running]:\n"
	if !bytes.HasPrefix(stack, []byte("goroutine ")) {
		return 0, fmt.Errorf("goid: unexpected stack prefix %q", truncate(stack, 24))
	}
	rest := stack[len("goroutine "):]
	sp := bytes.IndexByte(rest, ' ')
	if sp <= 0 {
		return 0, fmt.Errorf("goid: missing id in stack header")
	}
	return strconv.ParseUint(string(rest[:sp]), 10, 64)
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return string(b)
}
