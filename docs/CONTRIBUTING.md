# Contributing

Thanks for helping improve criticast. Read the charter for *why*; this page covers *how*.

## Before you open a PR

1. [CHARTER.md](../CHARTER.md) — scope for your change
2. [AGENTS.md](../AGENTS.md) — engineering rules and checklist
3. `./scripts/verify.sh` — must pass on Linux with BTF for BPF changes
4. [ROADMAP.md](ROADMAP.md) — confirm the change fits current milestones

## Licenses

| Path | License |
|------|---------|
| `bpf/**`, `*.bpf.c`, embedded BPF objects | **GPLv2** |
| Everything else (Go, docs, scripts) | **Apache-2.0** |

Do not mix license headers. Do not copy GPL BPF into Apache files.

## Commits

- Focused commits; message explains **why**
- No secrets or large binaries without justification
- Breaking `struct event` → bump trace version + note in commit body

## Tests

```bash
go test ./...
go vet ./...
make test-bpf    # Linux + BTF
```

| Area | Expectation |
|------|-------------|
| Attribution | Table-driven tests; adversarial fixture must not regress spawn/pool/mutex |
| Analyzer | Graph and path tests; cycles must terminate |
| BPF | Object compiles; `test-bpf-object.sh` checks symbols |
| Go offsets | `bpf/offsets.json` — no hardcoded goid offsets in source |

Spikes belong under `scripts/` or `testdata/`; label PRs `[spike]` when experimental.

## Benchmarks

When changing the hot path or attribution:

- Overhead: `scripts/bench-p0a.sh` — update [results/phase0/p0a-overhead.md](../results/phase0/p0a-overhead.md)
- Attribution: full `eval --mode all` — update [results/phase0/p0b-attribution.md](../results/phase0/p0b-attribution.md)

Include host kernel, Go version, commit, and load parameters in the report.

## CI (when wired)

- Kernel matrix: 5.8, 5.15, 6.1, 6.8, 6.12 — compile BPF
- Go 1.21+; regenerate `offsets.json` when runtime struct layout changes
- Attribution thresholds enforced in CI once baselines are frozen
