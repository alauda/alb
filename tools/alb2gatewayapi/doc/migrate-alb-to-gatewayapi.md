# Migrate ALB-served Ingress and ALB Rules to Gateway API

> This guide is self-contained. It targets ALB 4.3 and Envoy Gateway. External
> product-document links were removed; read the local `doc/` files for the
> migration contract.

## Overview

ALB is deprecated. Envoy Gateway (Gateway API) and Ingress NGINX are the recommended ingress controllers. New ALB instances cannot be created, and ALB is no longer available in the Web Console. Existing ALB instances in a cluster upgraded from an earlier version keep running so that their traffic can be migrated. This guide describes how to migrate L7 HTTP/HTTPS traffic from ALB to Gateway API managed by Envoy Gateway.

This guide targets ALB 4.3 as the source. Confirm the target ACP/Gateway API/Envoy Gateway versions on the target cluster before applying anything.

ALB receives L7 traffic in two forms. Identify which form a workload uses before you start:

| Source form | How it is created | What to migrate |
| --- | --- | --- |
| Ingress served by ALB | Each ALB creates an `IngressClass` with the same name and translates matching Ingress resources into generated `Frontend` and `Rule` resources. Generated rules carry `spec.source.type: ingress` and labels such as `alb2.cpaas.io/source-type: ingress`. | Migrate the original Ingress to `HTTPRoute`. Do not convert the generated `Rule` resources one by one. |
| ALB Rule created directly | A `Rule` created through the API or the ALB console without `spec.source.type: ingress`. It can be an L7 rule or an L4 TCP/UDP rule. | Convert an L7 `Rule` to `HTTPRoute`. Convert an L4 `Rule` to `TCPRoute` or `UDPRoute` separately. |

Gateway API is a new configuration model. It does not provide runtime compatibility for ALB rules, Ingress annotations, or ALB `dslx` expressions. Migration means creating equivalent `Gateway`, `HTTPRoute`, and policy resources, validating them, and then moving traffic.

> **Warning**
ALB is deprecated. Use Ingress NGINX Operator only when an application requires Ingress-specific behavior that Gateway API cannot represent. For new L7 traffic, use Envoy Gateway and Gateway API.

This guide covers L7 HTTP/HTTPS traffic. For ALB TCP/UDP frontends and rules, see Migrate L4 traffic and evaluate `TCPRoute`/`UDPRoute` support in the installed Envoy Gateway version.

## Migration model

| ALB object or field | Gateway API resource | Migration notes |
| --- | --- | --- |
| `ALB2` instance | `GatewayClass` and `Gateway` | One ALB usually maps to one Gateway. |
| `Frontend` (port and protocol) | `Gateway.spec.listeners[]` or `ListenerSet.spec.listeners[]` | Use one listener per exposed port. HTTPS listeners carry the certificate. |
| `Frontend` or `Rule` certificate | Listener `tls.certificateRefs` | Convert `.spec.certificate_name` (`namespace/secret`) to a Secret reference. |
| `Rule` generated from Ingress | `HTTPRoute` converted from the original Ingress | The generated Rule is a projection of the Ingress. Do not migrate it directly. |
| `Rule` created directly | `HTTPRoute` rule, or `TCPRoute`/`UDPRoute` for L4 | Convert `dslx`, backends, and actions. |
| `dslx` `HOST` | `HTTPRoute.spec.hostnames[]` | `EQ` maps to an exact hostname. Wildcard and suffix matches need care. |
| `dslx` `URL` | `HTTPRoute.spec.rules[].matches[].path` | `STARTS_WITH` maps to `PathPrefix`, `EQ` to `Exact`, `REGEX` to `RegularExpression`. ALB regex is a search/prefix match; Envoy `RegularExpression` must match the whole path, so append `.*` unless the pattern ends with `$`. |
| `vhost` | `URLRewrite` filter with `hostname` | Overrides the Host header sent to the backend. |
| `serviceGroup.services` | `backendRefs[]` | Preserve Service name, namespace, port, and weight. |
| `priority` | No direct field | HTTPRoute precedence is based on match specificity, not an ALB priority number. Re-check conflicting routes. |
| `.spec.config.timeout` | `HTTPRoute.spec.rules[].timeouts` or `BackendTrafficPolicy` | Connection and response timeouts have different equivalents. |
| `.spec.config.redirect` | `RequestRedirect` filter | Do not keep a placeholder backend. |
| `enableCORS`, `corsAllowOrigin`, `corsAllowHeaders` | `SecurityPolicy.spec.cors` or the `CORS` route filter | Check wildcard and multi-origin behavior. |
| `rewrite_base` and `rewrite_target` | `URLRewrite` filter, or `HTTPRouteFilter` with `ReplaceRegexMatch` | Regex capture substitutions need the Envoy Gateway extension. |
| `alb.ingress.cpaas.io/rewrite-request` or `alb.rule.cpaas.io/rewrite-request` | `RequestHeaderModifier` for static values; `EnvoyExtensionPolicy` with Lua for `headers_var` | `headers_var` values are nginx variables; use a Lua policy for dynamic values. |
| `alb.ingress.cpaas.io/rewrite-response` or `alb.rule.cpaas.io/rewrite-response` | `ResponseHeaderModifier` or `ClientTrafficPolicy` | Use `set` to match the ALB `ngx.header` semantics. |
| `backendProtocol: https` | `Backend` + `tls.insecureSkipVerify` (tool default); `BackendTLSPolicy` if verification is required | Match ALB's default no-verify behavior; enable `extensionApis.enableBackend`. |
| `backendProtocol: grpc` | `GRPCRoute`, or a Service with `appProtocol: kubernetes.io/h2c` | Use `GRPCRoute` when the backend speaks gRPC. |
| `session_affinity_policy` | `HTTPRoute.spec.rules[].sessionPersistence` or `BackendTrafficPolicy` | Cookie affinity maps to `sessionPersistence`; header and `sip-hash` affinity map to `BackendTrafficPolicy` consistent hashing. |
| ALB VIP | Gateway data-plane Service or MetalLB address | Use a separate VIP for the parallel run, then switch DNS. |
| ALB project assignment or port-project isolation | `Gateway.spec.listeners[].allowedRoutes.namespaces` | Use `Same`, `Selector`, or `All` to control which projects can attach routes. |

