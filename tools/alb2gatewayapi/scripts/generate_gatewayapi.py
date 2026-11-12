#!/usr/bin/env python3
"""Generate a first-pass Gateway API YAML from exported ALB2/Frontend/Rule YAML.

Usage:
  python3 generate_gatewayapi.py <input-dir> <output-dir> [--gateway-namespace NS]

This is deliberately a first-pass generator. It covers the common headers,
rewrites, CORS, timeout, redirect, backend TLS, session affinity and L4 cases,
and writes every uncertain item into the report's review list. It does not
apply anything to a cluster.
"""

from __future__ import annotations

import argparse
import collections
import hashlib
import io
import itertools
import json
import os
import re
import shutil
import sys
from contextlib import redirect_stdout
from typing import Any
from urllib.parse import urlparse

try:
    import yaml
except ImportError:
    sys.exit("PyYAML is required: pip install pyyaml")

SYSTEM_ALBS = {"cpaas-system", "global-alb2"}
DEFAULT_GATEWAY_NS = "gateway-user-default"
MAX_LISTENERS = 63
MAX_RULES_PER_ROUTE = 16
MAX_MATCHES_PER_RULE = 8


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


def sanitize(value: str, limit: int = 50) -> str:
    value = re.sub(r"[^a-z0-9-]+", "-", value.lower()).strip("-")
    if not value:
        value = "x"
    return value[:limit].strip("-")


def short_hash(value: str) -> str:
    return hashlib.sha1(value.encode()).hexdigest()[:8]


def parse_cert(value: str | None) -> tuple[str | None, str | None]:
    if not value:
        return None, None
    value = value.strip()
    if "/" in value:
        ns, secret = value.split("/", 1)
        return ns, secret
    if "_" in value:
        ns, secret = value.split("_", 1)
        return ns, secret
    return None, value


def ms(value: Any) -> str | None:
    if value is None or value == "":
        return None
    try:
        total_ms = int(value)
    except (TypeError, ValueError):
        return None
    if total_ms < 1000:
        return f"{total_ms}ms"
    total_s = total_ms // 1000
    if total_s <= 99999:
        return f"{total_s}s"
    total_m = total_ms // 60000
    if total_m <= 99999:
        return f"{total_m}m"
    total_h = total_ms // 3600000
    return f"{total_h}h"


def min_ms(*values: Any) -> int | None:
    vals = [int(v) for v in values if v is not None]
    return min(vals) if vals else None


def split_csv(value: str | None) -> list[str]:
    if not value:
        return []
    out: list[str] = []
    seen: set[str] = set()
    for item in str(value).split(","):
        item = item.strip()
        if not item:
            continue
        key = item.lower()
        if key in seen:
            continue
        seen.add(key)
        out.append(item)
    return out


def normalize_origins(value: str | None) -> tuple[list[str], bool]:
    """Split comma-separated origins and fix the common missing-comma typo.

    Example: ``http://*.example.com.https://*.example.com`` becomes two origins.
    """
    if not value:
        return [], False
    raw = str(value)
    fixed = False
    if re.search(r"\.(?=https?://)", raw):
        raw = re.sub(r"\.(?=https?://)", ",", raw)
        fixed = True
    out: list[str] = []
    seen: set[str] = set()
    for item in raw.split(","):
        item = item.strip()
        if not item:
            continue
        key = item.lower()
        if key in seen:
            continue
        seen.add(key)
        out.append(item)
    return out, fixed


def dslx(spec: dict[str, Any], kind: str) -> list[tuple[str, str, str | None]]:
    out: list[tuple[str, str, str | None]] = []
    for d in spec.get("dslx") or []:
        if d.get("type") != kind:
            continue
        key = d.get("key")
        for op in d.get("values") or []:
            if isinstance(op, list) and len(op) >= 2:
                out.append((str(op[0]), str(op[1]), key))
    return out


def hosts_for_rule(spec: dict[str, Any]) -> set[str]:
    hosts: set[str] = set()
    if spec.get("domain"):
        hosts.add(str(spec["domain"]))
    for op, value, _ in dslx(spec, "HOST"):
        if op in ("EQ", "IN") and value:
            hosts.add(value)
    return hosts


def path_match(op: str, value: str) -> dict[str, Any]:
    if op == "STARTS_WITH":
        if re.search(r"[()\[\]{}?+*|^$\\]", value):
            pattern = value if value.startswith("^") else "^" + value
            return {"path": {"type": "RegularExpression", "value": pattern}}
        return {"path": {"type": "PathPrefix", "value": value}}
    if op == "EQ":
        if re.search(r"[()\[\]{}?+*|^$\\]", value):
            pattern = "^" + re.escape(value) + "$"
            return {"path": {"type": "RegularExpression", "value": pattern}}
        return {"path": {"type": "Exact", "value": value}}
    if op == "REGEX":
        regex = value
        if not regex.endswith("$") and not regex.endswith(".*"):
            regex = regex + ".*"
        return {"path": {"type": "RegularExpression", "value": regex}}
    return {"path": {"type": "PathPrefix", "value": value}}


def matches_for_rule(spec: dict[str, Any], warnings: list[str], label: str) -> list[dict[str, Any]]:
    urls = dslx(spec, "URL")
    methods = list(dict.fromkeys(v for op, v, _ in dslx(spec, "METHOD") if op == "EQ"))
    headers = dslx(spec, "HEADER")
    params = dslx(spec, "PARAM")
    cookies = dslx(spec, "COOKIE")
    src_ips = dslx(spec, "SRC_IP")
    if cookies:
        warnings.append(f"{label}：COOKIE 匹配未转换；请用 Cookie header 正则或 Lua policy。")
    if src_ips:
        warnings.append(f"{label}：SRC_IP 匹配未转换；SecurityPolicy 是访问控制，不是路由。")

    def value_groups(items: list[tuple[str, str, str | None]], kind: str) -> list[list[dict[str, Any]]]:
        """Group values by key; each key is an AND dimension with OR alternatives.

        Gateway API headers/queryParams are list-map fields keyed by ``name``.
        Emitting two values for the same key in one match creates a duplicate
        map key, so alternatives must become separate matches.
        """
        grouped: dict[str, list[dict[str, Any]]] = {}
        order: list[str] = []
        unsupported: list[str] = []
        seen: dict[str, set[tuple[str, str]]] = {}
        for op, value, key in items:
            if not key:
                continue
            if op == "EQ":
                match_type = "Exact"
            elif op == "REGEX":
                match_type = "RegularExpression"
                value = str(value)
                if "|" in value:
                    value = f"(?:{value})"
                if not value.startswith("^"):
                    value = ".*" + value
                if not value.endswith("$") and not value.endswith(".*"):
                    value = value + ".*"
            else:
                unsupported.append(f"{key} {op}")
                continue
            option = {"name": key, "value": str(value), "type": match_type}
            marker = (match_type, str(value))
            if key not in grouped:
                grouped[key] = []
                order.append(key)
                seen[key] = set()
            if marker in seen[key]:
                continue
            seen[key].add(marker)
            grouped[key].append(option)
        if unsupported:
            warnings.append(
                f"{label}：{kind} 操作符未转换（{', '.join(unsupported)}）；已忽略这些条件。"
            )
        return [grouped[key] for key in order if grouped[key]]

    bases: list[dict[str, Any]] = []
    if urls:
        for op, value, _ in urls:
            bases.append(path_match(op, value))
    else:
        bases.append({"path": {"type": "PathPrefix", "value": "/"}})

    # Each group represents alternatives for one key; product makes one
    # match per combination, never two entries with the same name.
    param_groups = value_groups(params, "PARAM")
    header_groups = value_groups(headers, "HEADER")
    conditions = [("query", group) for group in param_groups] + [("header", group) for group in header_groups]
    matches: list[dict[str, Any]] = []
    for base in bases:
        for method in (methods or [None]):
            combinations = itertools.product(*[group for _, group in conditions]) if conditions else [()]
            for combo in combinations:
                match = dict(base)
                if method:
                    match["method"] = method
                query_params = [dict(option) for (kind, _), option in zip(conditions, combo) if kind == "query"]
                header_matches = [dict(option) for (kind, _), option in zip(conditions, combo) if kind == "header"]
                if query_params:
                    match["queryParams"] = query_params
                if header_matches:
                    match["headers"] = header_matches
                matches.append(match)
    if len(matches) > MAX_MATCHES_PER_RULE:
        dropped = []
        for match in matches[MAX_MATCHES_PER_RULE:]:
            path = str((match.get("path") or {}).get("value") or "?")
            method = str(match.get("method") or "")
            dropped.append(f"{method} {path}".strip())
        warnings.append(
            f"{label}：一条 Rule 有 {len(matches)} 个 match，已截断到 {MAX_MATCHES_PER_RULE}；"
            f"丢弃 match: {', '.join(dropped)}；需要拆分源 Rule。"
        )
        matches = matches[:MAX_MATCHES_PER_RULE]
    return matches

DOC_COMMENTS: dict[int, str] = {}


def add_doc(docs: list[dict[str, Any]], doc: dict[str, Any], comment: str | None = None) -> None:
    docs.append(doc)
    if comment:
        DOC_COMMENTS[id(doc)] = comment


def write_docs(path: str, docs: list[dict[str, Any]]) -> None:
    with open(path, "w", encoding="utf-8") as fh:
        for index, doc in enumerate(docs):
            if index:
                fh.write("---\n")
            comment = DOC_COMMENTS.get(id(doc))
            if comment:
                fh.write(comment.rstrip() + "\n")
            fh.write(yaml.safe_dump(doc, sort_keys=False))


ROUTE_KINDS = {"HTTPRoute", "TCPRoute", "UDPRoute", "GRPCRoute", "TLSRoute"}


def doc_identity(doc: dict[str, Any]) -> tuple[str, str, str]:
    meta = doc.get("metadata") or {}
    return (str(doc.get("kind") or ""), str(meta.get("namespace") or "default"), str(meta.get("name") or ""))


def route_refs(
    doc: dict[str, Any]
) -> tuple[set[tuple[str, str, str]], set[tuple[str, str, str]], set[tuple[str, str, str]]]:
    ns = doc_identity(doc)[1]
    filters: set[tuple[str, str, str]] = set()
    services: set[tuple[str, str, str]] = set()
    backends: set[tuple[str, str, str]] = set()
    for rule in ((doc.get("spec") or {}).get("rules") or []):
        for item in rule.get("filters") or []:
            ref = item.get("extensionRef") or {}
            if ref.get("kind") == "HTTPRouteFilter" and ref.get("name"):
                filters.add(("HTTPRouteFilter", str(ref.get("namespace") or ns), str(ref["name"])))
        for backend in rule.get("backendRefs") or []:
            if not backend.get("name"):
                continue
            ref_ns = str(backend.get("namespace") or ns)
            if backend.get("kind") == "Backend":
                backends.add(("Backend", ref_ns, str(backend["name"])))
            else:
                services.add(("Service", ref_ns, str(backend["name"])))
    return filters, services, backends


def split_alb_docs(
    docs: list[dict[str, Any]], alb_ns: str, alb: str
) -> tuple[list[dict[str, Any]], dict[tuple[str, str, str], list[dict[str, Any]]], list[dict[str, Any]]]:
    """Split one ALB's docs into gateway.yaml, one file per Route, and shared.yaml.

    A Route file contains the Route plus policies/filters that only target that
    Route. Policies shared by multiple Routes, backend-level policies and
    ReferenceGrants go to shared.yaml so applying one Route file does not
    accidentally delete another Route's policy.
    """
    route_docs = [d for d in docs if d.get("kind") in ROUTE_KINDS]
    route_by_key = {doc_identity(d): d for d in route_docs}
    filter_routes: dict[tuple[str, str, str], set[tuple[str, str, str]]] = collections.defaultdict(set)
    service_routes: dict[tuple[str, str, str], set[tuple[str, str, str]]] = collections.defaultdict(set)
    backend_routes: dict[tuple[str, str, str], set[tuple[str, str, str]]] = collections.defaultdict(set)
    for doc in route_docs:
        rk = doc_identity(doc)
        filters, services, backends = route_refs(doc)
        for f in filters:
            filter_routes[f].add(rk)
        for svc in services:
            service_routes[svc].add(rk)
        for backend in backends:
            backend_routes[backend].add(rk)

    gateway_docs: list[dict[str, Any]] = []
    route_groups: dict[tuple[str, str, str], list[dict[str, Any]]] = {rk: [d] for rk, d in route_by_key.items()}
    shared_docs: list[dict[str, Any]] = []

    for doc in docs:
        kind = str(doc.get("kind") or "")
        if kind in ("Namespace", "EnvoyProxy", "Gateway"):
            gateway_docs.append(doc)
            continue
        if kind in ROUTE_KINDS:
            continue

        doc_ns = doc_identity(doc)[1]
        targets: set[tuple[str, str, str]] = set()
        to_gateway = False
        spec = doc.get("spec") or {}
        if kind == "HTTPRouteFilter":
            targets |= filter_routes.get(doc_identity(doc), set())
        elif kind == "Backend":
            targets |= backend_routes.get(doc_identity(doc), set())
        elif kind == "BackendTLSPolicy":
            for target in spec.get("targetRefs") or []:
                if target.get("kind") == "Service" and target.get("name"):
                    key = ("Service", str(target.get("namespace") or doc_ns), str(target["name"]))
                    targets |= service_routes.get(key, set())
        elif kind in ("BackendTrafficPolicy", "ClientTrafficPolicy"):
            for target in spec.get("targetRefs") or []:
                if target.get("kind") == "Gateway":
                    to_gateway = True
                    break
                if target.get("kind") in ROUTE_KINDS and target.get("name"):
                    targets.add((str(target["kind"]), str(target.get("namespace") or doc_ns), str(target["name"])))

        if to_gateway:
            gateway_docs.append(doc)
            continue
        if len(targets) == 1:
            rk = next(iter(targets))
            if rk in route_groups:
                route_groups[rk].append(doc)
                comment = DOC_COMMENTS.get(id(doc)) or f"# ALB: {alb_ns}/{alb}"
                if "Related route:" not in comment:
                    DOC_COMMENTS[id(doc)] = comment + f"; Related route: {rk[0]}/{rk[2]}"
                continue
        shared_docs.append(doc)

    return gateway_docs, route_groups, shared_docs


