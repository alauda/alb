#!/usr/bin/env python3
"""Audit ALB features before generating Gateway API YAML.

Usage:
  python3 check_features.py <input-dir> [--json]

The audit is organized by check ID. It calls out source behavior that ALB
itself does not support (for example `corsAllowOrigin: http://host:*` and
`$http_origin`), so the user can review those rules instead of treating them
as a Gateway API migration gap.
"""

from __future__ import annotations

import argparse
import collections
import json
import os
import re
import sys
from typing import Any

try:
    import yaml
except ImportError:
    sys.exit("PyYAML is required: pip install pyyaml")

SYSTEM_ALBS = {"cpaas-system", "global-alb2"}
MAX_LISTENERS = 63


def load_items(path: str) -> list[dict[str, Any]]:
    files: list[str] = []
    if os.path.isdir(path):
        for root, _, names in os.walk(path):
            for name in sorted(names):
                if name.endswith((".yaml", ".yml")):
                    files.append(os.path.join(root, name))
    else:
        files.append(path)
    items: list[dict[str, Any]] = []
    for filename in files:
        with open(filename, "r", encoding="utf-8") as fh:
            for doc in yaml.safe_load_all(fh):
                if not doc:
                    continue
                if doc.get("kind") == "List":
                    items.extend([i for i in (doc.get("items") or []) if i])
                else:
                    items.append(doc)
    return items


def alb_name(item: dict[str, Any]) -> str:
    meta = item.get("metadata") or {}
    labels = meta.get("labels") or {}
    if labels.get("alb2.cpaas.io/name"):
        return str(labels["alb2.cpaas.io/name"])
    for owner in meta.get("ownerReferences") or []:
        if owner.get("kind") == "ALB2" and owner.get("name"):
            return str(owner["name"])
    if item.get("kind") == "ALB2":
        return str(meta.get("name") or "unknown")
    return "unknown"


def split_origins(value: str | None) -> tuple[list[str], bool]:
    if not value:
        return [], False
    raw = str(value)
    fixed = False
    if re.search(r"\.(?=https?://)", raw):
        raw = re.sub(r"\.(?=https?://)", ",", raw)
        fixed = True
    return [x.strip() for x in raw.split(",") if x.strip()], fixed


def check_row(check_id: str, count: int, examples: list[str], alb: str) -> dict[str, Any]:
    return {"check_id": check_id, "count": count, "examples": examples, "alb": alb}


