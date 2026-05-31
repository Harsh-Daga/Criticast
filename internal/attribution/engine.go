package attribution

import (
	"fmt"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
)

// Engine applies experiment mode E1–E4 to attribute request tokens.
type Engine struct {
	Mode    Mode
	Lineage *LineageStore
	naive   map[uint64]string // E4: forward waker cookie (baseline to beat)
}

// NewEngine builds an attribution engine for mode m.
func NewEngine(m Mode, cookieTTL time.Duration) (*Engine, error) {
	ev := &Evaluator{Mode: m}
	if err := ev.ValidateMode(); err != nil {
		return nil, err
	}
	return &Engine{
		Mode:    m,
		Lineage: NewLineageStore(cookieTTL),
		naive:   make(map[uint64]string),
	}, nil
}

// ReplayGroundTruth seeds lineage from GT log order.
func (e *Engine) ReplayGroundTruth(records []groundtruth.Record) {
	for _, rec := range records {
		e.Lineage.ApplyRecord(rec)
	}
}

// Attribute predicts the request token for one site edge.
func (e *Engine) Attribute(edge groundtruth.SiteEdge, wakerGoid uint64, wakerToken string, sudogElem uint64) PredictedEdge {
	pe := PredictedEdge{
		Edge: Edge{
			WakeeGoid: edge.Goid,
			WakerGoid: wakerGoid,
			Mechanism: edge.Mechanism,
		},
	}
	gold := edge.Token

	switch e.Mode {
	case E1LineageOnly:
		pe.PredictedToken = e.Lineage.Cookie(edge.Goid, edge.TS)
		pe.Confidence = confidenceFor(pe.PredictedToken, gold, false)

	case E2SudogElem:
		tok := e.Lineage.Cookie(edge.Goid, edge.TS)
		if edge.Mechanism == MechChanWorkHandoff {
			if sudogElem != 0 {
				if t := e.Lineage.SudogToken(sudogElem); t != "" {
					tok = t
				} else if wakerToken != "" {
					e.Lineage.NoteSudogElem(sudogElem, wakerToken)
					tok = wakerToken
				}
			} else if wakerToken != "" {
				tok = wakerToken
			}
		}
		pe.PredictedToken = tok
		pe.Confidence = confidenceFor(pe.PredictedToken, gold, false)

	case E3ResourceSuppress:
		pe.PredictedToken = e.Lineage.Cookie(edge.Goid, edge.TS)
		if IsResourceMechanism(edge.Mechanism) {
			pe.Ambiguous = true
			pe.Confidence = 50
		} else {
			pe.Confidence = confidenceFor(pe.PredictedToken, gold, false)
		}

	case E4NaiveForward:
		if wakerToken != "" {
			e.naive[edge.Goid] = wakerToken
		}
		pe.PredictedToken = e.naive[edge.Goid]
		if pe.PredictedToken == "" {
			pe.PredictedToken = wakerToken
		}
		pe.Confidence = confidenceFor(pe.PredictedToken, gold, true)

	default:
		pe.Ambiguous = true
	}
	if pe.PredictedToken != gold && gold != "" && !pe.Ambiguous {
		pe.Ambiguous = true
	}
	return pe
}

func confidenceFor(pred, gold string, naive bool) uint8 {
	if pred == "" {
		return 0
	}
	if pred == gold {
		if naive {
			return 70
		}
		return 95
	}
	if naive {
		return 30
	}
	return 40
}

// RunExperiment replays GT, attributes site edges, and scores per mechanism.
func RunExperiment(mode Mode, records []groundtruth.Record, mechanisms []string) (Matrix, error) {
	eng, err := NewEngine(mode, 0)
	if err != nil {
		return nil, err
	}
	eng.ReplayGroundTruth(records)
	edges := groundtruth.NewTimeline(records).EdgesFromSites()

	gold := make([]Label, 0, len(edges))
	pred := make([]PredictedEdge, 0, len(edges))
	for _, se := range edges {
		gold = append(gold, Label{WakeeToken: se.Token})
		pred = append(pred, eng.Attribute(se, 0, "", parseElem(se.Aux)))
	}
	return NewEvaluator(mode).Score(gold, pred, mechanisms)
}

// ModeName returns a CLI-friendly experiment name.
func ModeName(m Mode) string {
	switch m {
	case E1LineageOnly:
		return "e1-lineage"
	case E2SudogElem:
		return "e2-sudog"
	case E3ResourceSuppress:
		return "e3-suppress"
	case E4NaiveForward:
		return "e4-naive"
	default:
		return fmt.Sprintf("mode-%d", m)
	}
}

// ParseMode parses experiment mode from a flag value.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "e1", "e1-lineage", "lineage":
		return E1LineageOnly, nil
	case "e2", "e2-sudog", "sudog":
		return E2SudogElem, nil
	case "e3", "e3-suppress", "suppress":
		return E3ResourceSuppress, nil
	case "e4", "e4-naive", "naive":
		return E4NaiveForward, nil
	default:
		return 0, fmt.Errorf("unknown mode %q", s)
	}
}
