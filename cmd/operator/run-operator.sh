#!/bin/bash

if [[ -z "$TAIL_MODE" ]]; then
  tail -f /dev/null
fi

exec /alb/ctl/operator
