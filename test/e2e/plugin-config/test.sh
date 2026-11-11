#!/bin/bash

set -uo pipefail

cd "$(dirname "$0")"

PLUGIN_CONFIG="../../../deploy/chart/alb/scripts/plugin-config.yaml"
TESTS_PASSED=0
TESTS_FAILED=0

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

pass_test() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

fail_test() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

# 辅助函数：渲染模板
render_template() {
    local values_file="$1"
    if [[ -f "/tmp/render-plugin-config" ]]; then
        /tmp/render-plugin-config "$PLUGIN_CONFIG" "$values_file"
    else
        go run render.go "$PLUGIN_CONFIG" "$values_file"
    fi
}

# 辅助函数：检查 YAML 字段值
check_yaml_value() {
    local yaml_content="$1"
    local path="$2"
    local expected="$3"
    local test_name="$4"

    local actual=$(echo "$yaml_content" | yq eval "$path" -)

    if [[ "$actual" == "$expected" ]]; then
        pass_test "$test_name: $path = $expected"
    else
        fail_test "$test_name: $path expected '$expected', got '$actual'"
    fi
}

# 测试 1: Global 集群 (Baremetal, ImportedGlobal)
test_global_cluster_baremetal() {
    log_info "Test 1: Global Cluster (Baremetal, ImportedGlobal)"

    # 创建测试 values 文件
    cat >/tmp/test-global-baremetal.yaml <<EOF
ClusterName: global
ClusterType: Baremetal
SystemNamespace: cpaas-system
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2
  - node3

VIP: 192.168.1.100
Alb2Port: 443

Base:
  DeployType: ImportedGlobal
  HTTP:
    HTTPPort: 80
    HTTPSPort: 443
    ForceRedirectHTTPS: true
  Ingress:
    Controller:
      Nodes:
        - node1
        - node2
        - node3

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-bind-nic: eth0
      cpaas.io/alb-enable-ipv6: "false"
EOF

    # 渲染模板
    local result=$(render_template /tmp/test-global-baremetal.yaml)

    # 验证结果
    check_yaml_value "$result" ".replicas" "3" "test_global_cluster_baremetal"
    check_yaml_value "$result" ".loadbalancerName" "global-alb2" "test_global_cluster_baremetal"
    check_yaml_value "$result" ".defaultSSLCert" "cpaas-system/dex.tls" "test_global_cluster_baremetal"
    check_yaml_value "$result" ".defaultSSLStrategy" "Always" "test_global_cluster_baremetal"
    check_yaml_value "$result" ".ingressHTTPPort" "0" "test_global_cluster_baremetal"
    check_yaml_value "$result" ".ingressHTTPSPort" "0" "test_global_cluster_baremetal"
    check_yaml_value "$result" '.nodeSelector."node-role.kubernetes.io/control-plane"' '' "test_global_cluster_baremetal"
    check_yaml_value "$result" ".projects[0]" "cpaas-system" "test_global_cluster_baremetal"

    rm -f /tmp/test-global-baremetal.yaml
    echo ""
}

