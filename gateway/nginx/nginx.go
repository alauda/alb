package nginx

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/utils/log"

	. "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/gateway"
	httproute "alauda.io/alb2/gateway/nginx/http"
	pm "alauda.io/alb2/gateway/nginx/policyattachment"
	"alauda.io/alb2/gateway/nginx/types"
)

func GetLBConfig(ctx context.Context, drv *driver.KubernetesDriver, cfg *config.Config) (*LoadBalancer, error) {
	log := L().WithName(ALB_GATEWAY_NGINX)
	ret := &LoadBalancer{}
	ret.Frontends = []*Frontend{}
	ret.Name = config.GetConfig().GetAlbName()
	d := NewDriver(drv, log)
	gcfg := cfg.GetGatewayCfg()
	log.Info("get lb config start", "cfg", gcfg)
	ftMap := map[string]*Frontend{}
	lss, err := d.ListListener(gcfg.GatewaySelector)
	if err != nil {
		return nil, err
	}
	log.Info("listener", "total", len(lss), "status", showListenerList(lss))
	if len(lss) == 0 {
		return ret, nil
	}

	pm, err := pm.NewPolicyAttachmentManager(ctx, drv, log.WithName("pm"))
	if err != nil {
		return nil, err
	}
	http := httproute.NewHttpProtocolTranslate(drv, log, cfg)
	http.SetPolicyAttachmentHandle(pm)
	err = http.TransLate(lss, ftMap)
	if err != nil {
		return nil, err
	}

	tcp := NewTcpProtocolTranslate(drv, log)
	tcp.SetPolicyAttachmentHandle(pm)
	err = tcp.TransLate(lss, ftMap)
	if err != nil {
		return nil, err
	}

	udp := NewUdpProtocolTranslate(drv, log)
	err = udp.TransLate(lss, ftMap)
	if err != nil {
		return nil, err
	}

	fts := []*Frontend{}
	for _, ft := range ftMap {
		fts = append(fts, ft)
	}

	if len(fts) == 0 {
		log.Info("empty fts,could not generate valid nginx config from gateway cr.")
		return ret, nil
	}
	ret.Frontends = fts
	return ret, nil
}

func showLb(lb *LoadBalancer) string {
	ft_len := len(lb.Frontends)
	out := fmt.Sprintf("%v ", ft_len)
	for _, ft := range lb.Frontends {
		out += fmt.Sprintf("%v %v", ft.FtName, len(ft.Rules))
	}
	return out
}
func showListenerList(lss []*types.Listener) string {
	ret := map[string][]string{}
	for _, ls := range lss {
		lsName := fmt.Sprintf("%s/%s/%v/%v/%d", ls.Gateway, ls.Name, ls.Port, ls.Protocol, len(ls.Routes))
		rList := []string{}
		for _, r := range ls.Routes {
			key := GetObjectKey(r)
			kind := r.GetObject().GetObjectKind().GroupVersionKind().Kind
			rList = append(rList, fmt.Sprintf("%s/%s", key, kind))
		}
		ret[lsName] = rList
	}
	return fmt.Sprintf("%v", ret)
}
