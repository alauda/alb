package ing_utils

import (
	"context"
	"time"

	"alauda.io/alb2/config"
	ct "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	ing "alauda.io/alb2/ingress"
	. "alauda.io/alb2/pkg/utils/test_utils"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
)

func SyncIngressAndGetPolicyFromK8s(rest *rest.Config, ns, name string, log logr.Logger) (*ct.NgxPolicy, error) {
	ctx := context.Background()
	cfg := config.Mock(name, ns)
	drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: rest, Opt: driver.Cfg2opt(cfg)})
	if err != nil {
		return nil, err
	}
	ing_ctl := ing.NewController(drv, drv.Informers, cfg, log)
	ing_ctl.SyncAll()
	time.Sleep(1 * time.Second)
	go ing_ctl.RunWorker()
	for i := 0; i < 10; i++ {
		if ing_ctl.GetWorkqueueLen() == 0 {
			break
		}
		time.Sleep(1 * time.Second) // wait rule sync
	}
	time.Sleep(3 * time.Second) // wait rule sync
	policy, err := GetPolicy(PolicyGetCtx{
		Ctx: ctx, Name: name, Ns: ns, Drv: drv, L: log,
		Cfg: cfg,
	})
	if err != nil {
		return nil, err
	}
	return policy, nil
}
