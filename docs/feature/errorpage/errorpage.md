## Error Page
### 概述
我们不希望用户能看到openresty这种体现技术栈的字样。
1. 当应用返回错误时, alb返回的body和应用返回的body一致。
	1. 没有body时，alb也不会返回body
2. 当alb自身返回错误时, alb返回的body为
```
X-Error: $status
```
### 例外
当开启了 [[modsecurity]] 之后，如果因为waf规则返回了错误码，返回的body为
```
<html>
<head>
    <title>403 Forbidden</title>
</head>
<body>
    <center>
        <h1>403 Forbidden</h1>
    </center>
    <hr>
    <center>openresty</center>
</body>
</html>
```
