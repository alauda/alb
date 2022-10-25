#!/usr/bin/env sh
alb_init_sys() {
  # each connection will consume one entry of nf conntrack table if iptables enabled.
  # If set too small will see nf_conntrack: table full, dropping packet in dmesg.
  sysctl -w net.nf_conntrack_max=655360
  sysctl -w net.netfilter.nf_conntrack_max=655360
  sysctl -w net.netfilter.nf_conntrack_buckets=163840

  # accelerate nf conntrack entry release. To find nf conntrack status:
  # cat /proc/net/nf_conntrack | awk '/^.*tcp.*$/ {sum[$6]++} END {for(status in sum) print status, sum[status]}'
  sysctl -w net.netfilter.nf_conntrack_tcp_timeout_established=180
  sysctl -w net.netfilter.nf_conntrack_tcp_timeout_time_wait=30

  # defeat SYN flood attacks
  sysctl -w net.ipv4.tcp_syncookies=1

  # useful if there are ICMP blackholes between you and your clients. https://blog.cloudflare.com/ip-fragmentation-is-broken/
  sysctl -w net.ipv4.tcp_mtu_probing=1

  # improve keep-alive performance by not slow start tcp windows
  sysctl -w net.ipv4.tcp_slow_start_after_idle=0

  # disable coredump
  ulimit -c 0
}

if [ -n "$TAIL_MODE" ]; then
  echo "tail mode wait forever"
  tail -f /dev/null
fi


_term() {
  echo "Caught SIGTERM signal! term $child"
  kill -TERM "$child" 2>/dev/null
  echo "notify term to alb $child over"
  local max_term_seconds="$MAX_TERM_SECONDS"
  if [ -z "$max_term_seconds" ]; then
    max_term_seconds="30"
  fi
  echo "wait $max_term_seconds"
  sleep $max_term_seconds
  echo "wait over"
}

trap _term SIGTERM

umask 027
mkdir -p /alb/last_status
/alb/ctl/alb &
child=$!
echo "child is $child"
wait "$child"
echo "child $child exitd"
exit 1
