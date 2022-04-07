package types

import (
	"fmt"
	"time"

	pmType "alauda.io/alb2/gateway/nginx/policyattachment/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"

	. "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/gateway"
)

type Listener struct {
	gatewayType.Listener
	Gateway    client.ObjectKey
	Generation int64
	CreateTime time.Time
	Routes     []CommonRoute
}

type FtMap map[string]*Frontend

func (f FtMap) SetFt(protocol string, port int, ft *Frontend) {
	key := fmt.Sprintf("%v:%v", protocol, port)
	f[key] = ft
}

type GatewayAlbTranslate interface {
	TransLate(ls []*Listener, ftMap FtMap) error
}

// who implement this interface have responsibility to call OnRule when a rule been create.
type GatewayAlbPolicyAttachemt interface {
	SetPolicyAttachmentHandle(handle pmType.PolicyAttachmentHandle)
}
