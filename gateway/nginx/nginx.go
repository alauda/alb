package nginx

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/utils/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctltype "alauda.io/alb2/controller/types"
	"alauda.io/alb2/gateway"
	httproute "alauda.io/alb2/gateway/nginx/http"
	pm "alauda.io/alb2/gateway/nginx/policyattachment"
	"alauda.io/alb2/gateway/nginx/types"
)

func filterMetricsListener(lss []*types.Listener, cfg *config.Config) []*types.Listener {
	metricsPort := sets.NewInt(cfg.GetMetricsPort(), 1936, 11782)
	ret := []*types.Listener{}
	for _, l := range lss {
		if metricsPort.Has(int(l.Port)) {
			continue
		}
		ret = append(ret, l)
	}
	return ret
}

func GetLBConfig(ctx context.Context, drv *driver.KubernetesDriver, cfg *config.Config) (*ctltype.LoadBalancer, error) {
	log := log.L().WithName(gateway.ALB_GATEWAY_NGINX)
	ret := &ctltype.LoadBalancer{}
	ret.Frontends = []*ctltype.Frontend{}
	ret.Name = cfg.GetAlbName()
	d := NewDriver(drv, log)
	gcfg := cfg.GetGatewayCfg()
	log.Info("get lb config start", "cfg", gcfg)
	ftMap := map[string]*ctltype.Frontend{}
	lss, err := d.ListListener(gcfg.GatewaySelector)
	if err != nil {
		return nil, err
	}
	lss = filterMetricsListener(lss, cfg)
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

	fts := []*ctltype.Frontend{}
	for _, ft := range ftMap {
		fts = append(fts, ft)
	}

	if len(fts) == 0 {
		log.Info("empty fts,could not generate valid nginx config from gateway cr.")
		return ret, nil
	}
	ret.Frontends = fts
	ret.Labels = map[string]string{}
	ret.Annotations = map[string]string{}
	ret.Refs = ctltype.RefMap{
		ConfigMap: map[client.ObjectKey]*corev1.ConfigMap{},
		Secret:    map[client.ObjectKey]*corev1.Secret{},
	}
	return ret, nil
}

func showListenerList(lss []*types.Listener) string {
	ret := map[string][]string{}
	for _, ls := range lss {
		lsName := fmt.Sprintf("%s/%s/%v/%v/%d", ls.Gateway, ls.Name, ls.Port, ls.Protocol, len(ls.Routes))
		rList := []string{}
		for _, r := range ls.Routes {
			key := gateway.GetObjectKey(r)
			kind := r.GetObject().GetObjectKind().GroupVersionKind().Kind
			rList = append(rList, fmt.Sprintf("%s/%s", key, kind))
		}
		ret[lsName] = rList
	}
	return fmt.Sprintf("%v", ret)
}
