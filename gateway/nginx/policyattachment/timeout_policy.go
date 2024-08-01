package policyattachment

import (
	"context"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/gateway/nginx/policyattachment/types"
	gatewayPolicy "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"alauda.io/alb2/utils/log"
)

type TimeoutPolicyConfig gatewayPolicy.TimeoutPolicyConfig

type TimeoutPolicyWrapper gatewayPolicy.TimeoutPolicy

func (t TimeoutPolicyWrapper) GetDefault() PolicyAttachmentConfig {
	if t.Spec.Default == nil {
		return nil
	}
	cfg := TimeoutPolicyConfig(*t.Spec.Default)
	return cfg.IntoConfig()
}

func (t TimeoutPolicyWrapper) GetOverride() PolicyAttachmentConfig {
	if t.Spec.Override == nil {
		return nil
	}
	cfg := TimeoutPolicyConfig(*t.Spec.Override)
	return cfg.IntoConfig()
}

func (t TimeoutPolicyWrapper) GetTargetRef() gatewayPolicy.PolicyTargetReference {
	return t.Spec.TargetRef
}

func (t TimeoutPolicyWrapper) GetObject() client.Object {
	tp := gatewayPolicy.TimeoutPolicy(t)
	return &tp
}

func (tc *TimeoutPolicyConfig) IntoConfig() PolicyAttachmentConfig {
	ret := PolicyAttachmentConfig{}
	if tc.ProxyConnectTimeoutMs != nil {
		ret["proxy_connect_timeout_ms"] = *tc.ProxyConnectTimeoutMs
	}
	if tc.ProxySendTimeoutMs != nil {
		ret["proxy_send_timeout_ms"] = *tc.ProxySendTimeoutMs
	}
	if tc.ProxyReadTimeoutMs != nil {
		ret["proxy_read_timeout_ms"] = *tc.ProxyReadTimeoutMs
	}
	return ret
}

func (tc *TimeoutPolicyConfig) FromConfig(m PolicyAttachmentConfig) error {
	connect := m["proxy_connect_timeout_ms"]
	read := m["proxy_read_timeout_ms"]
	send := m["proxy_send_timeout_ms"]
	if connect != nil {
		connect := connect.(uint)
		log.L().Info("onrule connect not nil", "c", connect)
		tc.ProxyConnectTimeoutMs = &connect
	}
	if read != nil {
		read := read.(uint)
		tc.ProxyReadTimeoutMs = &read
	}
	if send != nil {
		send := send.(uint)
		tc.ProxySendTimeoutMs = &send
	}
	return nil
}

type TimeoutPolicy struct {
	ctx      context.Context
	log      logr.Logger
	drv      *driver.KubernetesDriver
	allPoliy []CommonPolicyAttachment
}

func NewTimeoutPolicy(ctx context.Context, log logr.Logger, drv *driver.KubernetesDriver) (*TimeoutPolicy, error) {
	allPolicy, err := getAllTimeoutPolicy(drv)
	if err != nil {
		return nil, err
	}
	return &TimeoutPolicy{
		ctx:      ctx,
		log:      log,
		drv:      drv,
		allPoliy: allPolicy,
	}, nil
}

func (t *TimeoutPolicy) OnRule(ft *Frontend, rule *Rule, ref Ref) error {
	log := t.log.V(3).WithName("onrule").WithValues("ref", ref.Describe())
	log.V(5).Info("len of all timeout policy", "len", len(t.allPoliy))
	config := getConfig(ref, t.allPoliy, PolicyAttachmentFilterConfig{AllowRouteKind: ALLRouteKind}, t.log.WithName("merge-attach"))
	if config == nil {
		t.log.V(3).Info("could not find timeoutconfig ignore")
		return nil
	}

	timeout := TimeoutPolicyConfig{}
	err := timeout.FromConfig(config)
	if err != nil {
		return err
	}
	log.V(5).Info("timeout cfg ", "cfg", timeout)

	if rule.Config == nil {
		rule.Config = &RuleConfigInPolicy{}
	}
	timeoutCfg := gatewayPolicy.TimeoutPolicyConfig(timeout)
	rule.Config.Timeout = &timeoutCfg
	return nil
}

func getAllTimeoutPolicy(drv *driver.KubernetesDriver) ([]CommonPolicyAttachment, error) {
	lister := drv.Informers.Alb.TimeoutPolicy.Lister()
	timeoutpolicies, err := lister.List(labels.Everything())
	ret := []CommonPolicyAttachment{}
	for _, p := range timeoutpolicies {
		ret = append(ret, TimeoutPolicyWrapper(*p))
	}
	return ret, err
}
