# ALB2 / Frontend / Rule to Gateway API

Offline conversion and review helper for exported ALB2, Frontend, Rule,
related Namespace, and source Ingress manifests. It is intended for users who are moving ALB traffic to Gateway API
and want a reviewable first cut, not a claim of lossless migration or an
automatic production cutover.

The generated manifests target Envoy Gateway. The default `GatewayClass` is
`envoy-gateway-operator-cpaas-default`; pass `--gateway-namespace` for the
namespace where your `Gateway` and `EnvoyProxy` should live.

## What this does

- inventories ALB2, Frontend, Rule, related Namespace, and source Ingress objects;
- audits supported, approximate, and manual-review features;
- generates `Gateway`, `EnvoyProxy`, Routes, filters, and policies;
- writes a conversion report and a feature audit;
- validates the generated YAML locally.

It does not contact a cluster by default, create resources, delete resources,
or cut traffic. `check_backends.py --check` is the only optional cluster check
and uses the current kubeconfig only when explicitly requested.

## Use with an AI assistant

Point the assistant at this directory and ask it to read:

1. `SKILL.md`
2. `doc/README.md`
3. `doc/migrate-alb-to-gatewayapi.md`

The assistant should guide you through exporting manifests, running the
scripts, and reviewing the report. It should stop at review, not deploy.

## Offline quick start

Requirements: Python 3.10+ and PyYAML.

```bash
python3 -m pip install pyyaml

# input is a directory of ALB2 / Frontend / Rule YAML
python3 scripts/alb_inventory.py input/

# Choose the output case and Gateway namespace. If --gateway-namespace is
# omitted, the generator may use the backend Service namespace when there is
# exactly one.
CASE_DIR=output/my-migration
python3 scripts/generate_gatewayapi.py input/ "$CASE_DIR" \
  --gateway-namespace gateway-system


# The generator writes conversion-report.md, feature-audit.md and warnings.md.
python3 scripts/validate_gatewayapi.py "$CASE_DIR"
```

If you have no input at all, bootstrap a full export with:

```bash
./scripts/fetch.sh --out alb-export-crs
```

This full bootstrap over-collects all namespaces and Ingresses; use it only
when no targeted input is available, and handle the result as private data.

If `missing-resources.md` is not empty, run the generated read-only fetcher:

```bash
"$CASE_DIR/fetch-missing.sh"
# add the fetched YAML files to the input case, then regenerate
```

The generated case directory contains:

```text
output/<case>/<alb>/gateway.yaml
output/<case>/<alb>/routes/<listener>/...
output/<case>/<alb>/routes/multi-listener/...
output/<case>/<alb>/shared.yaml
output/<case>/conversion-report.md
output/<case>/feature-audit.md
output/<case>/warnings.md
output/<case>/objects.txt
output/<case>/missing-resources.md
```

The tool is offline. It does not generate apply or prune scripts. Use normal
`kubectl apply -R -f <case-dir>` only when you have reviewed the output and
decided to apply it; delete or traffic cutover is a separate user action.

## Defaults and scope

- System ALBs `cpaas-system` and `global-alb2` are skipped by default.
- The default GatewayClass is `envoy-gateway-operator-cpaas-default`.
- The source behavior is ALB 4.3; target Gateway API/Envoy Gateway behavior
  should still be checked against the target cluster.
- Envoy Gateway extensions such as `EnvoyProxy`, `HTTPRouteFilter`,
  `Backend`, `BackendTrafficPolicy`, and `ClientTrafficPolicy` are generated
  where needed. Other Gateway API implementations are not guaranteed.
- `proxy_header.use_forwarded_headers` is reported for manual review.
- Unknown backends, certificates, hosts, unsupported matches, and
  implementation-specific behavior must be reviewed explicitly.

## Known semantic differences

- ALB `STARTS_WITH` is a string prefix; Gateway API `PathPrefix` is
  path-segment based. ALB regex is a search; Envoy regex is a full match.
- ALB per-operation timeouts become one total `HTTPRoute.timeouts` budget
  using the maximum value.
- Redirect fixed queries are dropped, `$host` is omitted, `prefix_match` is
  skipped when it cannot satisfy the Gateway API CEL rule, and Frontend-level
  redirects / `ingressSSLRedirect` are not generated automatically.
- Envoy answers CORS preflight itself; literal `*`, `*.domain`, port wildcards,
  and `$http_origin` are not widened into Gateway CORS filters.
- COOKIE/SRC_IP matches, unsupported HEADER/PARAM operators, and matches beyond
  the per-rule cap are not converted; see `warnings.md`.
- sip-hash SourceIP hashing needs `externalTrafficPolicy: Local` and does not
  use XFF.
- HTTPS backends use Envoy Gateway `Backend` + `insecureSkipVerify`, which
  requires `enableBackend`.
- Multi-owner ALBs are merged and only warned about; confirm the cluster before
  applying.
- The local validator and schema gate are not a substitute for
  `kubectl apply --dry-run=server` and functional traffic tests.

## Regression tests

```bash
python3 tests/test_high_fixes.py
python3 tests/test_feature_matrix.py
```

## Optional schema validation

`scripts/validate_schema.sh` is optional and requires `kubeconform`, the
Gateway API CRDs, and the Envoy Gateway CRDs:

```bash
KUBECONFORM=kubeconform \
GATEWAY_CRD_DIR=<gateway-api-crds> \
EG_CRD_DIR=<envoy-gateway-crds> \
scripts/validate_schema.sh output/<case>
```

Without the CRD directories the gate fails by default; pass `--allow-skip` only
when you explicitly accept that schema validation is not run.

## Privacy

Exports may contain customer names, domains, namespaces, Service names, and
certificate references. Keep `input/` and `output/` private. Do not publish
raw exports, generated reports, kubeconfigs, Secret data, tokens, or private
keys. `input/` and `output/` are excluded by `.gitignore`. When publishing this directory, exclude both directories entirely; do not copy their contents.

## Documentation

Start at `doc/README.md`. The main guide is
`doc/migrate-alb-to-gatewayapi.md`; it is self-contained and does not require
external product-document links.
