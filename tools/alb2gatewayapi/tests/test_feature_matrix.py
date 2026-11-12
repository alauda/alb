#!/usr/bin/env python3
"""Synthetic feature-matrix regression checks.

Run from the tool directory:
  python3 tests/test_feature_matrix.py
"""
from __future__ import annotations

import glob
import os
import subprocess
import sys
import tempfile

import yaml

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, "scripts"))
from generate_gatewayapi import matches_for_rule  # noqa: E402
ALB = "matrix-alb"


def meta(name, labels=None, annotations=None):
    item = {"name": name, "namespace": "demo"}
    if labels:
        item["labels"] = labels
    if annotations:
        item["annotations"] = annotations
    return item


def frontend(name, port, protocol, **spec):
    return {
        "apiVersion": "crd.alauda.io/v1",
        "kind": "Frontend",
        "metadata": meta(name, labels={"alb2.cpaas.io/name": ALB}),
        "spec": {"port": port, "protocol": protocol, **spec},
    }


def rule(name, frontend_name, dslx, **spec):
    body = {
        "source": {"name": "demo-ingress", "namespace": "demo", "type": "ingress"},
        "dslx": dslx,
        "serviceGroup": {
            "services": [{"name": "hello", "namespace": "backend", "port": 8080, "weight": 100}]
        },
        **spec,
    }
    return {
        "apiVersion": "crd.alauda.io/v1",
        "kind": "Rule",
        "metadata": meta(name, labels={"alb2.cpaas.io/name": ALB, "alb2.cpaas.io/frontend": frontend_name}),
        "spec": body,
    }


def direct_rule(name, frontend_name, dslx, **spec):
    item = rule(name, frontend_name, dslx, **spec)
    item["spec"].pop("source", None)
    return item


def host(value):
    return {"type": "HOST", "values": [["EQ", value]]}


def url(op, value):
    return {"type": "URL", "values": [[op, value]]}


def load_docs(path):
    docs = []
    for filename in glob.glob(os.path.join(path, "**", "*.yaml"), recursive=True):
        with open(filename, encoding="utf-8") as fh:
            docs.extend(d for d in yaml.safe_load_all(fh) if d)
    return docs


def generate(docs):
    tmp = tempfile.TemporaryDirectory(prefix="alb2gw-matrix-")
    root = tmp.name
    input_dir = os.path.join(root, "input")
    output_dir = os.path.join(root, "output")
    os.makedirs(input_dir)
    with open(os.path.join(input_dir, "input.yaml"), "w", encoding="utf-8") as fh:
        yaml.safe_dump_all(docs, fh, sort_keys=False)
    subprocess.run(
        [
            sys.executable,
            os.path.join(ROOT, "scripts", "generate_gatewayapi.py"),
            input_dir,
            output_dir,
            "--gateway-namespace",
            "gateway-system",
        ],
        check=True,
        cwd=ROOT,
    )
    return tmp, output_dir, load_docs(output_dir)


