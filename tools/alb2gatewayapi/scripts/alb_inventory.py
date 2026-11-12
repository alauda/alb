#!/usr/bin/env python3
"""Inventory exported ALB2 / Frontend / Rule YAML.

Usage:
  python3 alb_inventory.py <dir-or-file> [--json]

The output is intentionally boring: counts, ports, source types, Ingress names,
annotations, config keys and feature flags. The AI and the conversion report
should use it as the fixed first pass instead of eyeballing the YAML.
"""

from __future__ import annotations

import argparse
import collections
import json
import os
import sys
from typing import Any, Iterable

try:
    import yaml
except ImportError:
    sys.exit("PyYAML is required: pip install pyyaml")

SYSTEM_ALBS = {"cpaas-system", "global-alb2"}
MAX_LISTENERS = 63


def load_docs(path: str) -> Iterable[tuple[str, dict[str, Any]]]:
    files: list[str] = []
    if os.path.isdir(path):
        for root, _, names in os.walk(path):
            for name in sorted(names):
                if name.endswith((".yaml", ".yml")):
                    files.append(os.path.join(root, name))
    else:
        files.append(path)

    for filename in files:
        with open(filename, "r", encoding="utf-8") as fh:
            try:
                docs = list(yaml.safe_load_all(fh))
            except yaml.YAMLError as exc:
                yield filename, {"__error__": str(exc)}
                continue
        for doc in docs:
            if not doc:
                continue
            if doc.get("kind") == "List":
                for item in doc.get("items") or []:
                    if item:
                        yield filename, item
            else:
                yield filename, doc


def metadata(item: dict[str, Any]) -> dict[str, Any]:
    return item.get("metadata") or {}


def alb_name(item: dict[str, Any]) -> str:
    meta = metadata(item)
    labels = meta.get("labels") or {}
    name = labels.get("alb2.cpaas.io/name")
    if name:
        return str(name)
    for owner in meta.get("ownerReferences") or []:
        if owner.get("kind") == "ALB2" and owner.get("name"):
            return str(owner["name"])
    if item.get("kind") == "ALB2":
        return str(meta.get("name") or "unknown")
    return "unknown"


def add_counter(counter: collections.Counter, value: Any) -> None:
    if value not in (None, "", [], {}):
        counter[str(value)] += 1


def rule_features(rule: dict[str, Any]) -> dict[str, Any]:
    spec = rule.get("spec") or {}
    meta = metadata(rule)
    annotations = meta.get("annotations") or {}
    config = spec.get("config") or {}
    dslx = spec.get("dslx") or []
    services = (spec.get("serviceGroup") or {}).get("services") or []
    return {
        "source_type": (spec.get("source") or {}).get("type"),
        "source_name": (spec.get("source") or {}).get("name"),
        "source_namespace": (spec.get("source") or {}).get("namespace"),
        "dslx_types": sorted({str(d.get("type")) for d in dslx if d.get("type")}),
        "dslx_ops": sorted(
            {
                str(op[0])
                for d in dslx
                for op in (d.get("values") or [])
                if isinstance(op, list) and op
            }
        ),
        "rewrite_base": bool(spec.get("rewrite_base")),
        "rewrite_target": bool(spec.get("rewrite_target")),
        "headers_var": any("headers_var" in str(v) or "headers_add_var" in str(v) for v in annotations.values()),
        "rewrite_request": any("rewrite-request" in k for k in annotations),
        "rewrite_response": any("rewrite-response" in k for k in annotations),
        "cors": bool(spec.get("enableCORS")),
        "timeout": bool(config.get("timeout")),
        "redirect": bool(config.get("redirect")),
        "session_affinity": (spec.get("serviceGroup") or {}).get("session_affinity_policy"),
        "backend_protocol": spec.get("backendProtocol"),
        "vhost": bool(spec.get("vhost")),
        "certificate": bool(spec.get("certificate_name")),
        "service_count": len(services),
    }


