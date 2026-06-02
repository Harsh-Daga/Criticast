package analyzer

import "testing"

func TestLongestPathTemporalOverlappingChain(t *testing.T) {
	const (
		base = uint64(1_100_000_000)
		dur  = uint64(1_600_000)
		n    = 50
	)
	var edges []WaitEdge
	for i := 0; i < n; i++ {
		from := NodeFromTask(20+uint64(i), 0)
		to := NodeFromTask(20+uint64(i+1), 0)
		edges = append(edges, WaitEdge{
			From: from, To: to,
			BlockedNs: dur,
			StartNs:   base,
			EndNs:     base + dur,
		})
	}
	old := LongestPath(edges)
	temporal := LongestPathTemporal(edges, 0)
	if old.PathWeight <= dur*2 {
		t.Fatalf("non-temporal should overcount overlapping chain: got %d", old.PathWeight)
	}
	if temporal.PathWeight > dur+100_000 {
		t.Fatalf("temporal=%d want ~%d (old=%d)", temporal.PathWeight, dur, old.PathWeight)
	}
}

func TestPathWeightInvariantOverlapping(t *testing.T) {
	wall := uint64(14_500_000)
	path := CriticalPath{PathWeight: 1_600_000}
	if !PathWeightInvariantOK(path, wall, 2_000_000) {
		t.Fatal("expected pass")
	}
	path.PathWeight = 970_000_000
	if PathWeightInvariantOK(path, wall, 2_000_000) {
		t.Fatal("expected fail for 970ms path on 14.5ms wall")
	}
}
