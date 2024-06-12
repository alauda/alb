#!/bin/bash

function alb-sync-to-checklist() {
  local target="$1"
  for p in ./migrate/checklist/custom/*; do
    md5sum $p
    cp -r $p $target/checklist/custom/
  done
  echo "====="
  for p in $target/checklist/custom/*; do
    md5sum $p
  done
}

function alb-sync-from-checklist() {
  local target="$1"
  for p in $target/checklist/custom/*; do
    md5sum $p
    cp $p ./migrate/checklist/custom/
  done
  echo "====="
  for p in ./migrate/checklist/custom/*; do
    md5sum $p
  done
}
