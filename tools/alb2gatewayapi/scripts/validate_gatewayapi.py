#!/usr/bin/env python3
"""Validate generated Gateway API YAML files.

Usage:
  python3 validate_gatewayapi.py output/<case>
  python3 validate_gatewayapi.py output/<case>/<alb>

Checks:
  - YAML parses.
  - Every Gateway has spec.infrastructure.parametersRef to an EnvoyProxy.
  - The referenced EnvoyProxy exists in the same namespace.
  - Listener names are unique and each Gateway has <= 63 listeners.
  - Every HTTPRoute/TCPRoute/UDPRoute/GRPCRoute parentRef sectionName resolves
    to a listener on the referenced Gateway.
  - Every route has at least one backendRef.
"""

from __future__ import annotations

import collections
import os
import re
import sys
from typing import Any

try:
    import yaml
except ImportError:
    sys.exit("PyYAML is required: pip install pyyaml")

MAX_LISTENERS = 63
ROUTE_KINDS = {"HTTPRoute", "TCPRoute", "UDPRoute", "GRPCRoute", "TLSRoute"}
PATH_VALUE_RE = re.compile(r"^(?:[-A-Za-z0-9/._~!$&'()*+,;=:@]|[%][0-9a-fA-F]{2})+$")


def load_docs(path: str) -> list[dict[str, Any]]:
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


def ns(obj: dict[str, Any]) -> str:
    return (obj.get("metadata") or {}).get("namespace") or "default"


def name(obj: dict[str, Any]) -> str:
    return (obj.get("metadata") or {}).get("name") or "<unnamed>"


