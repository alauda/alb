#!/usr/bin/env bash
# Validate generated YAML against Gateway API + Envoy Gateway CRD schemas.
#
# Usage:
#   scripts/validate_schema.sh <output-case-dir>
#
# Optional. Requires kubeconform, python3 + PyYAML, and user-provided CRD dirs.
# Set GATEWAY_CRD_DIR and EG_CRD_DIR to the CRD YAML directories; without them
# the check is skipped rather than pretending to pass.
set -euo pipefail

CASE_DIR="${1:?usage: validate_schema.sh <output-case-dir> [--allow-skip]}"
ALLOW_SKIP=0
[[ "${2:-}" == "--allow-skip" ]] && ALLOW_SKIP=1
KUBECONFORM="${KUBECONFORM:-kubeconform}"
GATEWAY_CRD_DIR="${GATEWAY_CRD_DIR:-}"
EG_CRD_DIR="${EG_CRD_DIR:-}"

if [[ -z "$GATEWAY_CRD_DIR" || -z "$EG_CRD_DIR" ]]; then
  if [[ "$ALLOW_SKIP" -eq 1 ]]; then
    echo "schema gate skipped: set GATEWAY_CRD_DIR and EG_CRD_DIR to run it"
    exit 0
  fi
  echo "schema gate requires GATEWAY_CRD_DIR and EG_CRD_DIR; pass --allow-skip to skip explicitly" >&2
  exit 2
fi
command -v "$KUBECONFORM" >/dev/null || {
  if [[ "$ALLOW_SKIP" -eq 1 ]]; then
    echo "schema gate skipped: kubeconform not found"
    exit 0
  fi
  echo "schema gate requires kubeconform; pass --allow-skip to skip explicitly" >&2
  exit 2
}
[[ -d "$GATEWAY_CRD_DIR" ]] || { echo "Gateway CRD dir not found: $GATEWAY_CRD_DIR" >&2; exit 2; }
[[ -d "$EG_CRD_DIR" ]] || { echo "Envoy Gateway CRD dir not found: $EG_CRD_DIR" >&2; exit 2; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

python3 - "$GATEWAY_CRD_DIR" "$EG_CRD_DIR" "$TMP" <<'PY'
import glob
import json
import os
import sys

import yaml

gateway_crd_dir, eg_crd_dir, out_dir = sys.argv[1], sys.argv[2], sys.argv[3]
for filename in list(glob.glob(os.path.join(gateway_crd_dir, "*.yaml"))) + list(glob.glob(os.path.join(eg_crd_dir, "*.yaml"))):
    with open(filename, "r", encoding="utf-8") as fh:
        for doc in yaml.safe_load_all(fh):
            if not doc or doc.get("kind") != "CustomResourceDefinition":
                continue
            kind = (doc.get("spec", {}).get("names", {}) or {}).get("kind")
            if not kind:
                continue
            versions = (doc.get("spec", {}) or {}).get("versions") or []
            schema = None
            for version in versions:
                if version.get("storage"):
                    schema = (version.get("schema") or {}).get("openAPIV3Schema")
                    break
            if schema is None:
                for version in versions:
                    schema = (version.get("schema") or {}).get("openAPIV3Schema")
                    if schema:
                        break
            if not schema:
                continue
            payload = json.dumps(schema)
            with open(os.path.join(out_dir, f"{kind}.json"), "w", encoding="utf-8") as fh:
                fh.write(payload)
            with open(os.path.join(out_dir, f"{kind.lower()}.json"), "w", encoding="utf-8") as fh:
                fh.write(payload)
PY

set +e
summary=$("$KUBECONFORM" \
  -strict \
  -schema-location "default" \
  -schema-location "$TMP/{{.ResourceKind}}.json" \
  -summary \
  "$CASE_DIR" 2>&1)
rc=$?
set -e
printf '%s\n' "$summary"
if [[ "$rc" -ne 0 ]]; then
  exit "$rc"
fi
grep -q 'Skipped: 0' <<<"$summary" || {
  echo "schema validation skipped resources; refusing to pass" >&2
  exit 1
}
printf 'CRD dirs: %s, %s\n' "$GATEWAY_CRD_DIR" "$EG_CRD_DIR"
