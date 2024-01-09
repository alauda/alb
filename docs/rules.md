# DSLX

为了适应复杂情况的流量分发，ALB 提供了一组自定义流量匹配 DSL 名为 dslx。 在 Rule 中可以配置 dslx，最终会生成前序表达式供 ALB 规则引擎进行流量匹配。dslx 的格式如下：

```yaml
  - type: string # matcher
    key: string (optional)
    values:
     - - string # op 1
       - string # conditions
     - - string # op 2
       - string # condition
```

- `type` 指定具体要匹配那个属性，目前有 `HOST`,`URL SRC_IP`, `METHOD`, `HEADER`, `PARAM`, `COOKIE` 这几个 matcher。
  - 多个 matcher 之间的关系是 `AND`。
- `key` 指定额外的匹配条件，比如匹配 `HEADER` 时，具体是哪一个 `HEADER`：
   - 当 `type` 是 `HEADER`, `PARAM` 或 `COOKIE` 时，可以指定具体的 `key`。 
- 通过 `type` 和 `key` 我们可以明确要匹配的是哪个属性，通过 `values` 指定匹配条件： 
    - `values` 是一个表格，每行表示一个匹配条件：
      - 每行的第一个元素指定匹配条件的操作符。
      - 行与行之间是 `OR` 的关系。
  
## matcher

dslx 支持下列字段的匹配：

| 名字   | 描述                                                      |
|--------|----------------------------------------------------------|
| HOST   | 请求的域名                                                |
| URL    | 请求的url                                                |
| SRC_IP | 请求的源地址，会优先使用 x_real_ip 或者 x_forwarded_for 的 header |
| METHOD | 请求的方法                                                |
| HEADER | 请求的 header，可以指定 key                                  |
| PARAM  | url 的参数，可以指定 key                                     |
| COOKIE | 请求的 cookie，可以指定 key                                  |

## op

dslx 支持下列的操作符：

| 名字         | 描述                                          |
|-------------|-----------------------------------------------|
| EQ          | 相等                                          |
| STARTS_WITH | 前缀匹配                                       |
| ENDS_WITH   | 后缀匹配                                       |
| IN          | value 在数组内                                  |
| RANGE       | values 是 IP，并且在某个 IP 段内                    |
| EXIST       | 存在这个 key，比如存在某个 header，param，cookie    |

## Example

```yaml
  dslx:
  - type: HOST
    values:
    - - EQ
      - a.com
  - type: URL
    values:
    - - STARTS_WITH
      - /a
    - - STARTS_WITH
      - /b
```

最终生成的前序表达式为 `[AND [EQ HOST a.com] [OR [STARTS_WITH URL /a] [STARTS_WITH URL /a]]`。

```yaml
  dslx:
  - type: METHOD
    values:
    - - EQ
      - POST
  - type: URL
    values:
    - - STARTS_WITH
      - /app-a
    - - STARTS_WITH
      - /app-b
  - type: PARAM
    key: group
    values:
    - - EQ
      - vip
  - type: HOST 
    values:
    - - ENDS_WITH
      - .app.com
  - type: HEADER
    key: LOCATION 
    values:
    - - IN
      - east-1
      - east-2
  - type: COOKIE
    key: uid
    values:
    - - EXIST 
  - type: SRC_IP
    values:
    - - RANGE
      - "1.1.1.1"
      - "1.1.1.100"

```

最终生成的前序表达式为 `["AND",["EQ","METHOD","POST"],["OR",["STARTS_WITH","URL","/app-a"],["STARTS_WITH","URL","/app-b"]],["EQ","PARAM","group","vip"],["ENDS_WITH","HOST",".app.com"],["IN","HEADER","location","east-1","east-2"],["EXIST","COOKIE","uid"],["RANGE","SRC_IP","1.1.1.1","1.1.1.100"]]`。