## Step 1: Inventory the existing configuration

Export the ALB objects and only the source Ingress resources referenced by the
generated Rules:

```shell
kubectl get alaudaloadbalancer2 -A -o yaml > alb2-backup.yaml
kubectl get frontends.crd.alauda.io -A -o yaml > alb-frontends-backup.yaml
kubectl get rules.crd.alauda.io -A -o yaml > alb-rules-backup.yaml

# Only namespaces referenced by the migration, not all namespaces:
kubectl get namespace <related-ns> -o yaml > namespace-backup.yaml

# Do not export all Ingresses; export only Rule.spec.source targets:
kubectl get ingress -n <source-ns> <source-name> -o yaml > source-ingress.yaml
```

Do not export all namespaces, Services, or Secret contents. Frontend/Rule
already carry Service name/port and `certificate_name`; namespace labels are
needed only if you plan to tighten `allowedRoutes`.

Find generated rules and their source Ingress:

```shell
kubectl get rules.crd.alauda.io -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\t"}{.spec.source.type}{"\t"}{.spec.source.namespace}/{.spec.source.name}{"\n"}{end}'
```

For every source object, record:

- frontend port and protocol, and the certificate Secret;
- hostnames, path matches, and ALB rule priority;
- backend Service, port, weight, protocol (HTTP, HTTPS, or gRPC), and upstream Host override (`vhost`);
- rewrites, request and response header changes, redirects, CORS, and timeouts;
- session affinity and load-balancing method;
- Ingress annotations and Rule annotations.

The `Rule` spec is a useful starting point for the field mapping: `dslx` describes matches, `serviceGroup` describes backends, and `config`, `rewrite_base`, `rewrite_target`, `certificate_name`, and `backendProtocol` describe actions. The generated `dslx` also shows how ALB interpreted each Ingress path (`STARTS_WITH` or `REGEX`), which determines whether the HTTPRoute needs `PathPrefix` or `RegularExpression`.

## Step 2: Create or select a Gateway

Create a Gateway with one listener per ALB frontend port. ACP recommends one dedicated `EnvoyProxy` per Gateway; this is also what the Web Console creates by default. Reference that `EnvoyProxy` from `Gateway.spec.infrastructure.parametersRef`. The following example terminates TLS and exposes HTTP and HTTPS:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: public-gateway
  namespace: gateway-system
spec:
  provider:
    type: Kubernetes
    kubernetes:
      envoyService:
        type: LoadBalancer
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: public-gateway
  namespace: gateway-system
spec:
  gatewayClassName: envoy-gateway-operator-cpaas-default
  infrastructure:
    parametersRef:
      group: gateway.envoyproxy.io
      kind: EnvoyProxy
      name: public-gateway
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: app-example-tls
      allowedRoutes:
        namespaces:
          from: All
