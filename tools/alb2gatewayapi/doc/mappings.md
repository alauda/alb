# ALB 到 Gateway API 映射表

每个 ALB 功能一条，写清楚：ALB 来源、Gateway API 目标、映射规则、检查 ID、注意事项。`scripts/check_features.py` 会按检查 ID 输出审计结果。

## 基础对象

| ALB 功能 | ALB 字段/注解 | Gateway API 目标 | 映射规则 | 检查 ID | 说明 |
| --- | --- | --- | --- | --- | --- |
| ALB 实例 | `ALB2` | `GatewayClass` + `Gateway` + 独立 `EnvoyProxy` | 一个 ALB 对应一个 Gateway；Gateway 必须通过 `spec.infrastructure.parametersRef` 引用自己的 EnvoyProxy | `ALB-INSTANCE` | 默认 GatewayClass 用 `envoy-gateway-operator-cpaas-default` |
| ALB 语义 annotation | `ALB2.metadata.annotations` 的 `cpaas.io/display-name` / `cpaas.io/description` / `cpaas.io/creator` | `Gateway.metadata.annotations` | display-name/description 同名迁移；creator 写到 `cpaas.io/origin-creator` | `ALB-METADATA` | 不复制 `cpaas.io/updated-at`；ALB2 缺失时 creator 只在 Frontend/Rule 一致时推断 |
| 端口/协议 | `Frontend.spec.port` / `protocol` | `Gateway.spec.listeners[]` | 一个唯一端口/协议一个 listener | `ALB-FRONTEND` | HTTP/HTTPS/TCP/UDP 分别对应 listener |
| 证书 | `Frontend.spec.certificate_name`、`Rule.spec.certificate_name` | listener `tls.certificateRefs` | `namespace/secret` 或 `namespace_secret` 转 Secret 引用；跨 namespace 生成 ReferenceGrant | `ALB-CERT` | 同一 HTTPS 端口聚合多个证书，靠 SNI 选择 |
| Ingress 生成的 Rule | `Rule.spec.source.type: ingress` | `HTTPRoute` | 迁移原始 Ingress，不逐条转发生成的 Rule | `ALB-INGRESS-RULE` | 原始 Ingress 未导出时要单独提示 |
| 直接创建的 Rule | `Rule.spec.source` 为空 | `HTTPRoute` / `TCPRoute` / `UDPRoute` | L7 转 HTTPRoute；L4 转 TCPRoute/UDPRoute | `ALB-DIRECT-RULE` | 不能把 L4 写成 HTTPRoute |
| L4 Frontend 内嵌 backend | `Frontend.spec.serviceGroup` | `TCPRoute` / `UDPRoute` | Frontend 自己的 serviceGroup 转 L4 route | `ALB-L4-FRONTEND` | 目标版本支持 TCPRoute/UDPRoute，不再确认 CRD 支持 |

## 匹配条件

| ALB 功能 | ALB 字段/注解 | Gateway API 目标 | 映射规则 | 检查 ID | 说明 |
| --- | --- | --- | --- | --- | --- |
| Host 匹配 | `dslx` `HOST` / `spec.domain` | `HTTPRoute.spec.hostnames[]` | `EQ` -> 精确 hostname；`ENDS_WITH` -> 通配 hostname | `ALB-DSLX-HOST` | 多 host 的 Ingress 按 host 拆 HTTPRoute，避免跨 host 误匹配 |
| URL 前缀 | `dslx` `URL` `STARTS_WITH` | `PathPrefix` | 直接映射 | `ALB-DSLX-URL-PREFIX` | `PathPrefix` 是路径段匹配，`/v1` 不匹配 `/v1beta` |
| URL 精确 | `dslx` `URL` `EQ` | `Exact` | 直接映射 | `ALB-DSLX-URL-EXACT` | — |
| URL 正则 | `dslx` `URL` `REGEX` | `RegularExpression` | ALB 是 search/prefix，Envoy 是整路径匹配；不结束于 `$` 时补 `.*` | `ALB-DSLX-URL-REGEX` | 检查 RE2 不支持的 PCRE 特性 |
| Method | `dslx` `METHOD` | `matches[].method` | `EQ` 直接映射 | `ALB-DSLX-METHOD` | — |
| Header | `dslx` `HEADER` | `matches[].headers[]` | `EQ` -> `Exact`；多个值需要拆 match | `ALB-DSLX-HEADER` | header 之间是 AND，match 之间是 OR |
| Query 参数 | `dslx` `PARAM` | `matches[].queryParams[]` | 直接映射 | `ALB-DSLX-PARAM` | — |
| Cookie 匹配 | `dslx` `COOKIE` | 无标准 HTTPRouteMatch 字段 | 用 `Cookie` header + `RegularExpression`，或改应用 | `ALB-DSLX-COOKIE` | 不能直接映射，必须提示 |
| 源 IP 匹配 | `dslx` `SRC_IP` | allowlist -> `SecurityPolicy.authorization`；routing -> 无直接等价 | allowlist 用 `defaultAction: Deny` + `principal.clientCIDRs`；routing 单独列出 | `ALB-DSLX-SRC-IP` | SecurityPolicy 是访问控制，不是路由 |
| 优先级 | `Rule.spec.priority` | 无直接字段 | 用精确 match / 路由优先级表达；最后才考虑 `EnvoyProxy.spec.preserveRouteOrder` | `ALB-PRIORITY` | 不依赖 YAML 顺序 |

