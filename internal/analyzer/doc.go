// Package analyzer builds wait-for graphs and per-request critical paths (charter Part E).
//
// Analysis modes:
//
//   - Unscoped (Tier-0): process-wide dominant waits via [Analyze] without token scope.
//   - Token scope: GT-calibrated subgraph via [FilterScopedToken] (sudog + cookie attribution).
//   - Request epoch (Bar B literal): pinned handler entry→exit via [buildRequestEpoch] and
//     [analyzeRequestEpoch]; see docs/p2-bar-b-epoch-path.md.
//
// Critical-path selection uses [LongestPathTemporal] and [handlerTemporalCriticalPath] to avoid
// summing parallel waits at the same ktime.
package analyzer