```

Replace `envoy-gateway-operator-cpaas-default` with the `GatewayClass` created by your `EnvoyGatewayCtl`.

A listener without `hostname` accepts every hostname on that port. Put hostnames in `HTTPRoute.spec.hostnames` unless you need a different certificate per hostname. If one HTTPS port serves several certificates, list all certificate Secrets in `tls.certificateRefs` and let Envoy Gateway select the certificate by SNI.

Use `allowedRoutes` deliberately. `from: Same` restricts routes to the Gateway namespace; `from: All` permits routes from every namespace; `from: Selector` permits only namespaces matching the selector. If a certificate Secret is in another namespace, create a `ReferenceGrant` in that namespace.

`allowedRoutes` is also the recommended way to preserve ALB project assignment and port-project isolation. Use one listener per project port and set the listener's `allowedRoutes` to the project namespace (`from: Same` when the Gateway is in that namespace), a project label selector (`from: Selector`), or `from: All` only for intentionally shared listeners.

Confirm that the Gateway has been programmed before creating routes:

```shell
kubectl get gateway public-gateway -n gateway-system -o yaml
```

The Gateway should report `Programmed=True` and its status should contain an address that clients can reach.

Most ALB instances expose fewer than 63 ports. If the source ALB exposes more than 63 ports, see Listener limits and large ALB migrations at the end of this guide.

## Step 3: Convert ALB-served Ingress to HTTPRoute

When the source is an Ingress served by ALB, migrate the Ingress. The generated `Rule` resources are recreated by the ALB controller and are not a stable migration input.

An Ingress with a host, prefix path, and Service backend can be represented as follows:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: web
  namespace: default
spec:
  parentRefs:
    - name: public-gateway
      namespace: gateway-system
      sectionName: https
  hostnames:
    - app.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: web
          port: 8080
```

The conversion rules are:

- map `spec.rules[].host` to `hostnames`;
- map `pathType: Prefix` to `PathPrefix` and `pathType: Exact` to `Exact`;
- for `pathType: ImplementationSpecific`, inspect the generated Rule `dslx`. ALB uses `STARTS_WITH` when the path has no regex metacharacters and `REGEX` otherwise. Map a plain path to `PathPrefix` and a regex path to `RegularExpression`;
- ALB regexes are search or prefix matches, while Envoy Gateway `RegularExpression` must match the whole path. Append `.*` or `(/.*)?` unless the original pattern ends with `$`;
- `PathPrefix` matches path elements: `/v1` matches `/v1` and `/v1/...` but not `/v1beta`, while ALB `STARTS_WITH /v1` also matches `/v1beta`;
- map the Ingress backend Service and port to `backendRefs`;
- replace `ingressClassName` with the appropriate `parentRefs` and listener `sectionName`;
- keep one source Ingress as one `HTTPRoute` where possible, with one rule for each path;
- split a route only when a policy or compatibility behavior needs a narrower attachment scope.

Keep Ingress annotations at the same scope after migration. An annotation on the Ingress applies to the whole Ingress; an HTTPRoute filter can apply to a single rule, a whole route, or the Gateway. Do not copy every Ingress annotation to every HTTPRoute rule.

Read Configure Gateway API Route for route attachment and match behavior.

## Step 4: Convert ALB Rules

For a Rule created directly, map each part of the `Rule` spec to an `HTTPRoute`.

### Match conditions

`dslx` entries of different types are combined with AND. Multiple values inside one entry are combined with OR.

| ALB `dslx` | HTTPRoute equivalent | Notes |
| --- | --- | --- |
| `HOST` with `EQ` | `spec.hostnames[]` | Use an exact hostname. |
| `HOST` with `ENDS_WITH` | `spec.hostnames[]` with a wildcard | `*.example.com` does not match the apex `example.com`. |
| `URL` with `STARTS_WITH` | `matches[].path.type: PathPrefix` | Path matching is element by element. |
| `URL` with `EQ` | `matches[].path.type: Exact` | Use this for routes that must win over a broader prefix. |
| `URL` with `REGEX` | `matches[].path.type: RegularExpression` | The regex engine is RE2, not PCRE. ALB regex is a search/prefix match; Envoy `RegularExpression` must match the whole path, so append `.*` unless the pattern ends with `$`. |
| `METHOD` with `EQ` | `matches[].method` | — |
| `HEADER` | `matches[].headers[]` | Multiple headers in one match are combined with AND. |
| `PARAM` | `matches[].queryParams[]` | — |
| `COOKIE` | No standard `HTTPRouteMatch` field | Match the `Cookie` header with `matches[].headers[]` and a `RegularExpression` that matches the entire header value, for example `.*session=abc.*`. |
| `SRC_IP` used as an allowlist | `SecurityPolicy.spec.authorization` with `defaultAction: Deny` and an allow rule using `principal.clientCIDRs` | Add `ClientTrafficPolicy.clientIPDetection` when ALB used `X-Real-IP` or `X-Forwarded-For`. |
| `SRC_IP` used to select a backend | No direct `HTTPRoute` equivalent | `SecurityPolicy` is access control, not routing. Keep the rule split or change the application. |

