package standalone_gateway

import (
	"context"

	"alauda.io/alb2/pkg/operator/config"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrcli "sigs.k8s.io/controller-runtime/pkg/client"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1"
)

type CliExt struct {
	ctrcli.Client
	log logr.Logger
	cfg config.OperatorCfg
}

func (c *CliExt) GetGateway(ctx context.Context, req ctrl.Request) (*gv1b1t.Gateway, error) {
	g := &gv1b1t.Gateway{}
	err := c.Get(ctx, ctrcli.ObjectKey{Namespace: req.Namespace, Name: req.Name}, g)
	if err != nil {
		return nil, err
	}
	return g, nil
}
