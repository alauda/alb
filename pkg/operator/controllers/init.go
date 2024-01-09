package controllers

import (
	"context"
	"fmt"
	"reflect"

	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	. "alauda.io/alb2/pkg/operator/controllers/types"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func StandAloneGatewayClassInit(ctx context.Context, cfg config.OperatorCfg, cli crcli.Client, log logr.Logger) error {
	// 创建默认的独享型的gatewayclass
	labels := MergeMap(OperatorLabel(cfg.Version), map[string]string{
		fmt.Sprintf("gatewayclass.%s/deploy", cfg.BaseDomain): cfg.BaseDomain,
		fmt.Sprintf("gatewayclass.%s/type", cfg.BaseDomain):   "standalone",
	})
	gclass := &gv1.GatewayClass{}
	name := STAND_ALONE_GATEWAY_CLASS
	ctlName := gv1.GatewayController(fmt.Sprintf(FMT_STAND_ALONE_GATEWAY_CLASS_CTL_NAME, cfg.BaseDomain))
	err := cli.Get(ctx, crcli.ObjectKey{Name: name}, gclass)
	log = log.WithName("shared-gclass")
	if k8serrors.IsNotFound(err) {
		log.Info("not found create it")
		// do create
		return cli.Create(ctx, &gv1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Spec: gv1.GatewayClassSpec{
				ControllerName: ctlName,
			},
		})
	}
	if err != nil {
		return err
	}
	log.Info("already exist", "version", gclass.ResourceVersion)
	// upddate
	if gclass.Labels == nil {
		gclass.Labels = map[string]string{}
	}
	origin := gclass.DeepCopy()
	gclass.Labels = MergeMap(gclass.Labels, labels)
	gclass.Spec = gv1.GatewayClassSpec{
		ControllerName: ctlName,
	}
	if reflect.DeepEqual(origin.Labels, gclass.Labels) && reflect.DeepEqual(origin.Spec, gclass.Spec) {
		log.Info("all same ignore")
		return nil
	}
	log.Info("find diff update it", "diff", cmp.Diff(origin, gclass))
	return cli.Update(ctx, gclass)
}
