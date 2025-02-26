#!/bin/bash

function alb-ai-review() {
  # 为ai-review而优化的commit workflow
  # 不在两个commit中改同一个文件
  # commit message 作为prompt的一部分
  local commit=$1
  if [ -z "$commit" ]; then
    commit=$(git log --pretty=format:"%H %s" origin/master..HEAD | fzf | awk '{print $1}')
  fi
  local msg=$(git show -s --format=%s $commit | sed 's/\n/_/g' | sed 's/\s/_/g' | sed 's|/|_|g')
  echo "review $commit $msg"
  local full_msg=$(git show -s --format=%B $commit)
  local prompt=$(
    cat <<EOF
you are code reviewer helper assistant.
review the diff and the commit message.
if there are any problems, please point them out.
use chinese language to answer.

## commit message
$full_msg

## diff
EOF
  )
  mkdir -p ./.review
  (rm -f ./.review/$commit-* || true) >/dev/null 2>&1
  local p="./.review/$commit-$msg.review.prompt"
  echo "$prompt" >$p
  git show $commit >>$p
  echo "$p $(ls -alh $p)"
  return
}