def route_listener_group(doc: dict[str, Any]) -> str:
    """Return the routes/ subdirectory for a Route.

    Single-listener HTTP/HTTPS routes use the port (80, 443, ...); L4 routes
    use tcp-<port>/udp-<port>; a Route attached to more than one listener, or
    to all listeners (no sectionName), goes to multi-listener/.
    """
    sections = sorted(
        {
            str(ref.get("sectionName"))
            for ref in ((doc.get("spec") or {}).get("parentRefs") or [])
            if ref.get("sectionName")
        }
    )
    if len(sections) != 1:
        return "multi-listener"
    section = sections[0]
    if "-" in section:
        proto, port = section.rsplit("-", 1)
        if proto in ("http", "https") and port.isdigit():
            return port
        return sanitize(section, 40)
    return sanitize(section, 40)


def route_filename(rk: tuple[str, str, str]) -> str:
    kind, ns, name = rk
    return f"{kind.lower()}-{sanitize(ns, 20)}-{sanitize(name, 80)}.yaml"


def add_sip_hash_policy(
    docs: list[dict[str, Any]],
    name: str,
    namespace: str,
    target_kind: str,
    target_name: str,
    comment: str | None = None,
) -> None:
    add_doc(
        docs,
        {
            "apiVersion": "gateway.envoyproxy.io/v1alpha1",
            "kind": "BackendTrafficPolicy",
            "metadata": {"name": name, "namespace": namespace},
            "spec": {
                "targetRefs": [
                    {"group": "gateway.networking.k8s.io", "kind": target_kind, "name": target_name}
                ],
                "loadBalancer": {"type": "ConsistentHash", "consistentHash": {"type": "SourceIP"}},
            },
        },
        comment=comment,
    )


def make_https_backend(
    docs: list[dict[str, Any]],
    name_hint: str,
    svc: dict[str, Any],
    route_ns: str,
    alb_ns: str,
    alb: str,
    seen: set[tuple[str, str]],
) -> dict[str, Any]:
    """Create (or reuse) an Envoy Gateway Backend for an HTTPS Service.

    ALB's default nginx behavior does not verify the backend certificate.
    Standard Gateway API BackendTLSPolicy always validates, so use the
    Envoy Gateway Backend API with insecureSkipVerify instead of guessing a
    certificate hostname. Backend API requires enableBackend on EnvoyGatewayCtl.
    """
    svc_ns = str(svc.get("namespace") or route_ns)
    svc_name = str(svc.get("name") or "backend")
    port = svc.get("port")
    backend_name = sanitize(f"{name_hint}-{svc_name}-tls", 80)
    key = (route_ns, backend_name)
    if key not in seen:
        seen.add(key)
        add_doc(
            docs,
            {
                "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                "kind": "Backend",
                "metadata": {"name": backend_name, "namespace": route_ns},
                "spec": {
                    "endpoints": [
                        {
                            "fqdn": {
                                "hostname": f"{svc_name}.{svc_ns}.svc.cluster.local",
                                "port": int(port) if port is not None else 443,
                            }
                        }
                    ],
                    "tls": {"insecureSkipVerify": True},
                },
            },
            comment=(
                f"# ALB: {alb_ns}/{alb}; HTTPS backend {svc_ns}/{svc_name}:{port}; "
                "insecureSkipVerify (ALB default does not verify)"
            ),
        )
    return {
        "group": "gateway.envoyproxy.io",
        "kind": "Backend",
        "name": backend_name,
    }


def make_reference_grant(
    docs: list[dict[str, Any]],
    seen: set[tuple[str, str, str, str]],
    target_ns: str,
    from_ns: str,
    from_kind: str,
    to_kind: str,
) -> None:
    key = (target_ns, from_ns, from_kind, to_kind)
    if key in seen or target_ns == from_ns:
        return
    seen.add(key)
    add_doc(
        docs,
        {
            "apiVersion": "gateway.networking.k8s.io/v1beta1",
            "kind": "ReferenceGrant",
            "metadata": {
                "name": f"allow-{sanitize(from_ns)}-{sanitize(from_kind.lower())}",
                "namespace": target_ns,
            },
            "spec": {
                "from": [
                    {
                        "group": "gateway.networking.k8s.io",
                        "kind": from_kind,
                        "namespace": from_ns,
                    }
                ],
                "to": [{"group": "", "kind": to_kind}],
            },
        },
    )


def compute_feature_stats(frontends: dict[str, dict[str, Any]], rules: list[dict[str, Any]]) -> collections.Counter:
    stats: collections.Counter = collections.Counter()
    for fe in frontends.values():
        spec = fe.get("spec") or {}
        proto = str(spec.get("protocol") or "").lower()
        if spec.get("certificate_name"):
            stats["CertificateFrontend"] += 1
        if proto in ("tcp", "udp") and spec.get("serviceGroup"):
            stats["L4Frontend"] += 1
        if proto in ("tcp", "udp") and (spec.get("serviceGroup") or {}).get("session_affinity_policy"):
            stats["SessionAffinity"] += 1
    for rule in rules:
        spec = rule.get("spec") or {}
        src = spec.get("source") or {}
        stats["IngressRule" if src.get("type") == "ingress" else "DirectRule"] += 1
        for d in spec.get("dslx") or []:
            for op in d.get("values") or []:
                if isinstance(op, list) and op:
                    stats[f"{d.get('type')}_{op[0]}"] += 1
        if spec.get("rewrite_target"):
            stats["Rewrite"] += 1
        if spec.get("vhost"):
            stats["Vhost"] += 1
        if spec.get("enableCORS"):
            stats["CORS"] += 1
        if (spec.get("config") or {}).get("timeout"):
            stats["Timeout"] += 1
        if spec.get("certificate_name"):
            stats["CertificateRule"] += 1
        bp = str(spec.get("backendProtocol") or "").lower()
        if bp == "https":
            stats["BackendTLS"] += 1
        if bp == "grpc":
            stats["BackendGRPC"] += 1
        if (spec.get("serviceGroup") or {}).get("session_affinity_policy"):
            stats["SessionAffinity"] += 1
    return stats


def classify_warning(warning: str) -> str:
    if "位于系统 namespace" in warning:
        return "证书位于系统 namespace"
    if "$http_origin" in warning:
        return "CORS 动态 origin"
    if "端口通配符" in warning:
        return "CORS 端口通配符"
    if "缺少逗号" in warning:
        return "CORS 自动修正"
    if "全部无法直接映射" in warning:
        return "CORS 其他"
    if "CORS" in warning:
        return "CORS"
    if "read timeout" in warning or "connect/send" in warning or "proxy_connect_timeout_ms" in warning:
        return "超时"
    if "RequestRedirect" in warning or "redirect URL" in warning:
        return "Redirect"
    if "条 rule" in warning or "HTTPRoute" in warning:
        return "HTTPRoute 拆分"
    if "后端分布在" in warning or "路由分布在" in warning:
        return "namespace 与 allowedRoutes"
    if "BackendTLSPolicy" in warning:
        return "后端 TLS"
    if "sip-hash" in warning:
        return "会话保持"
    return "其他"


CATEGORY_INFO: dict[str, tuple[str, str]] = {
    "CORS": (
        "目标能力限制",
        "CORS origin 需要人工确认。",
    ),
    "证书位于系统 namespace": (
        "已生成，需客户决定",
        "证书 Secret 存放在 `cpaas-system` 等系统 namespace，工具已按跨 namespace Secret 生成 ReferenceGrant 保证能跑通。更干净的做法是把证书复制到 Gateway 所在 namespace，改用本地 `certificateRefs`，然后删除这条 ReferenceGrant。",
    ),
    "CORS 动态 origin": (
        "源配置无效，ALB 也不支持",
        "ALB 的 `cors_allow_origin` 按字面量匹配，不会把 `$http_origin` 当变量展开；这些 origin 在 ALB 中也不生效。迁移后行为与现在一致，不需要处理；如果确实依赖动态 Origin，请提供实际 Origin 或确认可用 `*`。",
    ),
    "CORS 端口通配符": (
        "源配置无效，ALB 也不支持",
        "ALB 的 `cors_allow_origin` 按字面量匹配，不处理 `:*` 端口通配符；这些 origin 在 ALB 中也不会匹配真实 Origin。迁移后行为与现在一致，不需要处理；如果确实需要这些地址，请提供实际端口。",
    ),
    "CORS 自动修正": (
        "已自动修正",
        "ALB 的 origin 字符串缺少逗号（例如一个 origin 后面直接跟另一个 `https://`），工具已自动拆成多个 origin；具体数量见报告。",
    ),
    "CORS 其他": (
        "需人工确认",
        "没有可用的静态 origin，未生成 CORS filter。",
    ),
    "超时": (
        "已生成，无需处理",
        "connect/read/send 取非空最大值写入 `HTTPRoute.spec.rules[].timeouts.request/backendRequest`；不创建 per-route policy。",
    ),
    "Redirect": (
        "已生成，无需处理",
        "`RequestRedirect` 已生成；Gateway API 默认保留原始 path/query，`$request_uri` 不再作为字面量写入 Location。",
    ),
    "HTTPRoute 拆分": (
        "自动处理",
        "超过 16 条 rule 的 Ingress 自动拆成多个 HTTPRoute；HTTPRoute 内部按匹配具体程度排序，一般不需要人工调整。",
    ),
    "namespace 与 allowedRoutes": (
        "需客户确认",
        "当前 `allowedRoutes: All` 允许集群里任何 namespace 的路由挂到这些公网 listener；上线前建议收紧为 `Selector`。确认 namespace label、补齐缺失的 `cpaas.io/project` 后收紧。",
    ),
    "会话保持": (
        "已生成，无需处理",
        "`sip-hash` -> `BackendTrafficPolicy` ConsistentHash SourceIP，HTTPRoute/TCPRoute/UDPRoute 都支持；cookie -> `sessionPersistence`；header -> ConsistentHash Header。",
    ),
    "后端 TLS": (
        "已生成，需启用 Backend API",
        "`backendProtocol: https` 用 Envoy Gateway `Backend` + `tls.insecureSkipVerify: true` 保持 ALB 默认不校验；需要 `EnvoyGatewayCtl.spec.config.envoyGateway.extensionApis.enableBackend: true`。",
    ),
    "其他": ("待确认", "需要逐条检查。"),
}


ALB_ANNOTATION_MAP = [
    ("cpaas.io/display-name", "cpaas.io/display-name"),
    ("alb2.cpaas.io/display-name", "cpaas.io/display-name"),
    ("cpaas.io/description", "cpaas.io/description"),
    ("alb2.cpaas.io/description", "cpaas.io/description"),
    ("cpaas.io/creator", "cpaas.io/origin-creator"),
    ("alb2.cpaas.io/creator", "cpaas.io/origin-creator"),
]


def migrate_alb_annotations(data: dict[str, Any], alb: str) -> tuple[dict[str, str], str]:
    """Map semantic ALB2 annotations to Gateway annotations.

    display-name/description keep the platform key; creator is written to
    cpaas.io/origin-creator so it does not overwrite the actual creator of the
    new Gateway. If ALB2 is missing, creator is inferred only when every
    Frontend/Rule agrees on cpaas.io/creator.
    """
    alb2 = data.get("alb2") or {}
    source = ((alb2.get("metadata") or {}).get("annotations") or {})
    migrated: dict[str, str] = {}
    for source_key, target_key in ALB_ANNOTATION_MAP:
        value = source.get(source_key)
        if value not in (None, ""):
            migrated.setdefault(target_key, str(value))
    if migrated.get("cpaas.io/origin-creator"):
        return migrated, "ALB2"

    creators = set()
    for item in list(data.get("frontends", {}).values()) + list(data.get("rules", [])):
        value = ((item.get("metadata") or {}).get("annotations") or {}).get("cpaas.io/creator")
        if value not in (None, ""):
            creators.add(str(value))
    if len(creators) == 1:
        migrated["cpaas.io/origin-creator"] = next(iter(creators))
        return migrated, "Frontend/Rule"
    if len(creators) > 1:
        return migrated, "ambiguous"
    return migrated, ""


