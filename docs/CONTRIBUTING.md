# Contributing

Thanks for helping improve criticast. Read the charter for *why*; this page covers *how*.

## Release policy

**No public release until the charter-complete milestone** (minimum P0–P3 per [PHASES.md](PHASES.md)). P1 is internal plumbing only; P2 closes the product thesis (Bar B) but still does not GA.

## Before you open a PR

1. [CHARTER.md](../CHARTER.md) — scope for your change
2. [AGENTS.md](../AGENTS.md) — engineering rules and checklist
3. [PHASES.md](PHASES.md) — which phase/branch your work belongs on
4. `./scripts/verify.sh` — must pass on Linux with BTF for BPF changes
5. [ROADMAP.md](ROADMAP.md) — confirm the change fits current milestones
6. P2 slices: `./scripts/validate-bar-b.sh` when touching attribution/BPF analyze path

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

## CI

Runs on **every pull request** and on **pushes to `main`** (no per-feature-branch allowlist).

| Job | What it checks |
|-----|----------------|
| **Go** | `go test`, `go vet`, build `criticast` + workloads |
| **Lint** | `golangci-lint` (see `.golangci.yml`) |
| **Vulnerability scan** | `govulncheck` |
| **Linux BPF** | bpftrace/clang/bpftool, `make test-bpf`, env preflight |

Local equivalents:

```bash
./scripts/ci-go.sh          # same as the Go job
./scripts/ci-linux-bpf.sh   # Linux + sudo; same as BPF job
./scripts/verify.sh         # ci-go + ci-linux-bpf on Linux
make lint                   # requires golangci-lint installed
```

Future: kernel version matrix (5.15–6.12) for BPF compile-only jobs.
