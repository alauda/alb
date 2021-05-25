#!/bin/bash
# clean alb environment which inited by init-alb-kind
kubectl delete secret -n cpaas-system alauda-harbor
helm uninstall alauda-alb2
helm uninstall alauda-cluster-base
kubectl delete ns cpaas-system