CHECK_DEFS: dict[str, tuple[str, str, str, str]] = {
    "ALB-INSTANCE": ("ALB 实例", "是", "Gateway + EnvoyProxy", "一个 ALB 一个 Gateway，显式引用独立 EnvoyProxy"),
    "ALB-METADATA": ("ALB 语义 annotation", "是", "Gateway annotations", "display-name/description 同名迁移；creator -> cpaas.io/origin-creator，不覆盖实际 creator"),
    "ALB-PROXY-HEADER": ("proxy_header.use_forwarded_headers", "人工确认", "无直接等价", "ALB 的 upstream X-Forwarded-* 覆盖语义需在目标 controller 验证"),
    "ALB-SYSTEM-LB": ("系统 LB", "不转换", "跳过", "只跳过 cpaas-system / global-alb2；global-alb 是用户 LB"),
    "ALB-FRONTEND": ("Frontend", "是", "Gateway listener", "一个唯一端口/协议一个 listener"),
    "ALB-INGRESS-RULE": ("Ingress 生成的 Rule", "是", "HTTPRoute", "迁移原始 Ingress，不逐条转发生成的 Rule"),
    "ALB-DIRECT-RULE": ("直接创建的 Rule", "是", "HTTPRoute / TCPRoute / UDPRoute", "L7 转 HTTPRoute；L4 转 L4 route"),
    "ALB-L4-FRONTEND": ("L4 Frontend 内嵌 serviceGroup", "是", "TCPRoute / UDPRoute", "目标版本支持，直接生成"),
    "ALB-DSLX-HOST": ("dslx HOST", "是", "HTTPRoute.hostnames", "EQ -> 精确 hostname；多 host 按 host 拆 HTTPRoute"),
    "ALB-DSLX-URL-PREFIX": ("dslx URL STARTS_WITH", "是", "PathPrefix", "PathPrefix 是路径段匹配；例如 /api 匹配 /api/...，不匹配 /apix。工具已按此生成。"),
    "ALB-DSLX-URL-EXACT": ("dslx URL EQ", "是", "Exact", "直接映射"),
    "ALB-DSLX-URL-REGEX": ("dslx URL REGEX", "是", "RegularExpression", "ALB 是 search/prefix，Envoy 是整路径匹配；工具为不结束于 $ 的正则补 .*，具体数量见审计结果。"),
    "ALB-DSLX-COOKIE": ("dslx COOKIE", "是", "Cookie header regex", "无标准 HTTPRouteMatch 字段"),
    "ALB-DSLX-SRC-IP": ("dslx SRC_IP", "是", "SecurityPolicy / 无直接路由等价", "allowlist 用 SecurityPolicy；routing 无直接等价"),
    "ALB-REWRITE-REGEX": ("regex rewrite", "是", "HTTPRouteFilter ReplaceRegexMatch", "检查整路径匹配和 RE2"),
    "ALB-HEADERS-VAR": ("headers_var / headers_add_var", "是", "EnvoyExtensionPolicy + Lua", "headers_var 用 replace；headers_add_var 用 add"),
    "ALB-CORS-STATIC": ("CORS 静态 origin", "是", "CORS filter", "只统计可生成的精确 origin；literal wildcard/bare * 单列"),
    "ALB-CORS-LITERAL-WILDCARD": ("CORS 字面量 wildcard", "否（ALB 按字面量匹配）", "不生成通配符", "避免扩大访问范围"),
    "ALB-CORS-BARE-STAR": ("CORS origin 为 *", "否（ALB 按字面量匹配）", "不生成 CORS filter", "避免 Gateway 把 * 解释为任意 Origin"),
    "ALB-CORS-PORT-WILDCARD": ("CORS 端口通配符", "否（ALB 按字面量匹配）", "CORS filter 需要明确端口", "迁移后保持现状，不需要处理；如确实需要，提供实际端口。"),
    "ALB-CORS-DYNAMIC-ORIGIN": ("CORS $http_origin", "否（ALB 不展开变量）", "无直接映射", "迁移后保持现状，不需要处理；如确实依赖，提供实际 Origin 或确认可用 *。"),
    "ALB-CORS-MISSING-COMMA": ("CORS 缺逗号", "是（数据格式问题）", "自动拆分", "工具自动拆分"),
    "ALB-TIMEOUT": ("timeout", "是", "HTTPRoute.timeouts", "connect/read/send 取非空最大值写入 HTTPRoute.timeouts；不创建 per-route policy。"),
    "ALB-REDIRECT": ("redirect", "是", "RequestRedirect 或跳过", "按实际生成结果检查：prefix_match 不满足时跳过 redirect（route 直接转发），legacy redirectURL/redirectCode 也计入；固定 query/$host 见 warnings.md"),
    "ALB-SESSION-AFFINITY": ("session affinity", "是", "sessionPersistence / BackendTrafficPolicy", "cookie/header/sip-hash 分别映射"),
    "ALB-BACKEND-TLS": ("backendProtocol https", "是", "Backend + insecureSkipVerify", "保持 ALB 默认不校验；需要 enableBackend=true"),
    "ALB-BACKEND-GRPC": ("backendProtocol grpc", "是", "GRPCRoute / appProtocol", "优先 GRPCRoute"),
    "ALB-VHOST": ("vhost", "是", "inline URLRewrite.hostname", "不要写 EG HTTPRouteFilter 的 string hostname"),
    "ALB-CERT": ("证书", "是", "listener tls.certificateRefs", "跨 namespace Secret 生成 ReferenceGrant"),
    "ALB-PROJECT-LABEL": ("无 cpaas.io/project", "是（默认规则无 project 正常）", "allowedRoutes 按 namespace 选择", "80 等默认规则无 project 正常；端口项目规则才需要 label；ALB2 缺失无法判断端口项目范围"),
    "ALB-LISTENER-LIMIT": ("listener >63", "是", "ListenerSet / 多 Gateway + 共享 VIP", "按产品文档处理"),
}