ALB reads `X-Real-IP` first, then the leftmost `X-Forwarded-For` entry, then the peer address. Envoy Gateway supports one of `customHeader` (for `X-Real-IP`) or `xForwardedFor` with `numTrustedHops` counted from the right. `ClientTrafficPolicy` applies to the whole Gateway or listener, not to a single route. Do not trust a client-supplied leftmost `X-Forwarded-For` value for an allowlist.

For example, a rule that matches `api.example.com/v1` and forwards to `api:8080` becomes:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api
  namespace: application
spec:
  parentRefs:
    - name: public-gateway
      namespace: gateway-system
      sectionName: https
  hostnames:
    - api.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /v1
      backendRefs:
        - name: api
          port: 8080
```

When one ALB rule contains several URL values, create one `matches[]` entry for each value. When one rule combines several types, put all conditions of one combination into the same match.

ALB rule `priority` has no direct Gateway API field. Do not rely on the order of rules in a YAML file. Make overlapping routes unambiguous: use `Exact` for the route that must win, and keep broader prefix or regex routes from capturing the same request. `EnvoyProxy.spec.preserveRouteOrder` can preserve rule order for the whole Gateway as a last resort, but it changes precedence for every route on that Gateway; use it only when match specificity cannot express the original priority.

### Backends and weights

Map `serviceGroup.services` to `backendRefs` and preserve the weights:

```yaml
rules:
  - matches:
      - path:
          type: PathPrefix
          value: /
    backendRefs:
      - name: api-v1
        port: 8080
        weight: 90
      - name: api-v2
        port: 8080
        weight: 10
```

When the backend Service is in another namespace, add a `ReferenceGrant` in the backend namespace.

### Rewrites

ALB `rewrite_base` and `rewrite_target` are a regex match and substitution pair. Standard Gateway API `URLRewrite` only supports `ReplaceFullPath` and `ReplacePrefixMatch`; it cannot express regex capture groups.

For a simple prefix rewrite, use the standard filter:

```yaml
filters:
  - type: URLRewrite
    urlRewrite:
      path:
        type: ReplacePrefixMatch
        replacePrefixMatch: /
```

For a rule that depends on capture groups, use an Envoy Gateway `HTTPRouteFilter` with `ReplaceRegexMatch`:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: HTTPRouteFilter
metadata:
  name: api-rewrite
  namespace: application
spec:
  urlRewrite:
    path:
      type: ReplaceRegexMatch
      replaceRegexMatch:
        pattern: "^/api(/|$)(.*)$"
        substitution: "/\\2"
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: api
  namespace: application
spec:
  parentRefs:
    - name: public-gateway
      namespace: gateway-system
      sectionName: https
  hostnames:
    - app.example.com
  rules:
    - matches:
        - path:
            type: RegularExpression
            value: "^/api(/|$)(.*)$"
      filters:
        - type: ExtensionRef
          extensionRef:
            group: gateway.envoyproxy.io
            kind: HTTPRouteFilter
            name: api-rewrite
      backendRefs:
        - name: api
          port: 8080
```

Check the following when converting a rewrite:

- Only add a rewrite filter to rules that actually change the path. A no-op rewrite such as `^/(console-portal)(/|$)(.*)$` to `/$1$2$3` is unnecessary; HTTPRoute forwards the original path when no rewrite is configured.
- Use a strict boundary such as `(/|$)` instead of `(.|$)`. A pattern such as `business-1(.|$)` can also match `business-10`.
- Do not treat an ALB wildcard path as a regex. `/dex/*` becomes `PathPrefix /dex/`, not a regular expression.
- Convert nginx regexes to RE2 carefully. Capture references and lookarounds may need a different expression.

### Request and response headers

Use `RequestHeaderModifier` and `ResponseHeaderModifier` for static header changes:

```yaml
filters:
  - type: RequestHeaderModifier
    requestHeaderModifier:
      set:
        - name: X-Forwarded-Proto
          value: https
      remove:
        - X-Internal-Debug
  - type: ResponseHeaderModifier
    responseHeaderModifier:
      set:
        - name: X-Frame-Options
          value: SAMEORIGIN
```

An ALB `rewrite-response` uses `ngx.header[key] = value`, which replaces the header. Use `set`, not `add`, when that behavior matters.

