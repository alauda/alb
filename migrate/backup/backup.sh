#!/bin/bash

function _clearup_cr() (
  yq4 eval 'del(.metadata.ownerReferences,.metadata.uid,.metadata.resourceVersion,.metadata.generation,.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"],.status)' -
)

function _ignore_default_alb() (
  grep -v 'cpaas-system' | grep -v 'global-alb2'
)

function backup_port_info() (
  echo "backup port info"
  while read -r cm; do
    echo "port info $cm"
    kubectl get configmap -n cpaas-system $cm -o yaml | _clearup_cr >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get configmap -n cpaas-system | _ignore_default_alb | grep port-info | awk '{print $1}')
)

function backup_user_rule() (
  echo "backup user created rule"
  while read -r rule; do
    echo "backup user created rule $rule"
    kubectl get rule -n cpaas-system $rule -o yaml | _clearup_cr >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get rule -n cpaas-system -o=custom-columns='NAME:.metadata.name,SOURCE_TYPE:.spec.source.type' | _ignore_default_alb | grep -v ingress | tail -n +2 | awk '{print $1}')
)

function _list_cert() (
  (
    kubectl get rule -n cpaas-system -o=custom-columns='NAME:.metadata.name,SOURCE_TYPE:.spec.certificate_name' | _ignore_default_alb | tail -n+2 | awk '{print $2}' | sort | uniq
    kubectl get frontends.crd.alauda.io -n cpaas-system -o=custom-columns='NAME:.metadata.name,SOURCE_TYPE:.spec.certificate_name' | _ignore_default_alb | tail -n+2 | awk '{print $2}' | sort | uniq
  ) | sort | uniq | grep -v 'none'
)

function backup_cert() {
  echo "backup cert"
  while read -r cert; do
    if [[ -z "$cert" ]]; then
      continue
    fi
    local ns="${cert%%_*}"
    local secret="${cert#*_}"
    echo "backup cert $cert | $ns | $secret"
    kubectl get secret -n $ns $secret -o yaml | _clearup_cr >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(_list_cert)
}

function backup_ingress() (
  echo "backup user created ingress"
  while read -r ns name; do
    echo "ing $ns | $name"
    kubectl get ingress -n $ns $name -o yaml | _clearup_cr >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get ingress -A | _ignore_default_alb | tail -n +2 | awk '{print $1,$2}')

)

function backup_alb() (
  echo "backup user created alb"
  while read -r name; do
    echo "alb $name"
    kubectl get alb2.v2beta1.crd.alauda -n cpaas-system $name -o yaml | _clearup_cr >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get alb2.v2beta1.crd.alauda -n cpaas-system | tail -n +2 | awk '{print $1}' | _ignore_default_alb)
)

function backup_ft() (
  echo "backup user created ft"
  while read -r name; do
    echo "ft $name"
    kubectl get frontends.crd.alauda.io -n cpaas-system $name -o yaml | _clearup_cr >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get frontends.crd.alauda.io -n cpaas-system | tail -n +2 | awk '{print $1}' | _ignore_default_alb)
)

if ! command -v yq4 >/dev/null; then
  echo "yq4 not found"
  exit
fi

backup_alb
backup_ft
backup_port_info
backup_user_rule
backup_cert
backup_ingress
