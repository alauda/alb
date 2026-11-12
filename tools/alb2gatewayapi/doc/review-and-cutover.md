# Review and cutover

## Before applying

- Confirm the target GatewayClass, Gateway namespace, and Envoy Gateway version.
- Check every `[注意]` item in `conversion-report.md`.
- Confirm backend Services and ports, including cross-namespace references.
- Confirm certificate Secrets and `ReferenceGrant` coverage.
- Confirm `allowedRoutes` scope before production.
- Review redirect, CORS, vhost, timeout, L4 affinity, and backend TLS behavior.
- Check that no placeholder values remain in the generated YAML.

## Validation

```bash
python3 scripts/validate_gatewayapi.py output/<case>
```

If the target cluster is available and the user explicitly asks, run a
server-side dry-run in a test namespace with the required CRDs installed.
Do not apply to production as part of the conversion step.

## Cutover

Use a parallel run where possible:

1. Keep ALB serving.
2. Apply the generated Gateway and Routes to a test target.
3. Verify listener, route, backend, certificate, redirect, CORS, and L4 paths.
4. Lower DNS TTL and move traffic in small batches.
5. Keep ALB and the old DNS records available for rollback.

## Rollback

Move DNS or the upstream load balancer back to ALB. Do not delete the ALB
during the observation window. If generated resources must be removed, delete
Routes and policies before the Gateway and EnvoyProxy; preserve namespaces that
may contain other workloads.