def audit(path: str) -> dict[str, Any]:
    items = load_items(path)
    albs: dict[str, dict[str, Any]] = {}
    for item in items:
        kind = item.get("kind")
        if kind not in ("ALB2", "Frontend", "Rule"):
            continue
        name = alb_name(item)
        entry = albs.setdefault(
            name,
            {
                "system": name in SYSTEM_ALBS,
                "alb2": 0,
                "frontends": 0,
                "rules": 0,
                "ports": set(),
                "ingress_rules": 0,
                "direct_rules": 0,
                "l4_frontends": 0,
                "checks": collections.Counter(),
                "examples": collections.defaultdict(list),
                "missing_project": 0,
            },
        )
        if kind == "ALB2":
            entry["alb2"] += 1
            proxy_header = ((item.get("spec") or {}).get("proxy_header") or {})
            if proxy_header.get("use_forwarded_headers"):
                entry["checks"]["ALB-PROXY-HEADER"] += 1
                entry["examples"]["ALB-PROXY-HEADER"].append((item.get("metadata") or {}).get("name") or "")
            ann = (item.get("metadata") or {}).get("annotations") or {}
            if any(
                key in ann
                for key in (
                    "cpaas.io/display-name",
                    "cpaas.io/description",
                    "cpaas.io/creator",
                    "alb2.cpaas.io/display-name",
                    "alb2.cpaas.io/description",
                    "alb2.cpaas.io/creator",
                )
            ):
                entry["checks"]["ALB-METADATA"] += 1
                entry["examples"]["ALB-METADATA"].append(((item.get("metadata") or {}).get("name") or ""))
            continue
        spec = item.get("spec") or {}
        meta = item.get("metadata") or {}
        if kind == "Frontend":
            entry["frontends"] += 1
            if ((spec.get("proxy_header") or {}).get("use_forwarded_headers")):
                entry["checks"]["ALB-PROXY-HEADER"] += 1
                entry["examples"]["ALB-PROXY-HEADER"].append((item.get("metadata") or {}).get("name") or "")
            if spec.get("port") is not None:
                entry["ports"].add((str(spec.get("protocol") or "unknown"), int(spec["port"])))
            proto = str(spec.get("protocol") or "").lower()
            if proto in ("tcp", "udp") and spec.get("serviceGroup"):
                entry["checks"]["ALB-L4-FRONTEND"] += 1
                entry["examples"]["ALB-L4-FRONTEND"].append(meta.get("name") or "")
            if spec.get("certificate_name"):
                entry["checks"]["ALB-CERT"] += 1
                entry["examples"]["ALB-CERT"].append(meta.get("name") or "")
            proto = str(spec.get("protocol") or "").lower()
            if proto in ("tcp", "udp") and (spec.get("serviceGroup") or {}).get("session_affinity_policy"):
                entry["checks"]["ALB-SESSION-AFFINITY"] += 1
                entry["examples"]["ALB-SESSION-AFFINITY"].append(
                    f"{meta.get('name')}:{(spec.get('serviceGroup') or {}).get('session_affinity_policy')}"
                )
            continue

        entry["rules"] += 1
        if ((spec.get("proxy_header") or {}).get("use_forwarded_headers")):
            entry["checks"]["ALB-PROXY-HEADER"] += 1
            entry["examples"]["ALB-PROXY-HEADER"].append((item.get("metadata") or {}).get("name") or "")
        source = spec.get("source") or {}
        if source.get("type") == "ingress":
            entry["ingress_rules"] += 1
            entry["checks"]["ALB-INGRESS-RULE"] += 1
        else:
            entry["direct_rules"] += 1
            entry["checks"]["ALB-DIRECT-RULE"] += 1

        project = (meta.get("labels") or {}).get("cpaas.io/project")
        if not project:
            entry["missing_project"] += 1
            entry["checks"]["ALB-PROJECT-LABEL"] += 1
            entry["examples"]["ALB-PROJECT-LABEL"].append(meta.get("name") or "")

        for d in spec.get("dslx") or []:
            for op in d.get("values") or []:
                if not isinstance(op, list) or not op:
                    continue
                kind_name = str(d.get("type"))
                op_name = str(op[0])
                if kind_name == "HOST":
                    entry["checks"]["ALB-DSLX-HOST"] += 1
                elif kind_name == "URL":
                    if op_name == "STARTS_WITH":
                        entry["checks"]["ALB-DSLX-URL-PREFIX"] += 1
                    elif op_name == "EQ":
                        entry["checks"]["ALB-DSLX-URL-EXACT"] += 1
                    elif op_name == "REGEX":
                        entry["checks"]["ALB-DSLX-URL-REGEX"] += 1
                elif kind_name == "COOKIE":
                    entry["checks"]["ALB-DSLX-COOKIE"] += 1
                    entry["examples"]["ALB-DSLX-COOKIE"].append(meta.get("name") or "")
                elif kind_name == "SRC_IP":
                    entry["checks"]["ALB-DSLX-SRC-IP"] += 1
                    entry["examples"]["ALB-DSLX-SRC-IP"].append(meta.get("name") or "")

        if spec.get("rewrite_target") and re.search(r"\$\d", str(spec.get("rewrite_target"))):
            entry["checks"]["ALB-REWRITE-REGEX"] += 1
            entry["examples"]["ALB-REWRITE-REGEX"].append(meta.get("name") or "")
        annotations = meta.get("annotations") or {}
        if any("headers_var" in str(v) or "headers_add_var" in str(v) for v in annotations.values()):
            entry["checks"]["ALB-HEADERS-VAR"] += 1
            entry["examples"]["ALB-HEADERS-VAR"].append(meta.get("name") or "")

        if spec.get("enableCORS"):
            origins, fixed_comma = split_origins(spec.get("corsAllowOrigin"))
            if fixed_comma:
                entry["checks"]["ALB-CORS-MISSING-COMMA"] += 1
                entry["examples"]["ALB-CORS-MISSING-COMMA"].append(meta.get("name") or "")
            if any("$" in o for o in origins):
                entry["checks"]["ALB-CORS-DYNAMIC-ORIGIN"] += 1
                entry["examples"]["ALB-CORS-DYNAMIC-ORIGIN"].append(meta.get("name") or "")
            if any(o.endswith(":*") for o in origins):
                entry["checks"]["ALB-CORS-PORT-WILDCARD"] += 1
                entry["examples"]["ALB-CORS-PORT-WILDCARD"].append(meta.get("name") or "")
            literal_wildcard = [o for o in origins if "*" in o and o != "*" and not o.endswith(":*")]
            bare_star = "*" in origins
            if literal_wildcard:
                entry["checks"]["ALB-CORS-LITERAL-WILDCARD"] += 1
                entry["examples"]["ALB-CORS-LITERAL-WILDCARD"].append(meta.get("name") or "")
            if bare_star:
                entry["checks"]["ALB-CORS-BARE-STAR"] += 1
                entry["examples"]["ALB-CORS-BARE-STAR"].append(meta.get("name") or "")
            valid_origins = [o for o in origins if "$" not in o and not o.endswith(":*") and "*" not in o]
            if valid_origins:
                entry["checks"]["ALB-CORS-STATIC"] += 1
                entry["examples"]["ALB-CORS-STATIC"].append(meta.get("name") or "")

        timeout = ((spec.get("config") or {}).get("timeout") or {})
        if timeout:
            entry["checks"]["ALB-TIMEOUT"] += 1
            entry["examples"]["ALB-TIMEOUT"].append(meta.get("name") or "")
        if (spec.get("config") or {}).get("redirect") or spec.get("redirectURL") or spec.get("redirectCode"):
            entry["checks"]["ALB-REDIRECT"] += 1
            entry["examples"]["ALB-REDIRECT"].append(meta.get("name") or "")
        affinity = ((spec.get("serviceGroup") or {}).get("session_affinity_policy") or "")
        if affinity:
            entry["checks"]["ALB-SESSION-AFFINITY"] += 1
            entry["examples"]["ALB-SESSION-AFFINITY"].append(f"{meta.get('name')}:{affinity}")
        if str(spec.get("backendProtocol") or "").lower() == "https":
            entry["checks"]["ALB-BACKEND-TLS"] += 1
            entry["examples"]["ALB-BACKEND-TLS"].append(meta.get("name") or "")
        if str(spec.get("backendProtocol") or "").lower() == "grpc":
            entry["checks"]["ALB-BACKEND-GRPC"] += 1
            entry["examples"]["ALB-BACKEND-GRPC"].append(meta.get("name") or "")
        if spec.get("vhost"):
            entry["checks"]["ALB-VHOST"] += 1
            entry["examples"]["ALB-VHOST"].append(meta.get("name") or "")
        if spec.get("certificate_name"):
            entry["checks"]["ALB-CERT"] += 1
            entry["examples"]["ALB-CERT"].append(meta.get("name") or "")

    for entry in albs.values():
        if len(entry["ports"]) > MAX_LISTENERS:
            entry["checks"]["ALB-LISTENER-LIMIT"] += 1
            entry["examples"]["ALB-LISTENER-LIMIT"].append(f"{len(entry['ports'])} ports")

    return {"path": path, "albs": albs}


