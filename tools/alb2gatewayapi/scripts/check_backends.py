#!/usr/bin/env python3
"""Check backendRefs from generated Gateway API YAML.

Usage:
  python3 check_backends.py output/<case>
  python3 check_backends.py output/<case> --check

Without --check it prints the backend list and the kubectl commands to run.
With --check it runs kubectl against the current kubeconfig and reports whether
each Service exists and exposes the requested port.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from typing import Any

try:
    import yaml
except ImportError:
    sys.exit("PyYAML is required: pip install pyyaml")

ROUTE_KINDS = {"HTTPRoute", "TCPRoute", "UDPRoute", "GRPCRoute", "TLSRoute"}


def load(path: str) -> list[dict[str, Any]]:
    files: list[str] = []
    if os.path.isdir(path):
        for root, _, names in os.walk(path):
            for n in sorted(names):
                if n.endswith((".yaml", ".yml")):
                    files.append(os.path.join(root, n))
    else:
        files.append(path)
    docs: list[dict[str, Any]] = []
    for f in files:
        with open(f, "r", encoding="utf-8") as fh:
            docs.extend([d for d in yaml.safe_load_all(fh) if d])
    return docs


def collect(docs: list[dict[str, Any]]) -> list[dict[str, Any]]:
    backend_ports: dict[tuple[str, str], Any] = {}
    for doc in docs:
        if doc.get("kind") != "Backend":
            continue
        meta = doc.get("metadata") or {}
        endpoints = ((doc.get("spec") or {}).get("endpoints") or [])
        port = None
        if endpoints:
            port = ((endpoints[0] or {}).get("fqdn") or {}).get("port")
        backend_ports[(str(meta.get("namespace") or "default"), str(meta.get("name") or ""))] = port

    rows: list[dict[str, Any]] = []
    for doc in docs:
        kind = doc.get("kind")
        if kind not in ROUTE_KINDS:
            continue
        meta = doc.get("metadata") or {}
        route_ns = meta.get("namespace") or "default"
        route_name = meta.get("name") or "<unnamed>"
        for ridx, rule in enumerate((doc.get("spec") or {}).get("rules") or []):
            for backend in rule.get("backendRefs") or []:
                backend_kind = backend.get("kind") or "Service"
                backend_ns = backend.get("namespace") or route_ns
                port = backend.get("port")
                if backend_kind == "Backend":
                    port = backend_ports.get((str(backend_ns), str(backend.get("name") or "")), port)
                rows.append(
                    {
                        "route_kind": kind,
                        "route_ns": route_ns,
                        "route_name": route_name,
                        "rule": ridx,
                        "group": backend.get("group") or "",
                        "kind": backend_kind,
                        "name": backend.get("name"),
                        "namespace": backend_ns,
                        "port": port,
                    }
                )
    return rows


def check_kubectl(row: dict[str, Any]) -> tuple[str, str]:
    cmd = [
        "kubectl",
        "get",
        "service",
        row["name"],
        "-n",
        row["namespace"],
        "-o",
        "json",
    ]
    try:
        out = subprocess.check_output(cmd, stderr=subprocess.STDOUT, text=True)
    except FileNotFoundError:
        return "MISSING", "kubectl not found"
    except subprocess.CalledProcessError as exc:
        return "MISSING", (exc.output or "").strip().splitlines()[0] if exc.output else "kubectl failed"
    try:
        svc = json.loads(out)
    except json.JSONDecodeError:
        return "MISSING", "cannot parse kubectl output"
    ports = {p.get("port") for p in (svc.get("spec") or {}).get("ports") or []}
    if row["port"] in ports:
        return "OK", ""
    return "PORT_MISSING", f"service ports: {sorted(str(p) for p in ports)}"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("yaml_file", help="YAML file or directory")
    parser.add_argument("--check", action="store_true", help="Run kubectl against the current kubeconfig")
    args = parser.parse_args()

    docs = load(args.yaml_file)
    rows = collect(docs)
    if not rows:
        print("没有找到 backendRefs。")
        return

    print("# Backend 检查")
    print()
    print("| Route | Backend | Namespace | Port |")
    print("| --- | --- | --- | ---: |")
    for row in rows:
        print(
            f"| {row['route_kind']} {row['route_ns']}/{row['route_name']} rule {row['rule']} "
            f"| {row['kind']} {row['name']} | {row['namespace']} | {row['port']} |"
        )
    print()

    print("## 手工检查命令")
    print()
    print("```shell")
    for row in rows:
        if row["kind"] == "Backend":
            print(f"kubectl get backend -n {row['namespace']} {row['name']} -o yaml")
        else:
            print(
                f"kubectl get svc -n {row['namespace']} {row['name']} "
                f"-o jsonpath='{{.spec.ports[*].port}}'"
            )
    print("```")
    print()

    if not args.check:
        print("加 `--check` 会直接用当前 kubeconfig 执行上面的检查。")
        return

    print("## kubectl 检查结果")
    print()
    missing = 0
    for row in rows:
        if row["kind"] != "Service" or row["group"] not in ("", "core"):
            print(f"- SKIP {row['kind']} {row['namespace']}/{row['name']}（非 Service backend，手工检查）")
            continue
        status, detail = check_kubectl(row)
        if status == "OK":
            print(f"- OK {row['namespace']}/{row['name']}:{row['port']}")
        else:
            missing += 1
            print(f"- {status} {row['namespace']}/{row['name']}:{row['port']} {detail}")
    if missing:
        print()
        print(f"共 {missing} 个 backend 需要确认。Service/port 不存在属于用户环境问题。")
        sys.exit(1)


if __name__ == "__main__":
    main()
