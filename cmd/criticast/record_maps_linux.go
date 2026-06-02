//go:build linux

package main

import (
	"github.com/criticast/criticast/internal/symbolize"
	"github.com/criticast/criticast/internal/trace"
)

func captureTraceModules(tgid uint32, targetBinary string) []trace.Module {
	mods, err := symbolize.ModulesFromPID(int(tgid), targetBinary)
	if err != nil || len(mods) == 0 {
		return nil
	}
	out := make([]trace.Module, len(mods))
	for i, m := range mods {
		out[i] = trace.Module{
			Path:    m.Path,
			Start:   m.Start,
			End:     m.End,
			BuildID: m.BuildID,
		}
	}
	return out
}