def to_serializable(data: dict[str, Any]) -> dict[str, Any]:
    out = {"path": data["path"], "albs": []}
    for name, entry in sorted(data["albs"].items()):
        out["albs"].append(
            {
                "name": name,
                "system": entry["system"],
                "alb2": entry["alb2"],
                "frontends": entry["frontends"],
                "rules": entry["rules"],
                "ports": sorted(entry["ports"]),
                "ingress_rules": entry["ingress_rules"],
                "direct_rules": entry["direct_rules"],
                "l4_frontends": entry["l4_frontends"],
                "missing_project": entry["missing_project"],
                "checks": dict(entry["checks"]),
                "examples": {k: v[:20] for k, v in entry["examples"].items()},
            }
        )
    return out


def print_markdown(data: dict[str, Any]) -> None:
    print(f"# ALB 功能检查：`{data['path']}`")
    print()
    print("> 技术附录：一般客户不需要逐条阅读；重点看“重点发现”。")
    print()
    print("## ALB 清单")
    print()
    print("| ALB | 系统 LB | Frontend | Rule | 端口数 | Ingress Rule | Direct Rule |")
    print("| --- | --- | ---: | ---: | ---: | ---: | ---: |")
    for name, entry in sorted(data["albs"].items()):
        print(
            f"| {name} | {'是' if entry['system'] else '否'} | {entry['frontends']} | {entry['rules']} | "
            f"{len(entry['ports'])} | {entry['ingress_rules']} | {entry['direct_rules']} |"
        )
    print()

    print("## 检查项")
    print()
    print("| Check ID | 条数 | ALB | 示例 | ALB 支持 | Gateway API 目标 | 建议 |")
    print("| --- | ---: | --- | --- | --- | --- | --- |")
    for name, entry in sorted(data["albs"].items()):
        if entry["system"]:
            continue
        for check_id, count in sorted(entry["checks"].items()):
            label, alb_support, target, action = CHECK_DEFS.get(check_id, (check_id, "?", "?", "?"))
            examples = ", ".join(entry["examples"].get(check_id, [])[:3])
            print(f"| {check_id} | {count} | {name} | {examples} | {alb_support} | {target} | {action} |")
    print()

    print("## 重点发现")
    print()
    found = False
    for name, entry in sorted(data["albs"].items()):
        if entry["system"]:
            continue
        for check_id in ("ALB-CORS-PORT-WILDCARD", "ALB-CORS-DYNAMIC-ORIGIN"):
            count = entry["checks"].get(check_id, 0)
            if not count:
                continue
            found = True
            label, alb_support, target, action = CHECK_DEFS[check_id]
            print(f"- **{check_id}**：`{name}` 有 {count} 条 Rule，{action} ALB 支持：{alb_support}。")
            for example in entry["examples"].get(check_id, []):
                print(f"  - {example}")
    if not found:
        print("- 没有发现 ALB 自己不支持的 CORS 配置。")
    print()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", help="Input directory or YAML file")
    parser.add_argument("--json", action="store_true", help="Print JSON instead of Markdown")
    args = parser.parse_args()
    if not os.path.exists(args.path):
        sys.exit(f"input path does not exist: {args.path}")
    data = audit(args.path)
    if args.json:
        print(json.dumps(to_serializable(data), indent=2, ensure_ascii=False))
    else:
        print_markdown(data)


if __name__ == "__main__":
    main()
