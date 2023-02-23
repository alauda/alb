package resources

import (
	"fmt"
	"strings"
)

// 标识这个资源的alb
var ALB2OperatorResourceLabel = "alb.cpaas.io/alb2-operator"
var ALB2OperatorLabel = "alb.cpaas.io/managed-by"

// 表示这个资源部署时operator的版本
var ALB2OperatorVersionLabel = "alb.cpaas.io/version"

func ALB2ResourceLabel(ns, name string, version string) map[string]string {
	return map[string]string{
		ALB2OperatorResourceLabel: fmt.Sprintf("%s_%s", ns, name),
		ALB2OperatorVersionLabel:  version,
		ALB2OperatorLabel:         "alb-operator",
	}
}

func MergeLabel(a map[string]string, b map[string]string) map[string]string {
	ret := map[string]string{}
	for k, v := range a {
		ret[k] = v
	}
	for k, v := range b {
		ret[k] = v
	}
	return ret
}

// 如果key中有某个prefix，就删除这个key
func RemovePrefixKey(m map[string]string, prefix string) map[string]string {
	ret := map[string]string{}
	for k, v := range m {
		if strings.HasPrefix(k, prefix) {
			continue
		}
		ret[k] = v
	}
	return ret
}
