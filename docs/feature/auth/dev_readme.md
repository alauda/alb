# ALB Auth 实现说明

ALB 在实现 auth 相关 annotation 时采用了不同于 ingress-nginx 的实现方式:

- ingress-nginx 是基于模板修改 nginx.conf 实现
- ALB 是基于 OpenResty 的 Lua 实现

这种实现方式带来以下优势:
1. 支持更复杂的认证功能
2. 认证配置变更时无需重载 nginx

虽然实现方式不同,但 ALB 会保持与 ingress-nginx 的行为一致性:
- 相同的 annotation 在 ingress-nginx 和 ALB 上效果一致,无需改动.

为了保证行为一致性,我们使用 ./test/conformance/ingress-nginx 中的测试用例来验证。这套测试用例可以同时在 ingress-nginx 和 ALB 上运行,确保功能的一致性。

# 代码
go 部分代码在 ./pkg/controller/ext/auth/auth.go
lua 部分代码在 ./template/nginx/lua/plugins/auth/auth.lua