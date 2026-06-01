// Package export writes critical-path profiles (pprof, OTLP stub).
package export

import (
	"fmt"
	"os"

	"github.com/google/pprof/profile"

	"github.com/criticast/criticast/internal/analyzer"
	"github.com/criticast/criticast/internal/symbolize"
)

// PprofInput is the data needed to build a critical-wait pprof profile.
type PprofInput struct {
	Edges    []analyzer.PathEdge
	Resolver symbolize.Resolver
}

// WritePprof writes a gzip-compressed pprof profile (charter Appendix P).
func WritePprof(path string, in PprofInput) error {
	if path == "" {
		return fmt.Errorf("export: empty output path")
	}
	p, err := BuildPprofProfile(in)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// profile.Write already emits gzip-compressed protobuf (do not wrap again).
	return p.Write(f)
}

// BuildPprofProfile maps critical-path edges to pprof samples (value = blocked_ns).
func BuildPprofProfile(in PprofInput) (*profile.Profile, error) {
	vt := &profile.ValueType{Type: "critical_wait", Unit: "nanoseconds"}
	prof := &profile.Profile{
		SampleType:        []*profile.ValueType{vt},
		DefaultSampleType: "critical_wait",
		PeriodType:        vt,
		Period:            1,
	}
	locByStack := make(map[int32]*profile.Location)
	nextFnID := uint64(1)

	getLoc := func(stackID int32) *profile.Location {
		if loc, ok := locByStack[stackID]; ok {
			return loc
		}
		loc := &profile.Location{ID: uint64(len(prof.Location) + 1)}
		if in.Resolver != nil && stackID >= 0 {
			frames, _ := in.Resolver.Resolve(stackID)
			for _, fr := range frames {
				fn := &profile.Function{
					ID:         nextFnID,
					Name:       fr.Function,
					SystemName: fr.Function,
					Filename:   fr.File,
				}
				nextFnID++
				prof.Function = append(prof.Function, fn)
				loc.Line = append(loc.Line, profile.Line{Function: fn})
				if loc.Address == 0 {
					loc.Address = fr.PC
				}
			}
		}
		if len(loc.Line) == 0 {
			fn := &profile.Function{ID: nextFnID, Name: "unknown", SystemName: "unknown"}
			nextFnID++
			prof.Function = append(prof.Function, fn)
			loc.Line = []profile.Line{{Function: fn}}
		}
		prof.Location = append(prof.Location, loc)
		locByStack[stackID] = loc
		return loc
	}

	for _, pe := range in.Edges {
		loc := getLoc(pe.WakerStkID)
		prof.Sample = append(prof.Sample, &profile.Sample{
			Value:    []int64{int64(pe.BlockedNs)},
			Location: []*profile.Location{loc},
		})
	}
	if len(prof.Sample) == 0 {
		fn := &profile.Function{ID: 1, Name: "criticast.empty", SystemName: "criticast.empty"}
		prof.Function = []*profile.Function{fn}
		loc := &profile.Location{ID: 1, Line: []profile.Line{{Function: fn}}}
		prof.Location = []*profile.Location{loc}
		prof.Sample = []*profile.Sample{{
			Value:    []int64{0},
			Location: []*profile.Location{loc},
		}}
	}
	if err := prof.CheckValid(); err != nil {
		return nil, fmt.Errorf("export: invalid profile: %w", err)
	}
	return prof, nil
}
