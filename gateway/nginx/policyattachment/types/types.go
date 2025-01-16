package policyattachment

import (
	"fmt"
	"time"

	. "alauda.io/alb2/controller/types"
	gateway "alauda.io/alb2/gateway"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1"

	gatewayPolicy "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
)

// represent the path of a rule
type Ref struct {
	Listener   *Listener
	Route      gateway.CommonRoute
	RuleIndex  int
	MatchIndex int
}

func (r *Ref) Describe() string {
	return fmt.Sprintf("ls: name %v host %v protocol %v port %v gateway: name %v ns %v route: kind %v name %v ns %v rule: index %v ",
		r.Listener.Name,
		r.Listener.Hostname,
		r.Listener.Protocol,
		r.Listener.Port,
		r.Listener.Gateway.Name,
		r.Listener.Gateway.Namespace,
		r.Route.GetObject().GetObjectKind(),
		r.Route.GetObject().GetName(),
		r.Route.GetObject().GetNamespace(),
		r.RuleIndex,
	)
}

type Listener struct {
	gatewayType.Listener
	Gateway    client.ObjectKey
	Generation int64
	CreateTime time.Time
}

type PolicyAttachmentConfig map[string]interface{}

// a policy attachment is sth that you could get default/override from it
type CommonPolicyAttachment interface {
	GetDefault() PolicyAttachmentConfig
	GetOverride() PolicyAttachmentConfig
	GetTargetRef() gatewayPolicy.PolicyTargetReference
	GetObject() client.Object
}

// a policy attachment config is sth that you could get/restore from config.
type IPolicyAttachmentConfig interface {
	IntoConfig() PolicyAttachmentConfig
	FromConfig(c PolicyAttachmentConfig)
}

type PolicyAttachmentHandle interface {
	OnRule(ft *Frontend, rule *InternalRule, ref Ref) error
}