If the same response headers apply to every Ingress behind the Gateway, a Gateway-level `ClientTrafficPolicy` with `lateResponseHeaders` avoids duplicating the filter on every route:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: ClientTrafficPolicy
metadata:
  name: security-response-headers
  namespace: gateway-system
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: public-gateway
  headers:
    lateResponseHeaders:
      set:
        - name: X-Frame-Options
          value: SAMEORIGIN
        - name: X-Content-Type-Options
          value: nosniff
```

Use a Gateway-level policy only when every route needs the same headers. If different Ingress resources set different values, attach the policy or filter to the affected routes.

### Dynamic header values from nginx variables

ALB `rewrite-request` also supports `headers_var` and `headers_add_var`. Their values are nginx variables, not static strings:

```json
{"headers_var":{"X-Original-Method":"request_method"}}
```

A standard `RequestHeaderModifier` cannot evaluate an nginx variable because it only sets static values. For this pattern, use an `EnvoyExtensionPolicy` with a Lua filter that reads the request, extracts the value, and sets the target header. The following example copies the HTTP method into the `X-Original-Method` header:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyExtensionPolicy
metadata:
  name: dynamic-request-headers
  namespace: application
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: api
  lua:
    - type: Inline
      inline: |
        function envoy_on_request(handle)
          local method = handle:headers():get(":method")
          if method ~= nil then
            handle:headers():replace("x-original-method", method)
          end
        end
```

Attach the policy only to the routes that need the dynamic header; attaching it to the whole Gateway changes unrelated applications. Use `handle:headers():replace(...)` for `headers_var`, which replaces the header, and `handle:headers():add(...)` for `headers_add_var`, which appends. If the value is static, use `RequestHeaderModifier` instead of Lua.

### Redirects

Do not migrate an ALB redirect rule as a normal backend route. ALB redirect rules often point at a placeholder backend such as `none:8080`; keeping that backend produces an Envoy 500 error and hides the real path.

Use the standard `RequestRedirect` filter:

```yaml
rules:
  - matches:
      - path:
          type: PathPrefix
          value: /
    filters:
      - type: RequestRedirect
        requestRedirect:
          scheme: https
          hostname: www.example.com
          statusCode: 301
```

Check that the redirect route does not shadow an existing application route. A catch-all `PathPrefix /` redirect must not be attached to a listener that also serves real application routes.

### CORS

ALB `enableCORS`, `corsAllowOrigin`, and `corsAllowHeaders` can be mapped to `SecurityPolicy.spec.cors` or the route-level `CORS` filter:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: api-cors
  namespace: application
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: HTTPRoute
      name: api
  cors:
    allowOrigins:
      - "https://app.example.com"
      - "https://*.example.com"
    allowMethods:
      - GET
      - POST
      - OPTIONS
    allowHeaders:
      - Authorization
      - Content-Type
```

Split comma-separated ALB origin and header lists into separate list items. Envoy Gateway origins support exact values, a single wildcard label, and a global `*`; they do not support an arbitrary regex or a value that reflects the request origin dynamically. An ALB origin with a wildcard port, such as `http://172.21.4.4:*`, needs an explicit port. If the ALB used `$http_origin`, verify the resulting behavior or change the application.

### Timeouts

ALB timeouts are configured in `.spec.config.timeout`:

| ALB field | Closest Gateway API equivalent | Notes |
| --- | --- | --- |
| `proxy_connect_timeout_ms` | `BackendTrafficPolicy.spec.timeout.tcp.connectTimeout` | Connection establishment, including the TCP and TLS handshake. |
| `proxy_read_timeout_ms` | `BackendTrafficPolicy.spec.timeout.http.requestTimeout` or `ClientTrafficPolicy.spec.timeout.http.streamIdleTimeout` | ALB measures the interval between successive reads; choose the field whose semantics match the workload. |
| `proxy_send_timeout_ms` | `ClientTrafficPolicy.spec.timeout.http.streamIdleTimeout` or `BackendTrafficPolicy.spec.timeout.http.connectionIdleTimeout` | There is no exact one-to-one field. Validate long-lived and streaming workloads. |

For a per-route timeout, use the standard HTTPRoute fields:

```yaml
timeouts:
  request: 30s
  backendRequest: 30s
```

For a timeout that applies to every route on a Gateway, use `BackendTrafficPolicy` or `ClientTrafficPolicy`. Do not set both a route timeout and a conflicting policy timeout without checking which one takes effect.

### Session affinity

ALB `session_affinity_policy` does not map to a single field. Choose the equivalent explicitly:

- For cookie-based affinity, use `HTTPRoute.spec.rules[].sessionPersistence`:

  ```yaml
  sessionPersistence:
    type: Cookie
    sessionName: app-session
  ```

