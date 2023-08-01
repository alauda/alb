package gateway

import (
	"context"

	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
)

type GatewayF struct {
	*Kubectl
	*K8sClient
	*ProductNs
	*AlbWaitFileExt
	ctx context.Context
	*GatewayAssert
	*TlsExt
	*SvcExt
	AlbName string
}

func (f *GatewayF) Wait(fn func() (bool, error)) {
	Wait(fn)
}

func (f *GatewayF) GetAlbAddress() string {
	return "127.0.0.1"
}

func NewGatewayF(opt AlbEnvOpt) (*GatewayF, *Env) {
	env := NewAlbEnvWithOpt(opt)
	f := &GatewayF{
		Kubectl:        env.Kt,
		K8sClient:      env.K8sClient,
		ProductNs:      env.ProductNs,
		AlbWaitFileExt: env.AlbWaitFileExt,
		GatewayAssert: &GatewayAssert{
			Cli: env.Kc,
			Ctx: env.Ctx,
		},
		TlsExt: &TlsExt{
			Kc:  env.Kc,
			Ctx: env.Ctx,
		},
		SvcExt:  env.SvcExt,
		AlbName: opt.Name,
		ctx:     env.Ctx,
	}
	return f, env
}

func DefaultGatewayF() (*GatewayF, *Env) {
	opt := AlbEnvOpt{
		BootYaml: `
        apiVersion: crd.alauda.io/v2beta1
        kind: ALB2
        metadata:
            name: alb-dev
            namespace: cpaas-system
        spec:
            address: "127.0.0.1"
            type: "nginx"
            config:
               project: ["project1"]
               gateway:
                  mode: "shared"
`,
		Ns:       "cpaas-system",
		Name:     "alb-dev",
		StartAlb: true,
	}
	return NewGatewayF(opt)
}
