package policyattachment

import (
	"fmt"

	. "alauda.io/alb2/gateway"
	. "alauda.io/alb2/gateway/nginx/policyattachment/types"
	"github.com/go-logr/logr"
)

type NamedPolicyAttachmentConfig struct {
	Describe string
	Config   PolicyAttachmentConfig
}

type OrderedPolicyAttachmentConfigList []NamedPolicyAttachmentConfig

// some option config when match policy to ref. used to filter invalid policy attachment.
type PolicyAttachmentFilterConfig struct {
	AllowRouteKind        map[string]bool
	AllowListenerProtocol map[string]bool // not implement yet.
	AllowAttachList       map[string]bool // not fully implement yet. gatewayclass/gateway/gateway-listener/route/route-rule now is gateway,route.
}

var ALLRouteKind map[string]bool = map[string]bool{
	HttpRouteKind: true,
	TcpRouteKind:  true,
	UdpRouteKind:  true,
}

func getConfigList(ref Ref, allPolicy []CommonPolicyAttachment, cfg PolicyAttachmentFilterConfig, log logr.Logger) OrderedPolicyAttachmentConfigList {
	// TODO it should/could optimize for speed,but the most important is that we need a way to know exactly how the finalized config come from.
	// TODO add more attach point here,such like gatewayclass gatewaylistener routeindex etc

	log = log.V(8).WithName("merge-config")
	var gateway CommonPolicyAttachment
	var route CommonPolicyAttachment
	{
		// get timeoutpolicy atttached to the chain of this ref
		// TODO now,we just ignore overlaped policy.
		for i, p := range allPolicy {
			target := p.GetTargetRef()
			log.Info("find p", "target", target, "name", p.GetObject().GetName(), "ns", p.GetObject().GetNamespace())
			if target.Kind == GatewayKind && target.Name == ref.Listener.Gateway.Name && target.Namespace == ref.Listener.Gateway.Namespace {
				if target.SectionIndex == nil && target.SectionName == nil {
					log.Info("find one attch the gateway")
					gateway = allPolicy[i]
					continue
				}
				// TODO allow listeners protocol filter
			}
			// allow route kind filter
			if _, ok := cfg.AllowRouteKind[target.Kind]; ok {
				routeObj := ref.Route.GetObject()
				if target.Name == routeObj.GetName() && target.Namespace == routeObj.GetNamespace() {
					if target.SectionIndex == nil && target.SectionName == nil {
						log.Info("find one attch the route")
						route = allPolicy[i]
						continue
					}
				}
			}
		}
	}

	defaultCfg := []NamedPolicyAttachmentConfig{}
	overideCfg := []NamedPolicyAttachmentConfig{}
	{
		add := func(t CommonPolicyAttachment, name string) {
			if t != nil && t.GetDefault() != nil {
				defaultCfg = append(defaultCfg, NamedPolicyAttachmentConfig{
					Describe: fmt.Sprintf("%s-default", name),
					Config:   t.GetDefault(),
				})
			}
			if t != nil && t.GetOverride() != nil {
				overideCfg = append(overideCfg, NamedPolicyAttachmentConfig{
					Describe: fmt.Sprintf("%s-overide", name),
					Config:   t.GetOverride(),
				})
			}
		}
		if gateway != nil {
			add(gateway, fmt.Sprintf("gateway-%s-%s", gateway.GetObject().GetName(), gateway.GetObject().GetNamespace()))
		}
		if route != nil {
			add(route, fmt.Sprintf("route-%s-%s", route.GetObject().GetName(), route.GetObject().GetNamespace()))
		}
	}

	list := OrderedPolicyAttachmentConfigList{}
	for i := 0; i < len(defaultCfg); i++ {
		list = append(list, defaultCfg[i])
	}
	// reverse order
	for i := len(overideCfg) - 1; i >= 0; i-- {
		list = append(list, overideCfg[i])
	}

	return list
}

// given a ref and list of policy, getConfig will find policy attach to the ref,and merge all those policy into a config.
// return nil if no such config
func getConfig(ref Ref, allPolicy []CommonPolicyAttachment, cfg PolicyAttachmentFilterConfig, log logr.Logger) PolicyAttachmentConfig {
	list := getConfigList(ref, allPolicy, cfg, log)
	log = log.V(8).WithName("merge-config")
	if len(list) == 0 {
		return nil
	}
	// merge
	ret := PolicyAttachmentConfig{}
	for _, pc := range list {
		for key, val := range pc.Config {
			log.Info("merge", "name", pc.Describe, "key", key, "val", fmt.Sprintf("%v", val))
			ret[key] = val
		}
	}
	return ret
}