- For header-based affinity, use a `BackendTrafficPolicy` with consistent hashing on the client-supplied header. Do not use `sessionPersistence.type: Header`: it makes the gateway write an encoded upstream address into the response header, while ALB hashes the value the client already sends.

  ```yaml
  apiVersion: gateway.envoyproxy.io/v1alpha1
  kind: BackendTrafficPolicy
  metadata:
    name: header-affinity
    namespace: application
  spec:
    targetRefs:
      - group: gateway.networking.k8s.io
        kind: HTTPRoute
        name: api
    loadBalancer:
      type: ConsistentHash
      consistentHash:
        type: Headers
        headers:
          - name: x-session-id
  ```

- For source-IP hash (ALB `sip-hash`), use a `BackendTrafficPolicy` with consistent hashing:

  ```yaml
  apiVersion: gateway.envoyproxy.io/v1alpha1
  kind: BackendTrafficPolicy
  metadata:
    name: source-ip-affinity
    namespace: application
  spec:
    targetRefs:
      - group: gateway.networking.k8s.io
        kind: HTTPRoute
        name: api
    loadBalancer:
      type: ConsistentHash
      consistentHash:
        type: SourceIP
  ```

Validate the affinity behavior with a real client. The hash input and the cookie behavior are not identical between ALB and Envoy Gateway.

### HTTPS backends

`HTTPRoute` uses HTTP when it forwards to a Service unless a backend TLS policy is configured. If the ALB rule has `backendProtocol: https`, create a `BackendTLSPolicy`:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: api-backend-tls
  namespace: application
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: api
      sectionName: https
  validation:
    hostname: api.application.svc
    wellKnownCACertificates: System
```

> ALB 默认不校验后端证书（`proxy_ssl_verify` 未开启）。要严格保持 ALB 行为，可以用 Envoy Gateway `Backend` + `tls.insecureSkipVerify: true`，并在 `EnvoyGatewayCtl` 开启 `extensionApis.enableBackend`；`BackendTLSPolicy` 会强制校验证书，不是默认等价迁移。

`targetRefs.sectionName` must be the actual Service port name, not the port number. `validation.hostname` must match a SAN in the backend certificate; it is not always the Service name. If the backend uses a private CA, reference the appropriate Secret or ConfigMap instead of `wellKnownCACertificates`. ACP 4.3 and later serve `BackendTLSPolicy` `v1`; ACP 4.2 serves `v1alpha3`, which uses the same `targetRefs` and `validation` fields, so change only the `apiVersion`; ACP 4.1 and earlier use the older `v1alpha2` schema with `targetRef` (singular) and `tls.*`.

## Step 5: Handle behavior that has no direct conversion

ALB `rewrite-request` and `rewrite-response` are covered by `RequestHeaderModifier` and `ResponseHeaderModifier` for static values, and by `EnvoyExtensionPolicy` with Lua for `headers_var` and `headers_add_var`. Do not mark a route migrated until each of the remaining behaviors has an explicit replacement or an accepted product limitation:

- regex rewrites and `ImplementationSpecific` paths that depend on PCRE features not available in RE2;
- custom load-balancing algorithms such as EWMA;
- WAF features not provided by the installed Envoy Gateway policy;
- `SRC_IP` used as a routing condition to select different backends;
- redirects whose target depends on dynamic nginx variables;
- ALB session affinity that must be preserved bit-for-bit.

ALB project assignment and port-project isolation do not have a direct `HTTPRoute` field. Use `Gateway.spec.listeners[].allowedRoutes.namespaces` to preserve them: `from: Same` limits routes to the Gateway namespace, `from: Selector` matches project namespaces by label, and `from: All` permits every namespace. Apply this per listener so that a project can only attach routes to the listeners that belong to it. Confirm the required isolation behavior before removing the ALB.

### Migrate L4 traffic 

ALB TCP and UDP frontends are not migrated with `HTTPRoute`. A directly created L4 `Frontend` can carry its own `spec.serviceGroup` instead of a separate `Rule`. For each L4 frontend, its embedded `serviceGroup`, and any directly created `Rule`, evaluate:

- a `LoadBalancer` Service when the traffic does not need Gateway API routing;
- `TCPRoute` or `UDPRoute` when the installed Envoy Gateway version supports the route type;
- keeping ALB for the L4 ports if the installed version does not support the required route type.

Map the `Frontend` or `Rule` `serviceGroup.services` to the `TCPRoute` or `UDPRoute` `backendRefs`. Do not create an `HTTPRoute` for a TCP or UDP frontend. Check the backend port and the session-affinity behavior separately; ALB `sip-hash` on an L4 rule is not the same as HTTP session persistence.

## Step 6: Validate before cutover

Apply the new resources without deleting the old ones:

```shell
kubectl apply -f gateway.yaml
kubectl apply -f routes-and-policies.yaml
```

Check status at every layer:

```shell
kubectl get gateway -A
kubectl get httproute -A -o yaml
kubectl get backendtlspolicy,clienttrafficpolicy,backendtrafficpolicy,securitypolicy -A
```

Verify all of the following:

1. The Gateway reports `Programmed=True`.
2. Each HTTPRoute has `Accepted=True` and `ResolvedRefs=True` under `status.parents`.
3. Each referenced Service, Secret, policy, and extension resource is resolved.
4. HTTP and HTTPS requests return the expected status, headers, redirects, and response body.
5. Regex paths, rewritten paths, session persistence, CORS, authentication, and backend TLS work as expected.
6. The test hostname reaches the new Gateway address and does not reach the old ALB.
7. Response time, error rate, and throughput are acceptable for the workload.

Use `curl` with an explicit `Host` header when testing before DNS changes:

```shell
curl --resolve app.example.com:443:<gateway-address> \
  --cacert <ca-file> https://app.example.com/api/health
