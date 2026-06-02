# Documentation

Start here, then drill into the charter for design depth.

| Document | Audience | Contents |
|----------|----------|----------|
| [PHASES.md](PHASES.md) | Everyone | Phase map, ship policy, active branch |
| [GETTING_STARTED.md](GETTING_STARTED.md) | Operators, contributors | Build, run, benchmarks, troubleshooting |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Everyone | Layers, data flow, invariants |
| [ROADMAP.md](ROADMAP.md) | Maintainers | Current capabilities, validated learnings, next work |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contributors | PRs, licenses, regression expectations |
| [CI.md](CI.md) | Contributors | GitHub Actions jobs, triggers, local parity |
| [P1_COMPLETION.md](P1_COMPLETION.md) | Maintainers | P1 plumbing vs thesis (Bar A / Bar B) |
| [p2-bar-b-epoch-path.md](p2-bar-b-epoch-path.md) | Analyzer contributors | Request epoch model (Bar B literal) |
| [p2-bar-b-falsification.md](p2-bar-b-falsification.md) | Validation | Targeted A-only worker slowdown test |
| [../results/p2-validation.md](../results/p2-validation.md) | Reviewers | P2 Linux validation (Bar B pass, prod0 2026-06-02) |
| [../CHARTER.md](../CHARTER.md) | Designers | Full technical specification |
| [../AGENTS.md](../AGENTS.md) | Contributors + LLMs | Day-to-day engineering rules |
| [../results/README.md](../results/README.md) | Reviewers | Overhead and attribution benchmark reports |

## Cursor rules

Auto-loaded when editing matching paths:

- `.cursor/rules/criticast-core.mdc` — project-wide
- `.cursor/rules/bpf-collector.mdc` — `bpf/**`
- `.cursor/rules/go-userspace.mdc` — `**/*.go`
- `.cursor/rules/attribution-analyzer.mdc` — attribution + analyzer

## Charter index

| Topic | Charter section |
|-------|-----------------|
| Scheduler model, `prev_state` | Part A |
| BPF maps, `struct event` | Part B |
| Lineage-first attribution, tiers | Part C |
| Agent threading, symbolization | Part D |
| Analyzer, critical path | Part E |
| Layer diagram | Part F |
| Tech stack | Part G |
| Validation methodology | Part H |
| Product roadmap | Part I |
| CLI, deployment | Part J |
| Edge cases | Part K |
| Risks | Part L |
| bpftrace spike, `offsets.json` | Appendix Q |
| Threat model, CI | Appendix R |
