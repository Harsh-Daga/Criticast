# Documentation

Start here, then drill into the charter for design depth.

| Document | Audience | Contents |
|----------|----------|----------|
| [GETTING_STARTED.md](GETTING_STARTED.md) | Operators, contributors | Build, run, benchmarks, troubleshooting |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Everyone | Layers, data flow, invariants |
| [ROADMAP.md](ROADMAP.md) | Maintainers | Current capabilities, validated learnings, next work |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contributors | PRs, licenses, regression expectations |
| [P1_IMPLEMENTATION_PROMPT.md](P1_IMPLEMENTATION_PROMPT.md) | Implementers | Phase 1 full execution brief |
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
