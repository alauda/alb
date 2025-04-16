## what it is
when proxy client request to upstream, in default alb will add some headers to this request.

| name                                                               | value                       |
| x-forwarded-for                                                    | $proxy_add_x_forwarded_for |
| x-forwarded-proto                                                  | $scheme                     |
| x-forwarded-host                                                   | $http_host                  |
| x-forwarded-port                                                   | $server_port                |
| x-real-ip                                                          | $remote_addr                |
| x-original-uri                                                     | $request_uri                |
| x-scheme                                                           | $scheme                     |
| x-original-url                                                     | $scheme://$http_host$request_uri |
| x-original-method                                                  | $request_method             |

you could use use-forwarded-headers to control alb upsert the x-forwarded headers to upstream.
## how to config

### via alb/ft/rule cr
```yaml
proxy_header:
  properties:
    use_forwarded_headers:
      type: boolean
  required:
  - use_forwarded_headers
  type: object
```

### via ingress annotation
use `alb.ingress.cpaas.io/use-forwarded-headers: "true"` . default or unset is false.

## other cases

### specify x-real-ip as first_forward_or_remote_addr
you could use the [rewrite-request](../rewrite_request/rewrite_request.md) to specify the x-real-ip as first_forward_or_remote_addr.

for example:
```yaml
alb.ingress.cpaas.io/rewrite-request: '{
    "headers_var": {
        "x-real-ip": "first_forward_or_remote_addr"
    }
}'
```


