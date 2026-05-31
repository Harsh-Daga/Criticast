package attribution

import (
	"testing"

	"github.com/criticast/criticast/internal/groundtruth"
)

func TestRunExperimentE1(t *testing.T) {
	records := []groundtruth.Record{
		{Token: "A", Site: groundtruth.SiteHandlerEntry, Goid: 1},
		{Token: "A", Site: groundtruth.SiteSpawn, Goid: 2},
		{Token: "A", Site: groundtruth.SiteSpawnWork, Goid: 2},
	}
	m, err := RunExperiment(E1LineageOnly, records, []string{MechSpawnLineage})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("matrix len %d", len(m))
	}
}

func TestRunExperimentE2ChanElem(t *testing.T) {
	// Two requests through one worker goid: E1 keeps stale cookie after first recv; E2 uses elem.
	records := []groundtruth.Record{
		{Token: "A", Site: groundtruth.SiteHandlerEntry, Goid: 10},
		{Token: "A", Site: groundtruth.SiteWorkerPoolSend, Goid: 10, Extra: "1"},
		{Token: "A", Site: groundtruth.SiteWorkerRecv, Goid: 99, Extra: "1"},
		{Token: "B", Site: groundtruth.SiteHandlerEntry, Goid: 11},
		{Token: "B", Site: groundtruth.SiteWorkerPoolSend, Goid: 11, Extra: "2"},
		{Token: "B", Site: groundtruth.SiteWorkerRecv, Goid: 99, Extra: "2"},
	}
	e1, err := RunExperiment(E1LineageOnly, records, []string{MechChanWorkHandoff})
	if err != nil {
		t.Fatal(err)
	}
	e2, err := RunExperiment(E2SudogElem, records, []string{MechChanWorkHandoff})
	if err != nil {
		t.Fatal(err)
	}
	if e1[0].Precision >= 0.9 {
		t.Fatalf("E1 expected low precision with stale worker cookie, got %v", e1[0].Precision)
	}
	if e2[0].Precision < 1.0 {
		t.Fatalf("E2 expected perfect chan match, got precision=%v", e2[0].Precision)
	}
}

func TestParseMode(t *testing.T) {
	m, err := ParseMode("e3-suppress")
	if err != nil || m != E3ResourceSuppress {
		t.Fatalf("got %v %v", m, err)
	}
}
