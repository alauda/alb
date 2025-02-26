package timeout

import (
	"fmt"
	"strconv"
	"strings"

	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/controller/ext/timeout/types"
	et "alauda.io/alb2/pkg/controller/extctl/types"
	"github.com/go-logr/logr"
	nv1 "k8s.io/api/networking/v1"
)

const (
	maxTimeoutMs uint64 = 1<<32 - 1 // 最大超时时间（毫秒）
)

// parseTimeout 解析超时时间字符串为毫秒值
// 支持的格式：
// - 纯数字：按秒处理
// - 带ms后缀：按毫秒处理
// - 带s后缀：按秒处理
func parseTimeout(s string) (*uint, error) {
	if s == "" {
		return nil, nil
	}

	var value uint64

	if strings.HasSuffix(s, "ms") {
		s = strings.TrimSuffix(s, "ms")
		value_float, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout value %q: %v", s, err)
		}
		value = uint64(value_float)
	} else {
		s = strings.TrimSuffix(s, "s")
		value_float, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout value %q: %v", s, err)
		}
		value = uint64(value_float * 1000) // 转换为毫秒
	}

	if value > maxTimeoutMs {
		return nil, fmt.Errorf("timeout value %d ms exceeds maximum allowed value %d ms", value, maxTimeoutMs)
	}

	ret := uint(value)
	return &ret, nil
}

// TimeoutCtl 处理超时相关的配置控制
// 支持处理 L4 (TCP) 和 L7 的超时配置
type TimeoutCtl struct {
	log    logr.Logger
	domain string
}

func NewTimeoutCtl(log logr.Logger, domain string) et.ExtensionInterface {
	x := &TimeoutCtl{
		log:    log,
		domain: domain,
	}
	return et.ExtensionInterface{
		IngressAnnotationToRule: x.IngressAnnotationToRule,
		ToInternalRule:          x.ToInternalRule,
		ToPolicy:                x.ToPolicy,
		InitL4Ft:                x.InitL4Ft,
		InitL4DefaultPolicy:     x.InitL4DefaultPolicy,
	}
}

// IngressAnnotationToRule 从 Ingress 注解中解析超时配置并应用到 Rule
// 配置优先级：index.{rindex}-{pindex}.alb.ingress.{domain} > alb.ingress.{domain} > nginx.ingress.kubernetes.io
func (t *TimeoutCtl) IngressAnnotationToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
	prefix := []string{fmt.Sprintf("index.%d-%d.alb.ingress.%s", rindex, pindex, t.domain), fmt.Sprintf("alb.ingress.%s", t.domain), "nginx.ingress.kubernetes.io"}
	ing_timeout := TimeoutIngress{}
	has, err := ResolverTimeoutIngressFromAnnotation(&ing_timeout, ing.Annotations, prefix)
	if err != nil {
		t.log.Error(err, "failed to resolve timeout ingress annotation", "ingress", ing.Name)
		return
	}
	if !has {
		return
	}
	cr_timeout := TimeoutCr{}
	err = ReAssignTimeoutIngressToTimeoutCr(&ing_timeout, &cr_timeout, &ReAssignTimeoutIngressToTimeoutCrOpt{
		Time_from_string: parseTimeout,
	})
	if err != nil {
		t.log.Error(err, "failed to resolve timeout cr", "ingress", ing.Name)
		return
	}
	rule.Spec.Config.Timeout = &cr_timeout
}

// ToInternalRule 将规则配置转换为内部规则
// 配置优先级：Rule Config > Frontend Config > ALB Config
func (t *TimeoutCtl) ToInternalRule(rule *m.Rule, ir *ct.InternalRule) {
	if rule.GetConfig() != nil && rule.GetConfig().Timeout != nil {
		ir.Config.Timeout = rule.GetConfig().Timeout
		ir.Config.Source[ct.Timeout] = rule.Name
		return
	}
	if rule.GetFtConfig() != nil && rule.GetFtConfig().Timeout != nil {
		ir.Config.Timeout = rule.GetFtConfig().Timeout
		ir.Config.Source[ct.Timeout] = rule.FT.Name
		return
	}
	if rule.GetAlbConfig() != nil && rule.GetAlbConfig().Timeout != nil {
		ir.Config.Timeout = rule.GetAlbConfig().Timeout
		ir.Config.Source[ct.Timeout] = rule.FT.LB.Alb.Name
		return
	}
}

// ToPolicy 将内部规则转换为策略配置
func (t *TimeoutCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	p.Config.Timeout = ir.Config.Timeout
}

// timeout 不止可以配置在7层还可以配置在4层
// InitL4Ft 初始化 L4 (TCP) 前端的超时配置
// 配置优先级：Frontend Config > ALB Config
func (t *TimeoutCtl) InitL4Ft(mft *m.Frontend, cft *ct.Frontend) {
	// 只能配置在tcp上
	if cft.Protocol != albv1.FtProtocolTCP {
		return
	}
	if mft.GetFtConfig() != nil && mft.GetFtConfig().Timeout != nil {
		cft.Config.Timeout = mft.GetFtConfig().Timeout
	}
	if mft.GetAlbConfig() != nil && mft.GetAlbConfig().Timeout != nil {
		cft.Config.Timeout = mft.GetAlbConfig().Timeout
	}
}

// InitL4DefaultPolicy 初始化 L4 默认策略的超时配置
func (t *TimeoutCtl) InitL4DefaultPolicy(cft *ct.Frontend, policy *ct.Policy) {
	if cft.Config.Timeout == nil {
		return
	}
	policy.Config.Timeout = cft.Config.Timeout
}
