// Package attribution implements lineage-first request identity (L3) and P0-B evaluators.
package attribution

import (
	"fmt"

	"github.com/criticast/criticast/internal/mechanism"
)

// Mode selects an attribution experiment (CHARTER Part H.2).
type Mode int

const (
	E1LineageOnly Mode = iota
	E2SudogElem
	E3ResourceSuppress
	E4NaiveForward
)

// Re-export mechanism names for the P0-B matrix (single source: internal/mechanism).
const (
	MechSpawnLineage    = mechanism.SpawnLineage
	MechChanWorkHandoff = mechanism.ChanWorkHandoff
	MechConnPool        = mechanism.ConnPool
	MechMutex           = mechanism.Mutex
	MechBroadcast       = mechanism.Broadcast
	MechNetpoll         = mechanism.Netpoll
)

// Edge is a labeled wait-for edge for evaluation.
type Edge struct {
	WakeeGoid uint64
	WakerGoid uint64
	BlockedNs uint64
	Mechanism string
}

// Label is ground-truth request token for an edge endpoint.
type Label struct {
	WakeeToken string
	WakerToken string
}

// MechanismScore holds precision/recall for one mechanism.
type MechanismScore struct {
	Mechanism     string
	Precision     float64
	Recall        float64
	FalseEdgeRate float64
	EdgeCount     int
}

// Matrix is the P0-B deliverable table.
type Matrix []MechanismScore

// PredictedEdge carries an attributed request token for one directed edge.
type PredictedEdge struct {
	Edge
	PredictedToken string
	Confidence     uint8
	Ambiguous      bool
}

// Evaluator scores predicted edges against ground truth.
type Evaluator struct {
	Mode Mode
}

// NewEvaluator returns an evaluator for experiment E1–E4.
func NewEvaluator(mode Mode) *Evaluator {
	return &Evaluator{Mode: mode}
}

// Score computes per-mechanism precision/recall.
// gold and pred must have the same length (one label row per predicted edge).
func (e *Evaluator) Score(gold []Label, pred []PredictedEdge, mechanisms []string) (Matrix, error) {
	if len(gold) != len(pred) {
		return nil, fmt.Errorf("attribution: gold len %d != pred len %d", len(gold), len(pred))
	}
	out := make(Matrix, 0, len(mechanisms))
	for _, m := range mechanisms {
		tp, fp, fn, n := 0, 0, 0, 0
		for i := range pred {
			if pred[i].Mechanism != m {
				continue
			}
			n++
			gt := gold[i].WakeeToken
			if pred[i].PredictedToken == gt && gt != "" {
				tp++
			} else if pred[i].PredictedToken != "" {
				fp++
			}
			if gt != "" && pred[i].PredictedToken != gt {
				fn++
			}
		}
		ms := MechanismScore{Mechanism: m, EdgeCount: n}
		if tp+fp > 0 {
			ms.Precision = float64(tp) / float64(tp+fp)
		}
		if tp+fn > 0 {
			ms.Recall = float64(tp) / float64(tp+fn)
		}
		if tp+fp > 0 {
			ms.FalseEdgeRate = float64(fp) / float64(tp+fp)
		}
		out = append(out, ms)
	}
	return out, nil
}

// ValidateMode returns an error for unknown experiment modes.
func (e *Evaluator) ValidateMode() error {
	switch e.Mode {
	case E1LineageOnly, E2SudogElem, E3ResourceSuppress, E4NaiveForward:
		return nil
	default:
		return fmt.Errorf("attribution: unknown mode %d", e.Mode)
	}
}

// Jaccard returns |A∩B|/|A∪B| for critical-path edge sets (string edge keys).
func Jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	setA := make(map[string]struct{}, len(a))
	for _, k := range a {
		setA[k] = struct{}{}
	}
	inter, union := 0, len(setA)
	for _, k := range b {
		if _, ok := setA[k]; ok {
			inter++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
