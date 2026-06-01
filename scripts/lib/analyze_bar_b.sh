#!/usr/bin/env bash
# Shared criticast analyze flags for Bar B literal (request epoch).
# shellcheck shell=bash
# Usage: source scripts/lib/analyze_bar_b.sh
#   bar_b_analyze "$TRACE" "$GT" "$TOKEN" "$GOID" "$FROM" "$TO" "$PAD_MS" "$OUT_JSON"

bar_b_analyze() {
  local trace="$1" gt="$2" token="$3" goid="$4" from="$5" to="$6" pad_ms="$7" out="$8"
  ./bin/criticast analyze "$trace" \
    --request "token=${token}" \
    --gt-log "$gt" \
    --scope-from "$from" \
    --scope-to "$to" \
    --scope-pad "${pad_ms}ms" \
    --scope-handler-goid "$goid" \
    --format json >"$out"
}

bar_b_pad_ms() {
  local wall_ns="$1"
  local pad=$(( wall_ns / 4000000 ))
  if [[ "$pad" -lt 4 ]]; then pad=4; fi
  if [[ "$pad" -gt 25 ]]; then pad=25; fi
  echo "$pad"
}