## 后端

| ALB 功能 | ALB 字段/注解 | Gateway API 目标 | 映射规则 | 检查 ID | 说明 |
| --- | --- | --- | --- | --- | --- |
| 后端 Service | `serviceGroup.services[]` | `backendRefs[]` | 保留 name/namespace/port/weight | `ALB-BACKEND` | HTTPRoute 放在后端 Service 的 namespace |
| 权重 | `services[].weight` | `backendRefs[].weight` | 100 可省略，其他保留 | `ALB-BACKEND-WEIGHT` | — |
| 后端 HTTPS | `Rule.spec.backendProtocol: https` | `Backend` | target 指向 Service DNS；`tls.insecureSkipVerify: true` 保持 ALB 默认不校验；需要 `EnvoyGatewayCtl` 开启 `enableBackend` | `ALB-BACKEND-TLS` | 不再猜证书 hostname；不校验行为与 ALB 一致 |
| 后端 gRPC | `Rule.spec.backendProtocol: grpc` | `GRPCRoute` 或 Service `appProtocol: kubernetes.io/h2c` | 优先 GRPCRoute | `ALB-BACKEND-GRPC` | — |
| 上游 Host 覆盖 | `Rule.spec.vhost` | `URLRewrite.hostname` | 在 HTTPRoute rule 的 inline `URLRewrite` filter 设置 hostname；regex rewrite 另用 EG `HTTPRouteFilter` | `ALB-VHOST` | 不再写 EG filter 的非法 string hostname |
| 会话保持 | `serviceGroup.session_affinity_policy` | `sessionPersistence` / `BackendTrafficPolicy` | `sip-hash` -> ConsistentHash SourceIP，HTTPRoute/TCPRoute/UDPRoute 都支持；cookie/header 尚未实现 | `ALB-SESSION-AFFINITY` | L4 也用 BackendTrafficPolicy；不要声称 cookie/header 已生成 |

## 改写和响应

| ALB 功能 | ALB 字段/注解 | Gateway API 目标 | 映射规则 | 检查 ID | 说明 |
| --- | --- | --- | --- | --- | --- |
| 简单前缀 rewrite | `rewrite_base` / `rewrite_target` | `URLRewrite` | `ReplacePrefixMatch` | `ALB-REWRITE-PREFIX` | 只改 path 的 rule 才挂 |
| Regex rewrite | `rewrite_base` / `rewrite_target` 含捕获组 | `HTTPRouteFilter` `ReplaceRegexMatch` | `$1` -> `\1`；pattern 用原 `rewrite_base` | `ALB-REWRITE-REGEX` | 检查整路径匹配 |
| 请求头改写 | `alb.ingress.cpaas.io/rewrite-request`、`alb.rule.cpaas.io/rewrite-request` | `RequestHeaderModifier` / Lua | 静态 header 用标准 filter；`headers_var`/`headers_add_var` 用 Lua | `ALB-REWRITE-REQUEST` | `headers_var` 用 `replace`，`headers_add_var` 用 `add` |
| 响应头改写 | `alb.ingress.cpaas.io/rewrite-response`、`alb.rule.cpaas.io/rewrite-response` | `ResponseHeaderModifier` / `ClientTrafficPolicy` | 静态 header 用 ResponseHeaderModifier；Gateway 级统一头用 lateResponseHeaders | `ALB-REWRITE-RESPONSE` | `ngx.header[k]=v` 对应 `set` |
| `headers_var` | `headers_var` / `headers_add_var` | `EnvoyExtensionPolicy` + Lua | 从请求 header/cookie 取值，动态写目标 header | `ALB-HEADERS-VAR` | 标准 filter 只能设静态值 |

## CORS

