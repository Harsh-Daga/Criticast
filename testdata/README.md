# Test workloads

Regression workloads for overhead and attribution benchmarks.

| Path | Purpose | Binary |
|------|---------|--------|
| `p0a/httpgo` | HTTP service with channel fan-out to backends | `bin/httpgo` |
| `p0b/server` | Adversarial topology + `CRITICAST_GT` ground truth + optional OTel | `bin/p0b-server` |

Build from repo root (uses `go.work`):

```bash
make workloads
```

Do not `go build ./testdata/...` without the workspace — use `make workloads` or build inside each module directory.