def inventory(path: str) -> dict[str, Any]:
    albs: dict[str, dict[str, Any]] = {}
    errors: list[str] = []
    kinds = collections.Counter()

    for filename, item in load_docs(path):
        if "__error__" in item:
            errors.append(f"{filename}: {item['__error__']}")
            continue
        kind = item.get("kind")
        if not kind:
            continue
        kinds[str(kind)] += 1
        if kind not in ("ALB2", "Frontend", "Rule"):
            continue

        name = alb_name(item)
        entry = albs.setdefault(
            name,
            {
                "name": name,
                "system": name in SYSTEM_ALBS,
                "alb2": 0,
                "frontends": 0,
                "rules": 0,
                "ports": set(),
                "protocols": collections.Counter(),
                "source_types": collections.Counter(),
                "ingresses": set(),
                "annotations": collections.Counter(),
                "config_keys": collections.Counter(),
                "dslx_types": collections.Counter(),
                "dslx_ops": collections.Counter(),
                "features": collections.Counter(),
            },
        )

        if kind == "ALB2":
            entry["alb2"] += 1
            continue

        spec = item.get("spec") or {}
        meta = metadata(item)
        for key in (meta.get("annotations") or {}):
            entry["annotations"][key] += 1

        if kind == "Frontend":
            entry["frontends"] += 1
            protocol = spec.get("protocol") or "unknown"
            port = spec.get("port")
            entry["protocols"][str(protocol)] += 1
            if port is not None:
                entry["ports"].add((str(protocol), int(port)))
            if spec.get("serviceGroup"):
                entry["features"]["frontend_service_group"] += 1
            if spec.get("certificate_name"):
                entry["features"]["certificate"] += 1
            continue

        # Rule
        entry["rules"] += 1
        source = spec.get("source") or {}
        if source.get("type") == "ingress":
            entry["source_types"]["ingress"] += 1
            if source.get("name"):
                entry["ingresses"].add(f"{source.get('namespace', '?')}/{source.get('name')}")
        else:
            entry["source_types"]["direct"] += 1
        for key in (spec.get("config") or {}):
            entry["config_keys"][key] += 1
        for d in spec.get("dslx") or []:
            if d.get("type"):
                entry["dslx_types"][str(d["type"])] += 1
            for op in d.get("values") or []:
                if isinstance(op, list) and op:
                    entry["dslx_ops"][str(op[0])] += 1
        feat = rule_features(item)
        if feat["rewrite_base"] or feat["rewrite_target"]:
            entry["features"]["rewrite"] += 1
        if feat["headers_var"]:
            entry["features"]["headers_var"] += 1
        if feat["rewrite_request"]:
            entry["features"]["rewrite_request"] += 1
        if feat["rewrite_response"]:
            entry["features"]["rewrite_response"] += 1
        if feat["cors"]:
            entry["features"]["cors"] += 1
        if feat["timeout"]:
            entry["features"]["timeout"] += 1
        if feat["redirect"]:
            entry["features"]["redirect"] += 1
        if feat["session_affinity"]:
            entry["features"][f"session_affinity:{feat['session_affinity']}"] += 1
        if feat["backend_protocol"]:
            entry["features"][f"backend_protocol:{feat['backend_protocol']}"] += 1
        if feat["vhost"]:
            entry["features"]["vhost"] += 1
        if feat["certificate"]:
            entry["features"]["certificate"] += 1

    # Make JSON friendly
    result_albs = []
    for entry in sorted(albs.values(), key=lambda e: (e["system"], e["name"])):
        entry["ports"] = sorted(entry["ports"])
        entry["ingresses"] = sorted(entry["ingresses"])
        entry["protocols"] = dict(entry["protocols"])
        entry["source_types"] = dict(entry["source_types"])
        entry["annotations"] = dict(entry["annotations"])
        entry["config_keys"] = dict(entry["config_keys"])
        entry["dslx_types"] = dict(entry["dslx_types"])
        entry["dslx_ops"] = dict(entry["dslx_ops"])
        entry["features"] = dict(entry["features"])
        entry["listener_count"] = len(entry["ports"])
        entry["listener_limit_ok"] = len(entry["ports"]) <= MAX_LISTENERS
        result_albs.append(entry)

    return {
        "path": path,
        "kinds": dict(kinds),
        "albs": result_albs,
        "errors": errors,
    }


