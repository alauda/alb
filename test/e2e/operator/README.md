这个package主要测试operator部署相关的东西

 matrix
| gateway:| enable-shared | enable-standalone | disable|
|--------|---------------|----------------------------|
| network:| host          | container                  |
|--------|---------------|----------------------------|
|  alb:     | enable        | disable                    |

 3 * 2 * 2 = 12
 gateway disable and alb disable is meaningless. left 11
## common used mode
###  1. global default 集群默认部署的alb
    gateway enable-shared
    network host
    alb enable
### 2. user deploy alb-host 用户部署的主机网络的alb
    gateway disable
    network host
    alb enable
### 3. user deploy alb-container 用户部署的容器网络的alb
    gateway disable
    network container
    alb enable
### 4. user deploy gateway    3.14之后加入的 独享型gateway
    gateway enable-standalone
    network container
    alb disable
### 5. user deploy gateway (with alb)
    gateway enable-standalone
    network container
    alb enable