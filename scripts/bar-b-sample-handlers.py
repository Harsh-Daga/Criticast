#!/usr/bin/env python3
"""Sample handler entry→exit epochs from a p0b GT log for Bar B batch validation."""
from __future__ import annotations

import argparse
import json
import random
import sys
from datetime import datetime, timedelta, timezone


def parse_ts(s: str) -> datetime:
    return datetime.fromisoformat(s.replace("Z", "+00:00"))


def trace_wall_bounds(trace_path: str) -> tuple[datetime, datetime] | None:
    """Return (wall_base, wall_end) covered by trace events (RFC3339 anchor)."""
    wall_base: datetime | None = None
    k_base = 0
    k_max = 0
    with open(trace_path, errors="replace") as f:
        for line in f:
            line = line.strip()
            if not line.startswith("{"):
                continue
            try:
                obj = json.loads(line)
            except json.JSONDecodeError:
                continue
            if obj.get("magic") == "CRTC" and obj.get("wall_base_utc"):
                wall_base = parse_ts(obj["wall_base_utc"])
                k_base = int(obj.get("ktime_base_ns") or 0)
                k_max = k_base
                continue
            ts_ns = obj.get("TsNs")
            if ts_ns is not None:
                k_max = max(k_max, int(ts_ns))
    if wall_base is None:
        return None
    span_ns = max(0, k_max - k_base)
    return wall_base, wall_base + timedelta(seconds=span_ns / 1e9)


def load_epochs(
    gt_path: str,
    min_wall_ns: int,
    max_membership_hint: int,
    trace_from: datetime | None = None,
    trace_to: datetime | None = None,
) -> list[dict]:
    open_pairs: dict[tuple[str, int], datetime] = {}
    epochs: list[dict] = []
    with open(gt_path, errors="replace") as f:
        for line in f:
            if "CRITICAST_GT " not in line:
                continue
            payload = line.split("CRITICAST_GT ", 1)[1].strip()
            try:
                rec = json.loads(payload)
            except json.JSONDecodeError:
                continue
            token = rec.get("token", "")
            site = rec.get("site", "")
            goid = int(rec.get("goid", 0))
            if not token or goid == 0:
                continue
            ts = parse_ts(rec["ts"])
            key = (token, goid)
            if site == "handler-entry":
                open_pairs[key] = ts
            elif site == "handler-exit" and key in open_pairs:
                t0 = open_pairs.pop(key)
                wall_ns = int((ts - t0).total_seconds() * 1e9)
                if wall_ns < min_wall_ns:
                    continue
                if trace_from is not None and trace_to is not None:
                    if t0 < trace_from or ts > trace_to:
                        continue
                epochs.append(
                    {
                        "token": token,
                        "handler_goid": goid,
                        "wall_ns": wall_ns,
                        "scope_from": t0.astimezone(timezone.utc)
                        .isoformat()
                        .replace("+00:00", "Z"),
                        "scope_to": ts.astimezone(timezone.utc)
                        .isoformat()
                        .replace("+00:00", "Z"),
                    }
                )
    # Drop ultra-short or duplicate-heavy windows is left to analyze (membership cap).
    _ = max_membership_hint
    return epochs


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("gt_log")
    ap.add_argument(
        "--trace",
        default="",
        help="optional .criticast trace: only sample handler epochs inside its wall span",
    )
    ap.add_argument("-n", "--count", type=int, default=50)
    ap.add_argument("--seed", type=int, default=42)
    ap.add_argument("--min-wall-ns", type=int, default=1_000_000)
    ap.add_argument(
        "--format",
        choices=("tsv", "json"),
        default="tsv",
        help="tsv: token\\tgoid\\tfrom\\tto\\twall_ns",
    )
    args = ap.parse_args()
    trace_from = trace_to = None
    if args.trace:
        bounds = trace_wall_bounds(args.trace)
        if bounds is None:
            print("bar-b-sample-handlers: could not read trace wall bounds", file=sys.stderr)
            return 2
        trace_from, trace_to = bounds
    epochs = load_epochs(args.gt_log, args.min_wall_ns, 16, trace_from, trace_to)
    if not epochs:
        print("bar-b-sample-handlers: no handler epochs in GT log", file=sys.stderr)
        return 2
    rng = random.Random(args.seed)
    if len(epochs) > args.count:
        epochs = rng.sample(epochs, args.count)
    epochs.sort(key=lambda e: (e["token"], e["handler_goid"], e["scope_from"]))
    if args.format == "json":
        json.dump(epochs, sys.stdout, indent=2)
        sys.stdout.write("\n")
    else:
        for e in epochs:
            print(
                f"{e['token']}\t{e['handler_goid']}\t{e['scope_from']}\t{e['scope_to']}\t{e['wall_ns']}",
            )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
