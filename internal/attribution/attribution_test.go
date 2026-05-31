package attribution

import "testing"

// scoreMust is like Score but fails the test on error.
func (e *Evaluator) scoreMust(t *testing.T, gold []Label, pred []PredictedEdge, mechanisms []string) Matrix {
	t.Helper()
	m, err := e.Score(gold, pred, mechanisms)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	return m
}

func TestJaccard(t *testing.T) {
	j := Jaccard([]string{"a", "b"}, []string{"b", "c"})
	want := 1.0 / 3.0
	if j < want-1e-9 || j > want+1e-9 {
		t.Fatalf("jaccard = %v want %v", j, want)
	}
}

func TestScoreSpawnLineage(t *testing.T) {
	ev := NewEvaluator(E1LineageOnly)
	gold := []Label{{WakeeToken: "A"}, {WakeeToken: "B"}}
	pred := []PredictedEdge{
		{Edge: Edge{Mechanism: MechSpawnLineage}, PredictedToken: "A"},
		{Edge: Edge{Mechanism: MechSpawnLineage}, PredictedToken: "B"},
	}
	m := ev.scoreMust(t, gold, pred, []string{MechSpawnLineage})
	if len(m) != 1 || m[0].Precision != 1 || m[0].Recall != 1 {
		t.Fatalf("score = %+v", m)
	}
}

func TestScoreLengthMismatch(t *testing.T) {
	ev := NewEvaluator(E1LineageOnly)
	_, err := ev.Score([]Label{{WakeeToken: "A"}}, []PredictedEdge{}, []string{MechSpawnLineage})
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
}

func TestValidateModeAllExperiments(t *testing.T) {
	for _, m := range []Mode{E1LineageOnly, E2SudogElem, E3ResourceSuppress, E4NaiveForward} {
		if err := NewEvaluator(m).ValidateMode(); err != nil {
			t.Fatalf("mode %d: %v", m, err)
		}
	}
}
