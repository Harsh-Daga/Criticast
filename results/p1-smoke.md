# Phase 1 smoke validation — production Linux host

Recorded on **2026-06-01** after `phase1/shippable-core` implementation.

**This report validates plumbing (Bar A) only** — not request-scoped causal critical path (Bar B). See [docs/P1_COMPLETION.md](../docs/P1_COMPLETION.md).

## Environment

| Item | Value |
|------|--------|
| Host | `prod0-telephony-voipmonitor-primary` (Debian, cloud) |
| Kernel | `6.1.0-40-cloud-amd64` |
| Go | `1.24.0` (`/usr/local/go`) |
| BTF | `/sys/kernel/btf/vmlinux` (~4.1 MB) |
| bpftool | `/usr/sbin/bpftool` (distro; kernel-matched package N/A) |

## CI parity (local)

| Gate | Command | Result |
|------|---------|--------|
| Preflight | `./scripts/check-linux-env.sh` | OK |
| Full verify | `./scripts/verify.sh` | OK (`ci-go` + `ci-linux-bpf`) |
| Lint | `./scripts/ci-lint.sh` | OK |
| BPF gate | `SKIP_APT=1 sudo ./scripts/ci-linux-bpf.sh` | OK (apt repos broken on host; tools preinstalled) |

Note: use `./scripts/ci-lint.sh` (not bare `golangci-lint`) so the linter is built with `GOTOOLCHAIN=go1.24.0`.

## Offline P1 (no live BPF)

```text
./bin/criticast analyze testdata/traces/golden_chain.jsonl --top 10
./bin/criticast export testdata/traces/golden_chain.jsonl --pprof /tmp/golden.pb.gz
go tool pprof -top /tmp/golden.pb.gz
```

Result: critical path + valid pprof (`critical_wait` samples).

## Live end-to-end (`demo-p1.sh`)

Workload: `bin/httpgo` on `:8080`, `wrk -t2 -c20` during `record`.

### Run A — sched + Go uprobes (default)

```text
userspace: consumed=122114 received=122114 chan_drops=0 read_errors=0 malformed=0
bpf stats: ringbuf_drops=0 emitted=122114 blocks=60850 preempts=10911 runq=61264
           short_filt=436 sampled_out=0 stack_fail=18373 switch_seen=1150338 target_prev=72453
```

Analyze: `format=v2` `events=122114` `edges=42745` · path weight **150.71ms** · 0 ringbuf drops.

### Run B — sched only (`GO_UPROBES=0`)

```text
userspace: consumed=134445 received=134445 chan_drops=0
bpf stats: emitted=134445 blocks=66835 preempts=4652 runq=67610 stack_fail=13512
```

Analyze: path weight **6.79s** (single-edge critical path in that window).

### Scheduler sanity (`sched-smoke.sh`)

```text
sched_switch hits for comm httpgo: 43742  (5s window)
```

### Probe counters (`probe-stats` + wrk)

Confirms BPF programs run and target map matches TGID before ringbuf drain:

```text
bpf stats: emitted=78724 … switch_seen=447148 target_prev=40725
```

## Interpretation (Tier-0/1)

| Observation | Expected for P1 |
|-------------|-----------------|
| `WC_UNKNOWN` dominant | Yes — no gopark/syscall refinement in BPF yet |
| Confidence **60%** | Tier-0/1 default; not Tier-2 chan |
| `task_id=4294901xxx` | Cookie/goid encoding in trace |
| pprof frames **`unknown`** | P1 exports wait weights; ELF symbolization deferred |
| `stack_fail` > 0 | User-stack walk at `sched_waking` often fails under load; events still recorded |

## Not in this smoke

- Overhead re-benchmark on this host (see [phase0/p0a-overhead.md](phase0/p0a-overhead.md))
- Adversarial `eval --mode all` on this host (see [phase0/p0b-attribution.md](phase0/p0b-attribution.md))
- `govulncheck` clean on Go 1.24.0 stdlib (upgrade to **1.24.6+** recommended)

## Conclusion

**Plumbing validated:** record → analyze → pprof with zero ringbuf drops under wrk on kernel 6.1 cloud.

**Not shown here:** scoped request critical path ≈ wall time, labeled wait classes, or Tier-2 chan on live BPF. Dominant waits on this trace are largely idle/runtime churn (`WC_UNKNOWN`, `task_id=1` self-loops). That is expected for unscoped Tier-0/1 analyze — not a failure of the transport layer.
