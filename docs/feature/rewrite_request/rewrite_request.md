## what it is
## how to config
### use annotation
| target  | annotation key                         |
|---------|----------------------------------------|
| ingress | `alb.ingress.cpaas.io/rewrite-request` |
| rule    | `alb.rule.cpaas.io/rewrite-request`    |
the value is a json string, the format is as follows:
```go
type RewriteRequestConfig struct {
	Headers       map[string]string   `json:"headers,omitempty"`
	HeadersVar    map[string]string   `json:"headers_var,omitempty"`
	HeadersRemove []string            `json:"headers_remove,omitempty"`
	HeadersAdd    map[string][]string `json:"headers_add,omitempty"`
	HeadersAddVar map[string][]string `json:"headers_add_var,omitempty"`
}
```
notice the `xx_var` is the key of the variable in the context of alb, the value is the name of the variable.
for example:
```yaml
alb.ingress.cpaas.io/rewrite-request: '{
    "headers_var": {
        "x-my-host": "http_host"
    }
}'
```
you can refer to the [nginx variable](http://nginx.org/en/docs/varindex.html) for the variable names.

## extra variables

| variable name                  | description                                                                 |
|--------------------------------|-----------------------------------------------------------------------------|
| `first_forward_or_remote_addr` | the first forwarded address or the remote address, default is `remote_addr` |
| `first_forward`                | the first forwarded address , default is empty string                       |

