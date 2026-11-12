---
name: alb2gatewayapi
description: Offline conversion and review of exported ALB2, Frontend, and Rule manifests to Gateway API manifests and a Markdown report. Use when a user requests ALB-to-Gateway-API migration or provides ALB exports.
---

# ALB to Gateway API

Work offline unless the user explicitly asks for a cluster check. Read
`README.md` and `doc/README.md` first.

## Required inputs

Ask for:

- exported `ALB2`, `Frontend`, and `Rule` YAML, including `kind: List` forms;
- original `Ingress` YAML for the sources referenced by `Rule.spec.source`;
- `Namespace` YAML for namespaces referenced by the migration;
- the target Gateway API implementation and version;
- the Gateway namespace, if the backend Services span multiple namespaces.

Do not request kubeconfig, Secret data, tokens, private keys, or real
certificate material. Do not invent missing hosts, Services, certificates, or
GatewayClass names.

## If input is missing or incomplete

- **No input at all**: ask for a targeted export of the five required kinds:
  ALB2, Frontend, Rule, related Namespace, and source Ingress. If the user can
  access the source cluster, use commands like:

  ```bash
  scripts/fetch.sh --out alb-export-crs
  # Full bootstrap over-collects all namespaces/Ingress; use it only when no
  # targeted input is available. Prefer the generated fetch-missing.sh after a
  # partial export.
  # or targeted commands:
  kubectl get alaudaloadbalancer2 -A -o yaml > alb2.yaml
  kubectl get frontends.crd.alauda.io -A -o yaml > frontends.yaml
  kubectl get rules.crd.alauda.io -A -o yaml > rules.yaml
  ```

  Once Rules are available, derive the related Namespace and source Ingress
  list; do not ask for all namespaces, all Ingresses, all Services, or Secret
  contents. If the user cannot provide exports, stop at the input gap; do not
  invent resources.

- **Partial input**: run the inventory/generator to create a first-pass output,
  then read `missing-resources.md`. If the user has cluster access, run the
  generated read-only `fetch-missing.sh` (or ask the user to run it), add the
  fetched YAML to the input, and regenerate. If they do not have cluster
  access, give them the exact `missing-resources.md` list and the commands from
  `fetch-missing.sh`.

- The fetch script only reads objects with targeted `kubectl get` commands. It
  does not apply, delete, or modify anything.

## Workflow

1. Preflight the input: if it is missing or incomplete, follow the section
   above before generating. Then run `scripts/alb_inventory.py <input-dir>`.
2. Generate into a user-chosen local output directory:

   ```bash
   python3 scripts/generate_gatewayapi.py <input-dir> <output-dir> \
     --gateway-namespace <namespace>
   ```

3. If `<output-dir>/missing-resources.md` lists gaps, run
   `<output-dir>/fetch-missing.sh` (read-only; writes to
   `<output-dir>/alb-export-crs/`), add the fetched YAML to the input, and
   regenerate.
4. Run `scripts/validate_gatewayapi.py <output-dir>`. If the user has
   `kubeconform` and matching Gateway API/Envoy Gateway CRD directories, also
   run `scripts/validate_schema.sh <output-dir>`; otherwise keep the schema
   gate visibly skipped, not silently passed.
5. Read `conversion-report.md`, `warnings.md` and the generated `feature-audit.md`; explain
   every item that needs review. Stop at review. Do not apply or delete
   resources unless the user explicitly requests a separate cluster action.

The generator writes `conversion-report.md`, `warnings.md` and `feature-audit.md`
into the case directory.

## Mapping guidance

Read `doc/mappings.md` and `doc/migrate-alb-to-gatewayapi.md` before explaining
mappings. Keep these points visible:

- generated manifests target Envoy Gateway;
- default system ALBs are `cpaas-system` and `global-alb2`;
- default GatewayClass is `envoy-gateway-operator-cpaas-default`;
- `proxy_header.use_forwarded_headers` is manual review;
- CORS, redirects, vhost, L4 affinity, backend TLS, and cross-namespace
  references have explicit limitations in the report;
- The tool does not generate apply or prune scripts; applying is a separate,
  explicit user action.

## Privacy

Treat exports and generated output as private. Do not upload them, commit them,
or place them in public issues. Do not include real customer names, domains,
IPs, namespaces, or certificate references in examples.
