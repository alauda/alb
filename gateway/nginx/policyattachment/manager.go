package policyattachment

import (
	"context"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/gateway/nginx/policyattachment/types"
	"github.com/go-logr/logr"
)

type PolicyAttachmentManager struct {
	ctx     context.Context
	log     logr.Logger
	drv     *driver.KubernetesDriver
	timeout TimeoutPolicy
}

// manager of all policyattachment, recreate when re-render config.
func NewPolicyAttachmentManager(ctx context.Context, drv *driver.KubernetesDriver, log logr.Logger) (*PolicyAttachmentManager, error) {
	timeout, err := NewTimeoutPolicy(ctx, log.WithName("timeout"), drv)
	if err != nil {
		return nil, err
	}
	return &PolicyAttachmentManager{
		ctx:     ctx,
		log:     log,
		drv:     drv,
		timeout: *timeout,
	}, nil
}

func (pm *PolicyAttachmentManager) OnRule(ft *Frontend, rule *InternalRule, ref Ref) error {
	// TODO 当一个policyattachment出错时,应该如何处理？
	err := pm.timeout.OnRule(ft, rule, ref)
	if err != nil {
		return err
	}
	return nil
}
