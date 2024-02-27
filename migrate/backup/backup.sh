#!/bin/bash

function backup_port_info() (
  echo "backup port info"
  while read -r cm; do
    echo "port info $cm"
    kubectl get configmap -n cpaas-system $cm -o yaml >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get configmap -n cpaas-system | grep -v global-alb2 | grep port-info | awk '{print $1}')
)

function backup_user_rule() (
  echo "backup user created rule"
  while read -r rule; do
    echo "backup user created rule $rule"
    kubectl get rule -n cpaas-system $rule -o yaml >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get rule -n cpaas-system -o=custom-columns='NAME:.metadata.name,SOURCE_TYPE:.spec.source.type' | grep -v ingress | tail -n +2 | awk '{print $1}')
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
    kubectl get secret -n $ns $secret -o yaml >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get rule -n cpaas-system -o=custom-columns='NAME:.metadata.name,SOURCE_TYPE:.spec.certificate_name' | tail -n+2 | awk '{print $2}' | xargs | sort | uniq)
}

function backup_ingress() (
  echo "backup user created ingress"
  while read -r ns name; do
    echo "ing $ns | $name"
    kubectl get ingress -n $ns $name -o yaml >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get ingress -A | grep -v 'global-alb2' | tail -n +2 | awk '{print $1,$2}')

)

function backup_alb() (
  echo "backup user created alb"
  while read -r name; do
    echo "alb $name"
    kubectl get alb2.v2beta1.crd.alauda.io -n cpaas-system $name -o yaml >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get alb2.v2beta1.crd.alauda.io -n cpaas-system | tail -n +2 | awk '{print $1}' | grep -v global-alb2)
)

function backup_ft() (
  echo "backup user created ft"
  while read -r name; do
    echo "ft $name"
    kubectl get frontends.crd.alauda.io -n cpaas-system $name -o yaml >>./alb.backup.yaml
    echo "---" >>./alb.backup.yaml
  done < <(kubectl get frontends.crd.alauda.io -n cpaas-system | tail -n +2 | awk '{print $1}' | grep -v global-alb2)
)

backup_alb
backup_ft
backup_port_info
backup_user_rule
backup_cert
backup_ingress
