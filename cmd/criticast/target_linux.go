//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// resolveTGID returns the thread-group ID for a PID (from /proc/pid/status).
// --pid may be any thread in the group; BPF targets map keys on TGID.
func resolveTGID(pid int) (uint32, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid %d", pid)
	}
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, fmt.Errorf("open /proc/%d/status: %w", pid, err)
	}
	defer f.Close()

	var tgid int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "Tgid:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			tgid, err = strconv.Atoi(fields[1])
			if err != nil {
				return 0, fmt.Errorf("parse Tgid: %w", err)
			}
			break
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if tgid <= 0 {
		return 0, fmt.Errorf("Tgid not found for pid %d", pid)
	}
	return uint32(tgid), nil
}
