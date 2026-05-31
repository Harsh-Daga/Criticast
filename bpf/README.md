# Kernel BPF (L2)

**License: GPL-2.0-only** (see file headers in this directory).

Userspace in the repository root is **Apache-2.0**. Do not merge GPL and Apache sources in the same file.

Design: [CHARTER.md](../CHARTER.md) Part B.

## Build

```bash
make bpf          # one clang -c; collector.c #includes go_probe.c
make test-bpf     # compile + llvm-objdump symbol check
```

Multiple BPF `.o` files are **not** merged with `ld.lld` (Debian `ld.lld` often lacks BPF emulation).

Sched probes use `SEC("tp_btf/...")`; userspace attaches with `link.AttachTracing` (not `AttachRawTracepoint`).