```

Some applications do not support `HEAD`; use a real `GET` request for the final functional check.

## Step 7: Cut over and roll back

After validation, move traffic in a controlled way:

1. Lower the DNS TTL before the change.
2. Point a test or small traffic slice at the Gateway address.
3. Monitor request errors, latency, backend health, and Gateway/HTTPRoute status.
4. Update the production DNS record or external load-balancer target.
5. Keep the old ALB or Ingress configuration available during the observation window.

If the new path fails, restore DNS or the external load-balancer target to the old address, then inspect the route and policy status before retrying. Do not scale down or delete the old ALB until the rollback window has ended and the new path has passed the agreed monitoring period.

## Listener limits and large ALB migrations 

Most ALB instances expose fewer than 63 ports and do not need this section. Use it only when the source ALB has a large number of listeners.

Listener limits depend on the ACP release. ACP supports up to 63 listeners per `Gateway` and per `ListenerSet`. When the source ALB exposes more ports than one object can hold, use ListenerSets or split the listeners across Gateways.

| ACP release | ListenerSet | Web Console | Migrating an ALB with more than 63 ports |
| --- | --- | --- | --- |
| Earlier than 4.4 | Not available as a supported product feature | Not available | A single Gateway cannot hold all listeners. Split the ports across multiple Gateways (each with at most 63 listeners) and optionally share one VIP across them; see Share a VIP across Gateways. You can also reduce the number of exposed ports or keep ALB for the excess ports. |
| 4.4.x | Supported through the API and YAML | Not available | Create one or more `ListenerSet` resources through YAML and attach them to the Gateway. Alternatively, split the ports across Gateways and share one VIP; see Share a VIP across Gateways. |
| 4.5.x and later | Supported | Supported | Create `ListenerSet` resources in the Web Console. |

The 63-listener limit is per Gateway and per ListenerSet, not per cluster. Multiple ListenerSets can attach to the same Gateway. On ACP 4.4, the migration is fully supported through YAML; the missing Web Console support only affects how you create and review ListenerSets.



### Exceed 63 listeners with ListenerSet

If one ALB exposes more than 63 ports, keep the first 63 listeners on the Gateway and create a `ListenerSet` for the additional ports. The parent Gateway must allow ListenerSets. The following example shows only the `http` listener and the new `allowedListeners` field; add `allowedListeners` to the Gateway from Step 2 instead of replacing its full listener list. It reuses the `EnvoyProxy` from Step 2:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: public-gateway
  namespace: gateway-system
spec:
  gatewayClassName: envoy-gateway-operator-cpaas-default
  infrastructure:
    parametersRef:
      group: gateway.envoyproxy.io
      kind: EnvoyProxy
      name: public-gateway
  allowedListeners:
    namespaces:
      from: Same
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
```

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: public-gateway-extra
  namespace: gateway-system
spec:
  parentRef:
    name: public-gateway
    namespace: gateway-system
  listeners:
    - name: tcp-1805
      protocol: TCP
      port: 1805
      allowedRoutes:
        namespaces:
          from: All
