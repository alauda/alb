## csv and rbac
现在csv是直接在chart中部署的
因为发现这种方式无法创建出serviceaccount,所以rbac相关的资源是chart直接创建的
## gatewayapi
| gatewayapi v0.6.2 | v1apha2    | v1beta1    | v1 |
|-------------------|------------|------------|----|
| gatewayclass      |    v       | v(storage) |    |
| gatway            |    v       | v(storage) |    |
| httproute         |    v       | v(storage) |    |
| tcproute          | v(storage) |            |    |
| udproute          | v(storage) |            |    |
| tlsroute          | v(storage) |            |    |
| referencegrant    | v(storage) | v          |
| grpcroute         | v(storage) |            |    |
|-------------------|------------|------------|----|

| gatewayapi v1.0.0 | v1apha2    | v1beta1    | v1 |
|-------------------|------------|------------|----|
| gatewayclass      |            | v(storage) | v  |
| gatway            |            | v(storage) | v  |
| httproute         |            | v(storage) | v  |
| tcproute          | v(storage) |            |    |
| udproute          | v(storage) |            |    |
| tlsroute          | v(storage) |            |    |
| referencegrant    | v          | v(storage) |    |
| grpcroute         | v(storage) |            |    |
| btlspolicy        | v(storage) |            |    |