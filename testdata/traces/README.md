# Golden traces

| File | Purpose |
|------|---------|
| `golden_chain.jsonl` | v1 trace for analyzer/CLI regression (wait-for chain, `WC_NET`) |
| `bar_b_scoped.jsonl` | v2 scoped Bar B fixture (`WC_CHAN`, `aux`, cookie `0x1234`) |

Events use Go `json` field names (`Type`, `TsNs`, …) matching `internal/event.Event`.

Committed so CI and offline `analyze` tests do not depend on live `record`.