```

Attach a Route to a ListenerSet listener by setting `parentRefs[].kind: ListenerSet` and `sectionName` to the listener name. 

On ACP 4.4, create and edit ListenerSets through YAML. In ACP 4.5 and later, the Web Console also provides the ListenerSet creation page.

### Share a VIP across Gateways 

On releases without ListenerSet, or when you prefer not to use ListenerSet, you can split the listeners across multiple Gateways and use MetalLB IP sharing to expose all of them through one VIP. Each Gateway still has its own `EnvoyProxy` and its own Envoy data-plane Service.

This is a workaround, not the default migration path. Prefer ListenerSet when it is available.

The following example creates two Gateways that share one VIP:

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: shared-gateway-vip
  namespace: metallb-system
spec:
  addresses:
    - 192.0.2.100/32
  autoAssign: false
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: shared-gateway-vip
  namespace: metallb-system
spec:
  ipAddressPools:
    - shared-gateway-vip
---
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: gw-a
  namespace: gateway-system
spec:
  provider:
    type: Kubernetes
    kubernetes:
      envoyService:
        type: LoadBalancer
        externalTrafficPolicy: Cluster
        annotations:
          metallb.io/allow-shared-ip: shared-gateway-vip
          metallb.io/loadBalancerIPs: 192.0.2.100
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gw-a
  namespace: gateway-system
spec:
  gatewayClassName: envoy-gateway-operator-cpaas-default
  infrastructure:
    parametersRef:
      group: gateway.envoyproxy.io
      kind: EnvoyProxy
      name: gw-a
  listeners:
    - name: http-a
      protocol: HTTP
      port: 8080
      allowedRoutes:
        namespaces:
          from: All
```

Create `gw-b` the same way, with its own `EnvoyProxy`, the same `allow-shared-ip` key and VIP, and a different listener port:

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: gw-b
  namespace: gateway-system
spec:
  provider:
    type: Kubernetes
    kubernetes:
      envoyService:
        type: LoadBalancer
        externalTrafficPolicy: Cluster
        annotations:
          metallb.io/allow-shared-ip: shared-gateway-vip
          metallb.io/loadBalancerIPs: 192.0.2.100
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gw-b
  namespace: gateway-system
spec:
  gatewayClassName: envoy-gateway-operator-cpaas-default
  infrastructure:
    parametersRef:
      group: gateway.envoyproxy.io
      kind: EnvoyProxy
      name: gw-b
  listeners:
    - name: http-b
      protocol: HTTP
      port: 8081
      allowedRoutes:
        namespaces:
          from: All
```

Use the following rules:

- Every Gateway must have its own `EnvoyProxy` and data-plane Service. Do not try to put listeners from different Gateways into one Service.
- Set `externalTrafficPolicy: Cluster` on every Service that shares the VIP. With `Local`, MetalLB's sharing backend key includes the Service selector; each Gateway has its own selector, so the second Service stays `<pending>` with a backend-key mismatch. `Local` can also drop traffic on nodes without a local endpoint.
- The ports of the Services that share the VIP must not overlap. MetalLB checks the `protocol/port` pair: two Services cannot both expose `TCP/8080` on the same VIP. If two Gateways must serve the same port, this workaround does not apply; use one listener with `HTTPRoute` hostnames, or use ListenerSet.
- Set `IPAddressPool.spec.autoAssign: false` and request the VIP explicitly with `metallb.io/loadBalancerIPs`. With `autoAssign: true`, an unrelated pending `LoadBalancer` Service can take the VIP first.
- Set the annotations through `EnvoyProxy.spec.provider.kubernetes.envoyService.annotations`, not by editing the Envoy data-plane Service directly; the operator reconciles the Service.
- Do not use `Gateway.spec.addresses` to pin the VIP. Envoy Gateway maps that field to `Service.spec.externalIPs`, not to the MetalLB load balancer IP.

MetalLB 0.15 also accepts the deprecated `metallb.universe.tf/` prefix. Use the prefix that your MetalLB version documents; do not mix prefixes for the same annotation.

`externalTrafficPolicy: Cluster` can lose the original client source IP because kube-proxy may SNAT the connection. `ClientTrafficPolicy.clientIPDetection` can only read an address that an upstream load balancer or proxy has already put into `X-Forwarded-For`, a custom header, or the PROXY protocol; it cannot recover an address that was SNATed away. If the application depends on the client IP, keep a single Gateway with ListenerSet and `Local`, or make sure an upstream device adds one of those headers. This also affects `ConsistentHash: SourceIP` and source-IP authorization, which will see the node or proxy IP instead of the client.

Verify that both Services received the same VIP and that both Gateways are programmed:

```shell
kubectl get gateway -n gateway-system
kubectl get svc -n <envoy-gateway-ctl-namespace> -l gateway.envoyproxy.io/owning-gateway-namespace=gateway-system -o wide
```

The Envoy data-plane Service is created in the namespace of the `EnvoyGatewayCtl`, not necessarily in the Gateway namespace.

If one Service stays in `<pending>`, check the MetalLB controller events for an allocation failure. A port conflict reports `port TCP/<port> is already in use on "<vip>"`; a `Local` sharing conflict reports a backend-key mismatch.

This workaround does not provide cross-Gateway health checking or failover. Each Gateway is an independent data plane; monitor each Gateway and its own backend health.
