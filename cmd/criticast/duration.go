package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseCLIDuration accepts Go durations (30s, 5m) or a plain integer meaning seconds.
func ParseCLIDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use 30s, 5m, or plain seconds)", s)
	}
	return time.Duration(n) * time.Second, nil
}

// ParseDurationNS parses min-block flags like 1us, 50us, 2ms into nanoseconds.
func ParseDurationNS(s string) (uint64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "1us", "1µs":
		return 1000, nil
	case "50us", "50µs":
		return 50000, nil
	}
	if strings.HasSuffix(s, "us") || strings.HasSuffix(s, "µs") {
		num := strings.TrimSuffix(strings.TrimSuffix(s, "us"), "µs")
		n, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse microseconds: %w", err)
		}
		return n * 1000, nil
	}
	if strings.HasSuffix(s, "ms") {
		n, err := strconv.ParseUint(strings.TrimSuffix(s, "ms"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse milliseconds: %w", err)
		}
		return n * 1_000_000, nil
	}
	return 0, fmt.Errorf("unsupported duration %q (try 1us, 50us, or Nms)", s)
}