| ALB 功能 | ALB 字段/注解 | Gateway API 目标 | 映射规则 | 检查 ID | 说明 |
| --- | --- | --- | --- | --- | --- |
| 静态 CORS origin | `enableCORS` + `corsAllowOrigin` + `corsAllowHeaders` | `CORS` filter / `SecurityPolicy.spec.cors` | 逗号拆分后作为 `allowOrigins`；methods 对齐 ALB；`allowCredentials: true` | `ALB-CORS-STATIC` | ALB 对 `*.domain` 是字面量匹配，不扩成 Gateway 通配符 |
| CORS 端口通配符 | `corsAllowOrigin` 含 `:*` | 无法直接映射 | 从 CORS filter 排除，提示用户确认实际端口 | `ALB-CORS-PORT-WILDCARD` | ALB `cors.lua` 也是字面量匹配，ALB 本身不支持 |
| CORS 动态 origin | `corsAllowOrigin` 为 `$http_origin` | 无法直接映射 | 从 CORS filter 排除；需要 Lua/ExtProc 或应用处理 | `ALB-CORS-DYNAMIC-ORIGIN` | ALB 不会展开 `$http_origin`，ALB 本身不支持 |
| CORS 缺逗号 | `http://a.com.https://b.com` | 自动修正 | 在 `https://` 前补逗号 | `ALB-CORS-MISSING-COMMA` | 工具自动拆分 |

## 超时、重定向、其他

| ALB 功能 | ALB 字段/注解 | Gateway API 目标 | 映射规则 | 检查 ID | 说明 |
| --- | --- | --- | --- | --- | --- |
| 连接/读/发送超时 | `proxy_connect_timeout_ms` / `proxy_read_timeout_ms` / `proxy_send_timeout_ms` | `HTTPRoute.spec.rules[].timeouts` | 取三个非空值的最大值，写入 `request` 和 `backendRequest` | `ALB-TIMEOUT` | 不创建 per-route policy；HTTPRoute timeout 是总时长，最大值是保守上限 |
| L4 端口超时 | `Frontend.spec.config.timeout` | `ClientTrafficPolicy.spec.timeout.tcp.idleTimeout` + `BackendTrafficPolicy.spec.timeout.tcp.connectTimeout` | 挂 Gateway listener | `ALB-TIMEOUT-L4` | 不需要 per-route policy |
| HTTP 重定向 | `config.redirect.*` + legacy `redirectURL/redirectCode` | `RequestRedirect` filter | 映射 code/scheme/host/port/path/prefix；`$request_uri` 默认保留原 path/query | `ALB-REDIRECT` | ALB 当前发送字面量；迁移后按原意保留 path/query，报告标行为变化 |
| >63 listener | 端口数 | ListenerSet / 多 Gateway + 共享 VIP | 按产品文档最后章节处理 | `ALB-LISTENER-LIMIT` | 目标版本支持 ListenerSet |
| 系统 LB | ALB 名 | 不转换 | 跳过 `cpaas-system`、`global-alb2` | `ALB-SYSTEM-LB` | `global-alb` 是用户 LB，不跳过 |
| Gateway namespace | 后端 Service 分布 | 由用户指定；单 ns 可复用后端 ns | HTTPRoute 放在 Service namespace；listener `allowedRoutes` 控制挂载 | `ALB-GATEWAY-NAMESPACE` | 使用 `--gateway-namespace` 明确指定；不要把用户 Gateway 放 `cpaas-system` |
| allowedRoutes | Rule namespace / project label | `Gateway.spec.listeners[].allowedRoutes.namespaces` | 同 ns `Same`；跨 ns `Selector`；全共享才 `All` | `ALB-ALLOWED-ROUTES` | 80 等默认规则无 project 正常；按 namespace 收紧，不要只用 project label |
| 跨 namespace Secret | `certificate_name` 的 namespace | `ReferenceGrant` | 在 Secret namespace 生成 ReferenceGrant，from Gateway | `ALB-REFERENCE-GRANT-SECRET` | 工具自动生成 |
| 跨 namespace Service | `serviceGroup.services[].namespace` | `backendRefs[].namespace` + `ReferenceGrant` | 只有同一 Route 的后端跨 ns 时才需要 | `ALB-REFERENCE-GRANT-SERVICE` | HTTPRoute 优先放 Service namespace |
| Backend Service 存在性 | `serviceGroup.services[]` | 无 | 不做转换前置条件；提供检查脚本 | `ALB-BACKEND-EXISTS` | `scripts/check_backends.py` |
| Policy 作用域 | 各 policy 的 targetRefs | Gateway/listener/HTTPRoute/TCPRoute/UDPRoute/Backend | 按最小作用面挂载 | `ALB-POLICY-SCOPE` | CORS route filter；timeout/affinity Route；后端 HTTPS 用 Backend |