def build_redirect_filter(
    spec: dict[str, Any], label: str, warnings: list[str], matches: list[dict[str, Any]]
) -> dict[str, Any] | None:
    """Map ALB config.redirect + legacy redirectURL/redirectCode to RequestRedirect."""
    cfg = dict((spec.get("config") or {}).get("redirect") or {})
    legacy_url = spec.get("redirectURL")
    legacy_code = spec.get("redirectCode")
    if legacy_url:
        cfg["url"] = legacy_url
    if legacy_code:
        cfg["code"] = legacy_code
    if not cfg:
        return None

    url = str(cfg.get("url") or "")
    code = int(cfg.get("code") or 302)
    scheme = str(cfg.get("scheme") or "")
    host = str(cfg.get("host") or "")
    port = cfg.get("port")
    prefix_match = str(cfg.get("prefix_match") or "")
    replace_prefix = str(cfg.get("replace_prefix") or "")
    has_request_uri = "$request_uri" in url
    target = url.replace("$request_uri", "").strip()
    parsed = urlparse(target) if target else None
    if parsed and parsed.scheme:
        scheme = scheme or parsed.scheme
    if parsed and parsed.hostname:
        host = host or parsed.hostname
    if parsed and parsed.port:
        port = port or parsed.port

    rr: dict[str, Any] = {"statusCode": code}
    if scheme:
        rr["scheme"] = scheme
    if host:
        if "$" in host:
            warnings.append(f"{label}：redirect host 含 nginx 变量 {host!r}；已省略 hostname，保留请求 Host。")
        else:
            host_value = host.lower().strip(".")
            if len(host_value) > 253 or not re.fullmatch(
                r"[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*", host_value
            ):
                warnings.append(f"{label}：redirect host {host!r} 不是合法 PreciseHostname；已省略 hostname。")
            else:
                rr["hostname"] = host_value
    if port:
        rr["port"] = int(port)

    fixed_path = parsed.path if parsed and parsed.path not in ("", "/") else ""
    fixed_query = parsed.query if parsed else ""
    if prefix_match:
        path_matches = [m for m in matches if (m.get("path") or {}).get("type") == "PathPrefix"]
        if (
            len(matches) != 1
            or len(path_matches) != 1
            or (path_matches[0].get("path") or {}).get("value") != prefix_match
        ):
            warnings.append(
                f"{label}：redirect 使用 prefix_match，但规则不是唯一的 PathPrefix 且值等于 prefix_match；"
                "Gateway API 的 ReplacePrefixMatch CEL 会拒绝该 Route，已跳过 redirect。"
            )
            return None
        rr["path"] = {"type": "ReplacePrefixMatch", "replacePrefixMatch": replace_prefix}
    elif fixed_path:
        if has_request_uri:
            warnings.append(f"{label}：redirect URL 同时包含固定 path 和 $request_uri；按固定 path 生成。")
        if fixed_query:
            warnings.append(
                f"{label}：redirect URL 含固定 query {fixed_query!r}；"
                "Gateway API RequestRedirect 不能替换 query，已忽略固定 query，保留请求原 query。"
            )
        rr["path"] = {"type": "ReplaceFullPath", "replaceFullPath": fixed_path}
    elif fixed_query:
        warnings.append(
            f"{label}：redirect URL 含固定 query {fixed_query!r}；"
            "Gateway API RequestRedirect 不能替换 query，已忽略固定 query，保留请求原 query。"
        )
        rr["path"] = {"type": "ReplaceFullPath", "replaceFullPath": "/"}
    elif url and not has_request_uri:
        # ALB passes a bare URL through verbatim; RequestRedirect otherwise keeps the request path.
        rr["path"] = {"type": "ReplaceFullPath", "replaceFullPath": "/"}

    leftover_vars = sorted(set(re.findall(r"\$[A-Za-z_][A-Za-z0-9_]*", target)))
    if leftover_vars:
        warnings.append(
            f"{label}：redirect URL 还包含 nginx 变量 {', '.join(leftover_vars)}；"
            "RequestRedirect 无法表达，已按去掉变量后的 URL 生成。"
        )
    return rr


