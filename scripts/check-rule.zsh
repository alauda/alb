#!/bin/zsh
# get all of rule's name
kubectl get rule -A -o json |jq ".items[] .spec.source.name"