def main() -> None:
    if len(sys.argv) == 2 and sys.argv[1] in ("-h", "--help"):
        print("usage: validate_gatewayapi.py <yaml-file-or-directory>")
        return
    if len(sys.argv) != 2:
        sys.exit("usage: validate_gatewayapi.py <yaml-file-or-directory>")
    path = sys.argv[1]
    docs = load_docs(path)

    gateways: dict[tuple[str, str], dict[str, Any]] = {}
    envoyproxies: set[tuple[str, str]] = set()
    backends: set[tuple[str, str]] = set()
    route_filters: set[tuple[str, str]] = set()
    routes: list[dict[str, Any]] = []
    policies: list[dict[str, Any]] = []
    resources: set[tuple[str, str, str]] = set()
    seen_objects: set[tuple[str, str, str]] = set()
    errors: list[str] = []
    warnings: list[str] = []
    kinds = collections.Counter()

    for doc in docs:
        kind = doc.get("kind")
        kinds[str(kind)] += 1
        ident = (str(kind), ns(doc), name(doc))
        if ident in seen_objects:
            errors.append(f"duplicate object {kind} {ns(doc)}/{name(doc)}")
        seen_objects.add(ident)
        if kind == "Gateway":
            gateways[(ns(doc), name(doc))] = doc
            resources.add((kind, ns(doc), name(doc)))
        elif kind == "EnvoyProxy":
            envoyproxies.add((ns(doc), name(doc)))
        elif kind == "Backend":
            backends.add((ns(doc), name(doc)))
        elif kind == "HTTPRouteFilter":
            route_filters.add((ns(doc), name(doc)))
        elif kind in ROUTE_KINDS:
            routes.append(doc)
            resources.add((kind, ns(doc), name(doc)))
        elif kind in ("BackendTrafficPolicy", "ClientTrafficPolicy", "SecurityPolicy"):
            policies.append(doc)

    if not gateways:
        errors.append("no Gateway found")
    if not routes:
        warnings.append("no Route found")

    listener_policy: dict[tuple[str, str, str], tuple[str, str]] = {}
    for key, gw in gateways.items():
        gns, gname = key
        spec = gw.get("spec") or {}
        if gns == "cpaas-system":
            warnings.append(
                f"Gateway {gns}/{gname}: user Gateway is in the system namespace cpaas-system; "
                "prefer the backend namespace or a dedicated gateway namespace"
            )
        ref = (spec.get("infrastructure") or {}).get("parametersRef")
        if not ref:
            errors.append(f"Gateway {gns}/{gname}: missing spec.infrastructure.parametersRef")
        else:
            if ref.get("kind") != "EnvoyProxy":
                errors.append(f"Gateway {gns}/{gname}: parametersRef kind is {ref.get('kind')!r}, expected EnvoyProxy")
            ref_ns = ref.get("namespace") or gns
            ref_name = ref.get("name")
            if not ref_name:
                errors.append(f"Gateway {gns}/{gname}: parametersRef has no name")
            elif (ref_ns, ref_name) not in envoyproxies:
                errors.append(
                    f"Gateway {gns}/{gname}: parametersRef {ref_ns}/{ref_name} does not resolve to an EnvoyProxy in this file"
                )

        listeners = spec.get("listeners") or []
        if len(listeners) > MAX_LISTENERS:
            errors.append(f"Gateway {gns}/{gname}: {len(listeners)} listeners > {MAX_LISTENERS}")
        names = [str(l.get("name")) for l in listeners]
        dupes = [n for n, c in collections.Counter(names).items() if c > 1]
        if dupes:
            errors.append(f"Gateway {gns}/{gname}: duplicate listener names: {dupes}")
        for listener in listeners:
            lname = str(listener.get("name"))
            if not listener.get("name"):
                errors.append(f"Gateway {gns}/{gname}: listener without name")
            if listener.get("port") is None or not listener.get("protocol"):
                errors.append(f"Gateway {gns}/{gname}: listener {listener.get('name')!r} missing port/protocol")
            if listener.get("protocol") == "HTTPS":
                tls = listener.get("tls") or {}
                if not (tls.get("certificateRefs") or []):
                    warnings.append(f"Gateway {gns}/{gname}: HTTPS listener {listener.get('name')} has no certificateRefs")
            allowed = ((listener.get("allowedRoutes") or {}).get("namespaces") or {})
            from_value = allowed.get("from") or "Same"
            listener_policy[(gns, gname, lname)] = (str(from_value), gns)
            if from_value == "All":
                warnings.append(
                    f"Gateway {gns}/{gname}: listener {lname} uses allowedRoutes from: All; "
                    "prefer Same or Selector for user workloads"
                )

    for route in routes:
        rns, rname = ns(route), name(route)
        kind = route.get("kind")
        spec = route.get("spec") or {}
        parents = spec.get("parentRefs") or []
        if not parents:
            errors.append(f"{kind} {rns}/{rname}: no parentRefs")
        for parent in parents:
            p_kind = parent.get("kind") or "Gateway"
            p_ns = parent.get("namespace") or rns
            p_name = parent.get("name")
            if p_kind != "Gateway":
                # ListenerSet references are allowed; do not try to resolve them here.
                continue
            key = (p_ns, p_name)
            if key not in gateways:
                errors.append(f"{kind} {rns}/{rname}: parentRef Gateway {p_ns}/{p_name} not found")
                continue
            section = parent.get("sectionName")
            if section:
                listeners = (gateways[key].get("spec") or {}).get("listeners") or []
                if section not in [l.get("name") for l in listeners]:
                    errors.append(
                        f"{kind} {rns}/{rname}: parentRef sectionName {section!r} not found on Gateway {p_ns}/{p_name}"
                    )
                else:
                    from_value, gateway_ns = listener_policy.get((p_ns, p_name, str(section)), ("Same", p_ns))
                    if from_value == "Same" and rns != gateway_ns:
                        errors.append(
                            f"{kind} {rns}/{rname}: route namespace {rns} cannot attach to "
                            f"Gateway {p_ns}/{p_name} listener {section} with allowedRoutes from: Same "
                            f"(Gateway namespace is {gateway_ns})"
                        )
            else:
                warnings.append(
                    f"{kind} {rns}/{rname}: parentRef has no sectionName; "
                    "attaches to all compatible listeners, verify allowedRoutes manually"
                )
        hostnames = [str(h).lower() for h in spec.get("hostnames") or []]
        if len(hostnames) != len(set(hostnames)):
            errors.append(f"{kind} {rns}/{rname}: duplicate hostnames")
        rules = spec.get("rules") or []
        if not rules:
            errors.append(f"{kind} {rns}/{rname}: no rules")
        for idx, rule in enumerate(rules):
            for match in rule.get("matches") or []:
                path_match = (match or {}).get("path") or {}
                if path_match.get("type") in ("PathPrefix", "Exact") and not PATH_VALUE_RE.fullmatch(str(path_match.get("value") or "")):
                    errors.append(
                        f"{kind} {rns}/{rname}: rule {idx} path {path_match.get('value')!r} contains characters invalid for {path_match.get('type')}"
                    )
                for field in ("headers", "queryParams"):
                    names = [str(item.get("name") or "") for item in (match or {}).get(field) or []]
                    if len(names) != len(set(names)):
                        errors.append(
                            f"{kind} {rns}/{rname}: rule {idx} {field} has duplicate name keys; "
                            "Gateway API list-map keys must be unique"
                        )
            filters = rule.get("filters") or []
            has_redirect = any((f or {}).get("type") == "RequestRedirect" for f in filters)
            backend_refs = rule.get("backendRefs") or []
            if has_redirect:
                if backend_refs:
                    errors.append(
                        f"{kind} {rns}/{rname}: rule {idx} has RequestRedirect together with backendRefs; "
                        "Gateway API requires backendRefs to be empty for redirect rules"
                    )
                for flt in filters:
                    hostname = str(((flt or {}).get("requestRedirect") or {}).get("hostname") or "")
                    if "$" in hostname:
                        errors.append(
                            f"{kind} {rns}/{rname}: rule {idx} RequestRedirect hostname {hostname!r} contains an "
                            "nginx variable; only scheme/hostname/path/statusCode are supported"
                        )
            elif not backend_refs:
                errors.append(f"{kind} {rns}/{rname}: rule {idx} has no backendRefs")
            for backend in backend_refs:
                if not backend.get("name"):
                    errors.append(f"{kind} {rns}/{rname}: rule {idx} backendRef without name")
                    continue
                bkind = backend.get("kind") or "Service"
                bns = backend.get("namespace") or rns
                bname = backend.get("name")
                if bkind == "Backend":
                    if (bns, bname) not in backends:
                        errors.append(
                            f"{kind} {rns}/{rname}: rule {idx} backendRef Backend {bns}/{bname} not found"
                        )
                elif backend.get("port") is None:
                    warnings.append(f"{kind} {rns}/{rname}: rule {idx} backendRef {bname} has no port")
            for flt in filters:
                if (flt or {}).get("type") == "CORS":
                    cors = (flt or {}).get("cors") or {}
                    for key in ("allowOrigins", "allowHeaders", "allowMethods", "exposeHeaders"):
                        values = [str(v).lower() for v in cors.get(key) or []]
                        if len(values) != len(set(values)):
                            errors.append(
                                f"{kind} {rns}/{rname}: rule {idx} CORS {key} contains duplicates; API list is a set"
                            )
                    if "*" in (cors.get("allowOrigins") or []) and cors.get("allowCredentials"):
                        errors.append(
                            f"{kind} {rns}/{rname}: rule {idx} CORS allowOrigins '*' with allowCredentials=true"
                        )
                    continue
                if (flt or {}).get("type") != "ExtensionRef":
                    continue
                ref = (flt or {}).get("extensionRef") or {}
                if ref.get("kind") == "HTTPRouteFilter":
                    fns = ref.get("namespace") or rns
                    fname = ref.get("name")
                    if (fns, fname) not in route_filters:
                        errors.append(
                            f"{kind} {rns}/{rname}: rule {idx} ExtensionRef HTTPRouteFilter {fns}/{fname} not found"
                        )

    for policy in policies:
        pkind = policy.get("kind")
        pns, pname = ns(policy), name(policy)
        for target in (policy.get("spec") or {}).get("targetRefs") or []:
            tkind = target.get("kind")
            tns = target.get("namespace") or pns
            tname = target.get("name")
            if tkind not in ("Gateway", "HTTPRoute", "TCPRoute", "UDPRoute", "GRPCRoute", "TLSRoute"):
                continue
            if (tkind, tns, tname) not in resources:
                errors.append(f"{pkind} {pns}/{pname}: targetRef {tkind} {tns}/{tname} not found")
                continue
            section = target.get("sectionName")
            if tkind == "Gateway" and section:
                listeners = (gateways[(tns, tname)].get("spec") or {}).get("listeners") or []
                if section not in [l.get("name") for l in listeners]:
                    errors.append(
                        f"{pkind} {pns}/{pname}: targetRef Gateway {tns}/{tname} sectionName {section!r} not found"
                    )

    print(f"# Gateway API validation: {path}")
    print()
    print("## Object counts")
    print()
    print("| Kind | Count |")
    print("| --- | ---: |")
    for kind, count in sorted(kinds.items()):
        print(f"| {kind} | {count} |")
    print()
    print("## Errors")
    print()
    if errors:
        for err in errors:
            print(f"- {err}")
    else:
        print("- None.")
    print()
    print("## Warnings")
    print()
    if warnings:
        for warning in warnings:
            print(f"- {warning}")
    else:
        print("- None.")
    print()

    if errors:
        sys.exit(1)


if __name__ == "__main__":
    main()
