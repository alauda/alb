#!/usr/bin/env python3
"""Offline regression checks for the public ALB-to-Gateway-API example.
Run from the tool directory:
  python3 tests/test_high_fixes.py
"""
from __future__ import annotations

import glob
import os
import subprocess
import sys
import tempfile

import yaml


ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def load_docs(path: str) -> list[dict]:
    docs: list[dict] = []
    for filename in glob.glob(os.path.join(path, "**", "*.yaml"), recursive=True):
        with open(filename, "r", encoding="utf-8") as fh:
            docs.extend([d for d in yaml.safe_load_all(fh) if d])
    return docs


def generate(input_dir: str) -> tuple[str, list[dict], str]:
    out = tempfile.mkdtemp(prefix="alb2gw-example-")
    subprocess.run(
        [
            sys.executable,
            os.path.join(ROOT, "scripts", "generate_gatewayapi.py"),
            input_dir,
            out,
            "--gateway-namespace",
            "gateway-system",
        ],
        check=True,
        cwd=ROOT,
    )
    report = open(os.path.join(out, "conversion-report.md"), encoding="utf-8").read()
    return out, load_docs(out), report


def main() -> None:
    basic_out, basic_docs, basic_report = generate(os.path.join(ROOT, "examples/basic-http"))
    l4_out, l4_docs, _ = generate(os.path.join(ROOT, "examples/l4-tcp"))

    # H1: vhost is an inline URLRewrite; no EG filter string hostname.
    for doc in basic_docs:
        if doc.get("kind") == "HTTPRouteFilter":
            assert not isinstance((doc.get("spec") or {}).get("urlRewrite", {}).get("hostname"), str)
    assert any(
        flt.get("type") == "URLRewrite" and (flt.get("urlRewrite") or {}).get("hostname")
        for doc in basic_docs if doc.get("kind") == "HTTPRoute"
        for rule in (doc.get("spec") or {}).get("rules") or []
        for flt in rule.get("filters") or []
    )

    # H2: redirect keeps scheme and preserves path/query when $request_uri is present.
    redirects = [
        flt["requestRedirect"]
        for doc in basic_docs if doc.get("kind") == "HTTPRoute"
        for rule in (doc.get("spec") or {}).get("rules") or []
        for flt in rule.get("filters") or []
        if flt.get("type") == "RequestRedirect"
    ]
    assert redirects and all(rr.get("scheme") for rr in redirects)
    assert any("path" not in rr for rr in redirects)

    # H3: CORS methods, credentials, integer maxAge, no widened wildcard.
    cors = [
        flt["cors"]
        for doc in basic_docs if doc.get("kind") == "HTTPRoute"
        for rule in (doc.get("spec") or {}).get("rules") or []
        for flt in rule.get("filters") or []
        if flt.get("type") == "CORS"
    ]
    assert cors, "expected CORS filter"
    for item in cors:
        assert item.get("allowMethods") == ["GET", "PUT", "POST", "DELETE", "PATCH", "OPTIONS"]
        assert item.get("maxAge") == 1728000
        assert item.get("allowCredentials") is True
        assert not any("*" in origin and origin != "*" for origin in item.get("allowOrigins") or [])
        for key in ("allowOrigins", "allowHeaders", "allowMethods", "exposeHeaders"):
            values = item.get(key) or []
            assert len(values) == len({str(v).lower() for v in values}), (key, values)

    # L4 sip-hash policy remains attached to TCPRoute.
    assert any(
        doc.get("kind") == "BackendTrafficPolicy"
        and any(t.get("kind") in ("TCPRoute", "UDPRoute") for t in (doc.get("spec") or {}).get("targetRefs") or [])
        for doc in l4_docs
    )

    # H5: labels and audit/gap artifacts exist; H4: no case-specific literals.
    for out in (basic_out, l4_out):
        assert os.path.exists(os.path.join(out, "objects.txt"))
        assert os.path.exists(os.path.join(out, "feature-audit.md"))
        assert os.path.exists(os.path.join(out, "missing-resources.md"))
        assert os.path.exists(os.path.join(out, "fetch-missing.sh"))
    assert all(
        ((doc.get("metadata") or {}).get("labels") or {}).get("alb2gw.alauda.io/case")
        for doc in basic_docs if doc.get("kind") not in ("Namespace",)
    )
    assert "同样处理" not in basic_report

    print("public example regression checks passed")


if __name__ == "__main__":
    main()