def print_markdown(data: dict[str, Any]) -> None:
    print(f"# ALB inventory: `{data['path']}`")
    print()
    print("## Kinds")
    print()
    print("| Kind | Count |")
    print("| --- | ---: |")
    for kind, count in sorted(data["kinds"].items()):
        print(f"| {kind} | {count} |")
    print()
    print("## ALBs")
    print()
    print("| ALB | System | Frontends | Rules | Ports | Protocols | Ingress Rules | Direct Rules | Listener limit |")
    print("| --- | --- | ---: | ---: | ---: | --- | ---: | ---: | --- |")
    for alb in data["albs"]:
        print(
            f"| {alb['name']} | {'yes' if alb['system'] else 'no'} | {alb['frontends']} | "
            f"{alb['rules']} | {alb['listener_count']} | {', '.join(alb['protocols']) or '-'} | "
            f"{alb['source_types'].get('ingress', 0)} | {alb['source_types'].get('direct', 0)} | "
            f"{'OK' if alb['listener_limit_ok'] else 'OVER 63'} |"
        )
    print()

    for alb in data["albs"]:
        print(f"### {alb['name']}{' (system LB, skipped)' if alb['system'] else ''}")
        print()
        print(f"- ALB2 objects: {alb['alb2']}")
        print(f"- Frontends: {alb['frontends']}; Rules: {alb['rules']}")
        print(
            "- Ports: "
            + (", ".join(f"{proto}/{port}" for proto, port in alb["ports"]) or "-")
        )
        if alb["ingresses"]:
            print("- Ingresses: " + ", ".join(alb["ingresses"]))
        if alb["annotations"]:
            print("- Annotations: " + ", ".join(f"{k} ({v})" for k, v in sorted(alb["annotations"].items())))
        if alb["config_keys"]:
            print("- Config keys: " + ", ".join(f"{k} ({v})" for k, v in sorted(alb["config_keys"].items())))
        if alb["dslx_types"]:
            print("- DSLX types: " + ", ".join(f"{k} ({v})" for k, v in sorted(alb["dslx_types"].items())))
        if alb["dslx_ops"]:
            print("- DSLX operators: " + ", ".join(f"{k} ({v})" for k, v in sorted(alb["dslx_ops"].items())))
        if alb["features"]:
            print("- Features: " + ", ".join(f"{k} ({v})" for k, v in sorted(alb["features"].items())))
        if not alb["listener_limit_ok"]:
            print(f"- WARNING: {alb['listener_count']} ports > {MAX_LISTENERS}; see the product doc Listener limits section.")
        print()

    system = [a for a in data["albs"] if a["system"]]
    if system:
        print("## Skipped system LBs")
        print()
        for alb in system:
            print(f"- `{alb['name']}`: {alb['frontends']} Frontends, {alb['rules']} Rules")
        print()

    if data["errors"]:
        print("## Parse errors")
        print()
        for err in data["errors"]:
            print(f"- {err}")
        print()

    print("## Warnings")
    print()
    warnings = []
    for alb in data["albs"]:
        if alb["system"]:
            continue
        if not alb["listener_limit_ok"]:
            warnings.append(f"`{alb['name']}` has {alb['listener_count']} listeners (> {MAX_LISTENERS}).")
        if alb["features"].get("headers_var"):
            warnings.append(f"`{alb['name']}` uses `headers_var` / `headers_add_var`; use an `EnvoyExtensionPolicy` with Lua.")
        if alb["features"].get("vhost"):
            warnings.append(f"`{alb['name']}` uses `vhost`; map it to `URLRewrite.hostname`.")
        if any(k.startswith("backend_protocol:grpc") for k in alb["features"]):
            warnings.append(f"`{alb['name']}` has a gRPC backend; use `GRPCRoute` or `appProtocol: kubernetes.io/h2c`.")
        if any(k.startswith("session_affinity:header") for k in alb["features"]):
            warnings.append(f"`{alb['name']}` uses header affinity; use `BackendTrafficPolicy` consistent hashing on the header.")
    if warnings:
        for warning in warnings:
            print(f"- {warning}")
    else:
        print("- None. Continue with the normal mapping.")
    print()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("path", help="Input directory or YAML file")
    parser.add_argument("--json", action="store_true", help="Print JSON instead of Markdown")
    args = parser.parse_args()
    if not os.path.exists(args.path):
        sys.exit(f"input path does not exist: {args.path}")
    data = inventory(args.path)
    if args.json:
        print(json.dumps(data, indent=2, ensure_ascii=False))
    else:
        print_markdown(data)


if __name__ == "__main__":
    main()
