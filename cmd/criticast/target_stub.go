//go:build !linux

package main

import "fmt"

func resolveTGID(pid int) (uint32, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid %d", pid)
	}
	return uint32(pid), nil
}