# 测试 2: Global 集群 (非 Baremetal)
test_global_cluster_with_ingress() {
    log_info "Test 2: Global Cluster (with Ingress nodes)"

    cat >/tmp/test-global-ingress.yaml <<EOF
ClusterName: global
ClusterType: DCS
SystemNamespace: cpaas-system
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 192.168.1.100
Alb2Port: 443

Base:
  DeployType: Standard
  HTTP:
    HTTPPort: 80
    HTTPSPort: 443
    ForceRedirectHTTPS: false
  Ingress:
    Controller:
      Nodes:
        - ingress-node1
        - ingress-node2

Cluster:
  ObjectMeta:
    Annotations: {}
EOF

    local result=$(render_template /tmp/test-global-ingress.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_global_cluster_with_ingress"
    check_yaml_value "$result" ".loadbalancerName" "global-alb2" "test_global_cluster_with_ingress"
    check_yaml_value "$result" ".ingressHTTPPort" "80" "test_global_cluster_with_ingress"
    check_yaml_value "$result" ".ingressHTTPSPort" "443" "test_global_cluster_with_ingress"
    check_yaml_value "$result" ".defaultSSLStrategy" "Both" "test_global_cluster_with_ingress"

    rm -f /tmp/test-global-ingress.yaml
    echo ""
}

# 测试 3: 普通 DCS 集群
test_normal_dcs_cluster() {
    log_info "Test 3: Normal DCS Cluster"

    cat >/tmp/test-normal-dcs.yaml <<EOF
ClusterName: test-cluster
ClusterType: DCS
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations: {}
EOF

    local result=$(render_template /tmp/test-normal-dcs.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_normal_dcs_cluster"
    check_yaml_value "$result" ".address" "10.0.0.100" "test_normal_dcs_cluster"
    check_yaml_value "$result" ".loadbalancerName" "alb-test" "test_normal_dcs_cluster"
    check_yaml_value "$result" ".defaultSSLCert" "alb-test/alb-test" "test_normal_dcs_cluster"
    check_yaml_value "$result" ".ingressHTTPSPort" "443" "test_normal_dcs_cluster"
    check_yaml_value "$result" '.nodeSelector."cpaas-system-alb"' '' "test_normal_dcs_cluster"
    check_yaml_value "$result" ".projects[0]" "alb-test" "test_normal_dcs_cluster"

    rm -f /tmp/test-normal-dcs.yaml
    echo ""
}

# 测试 4: Registered 集群（使用 VIP 配置）
test_registered_cluster() {
    log_info "Test 4: Registered Cluster (with VIP config)"

    cat >/tmp/test-registered.yaml <<EOF
ClusterName: registered-cluster
ClusterType: EKS
SystemNamespace: alb-system
ProvisionType: Registered
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1

VIP: 10.1.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations: {}
EOF

    local result=$(render_template /tmp/test-registered.yaml)

    check_yaml_value "$result" ".replicas" "1" "test_registered_cluster"
    check_yaml_value "$result" ".vip.allocateLoadBalancerNodePorts" "true" "test_registered_cluster"
    check_yaml_value "$result" ".vip.enableLbSvc" "false" "test_registered_cluster"
    check_yaml_value "$result" ".networkMode" "container" "test_registered_cluster"

    rm -f /tmp/test-registered.yaml
    echo ""
}

# 测试 5: HuaweiCloudCCE 集群（带 ELB 注解）
test_huawei_cce_cluster() {
    log_info "Test 5: HuaweiCloudCCE Cluster (with ELB annotations)"

    cat >/tmp/test-huawei-cce.yaml <<EOF
ClusterName: huawei-cluster
ClusterType: HuaweiCloudCCE
SystemNamespace: alb-huawei
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.2.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations: {}
EOF

    local result=$(render_template /tmp/test-huawei-cce.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_huawei_cce_cluster"
    check_yaml_value "$result" ".vip.enableLbSvc" "true" "test_huawei_cce_cluster"
    check_yaml_value "$result" '.vip.lbSvcAnnotations."kubernetes.io/elb.class"' "union" "test_huawei_cce_cluster"
    check_yaml_value "$result" '.vip.lbSvcAnnotations."kubernetes.io/elb.lb-algorithm"' "ROUND_ROBIN" "test_huawei_cce_cluster"
    check_yaml_value "$result" ".vip.lbSvcIpFamilyPolicy" "SingleStack" "test_huawei_cce_cluster"

    rm -f /tmp/test-huawei-cce.yaml
    echo ""
}

# 测试 6: 带 IPv6 和 bindNIC 注解的集群
test_cluster_with_annotations() {
    log_info "Test 6: Cluster with IPv6 and bindNIC annotations"

    cat >/tmp/test-annotations.yaml <<EOF
ClusterName: annotated-cluster
ClusterType: Baremetal
SystemNamespace: alb-annotated
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1

VIP: 10.3.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-bind-nic: eth1
      cpaas.io/alb-enable-ipv6: "true"
EOF

    local result=$(render_template /tmp/test-annotations.yaml)

    check_yaml_value "$result" ".bindNIC" "eth1" "test_cluster_with_annotations"
    check_yaml_value "$result" ".enableIPV6" "true" "test_cluster_with_annotations"

    rm -f /tmp/test-annotations.yaml
    echo ""
}

# 测试 7: ClusterAddress 禁用 VIP
test_cluster_address_no_vip() {
    log_info "Test 7: ClusterAddress annotation disables VIP"

    cat >/tmp/test-cluster-address.yaml <<EOF
ClusterName: test-cluster
ClusterType: HuaweiCloudCCE
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-address-type: ClusterAddress
EOF

    local result=$(render_template /tmp/test-cluster-address.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_cluster_address_no_vip"
    check_yaml_value "$result" ".vip" "null" "test_cluster_address_no_vip"
    check_yaml_value "$result" ".networkMode" "null" "test_cluster_address_no_vip"

    rm -f /tmp/test-cluster-address.yaml
    echo ""
}

# 测试 8: host networkmode 禁用 VIP
test_host_networkmode_no_vip() {
    log_info "Test 8: Host networkmode annotation disables VIP"

    cat >/tmp/test-host-networkmode.yaml <<EOF
ClusterName: test-cluster
ClusterType: HuaweiCloudCCE
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-networkmode: host
EOF

    local result=$(render_template /tmp/test-host-networkmode.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_host_networkmode_no_vip"
    check_yaml_value "$result" ".vip" "null" "test_host_networkmode_no_vip"
    check_yaml_value "$result" ".networkMode" "null" "test_host_networkmode_no_vip"

    rm -f /tmp/test-host-networkmode.yaml
    echo ""
}

# 测试 9: container networkmode 启用 VIP
test_container_networkmode_enables_vip() {
    log_info "Test 9: Container networkmode annotation enables VIP"

    cat >/tmp/test-container-networkmode.yaml <<EOF
ClusterName: test-cluster
ClusterType: DCS
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-networkmode: container
EOF

    local result=$(render_template /tmp/test-container-networkmode.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_container_networkmode_enables_vip"
    check_yaml_value "$result" ".vip.enableLbSvc" "true" "test_container_networkmode_enables_vip"
    check_yaml_value "$result" ".networkMode" "container" "test_container_networkmode_enables_vip"

    rm -f /tmp/test-container-networkmode.yaml
    echo ""
}

# 测试 10: alb-need-vip=true 强制启用 VIP
test_alb_need_vip_true() {
    log_info "Test 10: alb-need-vip=true forces VIP enabled"

    cat >/tmp/test-need-vip-true.yaml <<EOF
ClusterName: test-cluster
ClusterType: DCS
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-need-vip: "true"
EOF

    local result=$(render_template /tmp/test-need-vip-true.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_alb_need_vip_true"
    check_yaml_value "$result" ".vip.enableLbSvc" "true" "test_alb_need_vip_true"
    check_yaml_value "$result" ".networkMode" "container" "test_alb_need_vip_true"

    rm -f /tmp/test-need-vip-true.yaml
    echo ""
}

# 测试 11: alb-need-vip=false 强制禁用 VIP
test_alb_need_vip_false() {
    log_info "Test 11: alb-need-vip=false forces VIP disabled"

    cat >/tmp/test-need-vip-false.yaml <<EOF
ClusterName: test-cluster
ClusterType: HuaweiCloudCCE
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-need-vip: "false"
EOF

    local result=$(render_template /tmp/test-need-vip-false.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_alb_need_vip_false"
    check_yaml_value "$result" ".vip" "null" "test_alb_need_vip_false"
    check_yaml_value "$result" ".networkMode" "null" "test_alb_need_vip_false"

    rm -f /tmp/test-need-vip-false.yaml
    echo ""
}

# 测试 12: alb-need-vip=false 覆盖 container networkmode
test_alb_need_vip_overrides_container() {
    log_info "Test 12: alb-need-vip=false overrides container networkmode"

    cat >/tmp/test-need-vip-override.yaml <<EOF
ClusterName: test-cluster
ClusterType: DCS
SystemNamespace: alb-test
ProvisionType: Managed
LabelBaseDomain: cpaas.io

Alb2Nodes:
  - node1
  - node2

VIP: 10.0.0.100
Alb2Port: 443

Cluster:
  ObjectMeta:
    Annotations:
      cpaas.io/alb-networkmode: container
      cpaas.io/alb-need-vip: "false"
EOF

    local result=$(render_template /tmp/test-need-vip-override.yaml)

    check_yaml_value "$result" ".replicas" "2" "test_alb_need_vip_overrides_container"
    check_yaml_value "$result" ".vip" "null" "test_alb_need_vip_overrides_container"
    check_yaml_value "$result" ".networkMode" "null" "test_alb_need_vip_overrides_container"

    rm -f /tmp/test-need-vip-override.yaml
    echo ""
}

# 主函数
main() {
    echo "========================================"
    echo "Plugin Config Template Tests"
    echo "========================================"
    echo ""

    # 先编译 render 程序以加快测试速度
    log_info "Building renderer..."
    go build -o /tmp/render-plugin-config render.go
    echo ""

    test_global_cluster_baremetal
    test_global_cluster_with_ingress
    test_normal_dcs_cluster
    test_registered_cluster
    test_huawei_cce_cluster
    test_cluster_with_annotations
    test_cluster_address_no_vip
    test_host_networkmode_no_vip
    test_container_networkmode_enables_vip
    test_alb_need_vip_true
    test_alb_need_vip_false
    test_alb_need_vip_overrides_container

    # 清理编译的二进制
    rm -f /tmp/render-plugin-config

    echo "========================================"
    echo "Test Summary"
    echo "========================================"
    echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
    echo ""

    if [[ $TESTS_FAILED -eq 0 ]]; then
        log_info "All tests passed! ✓"
        exit 0
    else
        log_error "Some tests failed!"
        exit 1
    fi
}

main "$@"
