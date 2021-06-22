#! /usr/bin/python3
import json
import sys


def gen_rule(lb_name, ft_name, ft_uid, backend_ns, backend_svc, global_ns, rule_index, rule_priority):
    return {
        "apiVersion": "crd.alauda.io/v1",
        "kind": "Rule",
        "metadata": {
            "annotations": {
                "alauda.io/creator": "admin@alauda.io"
            },
            "labels": {
                "alb2.alauda.io/frontend": ft_name,
                "alb2.alauda.io/name": lb_name
            },
            "name": f"{ft_name}-{rule_index}",
            "namespace": global_ns,
            "ownerReferences": [
                {
                    "apiVersion": "crd.alauda.io/v1",
                    "kind": "frontends",
                    "name": ft_name,
                    "uid": ft_uid,
                }
            ]
        },
        "spec": {
            "backendProtocol": "HTTP",
            "description": "rule1",
            "domain": "",
            "dsl": f"(AND (STARTS_WITH URL /rule-{rule_index}))",
            "dslx": [
                {
                    "type": "URL",
                    "values": [
                        [
                            "STARTS_WITH",
                            f"/rule-{rule_index}"
                        ]
                    ]
                }
            ],
            "priority": rule_priority,
            "redirectURL": "",
            "rewrite_base": "",
            "rewrite_target": "",
            "serviceGroup": {
                "services": [
                    {
                        "name": f"{backend_svc}",
                        "namespace": backend_ns,
                        "port": 80,
                        "weight": 100
                    }
                ],
                "session_affinity_attribute": "",
                "session_affinity_policy": ""
            },
            "url": f"/rule-{rule_index}"
        }
    }


FTUID = "d3e57e8d-dc57-4b93-b0ee-699b3d73b29b"


def gen_rule_list(len, lb_name, ft_name, ft_uid, backend_ns, backend_svc, global_ns):
    rule_list = []
    for i in range(0, len):
        if i+1 == len:
            rule_list.append(gen_rule(lb_name, ft_name,
                             ft_uid, backend_ns, backend_svc, global_ns, i, 9))
        else:
            rule_list.append(gen_rule(lb_name, ft_name,
                             ft_uid, backend_ns, backend_svc, global_ns, i, 5))
    return {"apiVersion": "v1", "items": rule_list, "kind": "List"}


print(json.dumps(gen_rule_list(
    500, sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5],sys.argv[6]), indent=4))
