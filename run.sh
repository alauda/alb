#!/usr/bin/env sh

sync

mkdir -p /var/run/alb/last_status
chmod +x /alb/alb

LB_TYPE=${LB_TYPE:-"nginx"}

if [ "$LB_TYPE" = "nginx" ]; then
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
fi


if [ "$LB_TYPE" = "nginx" ]; then
    dhparam_file="/etc/ssl/dhparam.pem"
    if [ ! -f "$dhparam_file" ]; then
        openssl dhparam -dsaparam -out ${dhparam_file} 2048
    fi
fi

/alb/alb -log_dir=/var/log/mathilde/ -stderrthreshold=ERROR $ARGS
