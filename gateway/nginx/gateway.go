package nginx

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"alauda.io/alb2/driver"
	. "alauda.io/alb2/utils/log"

	. "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/gateway"
)

type Listener struct {
	gatewayType.Listener
	gateway    client.ObjectKey
	generation int64
	createTime time.Time
	routes     []CommonRoute
}

type FtMap map[string]*Frontend

func (f FtMap) SetFt(protocol string, port int, ft *Frontend) {
	key := fmt.Sprintf("%v:%v", protocol, port)
	f[key] = ft
}

type GatewayAlbTranslate interface {
	TransLate(ls []*Listener, ftMap FtMap) error
}

func GetLBConfig(ctx context.Context, drv *driver.KubernetesDriver, className string) (*LoadBalancer, error) {
	log := L().WithName(ALB_GATEWAY_NGINX).WithValues("class", className)
	log.Info("get lb config start")
	ftMap := map[string]*Frontend{}

	lss, err := ListListenerByClass(drv, className)
	if err != nil {
		return nil, err
	}

	log.V(2).Info("listener", "total", len(lss), "status", func() map[string][]string {
		ret := map[string][]string{}
		for _, ls := range lss {
			lsName := fmt.Sprintf("%s/%s/%v/%v", ls.gateway, ls.Name, ls.Port, ls.Protocol)
			rList := []string{}
			for _, r := range ls.routes {
				key := GetObjectKey(r)
				kind := r.GetObject().GetObjectKind().GroupVersionKind().Kind
				rList = append(rList, fmt.Sprintf("%s/%s", key, kind))
			}
			ret[lsName] = rList
		}
		return ret
	}())

	if len(lss) == 0 {
		return nil, nil
	}

	http := NewHttpProtocolTranslate(drv, log)
	err = http.TransLate(lss, ftMap)
	if err != nil {
		return nil, err
	}

	tcp := NewTcpProtocolTranslate(drv, log)
	err = tcp.TransLate(lss, ftMap)
	if err != nil {
		return nil, err
	}

	fts := []*Frontend{}
	for _, ft := range ftMap {
		fts = append(fts, ft)
	}

	if len(fts) == 0 {
		log.Info("empty fts,could not generate valid nginx config from gateway cr.")
		return nil, nil
	}

	lbConfig := &LoadBalancer{}
	lbConfig.Frontends = fts
	return lbConfig, nil
}
