#!/usr/bin/env bash
# Full bootstrap fetch for ALB-to-Gateway-API migration.
#
# This is the generic, cluster-wide fetcher. It reads all five required object
# types, not a Rule-derived gap. For a partial export, use the generated
# fetch-missing.sh inside the output case instead.
#
# Usage:
#   scripts/fetch.sh [--out DIR] [--dry-run]
#
# Output defaults to ./alb-export-crs and ./alb-export-crs.tgz.

set -euo pipefail

usage() {
  cat <<'USAGE'
Usage: fetch.sh [--out DIR] [--dry-run]

Options:
  --out DIR   Output directory (default: ./alb-export-crs).
  --dry-run   Print kubectl commands without executing them.
  -h, --help  Show this help.

Environment:
  KUBECTL     kubectl binary (default: kubectl)
USAGE
}

OUT="alb-export-crs"
DRY_RUN=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) OUT="${2:?--out requires a directory}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

KUBECTL="${KUBECTL:-kubectl}"
if [[ "$DRY_RUN" -eq 0 ]]; then
  mkdir -p "$OUT"
fi
fetch() {
  local resource="$1" file="$2"
  if [[ "$DRY_RUN" -eq 1 ]]; then
    printf '"$KUBECTL" get %s -A -o yaml > "%s"\n' "$resource" "$OUT/$file"
  else
    "$KUBECTL" get "$resource" -A -o yaml > "$OUT/$file"
  fi
}

fetch alaudaloadbalancer2 alb2.yaml
fetch frontends.crd.alauda.io frontends.yaml
fetch rules.crd.alauda.io rules.yaml
fetch namespace namespaces.yaml
fetch ingress ingresses.yaml

if [[ "$DRY_RUN" -eq 0 ]]; then
  tar czf "$OUT.tgz" -C "$OUT" .
  echo "fetched full bootstrap export to $OUT"
else
  echo "would fetch full bootstrap export to $OUT"
fi
