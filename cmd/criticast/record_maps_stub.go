//go:build !linux

package main

import "github.com/criticast/criticast/internal/trace"

func captureTraceModules(uint32, string) []trace.Module {
	return nil
}
