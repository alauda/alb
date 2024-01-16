# dslx
dslx的格式为
```yaml
  - type: string # matcher
    key: string (optional)
    values:
     - - string # op 1
       - string # conditions
     - - string # op 2
       - string # condition
```
- type指定具体要匹配什么，目前有 HOST URL SRC_IP METHOD HEADER PARAM COOKIE 这几个matcher
- key指定额外的匹配条件，比如匹配HEADER时，具体是哪一个HEADER.
   - 当type是HEADER PARAM COOKIE 时，可以指定具体的KEY  
- 通过type和key我们可以明确要匹配的是什么，通过values指定匹配条件  
    - values是一个表格，每行表示一个匹配条件
      - 每行的第一个元素指定匹配条件的操作符
      - 行与行之间是OR的关系  
## matcher
| HOST   | 请求的域名                                                   |
|--------|--------------------------------------------------------------|
| URL    | 请求的url                                                    |
| SRC_IP | 请求的源地址，会优先使用x_real_ip或者x_forwarded_for的header |
| METHOD | 请求的方法                                                   |
| HEADER | 请求的header，可以指定key                                    |
| PARAM  | url的参数，可以指定key                                       |
| COOKIE | 请求的cookie，可以指定key                                    |
## conditions
| conditions  |                                                |   |
|-------------|------------------------------------------------|---|
| EQ          | 相等                                           |   |
| STARTS_WITH | 前缀匹配                                       |   |
| ENDS_WITH   | 后缀匹配                                       |   |
| IN          | value在数组内                                  |   |
| RANGE       | values是ip，并且在某个ip段内                   |   |
| EXIST       | 存在这个key，比如存在某个header，param，cookie |   |
## example
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
为`[AND [EQ HOST a.com] [OR [STARTS_WITH URL /a] [STARTS_WITH URL /a]]`

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
为`["AND",["EQ","METHOD","POST"],["OR",["STARTS_WITH","URL","/app-a"],["STARTS_WITH","URL","/app-b"]],["EQ","PARAM","group","vip"],["ENDS_WITH","HOST",".app.com"],["IN","HEADER","location","east-1","east-2"],["EXIST","COOKIE","uid"],["RANGE","SRC_IP","1.1.1.1","1.1.1.100"]]`