package attribution

import (
	"testing"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
)

func TestTokenForChanHandoffWorkerBlock(t *testing.T) {
	t0 := time.Now()
	handoffs := []ChanHandoff{{
		Token: "A", Elem: 1, HandlerGoid: 10, WorkerGoid: 99,
		SendTS: t0, RecvTS: t0.Add(5 * time.Millisecond),
	}}
	tok := TokenForChanHandoff(handoffs, 99, t0.Add(2*time.Millisecond))
	if tok != "A" {
		t.Fatalf("got %q", tok)
	}
}

func TestBuildChanHandoffsPairs(t *testing.T) {
	t0 := time.Now()
	recs := []groundtruth.Record{
		{Token: "A", Site: groundtruth.SiteWorkerPoolSend, Goid: 10, Extra: "42", TS: t0},
		{Token: "A", Site: groundtruth.SiteWorkerRecv, Goid: 99, Extra: "42", TS: t0.Add(time.Millisecond)},
	}
	h := BuildChanHandoffs(recs)
	if len(h) != 1 || h[0].WorkerGoid != 99 {
		t.Fatalf("got %+v", h)
	}
}
