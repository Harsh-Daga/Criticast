package symbolize

import (
	"testing"
)

func TestTraceResolverResolve(t *testing.T) {
	r := NewTraceResolver(map[int32][]uint64{
		1: {0x401000, 0x402000},
	})
	frames, err := r.Resolve(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || frames[0].PC != 0x401000 {
		t.Fatalf("got %+v", frames)
	}
	// cache hit
	frames2, err := r.Resolve(1)
	if err != nil || len(frames2) != 2 {
		t.Fatalf("cache: %+v err=%v", frames2, err)
	}
}

func TestTraceResolverMissingStack(t *testing.T) {
	r := NewTraceResolver(nil)
	frames, err := r.Resolve(99)
	if err != nil || frames != nil {
		t.Fatalf("got frames=%v err=%v", frames, err)
	}
	if f, err := r.Resolve(-1); err != nil || f != nil {
		t.Fatalf("negative id: %v %v", f, err)
	}
}

func TestMemoryBuildIDCache(t *testing.T) {
	c := NewMemoryBuildIDCache()
	c.Store("abc", "/usr/bin/foo")
	p, ok := c.Lookup("abc")
	if !ok || p != "/usr/bin/foo" {
		t.Fatalf("lookup: %q %v", p, ok)
	}
}
