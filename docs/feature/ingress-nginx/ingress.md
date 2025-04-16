## 支持的 ingress-nginx 注解
| 名称                                                                 | 类型                       | 支持（v 表示支持 x 表示不支持 o 表示部分支持或可通过配置实现） |
|----------------------------------------------------------------------|----------------------------|--------------------------------------------------------------|
| nginx.ingress.kubernetes.io/app-root                                 | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/affinity                                 | cookie                     | o ingress 不支持。alb 规则可以配置 cookie 哈希                |
| nginx.ingress.kubernetes.io/use-regex                                | 布尔值                     |                                                              |
| nginx.ingress.kubernetes.io/affinity-mode                            | "balanced" 或 "persistent" | o ingress 不支持。alb 规则可以配置会话持久性                  |
| nginx.ingress.kubernetes.io/affinity-canary-behavior                 | "sticky" 或 "legacy"       | o ingress 不支持。alb 规则可以配置会话持久性                  |
| nginx.ingress.kubernetes.io/auth-realm                               | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/auth-secret                              | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/auth-secret-type                         | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/auth-type                                | "basic" 或 "digest"        | v                                                            |
| nginx.ingress.kubernetes.io/auth-tls-secret                          | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/auth-tls-verify-depth                    | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/auth-tls-verify-client                   | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/auth-tls-error-page                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/auth-tls-pass-certificate-to-upstream    | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/auth-tls-match-cn                        | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/auth-url                                 | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/auth-cache-key                           | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/auth-cache-duration                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/auth-keepalive                           | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/auth-keepalive-share-vars                | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/auth-keepalive-requests                  | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/auth-keepalive-timeout                   | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/auth-proxy-set-headers                   | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/auth-snippet                             | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/enable-global-auth                       | "true" 或 "false"          | o                                                            |
| nginx.ingress.kubernetes.io/backend-protocol                         | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/canary                                   | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/canary-by-header                         | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/canary-by-header-value                   | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/canary-by-header-pattern                 | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/canary-by-cookie                         | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/canary-weight                            | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/canary-weight-total                      | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/client-body-buffer-size                  | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/configuration-snippet                    | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/custom-http-errors                       | \[]int                     | x                                                            |
| nginx.ingress.kubernetes.io/custom-headers                           | 字符串                     | o                                                            |
| nginx.ingress.kubernetes.io/default-backend                          | 字符串                     | o 可以使用 ingress 的 default-backend                        |
| nginx.ingress.kubernetes.io/enable-cors                              | "true" 或 "false"          | v                                                            |
| nginx.ingress.kubernetes.io/cors-allow-origin                        | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/cors-allow-methods                       | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/cors-allow-headers                       | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/cors-expose-headers                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/cors-allow-credentials                   | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/cors-max-age                             | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/force-ssl-redirect                       | "true" 或 "false"          | v                                                            |
| nginx.ingress.kubernetes.io/from-to-www-redirect                     | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/http2-push-preload                       | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/limit-connections                        | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/limit-rps                                | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/global-rate-limit                        | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/global-rate-limit-window                 | duration                   | x                                                            |
| nginx.ingress.kubernetes.io/global-rate-limit-key                    | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/global-rate-limit-ignored-cidrs          | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/permanent-redirect                       | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/permanent-redirect-code                  | 数字                       | v                                                            |
| nginx.ingress.kubernetes.io/temporal-redirect                        | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/preserve-trailing-slash                  | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/proxy-body-size                          | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-cookie-domain                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-cookie-path                        | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-connect-timeout                    | 数字                       | v                                                            |
| nginx.ingress.kubernetes.io/proxy-send-timeout                       | 数字                       | v                                                            |
| nginx.ingress.kubernetes.io/proxy-read-timeout                       | 数字                       | v                                                            |
| nginx.ingress.kubernetes.io/proxy-next-upstream                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-next-upstream-timeout              | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/proxy-next-upstream-tries                | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/proxy-request-buffering                  | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-redirect-from                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-redirect-to                        | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-http-version                       | "1.0" 或 "1.1"             | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-secret                         | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-ciphers                        | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-name                           | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-protocols                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-verify                         | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-verify-depth                   | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/proxy-ssl-server-name                    | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/enable-rewrite-log                       | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/rewrite-target                           | URI                        | v                                                            |
| nginx.ingress.kubernetes.io/satisfy                                  | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/server-alias                             | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/server-snippet                           | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/service-upstream                         | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-change-on-failure         | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-conditional-samesite-none | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-domain                    | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-expires                   | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-max-age                   | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-name                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-path                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-samesite                  | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/session-cookie-secure                    | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/ssl-redirect                             | "true" 或 "false"          | v                                                            |
| nginx.ingress.kubernetes.io/ssl-passthrough                          | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/stream-snippet                           | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/upstream-hash-by                         | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/x-forwarded-prefix                       | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/load-balance                             | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/upstream-vhost                           | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/denylist-source-range                    | CIDR                       | o                                                            |
| nginx.ingress.kubernetes.io/whitelist-source-range                   | CIDR                       | o                                                            |
| nginx.ingress.kubernetes.io/proxy-buffering                          | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-buffers-number                     | 数字                       | x                                                            |
| nginx.ingress.kubernetes.io/proxy-buffer-size                        | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/proxy-max-temp-file-size                 | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/ssl-ciphers                              | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/ssl-prefer-server-ciphers                | "true" 或 "false"          | x                                                            |
| nginx.ingress.kubernetes.io/connection-proxy-header                  | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/enable-access-log                        | "true" 或 "false"          | o 默认启用 access_log，格式固定                               |
| nginx.ingress.kubernetes.io/enable-opentelemetry                     | "true" 或 "false"          | v                                                            |
| nginx.ingress.kubernetes.io/opentelemetry-trust-incoming-span        | "true" 或 "false"          | v                                                            |
| nginx.ingress.kubernetes.io/enable-modsecurity                       | 布尔值                     | v                                                            |
| nginx.ingress.kubernetes.io/enable-owasp-core-rules                  | 布尔值                     |                                                              |
| nginx.ingress.kubernetes.io/modsecurity-transaction-id               | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/modsecurity-snippet                      | 字符串                     | v                                                            |
| nginx.ingress.kubernetes.io/mirror-request-body                      | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/mirror-target                            | 字符串                     | x                                                            |
| nginx.ingress.kubernetes.io/mirror-host                              | 字符串                     | x                                                            |
## 支持的 ingress-nginx configmap 配置
### use-forwarded-headers
support via cr or ingress annotation [proxy-header](../../feature/proxy_header/proxy_header.md)