def base_docs():
    main = [host("app.example.com"), url("STARTS_WITH", "/")]
    docs = [
        {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": "demo"}},
        {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": "backend"}},
        {"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": "cert"}},
        {
            "apiVersion": "crd.alauda.io/v2beta1",
            "kind": "ALB2",
            "metadata": meta(ALB, annotations={"cpaas.io/display-name": "Matrix ALB"}),
            "spec": {
                "type": "nginx",
                "config": {"networkMode": "container", "loadbalancerName": ALB, "ingressSSLRedirect": True},
            },
        },
        frontend("fe-http", 80, "http"),
        frontend("fe-https", 443, "https", certificate_name="cert/tls-secret"),
        frontend("fe-orphan-https", 8443, "https", certificate_name="cert/orphan-tls"),
        frontend(
            "fe-timeout",
            8080,
            "http",
            config={"timeout": {"proxy_connect_timeout_ms": 3000, "proxy_read_timeout_ms": 20000}},
        ),
        frontend("fe-tcp", 9000, "tcp"),
        frontend(
            "fe-l4",
            9002,
            "tcp",
            serviceGroup={
                "services": [
                    {"name": "l4-a", "namespace": "backend", "port": 9002, "weight": 70},
                    {"name": "l4-b", "namespace": "backend", "port": 9002, "weight": 30},
                ]
            },
        ),
        {
            "apiVersion": "networking.k8s.io/v1",
            "kind": "Ingress",
            "metadata": meta("demo-ingress"),
            "spec": {"rules": [{"host": "app.example.com"}]},
        },
        rule("r-main", "fe-http", main, vhost="hello.backend.svc.cluster.local"),
        rule("r-main-https", "fe-https", main, vhost="hello.backend.svc.cluster.local"),
        rule("r-timeout", "fe-timeout", [host("timeout.example.com"), url("STARTS_WITH", "/")]),
        rule(
            "r-fixed",
            "fe-http",
            [host("fixed.example.com"), url("STARTS_WITH", "/")],
            config={"redirect": {"url": "https://fixed.example.com/fixed?x=1", "code": 301}},
        ),
        rule(
            "r-hostvar",
            "fe-http",
            [host("hostvar.example.com"), url("STARTS_WITH", "/")],
            config={"redirect": {"url": "https://$host$request_uri", "host": "$host", "code": 308}},
        ),
        rule(
            "r-prefix-bad",
            "fe-http",
            [host("prefix.example.com"), url("STARTS_WITH", "/other")],
            config={"redirect": {"prefix_match": "/old", "replace_prefix": "/new"}},
        ),
        rule(
            "r-cors-empty",
            "fe-http",
            [host("cors.example.com"), url("STARTS_WITH", "/")],
            enableCORS=True,
            corsAllowOrigin="",
        ),
        rule(
            "r-match",
            "fe-http",
            [
                host("match.example.com"),
                url("REGEX", "^/users/[0-9]+$"),
                {"type": "METHOD", "values": [["EQ", "GET"], ["EQ", "POST"]]},
                {"type": "HEADER", "key": "X-Tenant", "values": [["EQ", "blue"]]},
                {"type": "HEADER", "key": "X-Canary", "values": [["REGEX", "^canary.*"]]},
                {"type": "HEADER", "key": "X-Multi", "values": [["EQ", "a"], ["EQ", "b"]]},
                {"type": "HEADER", "key": "X-Search", "values": [["REGEX", "canary"]]},
                {"type": "PARAM", "key": "debug", "values": [["EQ", "1"]]},
                {"type": "COOKIE", "values": [["EQ", "session"]]},
                {"type": "SRC_IP", "values": [["EQ", "10.0.0.1"]]},
            ],
        ),
        direct_rule("r-direct", "fe-http", [host("direct.example.com"), url("STARTS_WITH", "/direct")]),
    ]
    return docs


def assert_base():
    tmp, out, docs = generate(base_docs())
    with tmp:
        report = open(os.path.join(out, "conversion-report.md"), encoding="utf-8").read()
        gateway = next(d for d in docs if d.get("kind") == "Gateway")
        listeners = {item["name"]: item for item in gateway["spec"]["listeners"]}
        assert "https-8443" in listeners
        assert any(
            ref.get("name") == "orphan-tls"
            for ref in listeners["https-8443"]["tls"]["certificateRefs"]
        ), "Frontend certificate must be used even without a Rule"
        assert any(d.get("kind") == "ReferenceGrant" and d["metadata"]["namespace"] == "cert" for d in docs)

        multi_dir = os.path.join(out, ALB, "routes", "multi-listener")
        multi_text = "".join(
            open(path, encoding="utf-8").read() for path in glob.glob(os.path.join(multi_dir, "*.yaml"))
        )
        assert "Source Rule(s): r-main, r-main-https" in multi_text
        assert "http-80" in multi_text and "https-443" in multi_text

        timeout_policies = [
            d for d in docs
            if d.get("kind") == "BackendTrafficPolicy"
            and "http-8080" in " ".join(t.get("sectionName", "") for t in d["spec"].get("targetRefs") or [])
        ]
        assert timeout_policies, "Frontend timeout should generate a listener policy"

        redirects = {}
        for doc in docs:
            if doc.get("kind") != "HTTPRoute":
                continue
            for rule_doc in doc["spec"]["rules"]:
                for flt in rule_doc.get("filters") or []:
                    if flt.get("type") == "RequestRedirect":
                        redirects[doc["metadata"]["name"]] = flt["requestRedirect"]
        fixed = next(rr for name, rr in redirects.items() if "fixed" in name)
        assert fixed["path"]["replaceFullPath"] == "/fixed"
        assert "?" not in fixed["path"]["replaceFullPath"]
        hostvar = next(rr for name, rr in redirects.items() if "hostvar" in name)
        assert "hostname" not in hostvar
        prefix_route = next(
            d for d in docs if d.get("kind") == "HTTPRoute"
            and d["metadata"]["name"].startswith("demo-demo-ingress-prefix")
        )
        assert not any(
            flt.get("type") == "RequestRedirect"
            for rule_doc in prefix_route["spec"]["rules"]
            for flt in rule_doc.get("filters") or []
        )
        assert prefix_route["spec"]["rules"][0]["backendRefs"]

        cors = next(
            flt["cors"]
            for d in docs if d.get("kind") == "HTTPRoute"
            for rule_doc in d["spec"]["rules"]
            for flt in rule_doc.get("filters") or []
            if flt.get("type") == "CORS"
        )
        assert cors["allowOrigins"] == ["*"]
        assert cors["allowCredentials"] is False

        match_route = next(
            d for d in docs if d.get("kind") == "HTTPRoute"
            and d["metadata"]["name"].startswith("demo-demo-ingress-match")
        )
        match = match_route["spec"]["rules"][0]["matches"][0]
        assert match["headers"][0]["name"] == "X-Tenant"
        assert match["queryParams"][0]["name"] == "debug"
        assert "COOKIE 匹配未转换" in report and "SRC_IP 匹配未转换" in report
        assert "ingressSSLRedirect" in report and "tcp 但没有 serviceGroup" in report
        assert "CORS filter 1" in report
        assert os.path.exists(os.path.join(out, "warnings.md"))
        warnings_text = open(os.path.join(out, "warnings.md"), encoding="utf-8").read()
        assert "固定 query" in warnings_text

        matches = match_route["spec"]["rules"][0]["matches"]
        assert {m.get("method") for m in matches} == {"GET", "POST"}
        assert any(
            h.get("name") == "X-Canary" and h.get("type") == "RegularExpression"
            for m in matches for h in m.get("headers") or []
        )
        assert any(
            h.get("name") == "X-Search" and h.get("value") == ".*canary.*"
            for m in matches for h in m.get("headers") or []
        )
        assert {h.get("value") for m in matches for h in m.get("headers") or [] if h.get("name") == "X-Multi"} == {"a", "b"}
        for m in matches:
            names = [h.get("name") for h in m.get("headers") or []]
            assert len(names) == len(set(names))

        direct_route = next(d for d in docs if d.get("kind") == "HTTPRoute" and d["metadata"]["name"] == "r-direct")
        assert direct_route["spec"]["hostnames"] == ["direct.example.com"]

        l4_route = next(d for d in docs if d.get("kind") == "TCPRoute" and d["metadata"]["name"] == "fe-l4")
        assert [ref.get("weight") for ref in l4_route["spec"]["rules"][0]["backendRefs"]] == [70, 30]


def assert_regex_alternation():
    matches = matches_for_rule(
        {
            "dslx": [
                {"type": "URL", "values": [["STARTS_WITH", "/"]]},
                {"type": "HEADER", "key": "X-Alt", "values": [["REGEX", "a|b"]]},
            ]
        },
        [],
        "regex-test",
    )
    assert matches[0]["headers"][0]["value"] == ".*(?:a|b).*"


def split_docs():
    docs = base_docs()
    # Keep the base rules; add one host with 17 distinct paths to force chunking.
    for index in range(17):
        docs.append(rule(f"split-{index:02d}", "fe-http", [host("split.example.com"), url("STARTS_WITH", f"/p{index:02d}")]))
    return docs


def assert_split():
    tmp, out, _ = generate(split_docs())
    with tmp:
        paths = sorted(glob.glob(os.path.join(out, ALB, "routes", "80", "*split*")))
        assert len(paths) == 2, paths
        texts = [open(path, encoding="utf-8").read() for path in paths]
        assert sum("split-00" in text for text in texts) == 1
        assert sum("split-15" in text for text in texts) == 1
        assert sum("split-16" in text for text in texts) == 1
        split_report = open(os.path.join(out, "conversion-report.md"), encoding="utf-8").read()
        assert "HTTPRoute 拆分" in split_report


if __name__ == "__main__":
    assert_base()
    assert_regex_alternation()
    assert_split()
    print("feature matrix regression checks passed")