def build(
    input_dir: str,
    output_dir: str,
    gateway_ns_override: str | None,
) -> tuple[list[dict[str, Any]], str]:
    items = load_items(input_dir)
    albs: dict[str, dict[str, Any]] = {}
    for item in items:
        if item.get("kind") not in ("ALB2", "Frontend", "Rule"):
            continue
        name = alb_name(item)
        alb = albs.setdefault(name, {"frontends": {}, "rules": [], "raw_frontends": 0, "owner_uids": set()})
        for owner in (item.get("metadata") or {}).get("ownerReferences") or []:
            if owner.get("kind") == "ALB2" and owner.get("uid"):
                alb["owner_uids"].add(str(owner["uid"]))
        if item.get("kind") == "ALB2":
            alb["alb2"] = item
        elif item.get("kind") == "Frontend":
            meta = item.get("metadata") or {}
            alb["raw_frontends"] = int(alb.get("raw_frontends", 0)) + 1
            alb["frontends"][meta.get("name")] = item
        elif item.get("kind") == "Rule":
            alb["rules"].append(item)

    source_ingresses = sorted({
        (str((item.get("spec") or {}).get("source", {}).get("namespace") or (item.get("metadata") or {}).get("namespace") or "default"),
         str((item.get("spec") or {}).get("source", {}).get("name")))
        for item in items
        if item.get("kind") == "Rule"
        and alb_name(item) not in SYSTEM_ALBS
        and (item.get("spec") or {}).get("source", {}).get("type") == "ingress"
        and (item.get("spec") or {}).get("source", {}).get("name")
    })

    namespace_objects = {
        str((item.get("metadata") or {}).get("name"))
        for item in items
        if item.get("kind") == "Namespace" and (item.get("metadata") or {}).get("name")
    }
    ingress_objects = {
        (str((item.get("metadata") or {}).get("namespace") or "default"), str((item.get("metadata") or {}).get("name")))
        for item in items
        if item.get("kind") == "Ingress" and (item.get("metadata") or {}).get("name")
    }
    frontend_objects = {
        (str((item.get("metadata") or {}).get("namespace") or "default"), str((item.get("metadata") or {}).get("name")))
        for item in items
        if item.get("kind") == "Frontend" and (item.get("metadata") or {}).get("name")
    }
    required_namespaces = set()
    for item in items:
        if item.get("kind") in ("ALB2", "Frontend", "Rule") and alb_name(item) in SYSTEM_ALBS:
            continue
        meta = item.get("metadata") or {}
        if meta.get("namespace"):
            required_namespaces.add(str(meta["namespace"]))
        spec = item.get("spec") or {}
        for svc in ((spec.get("serviceGroup") or {}).get("services") or []):
            if svc.get("namespace"):
                required_namespaces.add(str(svc["namespace"]))
        source = spec.get("source") or {}
        if source.get("type") == "ingress" and source.get("namespace"):
            required_namespaces.add(str(source["namespace"]))
    missing_namespaces = sorted(required_namespaces - namespace_objects)
    missing_albs = sorted(name for name, data in albs.items() if name not in SYSTEM_ALBS and not data.get("alb2"))
    alb_ns_map: dict[str, str] = {}
    for name, data in albs.items():
        for item in list(data["frontends"].values()) + data["rules"]:
            ns = (item.get("metadata") or {}).get("namespace")
            if ns:
                alb_ns_map[name] = str(ns)
                break
    required_frontends = set()
    for alb_name_key, data in albs.items():
        if alb_name_key in SYSTEM_ALBS:
            continue
        for rule in data["rules"]:
            label = ((rule.get("metadata") or {}).get("labels") or {}).get("alb2.cpaas.io/frontend")
            if label:
                required_frontends.add((str((rule.get("metadata") or {}).get("namespace") or "default"), str(label)))
    missing_frontends = sorted(required_frontends - frontend_objects)
    missing_source_ingresses = sorted(set(source_ingresses) - ingress_objects)

    final_dir = os.path.abspath(output_dir)
    staging_dir = f"{final_dir}.tmp-{os.getpid()}"
    shutil.rmtree(staging_dir, ignore_errors=True)
    os.makedirs(staging_dir, exist_ok=True)
    out_dir = staging_dir
    case_name = sanitize(os.path.basename(final_dir), 63)

    docs: list[dict[str, Any]] = []
    report: list[str] = []
    all_warnings: list[str] = []
    reference_grants: set[tuple[str, str, str, str]] = set()
    created_namespaces: set[str] = set()

    user_albs = {name: data for name, data in albs.items() if name not in SYSTEM_ALBS}
    system_albs = {name: data for name, data in albs.items() if name in SYSTEM_ALBS}

    report.append("# ALB 转 Gateway API 转换报告")
    report.append("")
    report.append("## 清单")
    report.append("")
    report.append("| ALB | 系统 LB | Frontend 数 | Rule 数 |")
    report.append("| --- | --- | ---: | ---: |")
    for name in sorted(albs):
        data = albs[name]
        report.append(
            f"| {name} | {'是' if name in SYSTEM_ALBS else '否'} | "
            f"{len(data['frontends'])} | {len(data['rules'])} |"
        )
    report.append("")
    duplicate_notes = []
    for name in sorted(albs):
        raw = int(albs[name].get("raw_frontends", 0))
        unique = len(albs[name]["frontends"])
        if raw != unique:
            duplicate_notes.append(f"`{name}` 原始 {raw} / 唯一 {unique}")
    if duplicate_notes:
        report.append("Frontend 数按唯一名称统计；存在重复对象：" + "；".join(duplicate_notes) + "。")
    else:
        report.append("Frontend 数按唯一名称统计。")
    report.append("")
    report.append("## 缺口")
    report.append("")
    has_alb2 = any(data.get("alb2") for data in albs.values())
    report.append("从 Rule/Frontend/ALB 关联可以推导出迁移需要补齐的对象；Service 和 Secret 不需要导出。")
    report.append("")
    report.append("| 对象 | 是否必须 | 缺口 | 用途 |")
    report.append("| --- | --- | --- | --- |")
    if missing_albs:
        report.append(f"| `ALB2` | 必须 | {len(missing_albs)} 个 | VIP/address、Gateway 暴露方式、annotation |")
    if missing_frontends:
        report.append(f"| `Frontend` | 必须 | {len(missing_frontends)} 个 | listener 端口/协议、证书、L4 serviceGroup |")
    if missing_namespaces:
        report.append(f"| `Namespace` | 必须（仅相关） | {len(missing_namespaces)} 个 | Route/Gateway namespace、allowedRoutes/project label |")
    if missing_source_ingresses:
        report.append(f"| `Ingress` | 必须（仅 source） | {len(missing_source_ingresses)} 个 | source host、TLS、其他 path、project 归属 |")
    report.append("| `Service` | 不需要 | — | Frontend/Rule 已有 name/port；`--check` 只查目标集群 |")
    report.append("| `Secret` | 不需要 | — | 证书内容不进入转换；在目标集群本地验证 SAN/有效期 |")
    report.append("")
    report.append("完整缺口清单见 `missing-resources.md`；在 case 目录执行 `./fetch-missing.sh` 只读拉取到 `alb-export-crs/`。")
    report.append("")
    if missing_source_ingresses:
        report.append(
            f"- 缺失 {len(missing_source_ingresses)} 个 source Ingress：这些 Rule 目前只按 ALB Rule/dslx 生成 first-pass；"
            "host、TLS、其他 path、annotation 驱动的 rewrite/CORS/timeout/vhost 可能不完整。补齐后必须重新生成并逐条对照，不能直接切流。"
        )
    report.append("- 80 等默认规则没有 `cpaas.io/project` 是正常的；只有端口项目规则才需要 project label。")
    report.append("## 结论")
    report.append("")
    report.append(f"- 本次转换 {len(user_albs)} 个用户 ALB：{', '.join(sorted(user_albs))}；跳过 {len(system_albs)} 个系统 ALB：{', '.join(sorted(system_albs)) or '无'}。")
    report.append("- 当前这版 YAML 用于测试环境验证；生产上线前请先补齐上面的缺口并重新生成。")
    report.append("")
    report.append("## 数字口径")
    report.append("")
    report.append("- Rule 数不等于路由数：同一个 Ingress 的多条 Rule 会合并成一个 HTTPRoute；超过 16 条 rule 会拆成多个 HTTPRoute。")
    all_rules = [
        rule
        for alb_name_key, data in albs.items()
        if alb_name_key not in SYSTEM_ALBS
        for rule in data["rules"]
    ]
    cors_total = sum(1 for rule in all_rules if (rule.get("spec") or {}).get("enableCORS"))
    if cors_total:
        report.append(f"- CORS 配置 {cors_total} 条；具体 filter 数和跳过原因见各 ALB 段。")
    else:
        report.append("- 本次没有 CORS 配置。")
    report.append("- Frontend 数按唯一名称；原始对象数见清单说明。")
    report.append("- 完整的功能检查清单和每条 Rule 名见 `feature-audit.md`。")
    report.append("")
    if system_albs:
        report.append("## 已跳过的系统 LB")
        report.append("")
        for name in sorted(system_albs):
            data = system_albs[name]
            raw = int(data.get("raw_frontends", 0))
            raw_text = f"（原始 {raw} 个）" if raw != len(data["frontends"]) else ""
            report.append(f"- `{name}`：{len(data['frontends'])} 个唯一 Frontend{raw_text}、{len(data['rules'])} 条 Rule；平台自用 ALB，不在本次迁移范围，请保留。")
        report.append("")

    multi_owner_albs = [
        (name, len(data.get("owner_uids") or []))
        for name, data in sorted(user_albs.items())
        if len(data.get("owner_uids") or []) > 1
    ]
    if multi_owner_albs:
        report.append("## 多 owner/多集群提示")
        report.append("")
        for name, count in multi_owner_albs:
            report.append(
                f"- [注意] `{name}` 检测到 {count} 个不同 ALB2 owner UID，可能来自多个集群；"
                "当前按同名 ALB 合并。这是 apply 前的阻断项：确认属于同一集群可合并，或拆分/重新导出，未确认前不要应用。"
            )
        report.append("")

    max_listener_alb = ""
    max_listener_count = 0
    gateway_namespaces: set[str] = set()
    for alb in sorted(user_albs):
        data = user_albs[alb]
        frontends: dict[str, dict[str, Any]] = data["frontends"]
        rules: list[dict[str, Any]] = data["rules"]
        warnings: list[str] = []
        chunking_details: list[str] = []
        proxy_header_sources = []
        for item in [data.get("alb2") or {}] + list(frontends.values()) + rules:
            proxy_header = ((item.get("spec") or {}).get("proxy_header") or {})
            if proxy_header.get("use_forwarded_headers"):
                proxy_header_sources.append(str((item.get("metadata") or {}).get("name") or "?"))
        if proxy_header_sources:
            warnings.append(
                f"{alb}：{', '.join(proxy_header_sources)} 使用 proxy_header.use_forwarded_headers；"
                "Gateway API/Envoy 的 upstream X-Forwarded-* 覆盖语义不同，需人工确认。"
            )
        if len(data.get("owner_uids") or []) > 1:
            warnings.append(
                f"{alb}：检测到 {len(data['owner_uids'])} 个不同的 ALB2 owner UID，可能来自两个集群；"
                "当前按同名 ALB 合并，请确认是否应拆成两个 Gateway。"
            )
        request_uri_redirects: list[str] = []
        alb_docs_start = len(docs)
        created_backends: set[tuple[str, str]] = set()
        feature_stats = compute_feature_stats(frontends, rules)
        alb_annotations, creator_source = migrate_alb_annotations(data, alb)
        alb_ns = "cpaas-system"
        for item in list(frontends.values()) + rules:
            ns = (item.get("metadata") or {}).get("namespace")
            if ns:
                alb_ns = ns
                break

        # backend namespaces
        service_ns: set[str] = set()
        for fe in frontends.values():
            for svc in ((fe.get("spec") or {}).get("serviceGroup") or {}).get("services") or []:
                if svc.get("namespace"):
                    service_ns.add(svc["namespace"])
        for rule in rules:
            for svc in ((rule.get("spec") or {}).get("serviceGroup") or {}).get("services") or []:
                if svc.get("namespace"):
                    service_ns.add(svc["namespace"])
        if gateway_ns_override:
            gateway_ns = gateway_ns_override
        elif len(service_ns) == 1:
            gateway_ns = next(iter(service_ns))
        else:
            gateway_ns = DEFAULT_GATEWAY_NS
        if gateway_ns == "cpaas-system":
            warnings.append(f"{alb}：Gateway namespace 是 cpaas-system；应放到后端 namespace 或专用 gateway namespace。")
        gateway_namespaces.add(gateway_ns)
        if gateway_ns not in created_namespaces:
            add_doc(docs, {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": gateway_ns}})
            created_namespaces.add(gateway_ns)

        # listeners
        listeners: dict[tuple[str, int], dict[str, Any]] = {}
        frontend_listener: dict[str, str] = {}
        for fe_name, fe in frontends.items():
            spec = fe.get("spec") or {}
            proto = str(spec.get("protocol") or "HTTP").upper()
            port = spec.get("port")
            if port is None:
                continue
            key = (proto, int(port))
            lname = f"{proto.lower()}-{port}"
            frontend_listener[fe_name] = lname
            listeners.setdefault(key, {"name": lname, "protocol": proto, "port": int(port)})

        if len(listeners) > max_listener_count:
            max_listener_alb = alb
            max_listener_count = len(listeners)

        alb2_config = ((data.get("alb2") or {}).get("spec") or {}).get("config") or {}
        if alb2_config.get("ingressSSLRedirect"):
            warnings.append(
                f"{alb}：ALB2 ingressSSLRedirect=true；当前生成器没有生成 HTTP->HTTPS catch-all redirect，"
                "需要补一个重定向 HTTPRoute 或确认保留明文访问。"
            )

        for fe_name, fe in frontends.items():
            fe_spec = fe.get("spec") or {}
            if (fe_spec.get("config") or {}).get("redirect"):
                warnings.append(
                    f"{alb}：Frontend {fe_name} 配置了 redirect；当前只迁移 Rule 级 redirect，"
                    "Frontend 级 redirect 需要手工补 HTTPRoute 或确认是否忽略。"
                )
            if str(fe_spec.get("protocol") or "").lower() in ("tcp", "udp") and not (fe_spec.get("serviceGroup") or {}).get("services"):
                has_l4_rule = any(
                    ((rule.get("metadata") or {}).get("labels") or {}).get("alb2.cpaas.io/frontend") == fe_name
                    and (((rule.get("spec") or {}).get("serviceGroup") or {}).get("services"))
                    for rule in rules
                )
                if not has_l4_rule:
                    warnings.append(
                        f"{alb}：Frontend {fe_name} 是 {fe_spec.get('protocol')} 但没有 serviceGroup，也没有带后端的 Rule；"
                        "只生成了 listener，没有 L4 route。"
                    )

        # Frontend-level timeout (port-level L4/L7 timeout)
        listener_timeout: dict[tuple[str, int], dict[str, Any]] = {}
        for fe_name, fe in frontends.items():
            spec = fe.get("spec") or {}
            proto = str(spec.get("protocol") or "HTTP").upper()
            port = spec.get("port")
            if port is None:
                continue
            t = ((spec.get("config") or {}).get("timeout") or {})
            if not any(t.get(k) is not None for k in ("proxy_connect_timeout_ms", "proxy_read_timeout_ms", "proxy_send_timeout_ms")):
                continue
            key = (proto, int(port))
            if key in listener_timeout and listener_timeout[key] != t:
                warnings.append(f"{alb}：listener {proto}/{port} 的 Frontend timeout 不一致，取第一个。")
            else:
                listener_timeout[key] = t

        # certificate refs for HTTPS listeners; Frontend certs count even when
        # the Frontend has no Rule (for example a listener prepared ahead of time).
        cert_refs: dict[tuple[str, str], dict[str, Any]] = {}
        for fe in frontends.values():
            fe_spec = fe.get("spec") or {}
            if str(fe_spec.get("protocol") or "").lower() != "https":
                continue
            ns, secret = parse_cert(fe_spec.get("certificate_name"))
            if secret:
                cert_refs[(ns or gateway_ns, secret)] = {"kind": "Secret", "name": secret} | (
                    {"namespace": ns} if ns and ns != gateway_ns else {}
                )
        for rule in rules:
            spec = rule.get("spec") or {}
            fe_name = ((rule.get("metadata") or {}).get("labels") or {}).get("alb2.cpaas.io/frontend")
            fe = frontends.get(fe_name) if fe_name else None
            if not fe:
                continue
            fe_spec = fe.get("spec") or {}
            if str(fe_spec.get("protocol") or "").lower() != "https":
                continue
            for value in [fe_spec.get("certificate_name"), spec.get("certificate_name")]:
                ns, secret = parse_cert(value)
                if secret:
                    cert_refs[(ns or gateway_ns, secret)] = {"kind": "Secret", "name": secret} | (
                        {"namespace": ns} if ns and ns != gateway_ns else {}
                    )
        for (cert_ns, _), _ in sorted(cert_refs.items()):
            make_reference_grant(docs, reference_grants, cert_ns, gateway_ns, "Gateway", "Secret")

        if len(listeners) > MAX_LISTENERS:
            warnings.append(f"{alb}：有 {len(listeners)} 个 listener，超过 {MAX_LISTENERS}；按产品文档的 ListenerSet/shared VIP 章节拆分。")

        if not listeners:
            warnings.append(f"{alb}：没有找到 listener，已跳过。")
            continue

        gateway_name = f"{sanitize(alb)}-gateway"
        add_doc(
            docs,
            {
                "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                "kind": "EnvoyProxy",
                "metadata": {"name": gateway_name, "namespace": gateway_ns},
                "spec": {"provider": {"type": "Kubernetes", "kubernetes": {"envoyService": {"type": "LoadBalancer"}}}},
            },
            comment=f"# ALB: {alb_ns}/{alb}; EnvoyProxy replacing ALB {alb}",
        )

        # allowedRoutes: Same if all route namespaces equal gateway_ns, else All
        route_namespaces: set[str] = set()
        for rule in rules:
            for svc in ((rule.get("spec") or {}).get("serviceGroup") or {}).get("services") or []:
                if svc.get("namespace"):
                    route_namespaces.add(svc["namespace"])
        allowed_from = "Same" if not route_namespaces or route_namespaces == {gateway_ns} else "All"
        projects: set[str] = set()
        missing_project = 0
        for rule in rules:
            project = ((rule.get("metadata") or {}).get("labels") or {}).get("cpaas.io/project")
            if project:
                projects.add(str(project))
            else:
                missing_project += 1
        missing_project_rules = [
            str((rule.get("metadata") or {}).get("name"))
            for rule in rules
            if not ((rule.get("metadata") or {}).get("labels") or {}).get("cpaas.io/project")
        ]
        if allowed_from == "All":
            warnings.append(
                f"{alb}：后端分布在 {len(service_ns)} 个 namespace；路由分布在 {len(route_namespaces)} 个 namespace；Gateway 在 `{gateway_ns}`；"
                f"有 {len(projects)} 个 cpaas.io/project label，{missing_project} 条 Rule 没有 project label；"
                "所有 listener 使用 allowedRoutes from: All。上线前建议收紧为 Selector。"
            )

        listener_docs = []
        for key in sorted(listeners, key=lambda k: (k[1], k[0])):
            listener = dict(listeners[key])
            listener["allowedRoutes"] = {"namespaces": {"from": allowed_from}}
            if listener["protocol"] == "HTTPS":
                refs = [v for (ns, _), v in sorted(cert_refs.items())]
                if refs:
                    listener["tls"] = {"mode": "Terminate", "certificateRefs": refs}
                else:
                    warnings.append(f"{alb}：HTTPS listener {listener['name']} 没有证书；请补 certificateRefs/ReferenceGrant。")
                    listener["tls"] = {"mode": "Terminate", "certificateRefs": [{"kind": "Secret", "name": "replace-with-tls-secret"}]}
            listener_docs.append(listener)

        add_doc(
            docs,
            {
                "apiVersion": "gateway.networking.k8s.io/v1",
                "kind": "Gateway",
                "metadata": {
                    "name": f"{sanitize(alb)}-gateway",
                    "namespace": gateway_ns,
                    **({"annotations": alb_annotations} if alb_annotations else {}),
                },
                "spec": {
                    "gatewayClassName": "envoy-gateway-operator-cpaas-default",
                    "infrastructure": {
                        "parametersRef": {
                            "group": "gateway.envoyproxy.io",
                            "kind": "EnvoyProxy",
                            "name": f"{sanitize(alb)}-gateway",
                        }
                    },
                    "listeners": listener_docs,
                },
            },
            comment=f"# ALB: {alb_ns}/{alb}; Gateway replacing ALB {alb}; listeners from ALB Frontend ports",
        )

        # Frontend-level timeout policies target the Gateway listener
        timeout_stats: collections.Counter = collections.Counter()
        for key, t in sorted(listener_timeout.items()):
            proto, port = key
            lname = f"{proto.lower()}-{port}"
            connect = ms(t.get("proxy_connect_timeout_ms"))
            idle = ms(min_ms(t.get("proxy_read_timeout_ms"), t.get("proxy_send_timeout_ms")))
            if not connect and not idle:
                continue
            target = [{"group": "gateway.networking.k8s.io", "kind": "Gateway", "name": gateway_name, "sectionName": lname}]
            if proto in ("HTTP", "HTTPS"):
                timeout_spec: dict[str, Any] = {}
                if connect:
                    timeout_spec["tcp"] = {"connectTimeout": connect}
                if idle:
                    timeout_spec["http"] = {"streamIdleTimeout": idle}
                add_doc(
                    docs,
                    {
                        "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                        "kind": "BackendTrafficPolicy",
                        "metadata": {"name": f"{gateway_name}-{lname}-timeout", "namespace": gateway_ns},
                        "spec": {"targetRefs": target, "timeout": timeout_spec},
                    },
                )
                timeout_stats["policies"] += 1
            else:
                if idle:
                    add_doc(
                        docs,
                        {
                            "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                            "kind": "ClientTrafficPolicy",
                            "metadata": {"name": f"{gateway_name}-{lname}-idle", "namespace": gateway_ns},
                            "spec": {"targetRefs": target, "timeout": {"tcp": {"idleTimeout": idle}}},
                        },
                    )
                    timeout_stats["policies"] += 1
                if connect:
                    add_doc(
                        docs,
                        {
                            "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                            "kind": "BackendTrafficPolicy",
                            "metadata": {"name": f"{gateway_name}-{lname}-connect", "namespace": gateway_ns},
                            "spec": {"targetRefs": target, "timeout": {"tcp": {"connectTimeout": connect}}},
                        },
                    )
                    timeout_stats["policies"] += 1

        # ingress rules grouped by ingress + host
        ingress_groups: dict[tuple[str, str], list[dict[str, Any]]] = collections.defaultdict(list)
        direct_rules: list[dict[str, Any]] = []
        for rule in rules:
            src = (rule.get("spec") or {}).get("source") or {}
            if src.get("type") == "ingress" and src.get("name"):
                ingress_groups[(src.get("namespace") or gateway_ns, src["name"])].append(rule)
            else:
                direct_rules.append(rule)

        route_count = 0
        cors_stats: collections.Counter = collections.Counter()
        timeout_rule_names: set[str] = set()
        for (ingress_ns, ingress_name), group in sorted(ingress_groups.items()):
            group_service_ns: set[str] = set()
            for rule in group:
                for svc in ((rule.get("spec") or {}).get("serviceGroup") or {}).get("services") or []:
                    if svc.get("namespace"):
                        group_service_ns.add(svc["namespace"])
            if len(group_service_ns) == 1:
                route_ns = next(iter(group_service_ns))
            elif group_service_ns:
                route_ns = sorted(group_service_ns)[0]
                warnings.append(
                    f"{ingress_ns}/{ingress_name}：后端跨多个 namespace（{', '.join(sorted(group_service_ns))}），"
                    f"HTTPRoute 放在 {route_ns}，其他 namespace 通过 backendRefs.namespace + ReferenceGrant。"
                )
            else:
                route_ns = ingress_ns
            base_name = ingress_name if route_ns == ingress_ns else f"{ingress_ns}-{ingress_name}"
            host_to_rules: dict[str | None, list[dict[str, Any]]] = collections.defaultdict(list)
            for rule in group:
                hosts = hosts_for_rule(rule.get("spec") or {})
                if not hosts:
                    host_to_rules[None].append(rule)
                else:
                    for host in hosts:
                        host_to_rules[host].append(rule)
            for idx, (host, host_rules) in enumerate(sorted(host_to_rules.items(), key=lambda x: (x[0] is not None, x[0] or ""))):
                host_label = sanitize(host, 40) if host else "default"
                base_route_name = base_name if not host else f"{base_name}-{host_label}"
                timeout_values: dict[tuple[Any, Any, Any], dict[str, Any]] = {}
                for r in host_rules:
                    t = ((r.get("spec") or {}).get("config") or {}).get("timeout") or {}
                    key = (t.get("proxy_connect_timeout_ms"), t.get("proxy_read_timeout_ms"), t.get("proxy_send_timeout_ms"))
                    if any(v is not None for v in key):
                        timeout_values[key] = t
                        timeout_rule_names.add(str((r.get("metadata") or {}).get("name")))
                if len(timeout_values) > 1:
                    warnings.append(f"{route_ns}/{base_route_name}：同一 HTTPRoute 内 timeout 不一致，已按第一个生成；需要拆分 route。")
                host_timeout = next(iter(timeout_values.values())) if timeout_values else {}
                rule_entries: list[tuple[str | None, dict[str, Any], dict[str, Any]]] = []
                for ridx, rule in enumerate(host_rules):
                    spec = rule.get("spec") or {}
                    fe_name = ((rule.get("metadata") or {}).get("labels") or {}).get("alb2.cpaas.io/frontend")
                    section = frontend_listener.get(fe_name) if fe_name else None
                    label = f"{route_ns}/{base_route_name} rule {ridx}"
                    matches = matches_for_rule(spec, warnings, label)
                    filters: list[dict[str, Any]] = []
                    has_redirect = False
                    rewrite_base = spec.get("rewrite_base")
                    rewrite_target = spec.get("rewrite_target")
                    vhost = spec.get("vhost")
                    redirect_rr = build_redirect_filter(spec, label, warnings, matches)
                    has_redirect = redirect_rr is not None
                    redirect_url = str(
                        ((spec.get("config") or {}).get("redirect") or {}).get("url")
                        or spec.get("redirectURL")
                        or ""
                    )
                    if has_redirect and "$request_uri" in redirect_url:
                        request_uri_redirects.append(str((rule.get("metadata") or {}).get("name") or label))
                    if has_redirect and (vhost or (rewrite_base and rewrite_target)):
                        warnings.append(
                            f"{label}：redirect 与 vhost/rewrite 同时存在；Gateway API 不允许 RequestRedirect 与 URLRewrite 同规则，已只生成 redirect。"
                        )
                    if has_redirect:
                        filters.append({"type": "RequestRedirect", "requestRedirect": redirect_rr})
                    else:
                        if vhost:
                            filters.append({"type": "URLRewrite", "urlRewrite": {"hostname": vhost}})
                        if rewrite_base and rewrite_target:
                            rule_name = str((rule.get("metadata") or {}).get("name") or f"rule-{ridx}")
                            fname = f"{sanitize(base_route_name)}-rw-{short_hash(rule_name)}"
                            url_rewrite: dict[str, Any] = {
                                "path": {
                                    "type": "ReplaceRegexMatch",
                                    "replaceRegexMatch": {
                                        "pattern": str(rewrite_base),
                                        "substitution": re.sub(r"\$(\d+)", r"\\\1", str(rewrite_target)),
                                    },
                                }
                            }
                            add_doc(
                                docs,
                                {
                                    "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                                    "kind": "HTTPRouteFilter",
                                    "metadata": {"name": fname, "namespace": route_ns},
                                    "spec": {"urlRewrite": url_rewrite},
                                },
                            )
                            filters.append(
                                {
                                    "type": "ExtensionRef",
                                    "extensionRef": {"group": "gateway.envoyproxy.io", "kind": "HTTPRouteFilter", "name": fname},
                                }
                            )
                    if spec.get("enableCORS"):
                        raw_origins, fixed_comma = normalize_origins(spec.get("corsAllowOrigin"))
                        empty_origin = not raw_origins
                        configured_wildcard = "*" in raw_origins
                        origins = [o for o in raw_origins if o != "*"] if configured_wildcard else raw_origins
                        if empty_origin:
                            origins = ["*"]
                            cors_stats["empty_origin"] += 1
                            warnings.append(f"{label}：CORS origin 为空；ALB 等价于 `*` + credentials，浏览器会忽略该组合。")
                        headers = split_csv(spec.get("corsAllowHeaders"))
                        if not headers:
                            headers = [
                                "DNT", "X-CustomHeader", "Keep-Alive", "User-Agent",
                                "X-Requested-With", "If-Modified-Since", "Cache-Control",
                                "Content-Type", "Authorization",
                            ]
                        dynamic = [o for o in origins if "$" in o]
                        wildcard_port = [o for o in origins if o.endswith(":*")]
                        literal_wildcard = [o for o in origins if "*" in o and o != "*" and not o.endswith(":*")]
                        valid_origins = [
                            o for o in origins
                            if o not in dynamic and o not in wildcard_port and o not in literal_wildcard and o != "*"
                        ]
                        if empty_origin:
                            valid_origins = ["*"]
                        cors_stats["total"] += 1
                        if fixed_comma:
                            cors_stats["fixed_comma"] += 1
                            warnings.append(f"{label}：CORS origin 缺少逗号，已自动拆分为多个 origin。")
                        if dynamic:
                            cors_stats["dynamic"] += 1
                            warnings.append(f"{label}：CORS origin 使用 $http_origin；ALB 也不会展开变量，迁移后保持现状，不需要处理；如确实依赖，请提供实际 Origin 或确认可用 `*`。")
                        if wildcard_port:
                            cors_stats["wildcard_port"] += 1
                            warnings.append(f"{label}：CORS origin 使用端口通配符（{', '.join(wildcard_port)}）；ALB 也不支持，迁移后保持现状，不需要处理；如确实需要，请提供实际端口。")
                        if literal_wildcard:
                            cors_stats["literal_wildcard"] += 1
                            warnings.append(
                                f"{label}：CORS origin 含 ALB 字面量通配符（{', '.join(literal_wildcard)}）；"
                                "ALB 只按字面量匹配，不会命中真实 Origin；未写成 Gateway 通配符，避免扩大访问范围。"
                            )
                        if configured_wildcard:
                            cors_stats["wildcard_disabled"] += 1
                            warnings.append(
                                f"{label}：CORS origin 为 `*`；ALB 对 `*` 按字面量匹配，不会命中真实 Origin，"
                                "因此不生成 CORS filter（避免 Gateway 把 `*` 解释成任意 Origin）。"
                            )
                        if valid_origins:
                            cors_stats["static"] += 1
                            cors_stats["filters"] += 1
                            cors = {
                                "allowOrigins": valid_origins,
                                "allowMethods": ["GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"],
                                "allowCredentials": not empty_origin,
                                "allowHeaders": headers,
                                "maxAge": 1728000,
                            }
                            filters.append({"type": "CORS", "cors": cors})
                        elif not dynamic and not wildcard_port and not literal_wildcard and not configured_wildcard:
                            warnings.append(f"{label}：CORS origin 全部无法直接映射，未生成 CORS filter。")
                    backend_refs = []
                    dropped_redirect_backends: list[str] = []
                    https_backend = str(spec.get("backendProtocol") or "").lower() == "https"
                    for svc in ((spec.get("serviceGroup") or {}).get("services") or []):
                        if has_redirect:
                            # Gateway API forbids RequestRedirect together with backendRefs.
                            dropped_redirect_backends.append(str(svc.get("name")))
                            continue
                        if https_backend:
                            ref = make_https_backend(
                                docs, base_route_name, svc, route_ns, alb_ns, alb, created_backends
                            )
                        else:
                            ref = {"name": svc.get("name"), "port": svc.get("port")}
                            if svc.get("namespace") and svc["namespace"] != route_ns:
                                ref["namespace"] = svc["namespace"]
                                make_reference_grant(docs, reference_grants, svc["namespace"], route_ns, "HTTPRoute", "Service")
                        if svc.get("weight") not in (None, 100):
                            ref["weight"] = svc.get("weight")
                        backend_refs.append(ref)
                    if dropped_redirect_backends:
                        warnings.append(
                            f"{label}：redirect 规则的原后端 {', '.join(dropped_redirect_backends)} 已省略；"
                            "Gateway API 不允许 RequestRedirect 和 backendRefs 同时存在。"
                        )
                    rr_obj: dict[str, Any] = {"matches": matches}
                    if backend_refs:
                        rr_obj["backendRefs"] = backend_refs
                    if filters:
                        rr_obj["filters"] = filters
                    timeout = ((spec.get("config") or {}).get("timeout") or {})
                    timeout_values = [
                        timeout.get("proxy_connect_timeout_ms"),
                        timeout.get("proxy_read_timeout_ms"),
                        timeout.get("proxy_send_timeout_ms"),
                    ]
                    max_timeout = max([v for v in timeout_values if v is not None], default=None)
                    if max_timeout is not None:
                        rr_obj["timeouts"] = {"request": ms(max_timeout), "backendRequest": ms(max_timeout)}
                    rule_entries.append((section, rr_obj, rule))
                if not rule_entries:
                    continue
                section_entries: dict[str | None, list[tuple[str | None, dict[str, Any], dict[str, Any]]]] = collections.defaultdict(list)
                for entry in rule_entries:
                    section_entries[entry[0]].append(entry)
                signatures: dict[str | None, tuple[str, ...]] = {}
                for section, entries in section_entries.items():
                    signatures[section] = tuple(json.dumps(entry[1], sort_keys=True) for entry in entries)
                merged = None not in section_entries and len(set(signatures.values())) == 1
                if merged:
                    deduped_entries: list[tuple[str | None, dict[str, Any], dict[str, Any]]] = []
                    seen_rule_signatures: set[str] = set()
                    for entry in rule_entries:
                        rule_signature = json.dumps(entry[1], sort_keys=True)
                        if rule_signature in seen_rule_signatures:
                            continue
                        seen_rule_signatures.add(rule_signature)
                        deduped_entries.append(entry)
                    groups: list[tuple[list[str], list[tuple[str | None, dict[str, Any], dict[str, Any]]]]] = [
                        (sorted(section_entries), deduped_entries)
                    ]
                else:
                    groups = []
                    for section, entries in sorted(section_entries.items(), key=lambda item: (item[0] is None, item[0] or "")):
                        groups.append(([section] if section else [], entries))
                for sections, entries in groups:
                    if not sections:
                        warnings.append(f"{route_ns}/{base_route_name}：有 rule 没有匹配的 listener，已跳过。")
                        continue
                    parent_sections = set(sections)
                    source_entries = [
                        entry for entry in rule_entries
                        if (entry[0] in parent_sections if parent_sections else entry[0] is None)
                    ]
                    section_suffix = "" if merged else "-" + sanitize("+".join(sorted(parent_sections)), 40)
                    group_base = f"{base_route_name}{section_suffix}"
                    chunks = [entries[i:i + MAX_RULES_PER_ROUTE] for i in range(0, len(entries), MAX_RULES_PER_ROUTE)]
                    if len(chunks) > 1:
                        warnings.append(
                            f"{route_ns}/{group_base}：有 {len(entries)} 条 rule，因 {MAX_RULES_PER_ROUTE} 条上限拆成 {len(chunks)} 个 HTTPRoute。"
                        )
                    chunk_names: list[str] = []
                    for chunk_entries in chunks:
                        chunk = [entry[1] for entry in chunk_entries]
                        chunk_signatures = {json.dumps(entry[1], sort_keys=True) for entry in chunk_entries}
                        chunk_source_rules = sorted({
                            str((entry[2].get("metadata") or {}).get("name"))
                            for entry in source_entries
                            if json.dumps(entry[1], sort_keys=True) in chunk_signatures
                        })
                        chunk_suffix = "" if len(chunks) == 1 else "-" + short_hash(json.dumps(chunk, sort_keys=True))
                        route_name = f"{group_base}{chunk_suffix}"
                        chunk_names.append(route_name)
                        route = {
                            "apiVersion": "gateway.networking.k8s.io/v1",
                            "kind": "HTTPRoute",
                            "metadata": {"name": route_name, "namespace": route_ns},
                            "spec": {
                                "parentRefs": [
                                    {"name": f"{sanitize(alb)}-gateway", "namespace": gateway_ns, "sectionName": s}
                                    for s in sorted(parent_sections)
                                ],
                                "rules": chunk,
                            },
                        }
                        if host:
                            route["spec"]["hostnames"] = [host]
                        route_comment = (
                            f"# ALB: {alb_ns}/{alb}; Source Ingress: {ingress_ns}/{ingress_name}; "
                            f"Source Rule(s): {', '.join(chunk_source_rules)}; Generated HTTPRoute: {route_name}"
                        )
                        if host:
                            route_comment += f"; Host: {host}"
                        route_comment += f"; Listener(s): {', '.join(sorted(parent_sections))}"
                        add_doc(docs, route, comment=route_comment)
                        if any(
                            ((r.get("spec") or {}).get("serviceGroup") or {}).get("session_affinity_policy") == "sip-hash"
                            for r in [entry[2] for entry in chunk_entries]
                        ):
                            add_sip_hash_policy(
                                docs,
                                f"{sanitize(route_name)}-affinity",
                                route_ns,
                                "HTTPRoute",
                                route_name,
                                comment=f"# ALB: {alb_ns}/{alb}; Source Rule(s): {', '.join(chunk_source_rules)}; sip-hash SourceIP",
                            )
                        route_count += 1
                    if len(chunks) > 1:
                        chunking_details.append(
                            f"{route_ns}/{group_base}: {len(entries)} 条 Rule -> {len(chunks)} 个 HTTPRoute："
                            f"{', '.join(chunk_names)}"
                        )
        # direct L7 rules and L4 frontends
        for idx, rule in enumerate(direct_rules):
            spec = rule.get("spec") or {}
            fe_name = ((rule.get("metadata") or {}).get("labels") or {}).get("alb2.cpaas.io/frontend")
            fe = frontends.get(fe_name) if fe_name else None
            if not fe:
                warnings.append(f"{alb}：direct Rule {((rule.get('metadata') or {}).get('name'))} 没有匹配的 Frontend，已跳过。")
                continue
            fe_spec = fe.get("spec") or {}
            proto = str(fe_spec.get("protocol") or "").lower()
            section = frontend_listener.get(fe_name)
            services = (spec.get("serviceGroup") or {}).get("services") or []
            if not services or not section:
                continue
            route_ns = services[0].get("namespace") or gateway_ns
            route_name = sanitize(((rule.get("metadata") or {}).get("name") or f"{alb}-direct-{idx}"))
            label = f"{route_ns}/{route_name} direct"
            matches = matches_for_rule(spec, warnings, label) or [{"path": {"type": "PathPrefix", "value": "/"}}]
            redirect_rr = build_redirect_filter(spec, label, warnings, matches)
            direct_filters: list[dict[str, Any]] = []
            if redirect_rr:
                direct_filters.append({"type": "RequestRedirect", "requestRedirect": redirect_rr})
            else:
                if spec.get("vhost"):
                    direct_filters.append({"type": "URLRewrite", "urlRewrite": {"hostname": spec.get("vhost")}})
                rewrite_base = spec.get("rewrite_base")
                rewrite_target = spec.get("rewrite_target")
                if rewrite_base and rewrite_target:
                    fname = f"{sanitize(route_name)}-rw"
                    add_doc(
                        docs,
                        {
                            "apiVersion": "gateway.envoyproxy.io/v1alpha1",
                            "kind": "HTTPRouteFilter",
                            "metadata": {"name": fname, "namespace": route_ns},
                            "spec": {
                                "urlRewrite": {
                                    "path": {
                                        "type": "ReplaceRegexMatch",
                                        "replaceRegexMatch": {
                                            "pattern": str(rewrite_base),
                                            "substitution": re.sub(r"\$(\d+)", r"\\\1", str(rewrite_target)),
                                        },
                                    }
                                }
                            },
                        },
                    )
                    direct_filters.append(
                        {
                            "type": "ExtensionRef",
                            "extensionRef": {"group": "gateway.envoyproxy.io", "kind": "HTTPRouteFilter", "name": fname},
                        }
                    )
            backend_refs = []
            https_backend = str(spec.get("backendProtocol") or "").lower() == "https"
            if not redirect_rr:
                for svc in services:
                    if https_backend:
                        ref = make_https_backend(
                            docs, route_name, svc, route_ns, alb_ns, alb, created_backends
                        )
                    else:
                        ref = {"name": svc.get("name"), "port": svc.get("port")}
                        if svc.get("namespace") and svc["namespace"] != route_ns:
                            ref["namespace"] = svc["namespace"]
                    if svc.get("weight") not in (None, 100):
                        ref["weight"] = svc.get("weight")
                    backend_refs.append(ref)
            if proto in ("http", "https"):
                rule_obj: dict[str, Any] = {"matches": matches}
                if backend_refs:
                    rule_obj["backendRefs"] = backend_refs
                if direct_filters:
                    rule_obj["filters"] = direct_filters
                direct_hosts = sorted(hosts_for_rule(spec))
                direct_spec: dict[str, Any] = {
                    "parentRefs": [{"name": f"{sanitize(alb)}-gateway", "namespace": gateway_ns, "sectionName": section}],
                    "rules": [rule_obj],
                }
                if direct_hosts:
                    direct_spec["hostnames"] = direct_hosts
                add_doc(
                    docs,
                    {
                        "apiVersion": "gateway.networking.k8s.io/v1",
                        "kind": "HTTPRoute",
                        "metadata": {"name": route_name, "namespace": route_ns},
                        "spec": direct_spec,
                    },
                    comment=f"# ALB: {alb_ns}/{alb}; Source Rule: {(rule.get('metadata') or {}).get('name')}; direct rule; Generated HTTPRoute: {route_name}; Listener: {section}",
                )
                route_count += 1
                if (spec.get("serviceGroup") or {}).get("session_affinity_policy") == "sip-hash":
                    add_sip_hash_policy(
                        docs,
                        f"{sanitize(route_name)}-affinity",
                        route_ns,
                        "HTTPRoute",
                        route_name,
                        comment=f"# ALB: {alb_ns}/{alb}; Source Rule: {(rule.get('metadata') or {}).get('name')}; sip-hash SourceIP",
                    )
            elif proto in ("tcp", "udp"):
                kind = "TCPRoute" if proto == "tcp" else "UDPRoute"
                add_doc(
                    docs,
                    {
                        "apiVersion": "gateway.networking.k8s.io/v1alpha2",
                        "kind": kind,
                        "metadata": {"name": route_name, "namespace": route_ns},
                        "spec": {
                            "parentRefs": [{"name": f"{sanitize(alb)}-gateway", "namespace": gateway_ns, "sectionName": section}],
                            "rules": [{"backendRefs": backend_refs}],
                        },
                    },
                    comment=f"# ALB: {alb_ns}/{alb}; Source Rule: {(rule.get('metadata') or {}).get('name')}; direct L4; Generated {kind}: {route_name}; Listener: {section}",
                )
                route_count += 1
                if (spec.get("serviceGroup") or {}).get("session_affinity_policy") == "sip-hash":
                    add_sip_hash_policy(
                        docs,
                        f"{sanitize(route_name)}-affinity",
                        route_ns,
                        kind,
                        route_name,
                        comment=f"# ALB: {alb_ns}/{alb}; Source Rule: {(rule.get('metadata') or {}).get('name')}; L4 sip-hash SourceIP",
                    )

        for fe_name, fe in sorted(frontends.items()):
            spec = fe.get("spec") or {}
            proto = str(spec.get("protocol") or "").lower()
            if proto not in ("tcp", "udp") or not spec.get("serviceGroup"):
                continue
            section = frontend_listener.get(fe_name)
            services = (spec.get("serviceGroup") or {}).get("services") or []
            if not section or not services:
                continue
            route_ns = services[0].get("namespace") or gateway_ns
            route_name = sanitize(fe_name)
            backend_refs = []
            for svc in services:
                ref = {"name": svc.get("name"), "port": svc.get("port")}
                if svc.get("namespace") and svc["namespace"] != route_ns:
                    ref["namespace"] = svc["namespace"]
                    make_reference_grant(docs, reference_grants, svc["namespace"], route_ns, "TCPRoute" if proto == "tcp" else "UDPRoute", "Service")
                if svc.get("weight") not in (None, 100):
                    ref["weight"] = svc.get("weight")
                backend_refs.append(ref)
            add_doc(
                docs,
                {
                    "apiVersion": "gateway.networking.k8s.io/v1alpha2",
                    "kind": "TCPRoute" if proto == "tcp" else "UDPRoute",
                    "metadata": {"name": route_name, "namespace": route_ns},
                    "spec": {
                        "parentRefs": [{"name": f"{sanitize(alb)}-gateway", "namespace": gateway_ns, "sectionName": section}],
                        "rules": [{"backendRefs": backend_refs}],
                    },
                },
                comment=f"# ALB: {alb_ns}/{alb}; Source Frontend: {fe_name}; Generated {'TCPRoute' if proto == 'tcp' else 'UDPRoute'}: {route_name}; Listener: {section}",
            )
            route_count += 1
            if (spec.get("serviceGroup") or {}).get("session_affinity_policy") == "sip-hash":
                add_sip_hash_policy(
                    docs,
                    f"{sanitize(route_name)}-affinity",
                    route_ns,
                    "TCPRoute" if proto == "tcp" else "UDPRoute",
                    route_name,
                    comment=f"# ALB: {alb_ns}/{alb}; Source Frontend: {fe_name}; L4 sip-hash SourceIP",
                )

        for warning in warnings:
            has_prefix = warning.startswith(f"{alb}:") or warning.startswith(f"{alb}：")
            all_warnings.append(warning if has_prefix else f"{alb}: {warning}")
        alb_docs = docs[alb_docs_start:]
        cr_counts = collections.Counter(d.get("kind") for d in alb_docs)
        generated_cors_filters = sum(
            1
            for doc in alb_docs if doc.get("kind") == "HTTPRoute"
            for rule_doc in (doc.get("spec") or {}).get("rules") or []
            for flt in rule_doc.get("filters") or []
            if flt.get("type") == "CORS"
        )
        generated_timeout_rules = sum(
            1
            for doc in alb_docs if doc.get("kind") == "HTTPRoute"
            for rule_doc in (doc.get("spec") or {}).get("rules") or []
            if rule_doc.get("timeouts")
        )
        report.append(f"## {alb}")
        report.append("")
        report.append(f"- Gateway/EnvoyProxy：`{gateway_name}` @ `{gateway_ns}`")
        if alb_annotations:
            annotation_text = "；".join(f"`{k}`={v}" for k, v in alb_annotations.items())
            source_text = ""
            if "cpaas.io/origin-creator" in alb_annotations:
                if creator_source == "ALB2":
                    source_text = "（creator 来源：ALB2）"
                elif creator_source == "Frontend/Rule":
                    source_text = "（creator 来源：Frontend/Rule 推断，ALB2 缺失）"
            report.append(f"- ALB annotation 迁移：{annotation_text}{source_text}")
        elif data.get("alb2"):
            report.append("- ALB2 没有可迁移的 display-name/description/creator annotation")
        else:
            report.append("- ALB annotation：导出里没有 ALB2，display-name/description/creator 无法迁移")
        if creator_source == "ambiguous":
            report.append("- [注意] 没有可用的 ALB2 creator，Frontend/Rule 的 `cpaas.io/creator` 不唯一，未生成 `cpaas.io/origin-creator`")
        ingress_count = feature_stats.get("IngressRule", 0)
        direct_count = feature_stats.get("DirectRule", 0)
        l4_count = feature_stats.get("L4Frontend", 0)
        report.append(
            f"- ALB：Frontend {len(frontends)} 个；Rule {len(rules)} 条"
            f"（Ingress {ingress_count} / 直接 {direct_count}）；L4 Frontend {l4_count}"
        )
        cr_line = (
            f"- 生成 CR：Gateway {cr_counts.get('Gateway',0)}、EnvoyProxy {cr_counts.get('EnvoyProxy',0)}、"
            f"listener {len(listeners)}、HTTPRoute {cr_counts.get('HTTPRoute',0)}、"
            f"TCPRoute {cr_counts.get('TCPRoute',0)}、UDPRoute {cr_counts.get('UDPRoute',0)}、"
            f"HTTPRouteFilter {cr_counts.get('HTTPRouteFilter',0)}、ReferenceGrant {cr_counts.get('ReferenceGrant',0)}、"
            f"Backend {cr_counts.get('Backend',0)}、BackendTrafficPolicy {cr_counts.get('BackendTrafficPolicy',0)}"
        )
        if cr_counts.get("ClientTrafficPolicy", 0):
            cr_line += f"、ClientTrafficPolicy {cr_counts['ClientTrafficPolicy']}"
        if cr_counts.get("Namespace", 0):
            cr_line += f"、Namespace {cr_counts['Namespace']}"
        report.append(cr_line)
        ports_str = ", ".join(f"{proto}/{port}" for proto, port in sorted(listeners, key=lambda k: (k[1], k[0])))
        report.append(f"- 端口：{ports_str}；allowedRoutes：`{allowed_from}`")
        for note in chunking_details:
            report.append(f"- HTTPRoute 拆分：{note}")
        report.append(
            f"- project label：{len(projects)} 个；无 project 的 Rule：{missing_project} 条"
            "（80 等默认规则通常没有 project，属于正常；只有端口项目规则才需要 label）"
        )
        if feature_stats.get("CertificateRule") or cert_refs:
            report.append(f"- 证书：Rule 引用 {feature_stats.get('CertificateRule',0)} 条 / 唯一证书 {len(cert_refs)} 个")
        if timeout_rule_names:
            report.append(
                f"- 超时：{len(timeout_rule_names)} 条 Rule 的 connect/read/send 取最大值写入 HTTPRoute.timeouts"
            )
        if cors_stats.get("total"):
            report.append(
                f"- CORS：{feature_stats.get('CORS',0)} 条；CORS filter {generated_cors_filters}；"
                f"端口通配符 {cors_stats['wildcard_port']}；字面量通配符 {cors_stats.get('literal_wildcard',0)}；"
                f"裸 `*` 禁用 {cors_stats.get('wildcard_disabled',0)}；空 origin {cors_stats.get('empty_origin',0)}；"
                f"`$http_origin` {cors_stats['dynamic']}；"
                f"缺逗号自动拆分 {cors_stats['fixed_comma']}"
            )
        report.append("- Policy：CORS 挂 route rule；timeout/affinity 挂 Route；后端 HTTPS 用 Backend + insecureSkipVerify；ClientTrafficPolicy 挂 listener；应用无感知")
        report.append("")
        report.append("| ALB 特性 | 数量 | Gateway API CR |")
        report.append("| --- | ---: | --- |")
        if feature_stats.get("IngressRule"):
            report.append(f"| Ingress 生成 Rule | {feature_stats['IngressRule']} | HTTPRoute {cr_counts.get('HTTPRoute',0)} |")
        if feature_stats.get("DirectRule"):
            report.append(f"| 直接 Rule | {feature_stats['DirectRule']} | HTTPRoute/TCPRoute/UDPRoute + policy |")
        if feature_stats.get("L4Frontend"):
            report.append(f"| L4 Frontend | {feature_stats['L4Frontend']} | TCPRoute {cr_counts.get('TCPRoute',0)} + UDPRoute {cr_counts.get('UDPRoute',0)} |")
        if feature_stats.get("HOST_EQ"):
            report.append(f"| HOST 匹配 | {feature_stats['HOST_EQ']} | HTTPRoute hostnames |")
        if feature_stats.get("URL_STARTS_WITH"):
            report.append(f"| URL Prefix | {feature_stats['URL_STARTS_WITH']} | HTTPRoute PathPrefix |")
        if feature_stats.get("URL_REGEX"):
            report.append(f"| URL Regex | {feature_stats['URL_REGEX']} | HTTPRoute RegularExpression |")
        if feature_stats.get("Vhost") or feature_stats.get("Rewrite"):
            report.append(
                f"| vhost / rewrite | {feature_stats.get('Vhost',0)} / {feature_stats.get('Rewrite',0)} "
                f"| inline URLRewrite hostname + HTTPRouteFilter {cr_counts.get('HTTPRouteFilter',0)} |"
            )
        if feature_stats.get("CORS"):
            report.append(
                f"| CORS | {feature_stats['CORS']} | CORS filter {generated_cors_filters}"
                f"（字面量通配符 {cors_stats.get('literal_wildcard',0)} 未生成） |"
            )
        if feature_stats.get("Timeout"):
            report.append(f"| timeout | {feature_stats['Timeout']} | HTTPRoute.timeouts {generated_timeout_rules} |")
        if feature_stats.get("CertificateRule"):
            report.append(f"| certificate | {feature_stats['CertificateRule']} | listener certificateRefs {len(cert_refs)} |")
        if feature_stats.get("SessionAffinity"):
            report.append(f"| session affinity | {feature_stats['SessionAffinity']} | BackendTrafficPolicy {cr_counts.get('BackendTrafficPolicy',0)} |")
        if feature_stats.get("BackendTLS"):
            report.append(f"| backend TLS | {feature_stats['BackendTLS']} | Backend (insecureSkipVerify) {cr_counts.get('Backend',0)} |")
        if feature_stats.get("BackendGRPC"):
            report.append(f"| backend gRPC | {feature_stats['BackendGRPC']} | GRPCRoute |")
        report.append("")
        attention: list[str] = []
        if allowed_from == "All":
            attention.append(
                "- [注意] 生产阻断：allowedRoutes: All，任何 namespace 的路由都能挂到这些公网 listener；"
                "如果要收紧，按 namespace 选择（不要只用 project label，80 等默认规则没有 project）"
            )
        if cors_stats.get("wildcard_port") or cors_stats.get("dynamic"):
            attention.append(
                f"- [注意] CORS（ALB 本身不生效，语义不等价）：{cors_stats['wildcard_port']} 条端口通配符"
                f"（与缺逗号 {cors_stats['fixed_comma']} 条同一批，缺逗号已自动拆分）"
                f" + {cors_stats['dynamic']} 条 `$http_origin`；ALB 本身不支持，迁移后保持现状；"
                "如果确实需要，提供实际端口/Origin"
            )
        if cors_stats.get("literal_wildcard"):
            attention.append(
                f"- [注意] CORS：{cors_stats['literal_wildcard']} 条 ALB 字面量通配符（如 `*.domain`）在 ALB 中不会命中真实 Origin；"
                "未写成 Gateway 通配符，避免扩大访问范围。"
            )
        if cors_stats.get("wildcard_disabled"):
            attention.append(
                f"- [注意] CORS：{cors_stats['wildcard_disabled']} 条 origin 为 `*`；ALB 按字面量匹配，不会发出 CORS 头，"
                "迁移后不生成 CORS filter。"
            )
        if request_uri_redirects:
            attention.append(
                f"- [注意] redirect（有意行为变化）：{len(request_uri_redirects)} 条使用 `$request_uri`；ALB 当前发送字面量，迁移后保留请求 path/query。"
            )
        if timeout_rule_names:
            attention.append(
                "- [注意] timeout（语义变化）：HTTPRoute.timeout 是总时长，不是 ALB 的 idle timeout；"
                "已取 connect/read/send 最大值作为保守上限，流式长连接需验证"
            )
        if not has_alb2:
            attention.append(
                "- [注意] 缺少 ALB2：无法确认 `ingressSSLRedirect`（HTTP→HTTPS）等 ALB2 级设置；"
                "需要补齐 ALB2 后再检查。"
            )
        if feature_stats.get("SessionAffinity"):
            attention.append(
                "- [注意] sip-hash 使用 `BackendTrafficPolicy` ConsistentHash SourceIP；EnvoyProxy 未设置 `externalTrafficPolicy: Local`，"
                "SNAT 后可能把同节点客户端映射到同一后端，且不使用 XFF，需要实测。"
            )
        if generated_cors_filters:
            attention.append(
                f"- [注意] CORS preflight 由 Envoy 直接回答（{generated_cors_filters} 个 filter）；ALB 原先把 OPTIONS 透传给后端，"
                "需要验证预检响应和 credentials 行为。"
            )
        if cr_counts.get("Backend", 0):
            attention.append(
                "- [注意] 后端 HTTPS 使用 Envoy Gateway `Backend` + `tls.insecureSkipVerify: true`，"
                "保持 ALB 默认不校验后端证书；需要 `EnvoyGatewayCtl.spec.config.envoyGateway.extensionApis.enableBackend: true`，否则 Backend 不生效。"
            )
        other_warnings = [
            w for w in warnings
            if not any(x in w for x in ("CORS", "allowedRoutes", "拆成"))
        ]
        for w in other_warnings[:10]:
            attention.append(f"- [注意] {w}")
        if len(other_warnings) > 10:
            attention.append(
                f"- 另有 {len(other_warnings)-10} 条，见 `warnings.md`（不是 feature-audit.md）"
            )
        if attention:
            report.append("### 需要关注")
            report.append("")
            for item in attention:
                report.append(item)
            report.append("")
        if allowed_from == "All":
            report.append("allowedRoutes 按 namespace 收紧示例：")
            report.append("")
            report.append("```yaml")
            report.append("allowedRoutes:")
            report.append("  namespaces:")
            report.append("    from: Selector")
            report.append("    selector:")
            report.append("      matchExpressions:")
            report.append("        - key: kubernetes.io/metadata.name")
            report.append("          operator: In")
            report.append("          values: [<route-ns-1>, <route-ns-2>]")
            report.append("```")
            report.append("")

        alb_docs = docs[alb_docs_start:]
        for doc in alb_docs:
            DOC_COMMENTS.setdefault(id(doc), f"# ALB: {alb_ns}/{alb}")
            if doc.get("kind") == "Namespace":
                continue
            meta = doc.setdefault("metadata", {})
            labels = meta.setdefault("labels", {})
            labels.update(
                {
                    "app.kubernetes.io/managed-by": "alb2gatewayapi",
                    "alb2gw.alauda.io/case": case_name,
                    "alb2gw.alauda.io/alb": sanitize(alb, 63),
                }
            )
        gateway_docs, route_groups, shared_docs = split_alb_docs(alb_docs, alb_ns, alb)
        alb_dir = os.path.join(out_dir, sanitize(alb))
        if os.path.isdir(alb_dir):
            shutil.rmtree(alb_dir)
        os.makedirs(alb_dir, exist_ok=True)
        write_docs(os.path.join(alb_dir, "gateway.yaml"), gateway_docs)
        if route_groups:
            routes_dir = os.path.join(alb_dir, "routes")
            os.makedirs(routes_dir, exist_ok=True)
            for rk, route_docs in sorted(route_groups.items()):
                group = route_listener_group(route_docs[0])
                group_dir = os.path.join(routes_dir, group)
                os.makedirs(group_dir, exist_ok=True)
                write_docs(os.path.join(group_dir, route_filename(rk)), route_docs)
        if shared_docs:
            write_docs(os.path.join(alb_dir, "shared.yaml"), shared_docs)

    report.append("## 全局说明")
    report.append("")
    report.append("- GatewayClass：默认使用平台安装的 `envoy-gateway-operator-cpaas-default`；如果环境用了自定义 `EnvoyGatewayCtl`，替换成对应的 GatewayClass。")
    report.append("- Backend Service/port：可用 `kubectl get svc -n <ns> <name>` 确认；Service/port 不存在是环境问题。")
    report.append("- 语义 annotation：ALB2 的 `cpaas.io/display-name` / `cpaas.io/description` 写到 Gateway 同名 annotation；`cpaas.io/creator` 写到 `cpaas.io/origin-creator`，不覆盖平台生成的实际 creator。")
    report.append("- HTTPS 证书：`certificateRefs` 已聚合；跨 namespace Secret 的 `ReferenceGrant` 已生成，apply 时一起下发。")
    report.append("- HTTPRoute/TCPRoute/UDPRoute 放在后端 Service namespace；Gateway/EnvoyProxy 单独放；listener 的 `allowedRoutes` 控制跨 namespace 挂载。")
    report.append("- Policy：CORS 挂 route rule；timeout/affinity 挂 Route；后端 HTTPS 用 Backend + insecureSkipVerify；ClientTrafficPolicy 挂 listener；应用无感知。")
    report.append("")
    report.append("## 转换元数据")
    report.append("")
    report.append("- Source ALB：4.3")
    report.append("- Target：Gateway API + Envoy Gateway")
    report.append("- Default GatewayClass：`envoy-gateway-operator-cpaas-default`")
    report.append(f"- Gateway namespace：{', '.join(f'`{ns}`' for ns in sorted(gateway_namespaces)) or '未生成'}")
    report.append(f"- 输入完整性：ALB2 缺失 {len(missing_albs)}；Frontend 缺失 {len(missing_frontends)}；Namespace 缺失 {len(missing_namespaces)}；source Ingress 缺失 {len(missing_source_ingresses)}")
    if missing_albs or missing_frontends or missing_namespaces or missing_source_ingresses:
        report.append("- [注意] 输入不完整时生成结果只是 first-pass，不能直接作为生产 apply 清单。")
    report.append("")
    report.append("## 上线步骤")
    report.append("")
    report.append("以下步骤先在测试环境执行；生产上线前先用根据导出包重新生成的版本。")
    report.append("")
    report.append("1. 确认目标集群已安装 Envoy Gateway Operator，并有 `envoy-gateway-operator-cpaas-default` GatewayClass；如果生成 Backend，确认 `EnvoyGatewayCtl.spec.config.envoyGateway.extensionApis.enableBackend: true`。")
    gateway_ns_text = "、".join(f"`{ns}`" for ns in sorted(gateway_namespaces)) or "`gateway-user-default`"
    report.append(f"2. 如果 {gateway_ns_text} 不存在，YAML 里的 Namespace 对象会创建它；也可以先手动创建。")
    report.append("3. 每个 ALB 一个文件夹：`gateway.yaml` 放 Gateway/EnvoyProxy/Namespace；`routes/<listener>/<kind>-<ns>-<name>.yaml` 每个 Route 一个文件，按 listener 端口分目录（80/443/tcp-5001/udp-1705），attach 到多个 listener 的放 `routes/multi-listener/`；`shared.yaml`（如有）放 ReferenceGrant 和跨多个 Route 的 policy。")
    report.append("4. 先 dry-run：`kubectl apply --dry-run=server -R -f output/<case>`。")
    report.append("5. apply：`kubectl apply -R -f output/<case>`。")
    report.append("6. 检查状态：Gateway `Programmed=True`；每个 Route 的 `status.parents[].conditions` 里 `Accepted=True` 且 `ResolvedRefs=True`；Envoy 数据面 Pod 都是 Ready。")
    report.append("7. 获取新 Gateway 地址：`kubectl get gateway <name> -n <namespace> -o jsonpath='{.status.addresses[*].value}'`。")
    if has_alb2:
        report.append("8. EnvoyProxy 的 LoadBalancer 会分配一个新 IP；如需沿用 ALB VIP，请从 ALB2 的 address/VIP 配置确认后调整 Service。")
    else:
        report.append("8. 当前导出没有 ALB2 VIP 信息，EnvoyProxy 的 LoadBalancer 会分配一个新 IP，不是原 ALB VIP。如果要沿用原 VIP，请先提供 ALB2 导出。")
    report.append("9. 切流量：提前调低 DNS TTL，按域名分批把 DNS 或上游负载均衡切到新地址，观察 5xx、连接数和后端错误。")
    if max_listener_alb:
        report.append(f"10. 容量：`{max_listener_alb}` 的 {max_listener_count} 个端口由同一组 Envoy 承载；当前 EnvoyProxy 未设置 replicas/resources，上线前按流量补上。")
    else:
        report.append("10. 容量：当前 EnvoyProxy 未设置 replicas/resources，上线前按流量补上。")
    report.append("")
    report.append("生产门槛：生产版本应已将 `allowedRoutes` 收紧为 `Selector`；如果仍为 `All`，不要上生产。")
    report.append("")
    report.append("## 验证状态")
    report.append("")
    report.append("- 本地 YAML 结构校验：请运行 `python3 scripts/validate_gatewayapi.py output/<case>`。")
    report.append("- CRD schema 校验：请按目标版本运行 `validate_schema.sh`；没有 CRD 目录时会明确跳过。")
    report.append("- Service/Secret/ReferenceGrant/GatewayClass/EnvoyGatewayCtl：生成器不连接集群，需在目标环境做只读检查。")
    report.append("- 本报告是生成时快照；apply 前必须重新执行上述检查。")
    report.append("")
    report.append("## 上线前验证")
    report.append("")
    report.append("- Gateway：`Programmed=True`；每个 Route 的 `status.parents[].conditions` 同时有 `Accepted=True`、`ResolvedRefs=True`。")
    report.append("- HTTP：逐端口选择代表域名/路径，直连新地址并指定 Host；验证 backend 响应、redirect Location、CORS preflight、vhost 和 timeout。")
    report.append("- HTTPS：逐证书验证 SNI、证书链和 SAN；确认 `certificateRefs`/`ReferenceGrant` 在目标 namespace 可解析。")
    report.append("- L4：逐 TCP/UDP 端口验证监听、后端端口、会话保持和长连接；不要只用 HTTP 请求代替。")
    report.append("- 任何一项未通过都不要切流；先补输入/改配置，再重新生成并对照差异。")
    report.append("")
    report.append("## 回滚")
    report.append("")
    report.append("- 切流量后发现异常：把 DNS 或上游负载均衡切回旧 ALB 地址即可，ALB 保留。")
    report.append(f"- 不要删除 {gateway_ns_text} namespace；它可能包含其他资源。只有确认是本次新建且已清空时才删除。")
    report.append("- 不要直接 `kubectl delete -R -f output/<case>`，因为文件里包含 Namespace；请按 kind 删除 Route/Policy/Filter/Backend/ReferenceGrant -> Gateway -> EnvoyProxy，保留 Namespace。")
    report.append("- 观察期内不要删除 ALB。")
    report.append("")
    report.append("## 校验")
    report.append("")
    report.append("客户侧执行：")
    report.append("")
    report.append("```bash")
    report.append("kubectl apply --dry-run=server -R -f output/<case>")
    report.append("kubectl get gateway -A")
    report.append("kubectl get httproute,tcproute,udproute -A")
    report.append("kubectl get backend -A")
    report.append("```")
    report.append("")
    report.append("功能审计见 `feature-audit.md`。")
    report.append("")
    report_path = os.path.join(out_dir, "conversion-report.md")
    with open(report_path, "w", encoding="utf-8") as fh:
        fh.write("\n".join(report))
    if all_warnings:
        with open(os.path.join(out_dir, "warnings.md"), "w", encoding="utf-8") as fh:
            fh.write("# Migration warnings\n\n")
            for warning in all_warnings:
                fh.write(f"- {warning}\n")
    object_lines = []
    for doc in docs:
        kind = str(doc.get("kind") or "")
        if not kind or kind == "Namespace":
            continue
        meta = doc.get("metadata") or {}
        name = meta.get("name")
        if not name:
            continue
        object_lines.append(f"{kind.lower()} {meta.get('namespace') or 'default'} {name}")
    with open(os.path.join(out_dir, "objects.txt"), "w", encoding="utf-8") as fh:
        fh.write("\n".join(sorted(set(object_lines))) + "\n")

    # Keep the feature audit next to the report so a fresh reader always has
    # the per-Rule audit referenced by conversion-report.md.
    try:
        import check_features

        buffer = io.StringIO()
        with redirect_stdout(buffer):
            check_features.print_markdown(check_features.audit(input_dir))
        with open(os.path.join(out_dir, "feature-audit.md"), "w", encoding="utf-8") as fh:
            fh.write(buffer.getvalue())
    except Exception as exc:  # audit is best-effort; do not hide the conversion
        with open(os.path.join(out_dir, "feature-audit.md"), "w", encoding="utf-8") as fh:
            fh.write(f"# Feature audit not generated\n\n{exc}\n")

    missing_rows = [("ALB2", alb_ns_map.get(name, "cpaas-system"), name) for name in missing_albs]
    missing_rows += [("Frontend", ns, name) for ns, name in missing_frontends]
    missing_rows += [("Namespace", "", name) for name in missing_namespaces]
    missing_rows += [("Ingress", ns, name) for ns, name in missing_source_ingresses]
    with open(os.path.join(out_dir, "missing-resources.md"), "w", encoding="utf-8") as fh:
        fh.write("# Missing migration inputs\n\n")
        if not missing_rows:
            fh.write("No missing required resources were detected.\n")
        else:
            fh.write("| Kind | Namespace | Name |\n| --- | --- | --- |\n")
            for kind, ns, name in missing_rows:
                fh.write(f"| {kind} | {ns or '-'} | {name} |\n")
            fh.write("\nRun `./fetch-missing.sh` to fetch them (read-only).\n")

    fetch_path = os.path.join(out_dir, "fetch-missing.sh")
    with open(fetch_path, "w", encoding="utf-8") as fh:
        fh.write("#!/usr/bin/env bash\n")
        fh.write("# Generated from missing-resources.md. Read-only: kubectl get only.\n")
        fh.write("set -euo pipefail\n")
        fh.write('SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"\n')
        fh.write('OUT="${OUT_DIR:-$SCRIPT_DIR/alb-export-crs}"\n')
        fh.write('KUBECTL="${KUBECTL:-kubectl}"\n')
        fh.write('mkdir -p "$OUT"\n')
        for kind, ns, name in missing_rows:
            safe = sanitize(f"{kind}-{ns or 'cluster'}-{name}", 100)
            if kind == "ALB2":
                fh.write(f'"$KUBECTL" get alaudaloadbalancer2 -n {ns} {name} -o yaml > "$OUT/{safe}.yaml"\n')
            elif kind == "Frontend":
                fh.write(f'"$KUBECTL" get frontends.crd.alauda.io -n {ns} {name} -o yaml > "$OUT/{safe}.yaml"\n')
            elif kind == "Namespace":
                fh.write(f'"$KUBECTL" get namespace {name} -o yaml > "$OUT/{safe}.yaml"\n')
            elif kind == "Ingress":
                fh.write(f'"$KUBECTL" get ingress -n {ns} {name} -o yaml > "$OUT/{safe}.yaml"\n')
        fh.write('tar czf "$SCRIPT_DIR/alb-export-crs.tgz" -C "$OUT" .\n')
        fh.write('echo "fetched to $OUT"\n')
    os.chmod(fetch_path, 0o755)

    backup_dir = f"{final_dir}.old"
    shutil.rmtree(backup_dir, ignore_errors=True)
    if os.path.exists(final_dir):
        os.rename(final_dir, backup_dir)
    os.rename(staging_dir, final_dir)
    shutil.rmtree(backup_dir, ignore_errors=True)
    report_path = os.path.join(final_dir, "conversion-report.md")
    return docs, report_path


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input_dir")
    parser.add_argument("output_dir")
    parser.add_argument("--gateway-namespace", default=None)
    args = parser.parse_args()
    docs, report_path = build(args.input_dir, args.output_dir, args.gateway_namespace)
    print(f"wrote {len(docs)} objects to {args.output_dir} (per-ALB gateway.yaml/routes/*.yaml/shared.yaml)")
    print(f"wrote report to {report_path}")


if __name__ == "__main__":
    main()
